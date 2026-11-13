package mysqlrepo

import (
	"context"
	"database/sql"

	"workflow/domain"
	"workflow/repo"
)

func (r *assetWorkbenchRepo) ListQualityActualDeductions(ctx context.Context, tx repo.Tx, month string) ([]*domain.AssetWorkbenchSettlementAdjustment, error) {
	if tx != nil {
		return readQualityActualDeductions(ctx, Unwrap(tx), "business_month = ?", month)
	}
	return readQualityActualDeductions(ctx, r.db.db, "business_month = ?", month)
}

func readQualityActualDeductions(ctx context.Context, executor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, filter string, value any) ([]*domain.AssetWorkbenchSettlementAdjustment, error) {
	rows, err := executor.QueryContext(ctx, assetWorkbenchSettlementAdjustmentSelect()+`
		WHERE `+filter+` AND batch_id IS NULL AND adjustment_type = ?
		ORDER BY id DESC`, value, domain.AssetWorkbenchAdjustmentTypeQualityActualDeduction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*domain.AssetWorkbenchSettlementAdjustment{}
	seen := make(map[struct {
		month   string
		payee   int64
		rowType string
	}]bool)
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementAdjustment(rows)
		if err != nil {
			return nil, err
		}
		key := struct {
			month   string
			payee   int64
			rowType string
		}{item.BusinessMonth, item.PayeeUserID, domain.WorkbenchPayrollRowType(item.Payload)}
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	return result, rows.Err()
}

func (r *assetWorkbenchRepo) ListQualityImportWorkReferences(ctx context.Context, month string, payeeID int64) ([]repo.AssetWorkbenchQualityWorkReference, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT i.id, i.entry_kind = ?, i.order_no,
		       COALESCE(f.original_filename, ''), COALESCE(f.display_name, ''), COALESCE(f.relative_path, '')
		FROM asset_workbench_submission_items i
		LEFT JOIN asset_workbench_submission_files f ON f.submission_item_id = i.id AND f.deleted_at IS NULL
		WHERE i.payee_user_id = ? AND i.business_month = ? AND i.qc_status <> ? AND i.voided_at IS NULL
		UNION ALL
		SELECT -s.id, 1, s.order_no, '', '', ''
		FROM asset_workbench_settlement_supplements s
		WHERE s.payee_user_id = ? AND s.business_month = ? AND s.submission_item_id IS NULL
		  AND s.status IN (?, ?, ?)`,
		domain.AssetWorkbenchSubmissionEntryKindSupplement, payeeID, month, domain.AssetWorkbenchSubmissionStatusVoided,
		payeeID, month, domain.AssetWorkbenchSupplementStatusApproved, domain.AssetWorkbenchSupplementStatusInBatch, domain.AssetWorkbenchSupplementStatusSettled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []repo.AssetWorkbenchQualityWorkReference
	for rows.Next() {
		var ref repo.AssetWorkbenchQualityWorkReference
		if err := rows.Scan(&ref.WorkID, &ref.Supplement, &ref.OrderNo, &ref.OriginalName, &ref.DisplayName, &ref.RelativePath); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
