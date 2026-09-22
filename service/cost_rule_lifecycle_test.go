package service

import (
	"context"
	"testing"
	"time"
	"workflow/domain"
)

func TestRuleDatesCanBeClearedAndRetirementDoesNotRestoreOldPrice(t *testing.T) {
	r := newCostRuleRepoStub()
	c := newCategoryRepoStub()
	c.mustCreate(&domain.Category{CategoryID: 1, CategoryCode: "KT", CategoryName: "KT", CategoryType: domain.CategoryTypeBoard, IsActive: true, Level: 1})
	s := NewCostRuleService(r, c, noopTxRunner{})
	ctx := context.Background()
	end := time.Now().Add(time.Hour)
	first, err := s.Create(ctx, CreateCostRuleParams{RuleName: "旧价格", CategoryCode: "KT", RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: float64Ptr(10), EffectiveTo: &end})
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := s.Patch(ctx, PatchCostRuleParams{RuleID: first.RuleID, EffectiveToSet: true})
	if err != nil || cleared.EffectiveTo != nil {
		t.Fatalf("clear failed: %+v %v", cleared, err)
	}
	second, err := s.Create(ctx, CreateCostRuleParams{RuleName: "新价格", CategoryCode: "KT", RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: float64Ptr(12), SupersedesRuleID: &first.RuleID})
	if err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := s.Patch(ctx, PatchCostRuleParams{RuleID: second.RuleID, IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	old, _ := s.GetByID(ctx, first.RuleID)
	latest, _ := s.GetByID(ctx, second.RuleID)
	if old.IsActive || latest.IsActive || *old.BasePrice != 10 || *latest.BasePrice != 12 {
		t.Fatal("retirement changed history or restored a predecessor")
	}
}
