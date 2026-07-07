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
