package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type erpSyncRunRepo struct{ db *DB }

func NewERPSyncRunRepo(db *DB) repo.ERPSyncRunRepo { return &erpSyncRunRepo{db: db} }

func (r *erpSyncRunRepo) Create(ctx context.Context, tx repo.Tx, run *domain.ERPSyncRun) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO erp_sync_runs (
			trigger_mode, source_mode, status, total_received, total_upserted,
			error_message, started_at, finished_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		run.TriggerMode,
		run.SourceMode,
		run.Status,
		run.TotalReceived,
		run.TotalUpserted,
		toNullString(run.ErrorMessage),
		run.StartedAt,
		run.FinishedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert erp_sync_run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("erp_sync_run last insert id: %w", err)
	}
	return id, nil
}

func (r *erpSyncRunRepo) GetLatest(ctx context.Context) (*domain.ERPSyncRun, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, trigger_mode, source_mode, status, total_received, total_upserted,
		       error_message, started_at, finished_at, created_at
		FROM erp_sync_runs
		ORDER BY id DESC
		LIMIT 1`)
	return scanERPSyncRun(row)
}

func scanERPSyncRun(row *sql.Row) (*domain.ERPSyncRun, error) {
	var run domain.ERPSyncRun
	var errorMessage sql.NullString
	err := row.Scan(
		&run.ID,
		&run.TriggerMode,
		&run.SourceMode,
		&run.Status,
		&run.TotalReceived,
		&run.TotalUpserted,
		&errorMessage,
		&run.StartedAt,
		&run.FinishedAt,
		&run.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan erp_sync_run: %w", err)
	}
	run.ErrorMessage = fromNullString(errorMessage)
	return &run, nil
}
