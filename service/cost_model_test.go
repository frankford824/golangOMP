package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"workflow/domain"
)

func TestCostModelRevisionPreservesHistoricalFormula(t *testing.T) {
	rules := newCostRuleRepoStub()
	categories := newCategoryRepoStub()
	categories.mustCreate(&domain.Category{CategoryID: 1, CategoryCode: "KT", CategoryName: "KT", CategoryType: domain.CategoryTypeBoard, IsActive: true, Level: 1})
	svc := NewCostRuleService(rules, categories, noopTxRunner{})
	original := `{"version":1,"material":"KT","basis":"area","unit_price":10,"multiplier":1.1}`
	first, err := svc.Create(context.Background(), CreateCostRuleParams{RuleName: "KT", CategoryCode: "KT", RuleType: domain.CostRuleTypeModel, FormulaExpression: original})
	if err != nil {
		t.Fatal(err)
	}
	revised := `{"version":1,"material":"KT","basis":"area","unit_price":12,"multiplier":1.1}`
	second, err := svc.Patch(context.Background(), PatchCostRuleParams{RuleID: first.RuleID, FormulaExpression: &revised})
	if err != nil {
		t.Fatal(err)
	}
	old, err := svc.GetByID(context.Background(), first.RuleID)
	if err != nil || old.FormulaExpression != original || second.RuleID == first.RuleID || second.RuleVersion != 2 || second.SupersedesRuleID == nil || *second.SupersedesRuleID != first.RuleID {
		t.Fatalf("history changed: old=%+v new=%+v error=%v", old, second, err)
	}
}

func TestCostModelAreaModesAndSalesUnit(t *testing.T) {
	m := &domain.CostModel{Version: 1, Material: "KT", Basis: "area", UnitPrice: 10, Multiplier: 1.1}
	cases := []struct {
		name string
		in   domain.CostInput
		want float64
	}{
		{"total never multiplies set pieces", domain.CostInput{AreaMode: "total", AreaM2: .58, Pieces: 6}, 6.38},
		{"flat per piece", domain.CostInput{AreaMode: "flat", WidthM: .1, HeightM: .2, Pieces: 6}, 1.32},
		{"layout complete sheet", domain.CostInput{AreaMode: "layout", WidthM: 1.2, HeightM: .9, Pieces: 6}, 11.88},
		{"explicit faces", domain.CostInput{AreaMode: "faces", Pieces: 6, Faces: []domain.CostFace{{WidthM: .3, HeightM: .3, Count: 6}}}, 5.94},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, v := calculateCostModel(m, tt.in)
			if v == nil || math.Abs(*v-tt.want) > .00001 || c.Status != "calculated" {
				t.Fatalf("%+v value=%v want=%v", c, v, tt.want)
			}
		})
	}
}
func TestCostModelRequiresExplicitProcessAndThickness(t *testing.T) {
	m := &domain.CostModel{Version: 1, Material: "KT", Basis: "area", Multiplier: 1, ThicknessPrices: []domain.CostThicknessPrice{{ThicknessMM: 5.5, UnitPrice: 12}}, Processes: []domain.CostProcessPrice{{Code: "punch", Unit: "hole", UnitPrice: .5, Multiplier: 1}}}
	in := domain.CostInput{AreaMode: "total", AreaM2: 1, Pieces: 1, ThicknessMM: 5.5}
	if c, v := calculateCostModel(m, in); v != nil || len(c.Missing) == 0 {
		t.Fatal("must require process confirmation")
	}
	in.Processes = map[string]bool{"punch": false}
	if _, v := calculateCostModel(m, in); v == nil || *v != 12 {
		t.Fatal("no punch must not charge")
	}
	in.Processes["punch"] = true
	in.HoleCount = 4
	if _, v := calculateCostModel(m, in); v == nil || *v != 14 {
		t.Fatal("four holes add two")
	}
	in.ThicknessMM = 4
	if _, v := calculateCostModel(m, in); v != nil {
		t.Fatal("unknown thickness must not guess")
	}
}
func TestCostModelDoesNotReadReferenceDimensionsOrStackLegacyRules(t *testing.T) {
	m := domain.CostModel{Version: 1, Material: "KT", Basis: "area", UnitPrice: 12, Multiplier: 1.1}
	raw, _ := json.Marshal(m)
	r := &domain.CostRule{RuleID: 99, RuleVersion: 1, RuleType: domain.CostRuleTypeModel, FormulaExpression: string(raw)}
	result := previewCostRules(domain.CostRulePreviewRequest{Area: float64Ptr(1), Notes: "70*70cm 源文件放大", Quantity: int64Ptr(1)}, []*domain.CostRule{r, {RuleID: 1, RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: float64Ptr(50)}}).Response
	if result.EstimatedCost == nil || *result.EstimatedCost != 13.2 {
		t.Fatalf("unexpected %+v", result)
	}
	result = previewCostRules(domain.CostRulePreviewRequest{Width: float64Ptr(1), Height: float64Ptr(1), Quantity: int64Ptr(200)}, []*domain.CostRule{r}).Response
	if result.EstimatedCost == nil || *result.EstimatedCost != 13.2 {
		t.Fatal("order quantity must not multiply the SKU cost")
	}
	result = previewCostRules(domain.CostRulePreviewRequest{}, []*domain.CostRule{r, r}).Response
	if result.EstimatedCost != nil || result.Calculation.Status != "rule_conflict" {
		t.Fatal("duplicate active models must fail")
	}
}
func TestCostModelInvalidConfigurationAndFilingGuard(t *testing.T) {
	for _, raw := range []string{`{"version":1,"material":"KT","basis":"area","unit_price":-1,"multiplier":1}`, `{"version":1,"material":"KT","basis":"eval","multiplier":1}`, `{"version":1,"material":"KT","basis":"area","multiplier":1,"unknown":1}`} {
		if _, err := parseCostModel(raw); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	if costReadyForFiling(nil, false, false) || costReadyForFiling(float64Ptr(10), true, false) || costReadyForFiling(float64Ptr(math.NaN()), false, true) {
		t.Fatal("unconfirmed costs allowed")
	}
	if !costReadyForFiling(float64Ptr(10), true, true) {
		t.Fatal("audited manual override must remain usable")
	}
}

func TestCostModelRequiresExactBindingForTask(t *testing.T) {
	r := &domain.CostRule{RuleType: domain.CostRuleTypeModel}
	result := applyCostRuleMatchMetadata(costPreviewComputation{MatchedRule: r, Response: &domain.CostRulePreviewResponse{EstimatedCost: float64Ptr(6.38)}}, domain.CostRuleMatchTrace{MatchMode: domain.CostRuleMatchModeLegacyAlias})
	if result.Response.EstimatedCost != nil || !result.Response.RequiresManualReview {
		t.Fatal("text match must not price a unified model")
	}
}

func TestCostModelRejectsStaleLegacyDimensionEdit(t *testing.T) {
	raw := json.RawMessage(`{"area":0.58,"cost_input":{"area_mode":"total","area_m2":0.58,"pieces":6}}`)
	if _, err := setTaskSKUItemSpecInVariantJSON(raw, UpdateTaskSKUItemInfoParams{Area: float64Ptr(.24)}); err == nil {
		t.Fatal("legacy area change would leave structured cost stale")
	}
	if _, err := setTaskSKUItemSpecInVariantJSON(raw, UpdateTaskSKUItemInfoParams{Area: float64Ptr(.58)}); err != nil {
		t.Fatal(err)
	}
}
