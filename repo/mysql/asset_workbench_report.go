package mysqlrepo

import (
	"context"
	"database/sql"

	"workflow/domain"
	"workflow/repo"
)

func (r *assetWorkbenchRepo) ReadSettlementReportSources(ctx context.Context, month string) (*repo.AssetWorkbenchReportSources, error) {
	tx, err := r.db.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	sources := &repo.AssetWorkbenchReportSources{}
	err = readIncomeRows(ctx, tx, assetWorkbenchSubmissionItemSelect()+`
		WHERE business_month = ? AND entry_kind = ? AND qc_status <> ?
		  AND ((pricing_status = ? AND qc_status IN (?, ?) AND settlement_status = ? AND current_settlement_batch_id IS NULL)
		       OR current_settlement_batch_id IN (SELECT id FROM asset_workbench_settlement_batches WHERE status IN (?, ?)))
		ORDER BY id`, []any{
		month, domain.AssetWorkbenchSubmissionEntryKindNormal, domain.AssetWorkbenchSubmissionStatusVoided,
		domain.AssetWorkbenchPricingStatusPriced, domain.AssetWorkbenchSubmissionStatusSubmitted, domain.AssetWorkbenchSubmissionStatusChecked, domain.AssetWorkbenchSettlementStatusUnsettled,
		domain.AssetWorkbenchBatchStatusGenerated, domain.AssetWorkbenchBatchStatusConfirmed,
	}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchSubmissionItem(rows)
		if err == nil {
			sources.Items = append(sources.Items, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readIncomeRows(ctx, tx, assetWorkbenchSettlementSupplementSelect()+`
		WHERE business_month = ? AND status IN (?, ?, ?)
		ORDER BY id`, []any{month, domain.AssetWorkbenchSupplementStatusApproved, domain.AssetWorkbenchSupplementStatusInBatch, domain.AssetWorkbenchSupplementStatusSettled}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchSettlementSupplement(rows)
		if err == nil {
			sources.Supplements = append(sources.Supplements, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readIncomeRows(ctx, tx, assetWorkbenchErrorRecordSelect()+` WHERE business_month = ? ORDER BY id`, []any{month}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchErrorRecord(rows)
		if err == nil {
			sources.Errors = append(sources.Errors, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readIncomeRows(ctx, tx, assetWorkbenchSettlementItemSelect()+`
		WHERE business_month = ? AND batch_id IN (SELECT id FROM asset_workbench_settlement_batches WHERE status IN (?, ?))
		ORDER BY id`, []any{month, domain.AssetWorkbenchBatchStatusGenerated, domain.AssetWorkbenchBatchStatusConfirmed}, func(rows *sql.Rows) error {
		item, err := scanAssetWorkbenchSettlementItem(rows)
		if err == nil {
			sources.Ledger = append(sources.Ledger, item)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	sources.ActualDeductions, err = readQualityActualDeductions(ctx, tx, "business_month = ?", month)
	if err != nil {
		return nil, err
	}
	return sources, tx.Commit()
}
