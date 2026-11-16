package assetworkbench

import (
	"context"
	"math"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestMonthlyContributionUsesWorksAndOneErrorRatePerPerson(t *testing.T) {
	report := &SettlementReport{
		DifficultyClasses: []string{"A", "B", "C"},
		Rows: []SettlementReportRow{
			{PayeeUserID: 1, WorkerType: "parttime", PageCount: 6, ErrorCount: 2, DifficultyMetrics: []SettlementReportDifficultyMetric{{DifficultyClass: "A", ItemCount: 2}}},
			{PayeeUserID: 1, WorkerType: "parttime", PageCount: 4, ErrorCount: 1, DifficultyMetrics: []SettlementReportDifficultyMetric{{DifficultyClass: "B", ItemCount: 1}}},
			{PayeeUserID: 2, WorkerType: "parttime", PageCount: 100, ErrorCount: 10, DifficultyMetrics: []SettlementReportDifficultyMetric{{DifficultyClass: "C", ItemCount: 2}}},
			{PayeeUserID: 3, WorkerType: "fulltime", PageCount: 10, ErrorCount: 5},
			{PayeeUserID: 4, WorkerType: "parttime", ErrorCount: 10},
		},
		Totals: SettlementReportRow{DifficultyMetrics: []SettlementReportDifficultyMetric{{DifficultyClass: "A", ItemCount: 2}, {DifficultyClass: "B", ItemCount: 1}, {DifficultyClass: "C", ItemCount: 2}}},
	}
	fillSettlementReportMetrics(report, map[string]float64{"A": 2.1})
	if report.Totals.MonthlyCoefficient != 6 || report.Rows[0].DifficultyMetrics[0].CoefficientSubtotal != 4.2 ||
		report.Rows[0].MonthlyCoefficient != 5 || report.Rows[1].MonthlyCoefficient != 5 ||
		report.Rows[0].MonthlyContributionRate != 5.0/6 || report.Rows[0].MonthlyErrorRate != .3 ||
		math.Abs(report.WorkerTypeAverageErrorRates["parttime"]-.2) > 1e-9 ||
		report.WorkerTypeAverageErrorRates["fulltime"] != .5 {
		t.Fatalf("unexpected contribution and benchmark output: %+v", report)
	}
}

func TestSettlementReportCombinesFrozenAndOpenWithoutDoubleDeduction(t *testing.T) {
	payee, workID, errorID := int64(1), int64(2), int64(3)
	r := &settlementReportRepo{
		profiles: map[int64]*domain.AssetWorkbenchProfile{payee: {UserID: payee, WorkerType: "parttime"}},
		rule:     &domain.AssetWorkbenchDeductionRule{DeductionAmount: 45},
	}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	items := []*domain.AssetWorkbenchSubmissionItem{{ID: workID, PayeeUserID: payee, DifficultyClass: "A", PageCount: 4, GrossAmount: 999}}
	errors := []*domain.AssetWorkbenchErrorRecord{{ID: errorID, PayeeUserID: &payee, DifficultyClass: "A", MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched, ErrorCount: 1}}
	supplements := []*domain.AssetWorkbenchSettlementSupplement{{ID: 4, PayeeUserID: payee, DifficultyClass: "C", PageCount: 1, GrossAmount: 30}}
	actual := 25.0
	ledger := []*domain.AssetWorkbenchSettlementItem{
		{ItemType: domain.AssetWorkbenchItemTypeGrossPiecework, SubmissionItemID: &workID, PayeeUserID: payee, Amount: 283.06},
		{ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, SourceRefID: &errorID, PayeeUserID: payee, Amount: 45, Quantity: 1},
		{ItemType: domain.AssetWorkbenchItemTypeQualityDeductionAdjustment, PayeeUserID: payee, Amount: 20, UnitPrice: &actual},
	}
	got, appErr := s.buildSettlementReport(context.Background(), "2026-10", items, errors, supplements, &repo.AssetWorkbenchReportSources{Ledger: ledger})
	if appErr != nil {
		t.Fatal(appErr)
	}
	if got.Totals.ItemCount != 2 || got.Totals.MonthlyCoefficient != 2 ||
		math.Abs(got.Totals.NetAmount-268.06) > 1e-9 || math.Abs(got.Totals.ActualNetAmount-288.06) > 1e-9 ||
		got.Totals.QualityActualDeductionAmount != 25 || got.Rows[0].WorkerTypeAverageErrorRate == nil || *got.Rows[0].WorkerTypeAverageErrorRate != .2 {
		t.Fatalf("unexpected combined report: %+v", got)
	}
}
