package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestCostRecalculationListActiveRunCostRulesCanDisableTextAliasFallback(t *testing.T) {
	costRuleRepo := newCostRuleRepoStub()
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        1,
			RuleVersion:   1,
			RuleName:      "通用基础单价",
			CategoryCode:  "GENERAL",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     float64Ptr(1),
			TaxMultiplier: float64Ptr(1),
			IsActive:      true,
		},
		{
			RuleID:        2,
			RuleVersion:   1,
			RuleName:      "常规喷绘布基础单价",
			CategoryCode:  "SPRAY_CLOTH_STANDARD",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     float64Ptr(4),
			TaxMultiplier: float64Ptr(1.1),
			IsActive:      true,
		},
	}
	svc := NewCostRecalculationService(nil, nil, nil, costRuleRepo, nil, nil,
		WithCostRecalculationLegacyAliasFallbackEnabled(false),
	).(*costRecalculationService)

	rules, err := svc.listActiveRunCostRules(context.Background(), nil, "GENERAL", "CPT-常规喷绘布/130*240cm")
	if err != nil {
		t.Fatalf("listActiveRunCostRules() error = %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("rule count = %d, want only direct GENERAL rule", len(rules))
	}
	if rules[0].CategoryCode != "GENERAL" {
		t.Fatalf("category = %q, want GENERAL", rules[0].CategoryCode)
	}
}

func TestCostRecalculationListActiveRunCostRulesFallsBackWhenDirectCategoryHasNoRules(t *testing.T) {
	costRuleRepo := newCostRuleRepoStub()
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:            31,
			RuleVersion:       2,
			RuleName:          "教师节亚克力面积成本",
			CategoryCode:      "ACRYLIC",
			RuleType:          domain.CostRuleTypeSizeBasedFormula,
			FormulaExpression: "keyword_area_unit_price:教师节=264",
			IsActive:          true,
		},
	}
	svc := NewCostRecalculationService(nil, nil, nil, costRuleRepo, nil, nil).(*costRecalculationService)

	rules, err := svc.listActiveRunCostRules(context.Background(), nil, "亚克力", "CPT紫定制亚克力/教师节/24.5*17cm厚4.5cm")
	if err != nil {
		t.Fatalf("listActiveRunCostRules() error = %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != 31 {
		t.Fatalf("rules = %+v, want rule 31", rules)
	}
}

func TestCostRecalculationExplicitModeResolvesExactSKUCodes(t *testing.T) {
	records := &productManagementRecordRepoFake{items: []*domain.ProductManagementRecord{
		{ID: 101, SKUCode: "DZA000036"},
		{ID: 102, SKUCode: "DZA000037"},
		{ID: 103, SKUCode: "OTHER"},
	}}
	svc := NewCostRecalculationService(records, nil, nil, nil, nil, nil).(*costRecalculationService)

	matched, filters, appErr := svc.collectRunRecords(context.Background(), domain.CostRecalculationRunModeExplicit, domain.CreateCostRecalculationRunRequest{
		SKUCodes: []string{" DZA000036 ", "dza000037", "DZA000036"},
	})
	if appErr != nil {
		t.Fatalf("collectRunRecords() appErr = %+v", appErr)
	}
	if len(matched) != 2 || matched[0].ID != 101 || matched[1].ID != 102 {
		t.Fatalf("matched records = %+v, want ids 101/102", matched)
	}
	if got, ok := filters["sku_codes"].([]string); !ok || len(got) != 2 || got[0] != "DZA000036" || got[1] != "dza000037" {
		t.Fatalf("filters sku_codes = %#v", filters["sku_codes"])
	}

	_, _, appErr = svc.collectRunRecords(context.Background(), domain.CostRecalculationRunModeExplicit, domain.CreateCostRecalculationRunRequest{SKUCodes: []string{"MISSING"}})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("missing SKU appErr = %+v, want invalid request", appErr)
	}
}
