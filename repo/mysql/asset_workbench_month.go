package mysqlrepo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// A connection-owned named lock also covers an empty month (no batch row yet).
// Holding it through commit prevents a concurrent batch from missing a supplement.
func (r *assetWorkbenchRepo) RunSettlementMonthTx(ctx context.Context, month string, fn func(repo.Tx) error) error {
	conn, err := r.db.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var name string
	if err := conn.QueryRowContext(ctx, `SELECT CONCAT('aw-settlement:', LEFT(SHA2(DATABASE(), 256), 32), ':', ?)`, month).Scan(&name); err != nil {
		return err
	}
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 15)`, name).Scan(&acquired); err != nil {
		// A cancelled read may have acquired the server lock without returning its result.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return domain.NewAppError(domain.ErrCodeConflict, "该月正在结算或补录，请稍后重试。", nil)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		if err := conn.QueryRowContext(cleanup, `SELECT RELEASE_LOCK(?)`, name).Scan(&released); err != nil || !released.Valid || released.Int64 != 1 {
			// Never return a connection with a possibly held named lock to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	sqlTx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer sqlTx.Rollback()
	tx := &MySQLTx{tx: sqlTx}
	if err := fn(tx); err != nil {
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return err
	}
	tx.runAfterCommit()
	return nil
}

// All known business months, with a global month lock whenever any live batch exists.
// MIN avoids scanning every upload just to discover the beginning of the calendar.
func (r *assetWorkbenchRepo) ListSettlementMonthStates(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT business_month, MAX(locked) FROM (
		 SELECT MIN(business_month) AS business_month, 0 AS locked FROM asset_workbench_submissions
		 UNION ALL SELECT MIN(business_month), 0 FROM asset_workbench_settlement_supplements
		 UNION ALL SELECT MIN(business_month), 0 FROM asset_workbench_supplement_permissions
		 UNION ALL SELECT business_month, status <> 'cancelled' FROM asset_workbench_settlement_batches
		) months WHERE business_month IS NOT NULL GROUP BY business_month`)
	if err != nil {
		return nil, fmt.Errorf("read settlement month states: %w", err)
	}
	defer rows.Close()
	states := map[string]bool{}
	for rows.Next() {
		var month string
		var locked bool
		if err := rows.Scan(&month, &locked); err != nil {
			return nil, err
		}
		states[month] = locked
	}
	return states, rows.Err()
}
