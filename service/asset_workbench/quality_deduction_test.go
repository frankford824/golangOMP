package assetworkbench

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
)

type qualityDeductionRepo struct {
	settlementBatchRepo
	locked bool
	saved  []*domain.AssetWorkbenchSettlementAdjustment
}

func (r *qualityDeductionRepo) ListSettlementMonthStates(context.Context) (map[string]bool, error) {
	return map[string]bool{"2026-10": r.locked}, nil
}

func (r *qualityDeductionRepo) ListQualityActualDeductions(context.Context, repo.Tx, string) ([]*domain.AssetWorkbenchSettlementAdjustment, error) {
	var result []*domain.AssetWorkbenchSettlementAdjustment
	seen := map[qualityPayrollKey]bool{}
	for i := len(r.saved) - 1; i >= 0; i-- {
		row := r.saved[i]
		key := qualityPayrollKey{row.PayeeUserID, domain.WorkbenchPayrollRowType(row.Payload)}
		if !seen[key] {
			result = append(result, row)
			seen[key] = true
		}
	}
	return result, nil
}

func TestSupplementQualityDeductionStaysOnSupplementPayroll(t *testing.T) {
	ctx := context.Background()
	r := &qualityDeductionRepo{}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	s.nowFn = func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }
	admin := domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAssetSettlement}}
	normal := domain.AssetWorkbenchPayrollRowTypeNormalPiecework
	supplement := domain.AssetWorkbenchPayrollRowTypeSupplementPiecework
	if err := s.SaveQualityActualDeductionForPayroll(ctx, admin, "2026-10", 350, 25, normal); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveQualityActualDeductionForPayroll(ctx, admin, "2026-10", 350, 5, supplement); err != nil {
		t.Fatal(err)
	}
	lines := []*domain.AssetWorkbenchSettlementItem{
		{PayeeUserID: 350, ItemType: domain.AssetWorkbenchItemTypeGrossPiecework, Amount: 283.06},
		{PayeeUserID: 350, ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, Amount: 45, Quantity: 6},
		{PayeeUserID: 350, ItemType: domain.AssetWorkbenchItemTypeSupplement, Amount: 30},
		{PayeeUserID: 350, ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, Amount: 10, Quantity: 1, Snapshot: mustJSON(map[string]string{"payroll_row_type": supplement})},
	}
	check := func(want float64) {
		t.Helper()
		entries, _ := r.ListQualityActualDeductions(ctx, nil, "2026-10")
		preview := &SettlementPreview{BusinessMonth: "2026-10", PayrollRows: buildSettlementPayrollRowsFromItems("2026-10", lines)}
		frozen := applyQualityActualDeductions(preview, entries, nil)
		rows := buildSettlementPayrollRowsFromItems("2026-10", append(lines, frozen...))
		if rows[0].ActualNetAmount != 258.06 || rows[0].DeductionAmount != 45 || rows[1].NetAmount != 20 ||
			rows[1].ActualNetAmount != want || rows[1].DeductionAmount != 10 || rows[1].ErrorCount != 1 {
			t.Fatalf("payroll rows = %+v", rows)
		}
	}
	check(25)
	if err := s.SaveQualityActualDeductionForPayroll(ctx, admin, "2026-10", 350, 0, supplement); err != nil {
		t.Fatal(err)
	}
	check(20)
}

func TestQualityExcelKeepsSupplementPayrollType(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	if err := f.SetSheetRow(sheet, "A1", &[]any{"日期", "出错人", "出错分类", "出错张数", "工资条类型"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetRow(sheet, "A2", &[]any{"2026-10-12", "测试人员", "A类", 1, "补录计件工资"}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	rows, err := parseErrorRecordsExcel(&buf)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rowType, ok := normalizeQualityPayrollRowType(rows[0].PayrollRowType); !ok || rowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework {
		t.Fatalf("imported payroll type = %q", rows[0].PayrollRowType)
	}
}

func (r *qualityDeductionRepo) CreateSettlementAdjustment(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementAdjustment) (*domain.AssetWorkbenchSettlementAdjustment, error) {
	item.ID = int64(len(r.saved) + 1)
	r.saved = append(r.saved, item)
	return item, nil
}

func TestQualityActualDeductionSaveResetAndLock(t *testing.T) {
	ctx := context.Background()
	r := &qualityDeductionRepo{}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	s.nowFn = func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }
	admin := domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAssetSettlement}}
	worker := domain.RequestActor{ID: 350, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	if err := s.SaveQualityActualDeduction(ctx, worker, "2026-10", 350, 50); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("worker save error = %v", err)
	}
	for _, amount := range []float64{50, 50, 0} {
		if err := s.SaveQualityActualDeduction(ctx, admin, "2026-10", 350, amount); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := s.QualityActualDeductions(ctx, admin, "2026-10")
	if err != nil || len(settings.Deductions) != 0 || len(r.saved) != 2 || len(r.events) != 2 {
		t.Fatalf("reset and retry must leave one active default and two audit rows: settings=%+v err=%v saved=%d", settings, err, len(r.saved))
	}
	r.locked = true
	if err := s.SaveQualityActualDeduction(ctx, admin, "2026-10", 350, 50); err == nil || err.Code != domain.ErrCodeConflict || len(r.saved) != 2 {
		t.Fatalf("locked save error = %v saved=%d", err, len(r.saved))
	}
}

func TestQualityActualDeductionReplacesSystemDeductionOnce(t *testing.T) {
	const payeeID = 350
	const month = "2026-10"
	gross := &domain.AssetWorkbenchSettlementItem{PayeeUserID: payeeID, ItemType: domain.AssetWorkbenchItemTypeGrossPiecework, Amount: 1141.96}
	deduction := &domain.AssetWorkbenchSettlementItem{PayeeUserID: payeeID, ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, Amount: 134, Quantity: 24}
	supplement := &domain.AssetWorkbenchSettlementItem{PayeeUserID: payeeID, ItemType: domain.AssetWorkbenchItemTypeSupplement, Amount: .8}
	original := []*domain.AssetWorkbenchSettlementItem{gross, deduction, supplement}
	preview := &SettlementPreview{BusinessMonth: month, PayrollRows: buildSettlementPayrollRowsFromItems(month, original)}
	override := []*domain.AssetWorkbenchSettlementAdjustment{{ID: 1, PayeeUserID: payeeID, Amount: 50}}
	lines := applyQualityActualDeductions(preview, override, nil)
	if len(lines) != 1 || lines[0].Amount != 84 || lines[0].Direction != "credit" {
		t.Fatalf("frozen lines = %+v", lines)
	}
	previewRow := preview.PayrollRows[0]
	batchRows := buildSettlementPayrollRowsFromItems(month, append(original, lines...))
	if incomeMoney(previewRow.NetAmount) != 1007.96 || previewRow.ActualNetAmount != 1091.96 ||
		batchRows[0].ActualNetAmount != 1091.96 || batchRows[0].DeductionAmount != 134 ||
		batchRows[1].ActualNetAmount != .8 || batchRows[1].QualityActualDeductionAmount != nil {
		t.Fatalf("preview=%+v batch=%+v", preview.PayrollRows, batchRows)
	}
	later := &SettlementPreview{BusinessMonth: month, PayrollRows: buildSettlementPayrollRowsFromItems(month, []*domain.AssetWorkbenchSettlementItem{gross, deduction})}
	laterLines := applyQualityActualDeductions(later, override, lines)
	if len(laterLines) != 1 || laterLines[0].Amount != 134 || later.PayrollRows[0].ActualNetAmount != 1141.96 {
		t.Fatalf("later batch must not charge the same monthly override twice: %+v", later.PayrollRows)
	}
	// Cancelling the original batch releases the override. A remaining batch
	// which refunded its system deduction did not charge the manual amount.
	regenerated := &SettlementPreview{BusinessMonth: month, PayrollRows: buildSettlementPayrollRowsFromItems(month, original)}
	regeneratedLines := applyQualityActualDeductions(regenerated, override, laterLines)
	if len(regeneratedLines) != 1 || regeneratedLines[0].UnitPrice == nil || *regeneratedLines[0].UnitPrice != 50 || regenerated.PayrollRows[0].ActualNetAmount != 1091.96 {
		t.Fatalf("regenerated batch must restore the released override: %+v", regenerated.PayrollRows)
	}
}
