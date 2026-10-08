package service

import (
	"math"
	"testing"
	"workflow/domain"
)

func TestRestoredLegacyTariffsPreserveProductionCalculations(t *testing.T) {
	p := func(x float64) *float64 { return &x }
	base := func(price float64) *domain.CostRule {
		return &domain.CostRule{RuleID: 1, RuleVersion: 1, RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: p(price), TaxMultiplier: p(1.1), Priority: 10, Source: "phase_020_sample", IsActive: true}
	}
	small := &domain.CostRule{RuleID: 2, RuleVersion: 1, RuleType: domain.CostRuleTypeAreaThresholdSurcharge, AreaThreshold: p(.15), SurchargeAmount: p(3), Priority: 20, Source: "phase_020_sample", IsActive: true}
	slot := &domain.CostRule{RuleID: 3, RuleVersion: 1, RuleType: domain.CostRuleTypeSpecialProcessPrice, SpecialProcessKeyword: "开槽拼接", SpecialProcessPrice: p(1), Priority: 30, Source: "phase_020_sample", IsActive: true}
	for _, tc := range []struct {
		name, group, process, notes string
		area                        float64
		rules                       []*domain.CostRule
		want                        float64
		manual                      bool
	}{
		{"KT historical CGK004532", "KT_STANDARD", "", "", .35, []*domain.CostRule{base(11), small, slot}, 4.235, false},
		{"KT small with slot", "KT_STANDARD", "开槽拼接", "", .1, []*domain.CostRule{base(11), small, slot}, 2.54, false},
		{"Henan minimum", "KT_HENAN", "", "", .5, []*domain.CostRule{{RuleID: 6, RuleType: domain.CostRuleTypeMinimumBillableArea, MinArea: p(3), Priority: 5, Source: "phase_020_sample"}, base(11)}, 36.3, false},
		{"poster small", "POSTER_STANDARD", "", "", .1, []*domain.CostRule{base(5), small}, .88, false},
		{"flag", "FLAG_CLOTH_STANDARD", "", "", 1, []*domain.CostRule{base(4)}, 4.4, false},
		{"A3 double", "A3_PRINT", "双面", "", 0, []*domain.CostRule{{RuleID: 20, RuleType: domain.CostRuleTypeSizeBasedFormula, FormulaExpression: "print_side:single=0.5,double=0.6", Source: "phase_020_sample"}}, .6, false},
		{"copper card set", "COPPER_PAPER", "双面", "接亲卡片共10张", 0, []*domain.CostRule{{RuleID: 22, CategoryCode: "COPPER_PAPER", RuleType: domain.CostRuleTypeSizeBasedFormula, FormulaExpression: "size_lookup_required", Source: "phase_020_sample"}}, 1.2, false},
		{"white card missing table", "WHITE_CARD", "", "", 0, []*domain.CostRule{{RuleID: 23, CategoryCode: "WHITE_CARD", RuleType: domain.CostRuleTypeSizeBasedFormula, FormulaExpression: "size_lookup_required", Source: "phase_020_sample"}}, 0, true},
		{"manual remains manual", "DIECUT_STICKER", "", "", 1, []*domain.CostRule{{RuleID: 24, RuleType: domain.CostRuleTypeManualQuote, Source: "phase_020_sample"}}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := LegacyCostPlanDigest(tc.rules)
			r := PreviewRestoredLegacyCost(domain.CostRulePreviewRequest{CategoryCode: tc.group, Area: p(tc.area), Process: tc.process, Notes: tc.notes}, tc.rules)
			if r.RequiresManualReview != tc.manual {
				t.Fatalf("review=%v explanation=%s", r.RequiresManualReview, r.Explanation)
			}
			if tc.manual {
				if r.EstimatedCost != nil {
					t.Fatal("missing business tariff must not invent a price")
				}
			} else if r.EstimatedCost == nil || math.Abs(*r.EstimatedCost-tc.want) > .00001 {
				t.Fatalf("cost=%v want=%v explanation=%s", r.EstimatedCost, tc.want, r.Explanation)
			}
			if LegacyCostPlanDigest(tc.rules) != before {
				t.Fatal("preview mutated historical rule")
			}
		})
	}
}

func TestLegacyBindingRestorationUsesStyleIdentityAndKeepsConflicts(t *testing.T) {
	rules := []*domain.CostRule{{CategoryCode: "KT_STANDARD", IsActive: true}, {CategoryCode: "KT_STANDARD_FILM", IsActive: true}, {CategoryCode: "POSTER_STANDARD", IsActive: true}, {CategoryCode: "PHOTO_CLOTH_STANDARD", IsActive: true}}
	e := []LegacyCostBindingEvidence{{IID: "常规kt板", Groups: []string{"KT_STANDARD", "KT_STANDARD_FILM"}}, {IID: "常规海报", Groups: []string{"PHOTO_CLOTH_STANDARD", "POSTER_STANDARD"}}, {IID: "HZS", Groups: []string{"KT_STANDARD", "POSTER_STANDARD"}}, {IID: "X001", Groups: []string{"KT_STANDARD"}}, {IID: "常规PP无背胶"}}
	out := PlanLegacyCostBindings(rules, nil, e)
	by := map[string]LegacyCostBindingDecision{}
	for _, d := range out {
		by[d.IID] = d
	}
	if by["常规kt板"].Group != "KT_STANDARD" || by["常规海报"].Group != "POSTER_STANDARD" || by["X001"].Group != "" {
		t.Fatalf("wrong deterministic mappings: %+v", out)
	}
	if by["HZS"].Group != "" || by["常规PP无背胶"].Group != "" {
		t.Fatal("ambiguous or missing tariffs must remain unresolved")
	}
	existing := []*domain.CostRuleBinding{{NormalizedIID: domain.NormalizeIID("常规海报"), RuleGroup: "PHOTO_CLOTH_STANDARD", IsActive: true}}
	for _, d := range PlanLegacyCostBindings(rules, existing, e) {
		if d.IID == "常规海报" && d.Group != "PHOTO_CLOTH_STANDARD" {
			t.Fatal("overwrote explicit binding")
		}
	}
}
