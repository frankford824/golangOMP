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
		month string
		payee int64
	}]bool)
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementAdjustment(rows)
		if err != nil {
			return nil, err
		}
		key := struct {
			month string
			payee int64
		}{item.BusinessMonth, item.PayeeUserID}
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	return result, rows.Err()
}
