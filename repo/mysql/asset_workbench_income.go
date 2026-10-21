package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

// Read all months without pagination. The snapshot prevents a concurrent batch
// generation/confirmation from moving a source between two separate reads.
func (r *assetWorkbenchRepo) ReadPersonalIncome(ctx context.Context, userID int64) (*repo.AssetWorkbenchIncomeSources, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("personal income requires a payee")
	}
	tx, err := r.db.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result := &repo.AssetWorkbenchIncomeSources{}
	err = readIncomeRows(ctx, tx, assetWorkbenchSubmissionItemSelect()+` WHERE payee_user_id = ? AND entry_kind = ? AND settlement_status = ? AND current_settlement_batch_id IS NULL AND qc_status <> ? ORDER BY id`, []any{userID, domain.AssetWorkbenchSubmissionEntryKindNormal, domain.AssetWorkbenchSettlementStatusUnsettled, domain.AssetWorkbenchSubmissionStatusVoided}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchSubmissionItem(rows)
		if err == nil {
			result.Items = append(result.Items, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readIncomeRows(ctx, tx, assetWorkbenchSettlementSupplementSelect()+` WHERE payee_user_id = ? ORDER BY business_month DESC, id DESC`, []any{userID}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchSettlementSupplement(rows)
		if err == nil {
			result.Supplements = append(result.Supplements, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readIncomeRows(ctx, tx, assetWorkbenchErrorRecordSelect()+` WHERE payee_user_id = ? ORDER BY id`, []any{userID}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchErrorRecord(rows)
		if err == nil {
			result.Errors = append(result.Errors, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	// Keep the standard settlement scanner and append the joined status explicitly.
	query := `SELECT si.id, si.batch_id, si.item_type, si.submission_item_id, si.payee_user_id, si.paid_to_user_id, si.business_month,
	 si.amount, si.quantity, si.unit_price, si.direction, si.source_ref_type, si.source_ref_id, si.snapshot_json, si.payout_snapshot_json, si.created_at, sb.status
	 FROM asset_workbench_settlement_items si JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
	 WHERE si.payee_user_id = ? AND sb.status IN (?, ?) ORDER BY si.id`
	err = readIncomeRows(ctx, tx, query, []any{userID, domain.AssetWorkbenchBatchStatusGenerated, domain.AssetWorkbenchBatchStatusConfirmed}, func(rows *sql.Rows) error {
		var status string
		item, err := scanAssetWorkbenchSettlementItem(incomeStatusScanner{rows, &status})
		if err == nil {
			result.Ledger = append(result.Ledger, repo.AssetWorkbenchIncomeLedgerRow{Item: item, BatchStatus: status})
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

type incomeStatusScanner struct {
	rows   *sql.Rows
	status *string
}

func (r *assetWorkbenchRepo) SetSettlementVisibility(ctx context.Context, tx repo.Tx, userID int64, visible bool, actorID int64) error {
	// Owner lock distinguishes an absent profile from an unchanged toggle.
	var id int64
	if err := Unwrap(tx).QueryRowContext(ctx, `SELECT id FROM asset_workbench_profiles WHERE user_id = ? FOR UPDATE`, userID).Scan(&id); err != nil {
		return err
	}
	_, err := Unwrap(tx).ExecContext(ctx, `UPDATE asset_workbench_profiles SET settlement_visible = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`, visible, actorID, userID)
	return err
}

func (r *assetWorkbenchRepo) ListReservedSettlementSources(ctx context.Context, tx repo.Tx, month string) ([]*domain.AssetWorkbenchSettlementItem, error) {
	var executor interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	} = r.db.db
	if tx != nil {
		executor = Unwrap(tx)
	}
	rows, err := executor.QueryContext(ctx, assetWorkbenchSettlementItemSelect()+` WHERE business_month = ? AND item_type IN (?, ?) AND batch_id IN (SELECT id FROM asset_workbench_settlement_batches WHERE status IN (?, ?)) ORDER BY id`, month, domain.AssetWorkbenchItemTypeErrorDeduction, domain.AssetWorkbenchItemTypeWelfare, domain.AssetWorkbenchBatchStatusGenerated, domain.AssetWorkbenchBatchStatusConfirmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSettlementItem{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s incomeStatusScanner) Scan(dest ...interface{}) error {
	return s.rows.Scan(append(dest, s.status)...)
}

func readIncomeRows(ctx context.Context, tx *sql.Tx, query string, args []any, scan func(*sql.Rows) error) error {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
