package assetworkbench

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type QualityActualDeduction struct {
	PayeeUserID                  int64   `json:"payee_user_id"`
	QualityActualDeductionAmount float64 `json:"quality_actual_deduction_amount"`
	PayrollRowType               string  `json:"payroll_row_type"`
}

type QualityActualDeductionSettings struct {
	BusinessMonth string                   `json:"business_month"`
	Locked        bool                     `json:"locked"`
	Deductions    []QualityActualDeduction `json:"deductions"`
}

func signedSettlementAmount(item *domain.AssetWorkbenchSettlementItem) float64 {
	if item.Direction == "debit" {
		return -item.Amount
	}
	return item.Amount
}

func (s *Service) QualityActualDeductions(ctx context.Context, actor domain.RequestActor, month string) (*QualityActualDeductionSettings, *domain.AppError) {
	if appErr := s.checkQualityDeductionAccess(actor, month); appErr != nil {
		return nil, appErr
	}
	states, err := s.repo.ListSettlementMonthStates(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to check settlement month state.", nil)
	}
	rows, err := s.repo.ListQualityActualDeductions(ctx, nil, month)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load quality actual deductions.", nil)
	}
	result := &QualityActualDeductionSettings{BusinessMonth: month, Locked: states[month], Deductions: []QualityActualDeduction{}}
	records, err := s.repo.ListErrorRecordsByMonth(ctx, month)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load error records.", nil)
	}
	active := qualityImportPayrollKeys(records, 0)
	for _, row := range rows {
		if active[qualityPayrollKey{row.PayeeUserID, domain.WorkbenchPayrollRowType(row.Payload)}] {
			result.Deductions = append(result.Deductions, QualityActualDeduction{row.PayeeUserID, row.Amount, domain.WorkbenchPayrollRowType(row.Payload)})
		}
	}
	return result, nil
}

func (s *Service) SaveQualityActualDeduction(ctx context.Context, actor domain.RequestActor, month string, payeeID int64, amount float64) *domain.AppError {
	return s.SaveQualityActualDeductionForPayroll(ctx, actor, month, payeeID, amount, "")
}

func (s *Service) SaveQualityActualDeductionForPayroll(ctx context.Context, actor domain.RequestActor, month string, payeeID int64, amount float64, rowType string) *domain.AppError {
	if appErr := s.checkQualityDeductionAccess(actor, month); appErr != nil {
		return appErr
	}
	rowType, valid := normalizeQualityPayrollRowType(rowType)
	if !valid {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择日常计件或补录计件。", nil)
	}
	if payeeID <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > 9999999999.99 || math.Abs(amount-incomeMoney(amount)) > 1e-8 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "填写有效人员及非负金额，金额最多保留两位小数。", nil)
	}
	amount = incomeMoney(amount)
	profile, err := s.repo.GetProfileByUserID(ctx, payeeID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && profile == nil) {
		return domain.NewAppError(domain.ErrCodeNotFound, "未找到该人员的工作台档案。", nil)
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to load payee profile.", nil)
	}
	err = s.repo.RunSettlementMonthTx(ctx, month, func(tx repo.Tx) error {
		states, err := s.repo.ListSettlementMonthStates(ctx)
		if err != nil {
			return err
		}
		if states[month] {
			return domain.NewAppError(domain.ErrCodeConflict, "该月已有生成或确认的结算批次，质检实际扣款已锁定。", nil)
		}
		rows, err := s.repo.ListQualityActualDeductions(ctx, tx, month)
		if err != nil {
			return err
		}
		var before *domain.AssetWorkbenchSettlementAdjustment
		for _, row := range rows {
			if row.PayeeUserID == payeeID && domain.WorkbenchPayrollRowType(row.Payload) == rowType {
				before = row
				break
			}
		}
		if amount > 0 {
			records, err := s.repo.ListErrorRecordsByMonth(ctx, month)
			if err != nil {
				return err
			}
			if !qualityImportPayrollKeys(records, 0)[qualityPayrollKey{payeeID, rowType}] {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "该人员在这个月的工资条没有匹配的质检记录，请先导入。", nil)
			}
		}
		if before != nil && before.Amount == amount {
			return nil
		}
		saved, err := s.repo.CreateSettlementAdjustment(ctx, tx, &domain.AssetWorkbenchSettlementAdjustment{
			PayeeUserID: payeeID, BusinessMonth: month, AdjustmentType: domain.AssetWorkbenchAdjustmentTypeQualityActualDeduction,
			Amount: amount, Reason: "手工录入质检实际扣款", Status: domain.AssetWorkbenchAdjustmentStatusApplied, CreatedBy: actor.ID,
			Payload: mustJSON(map[string]string{"payroll_row_type": rowType}),
		})
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventQualityActualDeductionSaved, domain.AssetWorkbenchEntityAdjustment,
			&saved.ID, before, saved, saved.Reason)
	})
	if appErr := asAppError(err); appErr != nil {
		return appErr
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to save quality actual deduction.", nil)
	}
	return nil
}

func normalizeQualityPayrollRowType(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "", "日常", "日常计件", "日常计件工资", domain.AssetWorkbenchPayrollRowTypeNormalPiecework:
		return domain.AssetWorkbenchPayrollRowTypeNormalPiecework, true
	case "补录", "补录计件", "补录计件工资", domain.AssetWorkbenchPayrollRowTypeSupplementPiecework:
		return domain.AssetWorkbenchPayrollRowTypeSupplementPiecework, true
	default:
		return "", false
	}
}

type qualityPayrollKey struct {
	payeeID int64
	rowType string
}

func qualityImportPayrollKeys(records []*domain.AssetWorkbenchErrorRecord, excludedBatchID int64) map[qualityPayrollKey]bool {
	keys := make(map[qualityPayrollKey]bool)
	for _, record := range records {
		if record != nil && (excludedBatchID == 0 || record.ImportBatchID != excludedBatchID) &&
			record.PayeeUserID != nil && record.ErrorCount > 0 && record.MatchStatus == domain.AssetWorkbenchErrorMatchStatusMatched {
			keys[qualityPayrollKey{*record.PayeeUserID, domain.WorkbenchPayrollRowType(record.RawPayload)}] = true
		}
	}
	return keys
}

func (s *Service) clearDeletedQualityDeductions(ctx context.Context, tx repo.Tx, actor domain.RequestActor, month string, batchID int64, deleted []*domain.AssetWorkbenchErrorRecord) error {
	affected := qualityImportPayrollKeys(deleted, 0)
	if len(affected) == 0 {
		return nil
	}
	records, err := s.repo.ListErrorRecordsByMonth(ctx, month)
	if err != nil {
		return err
	}
	remaining := qualityImportPayrollKeys(records, batchID)
	return s.clearOrphanQualityDeductions(ctx, tx, actor, month, affected, remaining,
		"删除最后一份关联质检记录，清除手填实扣")
}

// Also clear old orphaned overrides before a new import can bring them back.
func (s *Service) clearOrphanQualityDeductions(ctx context.Context, tx repo.Tx, actor domain.RequestActor, month string, affected, remaining map[qualityPayrollKey]bool, reason string) error {
	if len(affected) == 0 {
		return nil
	}
	overrides, err := s.repo.ListQualityActualDeductions(ctx, tx, month)
	if err != nil {
		return err
	}
	for _, before := range overrides {
		key := qualityPayrollKey{before.PayeeUserID, domain.WorkbenchPayrollRowType(before.Payload)}
		if before.Amount == 0 || !affected[key] || remaining[key] {
			continue
		}
		cleared, err := s.repo.CreateSettlementAdjustment(ctx, tx, &domain.AssetWorkbenchSettlementAdjustment{
			PayeeUserID: key.payeeID, BusinessMonth: month, AdjustmentType: domain.AssetWorkbenchAdjustmentTypeQualityActualDeduction,
			Amount: 0, Reason: reason, Status: domain.AssetWorkbenchAdjustmentStatusApplied,
			Payload: mustJSON(map[string]string{"payroll_row_type": key.rowType}), CreatedBy: actor.ID,
		})
		if err != nil {
			return err
		}
		if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventQualityActualDeductionSaved, domain.AssetWorkbenchEntityAdjustment,
			&cleared.ID, before, cleared, cleared.Reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) checkQualityDeductionAccess(actor domain.RequestActor, month string) *domain.AppError {
	if appErr := s.requireRepo(); appErr != nil {
		return appErr
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can manage quality actual deductions.", nil)
	}
	if _, err := time.Parse("2006-01", month); err != nil || month > s.businessMonth(s.nowFn().UTC()) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择当前或历史结算月。", nil)
	}
	return nil
}

// The original deduction remains the amount due. Freeze only the difference
// and the manual amount as a separate line when generating the batch.
func applyQualityActualDeductions(preview *SettlementPreview, entries []*domain.AssetWorkbenchSettlementAdjustment, reserved []*domain.AssetWorkbenchSettlementItem) []*domain.AssetWorkbenchSettlementItem {
	used := map[qualityPayrollKey]*domain.AssetWorkbenchSettlementItem{}
	for _, item := range reserved {
		if item != nil && item.ItemType == domain.AssetWorkbenchItemTypeQualityDeductionAdjustment && item.UnitPrice != nil && *item.UnitPrice > 0 {
			used[qualityPayrollKey{item.PayeeUserID, domain.WorkbenchPayrollRowType(item.Snapshot)}] = item
		}
	}
	overrides := map[qualityPayrollKey]*domain.AssetWorkbenchSettlementAdjustment{}
	for _, entry := range entries {
		if entry.Amount > 0 {
			overrides[qualityPayrollKey{entry.PayeeUserID, domain.WorkbenchPayrollRowType(entry.Payload)}] = entry
		}
	}
	lines := []*domain.AssetWorkbenchSettlementItem{}
	differences := map[int64]float64{}
	for i := range preview.PayrollRows {
		row := &preview.PayrollRows[i]
		var actual float64
		var sourceID *int64
		key := qualityPayrollKey{row.PayeeUserID, row.RowType}
		if previous := used[key]; previous != nil {
			// Later batches for this person/month must not deduct the manual
			// amount again. The first batch already froze that amount.
			sourceID = previous.SourceRefID
		} else if entry := overrides[key]; entry != nil && (row.ErrorCount > 0 || row.DeductionAmount > 0) {
			actual, sourceID = entry.Amount, &entry.ID
		}
		delta := math.Round((row.DeductionAmount-actual)*10000) / 10000
		row.QualityActualDeductionAmount = &actual
		row.ActualNetAmount = incomeMoney(row.NetAmount + delta)
		differences[row.PayeeUserID] += delta
		if delta == 0 && actual == 0 {
			continue
		}
		direction := "credit"
		if delta < 0 {
			direction = "debit"
		}
		lines = append(lines, &domain.AssetWorkbenchSettlementItem{
			ItemType: domain.AssetWorkbenchItemTypeQualityDeductionAdjustment, PayeeUserID: row.PayeeUserID,
			BusinessMonth: preview.BusinessMonth, Amount: math.Abs(delta), Quantity: 1, UnitPrice: &actual, Direction: direction,
			SourceRefType: "settlement_adjustment", SourceRefID: sourceID,
			Snapshot: mustJSON(map[string]any{"quality_actual_deduction_amount": actual, "deduction_amount": row.DeductionAmount, "payroll_row_type": row.RowType}),
		})
	}
	for i := range preview.Rows {
		row := &preview.Rows[i]
		row.ActualNetAmount = incomeMoney(row.NetAmount + differences[row.PayeeUserID])
	}
	preview.Totals.ActualNetAmount = preview.Totals.NetAmount
	for _, delta := range differences {
		preview.Totals.ActualNetAmount += delta
	}
	preview.Totals.ActualNetAmount = incomeMoney(preview.Totals.ActualNetAmount)
	return lines
}
