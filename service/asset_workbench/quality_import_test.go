package assetworkbench

import (
	"bytes"
	"context"
	"testing"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
)

func TestQualityImportReviewsAllSheetsAndPersistsPayrollAssignment(t *testing.T) {
	const month = "2026-10"
	const normal = domain.AssetWorkbenchPayrollRowTypeNormalPiecework
	const supplement = domain.AssetWorkbenchPayrollRowTypeSupplementPiecework
	r := &errorImportRepo{
		profiles: []*domain.AssetWorkbenchProfile{{UserID: 100, RealName: "测试人员"}},
		qualityRefs: map[string]map[int64][]repo.AssetWorkbenchQualityWorkReference{
			month: {
				100: {
					{WorkID: 5, OrderNo: "共同订单", OriginalName: "日常.psd"},
					{WorkID: 6, Supplement: true, OrderNo: "共同订单", OriginalName: "补录.psd"},
					{WorkID: 6, Supplement: true, OrderNo: "共同订单", OriginalName: "补录2.psd"},
					{WorkID: -7, Supplement: true, OrderNo: "手工补录"},
				},
				101: {{WorkID: 9, OrderNo: "其他人员"}},
			},
			"2026-09": {100: {{WorkID: 8, OrderNo: "上月作品"}}},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}
	ctx := context.Background()
	f := excelize.NewFile()
	defer f.Close()
	if _, err := f.NewSheet("补录"); err != nil {
		t.Fatal(err)
	}
	add := func(sheet, cell string, data ...any) {
		t.Helper()
		if err := f.SetSheetRow(sheet, cell, &data); err != nil {
			t.Fatal(err)
		}
	}
	for _, sheet := range f.GetSheetList() {
		add(sheet, "A1", "说明：旧表也可用")
		add(sheet, "A2", "日期", "出错人", "分类", "出错张数", "线上订单号", "文件名", "工资条类型")
	}
	add("Sheet1", "A3", "2026-10-05", "测试人员", "A", 1)
	add("Sheet1", "A4", "2026-10-05", "测试人员", "A", 2, "共同订单", "日常.psd")
	add("补录", "A3", "2026-10-05", "测试人员", "A", 3, "共同订单", "补录.PSD")
	add("补录", "A5", "2026-10-05", "测试人员", "A", 4, "共同订单")
	add("补录", "A6", "2026-10-05", "测试人员", "A", 5, "其他人员")
	add("补录", "A7", "2026-10-05", "测试人员", "A", 6, "手工补录")
	add("补录", "A8", "2026-10-05", "测试人员", "A", 7, "上月作品")
	add("补录", "A9", "2026-10-05", "测试人员", "A", 8, "", "", "错误类型")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	read := func() *bytes.Reader { return bytes.NewReader(buf.Bytes()) }
	review, appErr := svc.PreviewQualityImportExcel(ctx, actor, month, read())
	if appErr != nil || len(review.Rows) != 8 || len(r.records) != 0 {
		t.Fatalf("preview=%+v error=%v records=%d", review, appErr, len(r.records))
	}
	want := []string{normal, normal, supplement, "", "", supplement, "", ""}
	for i, row := range review.Rows {
		if row.PayrollRowType != want[i] || row.NeedsPayrollType != (want[i] == "") || row.Blocking != (want[i] == "") {
			t.Errorf("review row %d: %+v", i, row)
		}
	}
	if review.Rows[3].Sheet != 2 || review.Rows[3].Row != 5 {
		t.Fatalf("wrong row coordinates: %+v", review.Rows[3])
	}
	if _, appErr := svc.ImportErrorRecordsExcel(ctx, actor, month, "test.xlsx", read()); appErr == nil || len(r.records) != 0 || r.batch != nil {
		t.Fatalf("ambiguous workbook wrote data: %v records=%d", appErr, len(r.records))
	}
	overrides := []QualityImportRowOverride{
		{Sheet: 2, Row: 5, PayrollRowType: supplement},
		{Sheet: 2, Row: 6, PayrollRowType: normal},
		{Sheet: 2, Row: 8, PayrollRowType: supplement},
		{Sheet: 2, Row: 9, PayrollRowType: normal},
	}
	if _, appErr := svc.ImportErrorRecordsExcel(ctx, actor, month, "test.xlsx", read(), append(overrides, overrides[0])...); appErr == nil || len(r.records) != 0 {
		t.Fatalf("duplicate override wrote data: %v", appErr)
	}
	r.qualityMonthStates = map[string]bool{month: true}
	if _, appErr := svc.ImportErrorRecordsExcel(ctx, actor, month, "test.xlsx", read(), overrides...); appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("locked month: %v", appErr)
	}
	r.qualityMonthStates[month] = false
	batch, appErr := svc.ImportErrorRecordsExcel(ctx, actor, month, "test.xlsx", read(), overrides...)
	if appErr != nil || batch.TotalRows != 8 || batch.MatchedRows != 8 || len(r.records) != 8 {
		t.Fatalf("batch=%+v err=%v records=%d", batch, appErr, len(r.records))
	}
	want = []string{normal, normal, supplement, supplement, normal, supplement, supplement, normal}
	for i, record := range r.records {
		if got := domain.WorkbenchPayrollRowType(record.RawPayload); got != want[i] {
			t.Errorf("persisted row %d has type %s, want %s", i, got, want[i])
		}
	}
}
