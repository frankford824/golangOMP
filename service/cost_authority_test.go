package service

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"workflow/domain"
)

func TestTaskSKUItemCostDimensionsPreserveStructuredFinalSize(t *testing.T) {
	item := &domain.TaskSKUItem{
		ProductNameSnapshot: "露冉常规kt板/中秋/月饼中秋/100*100cm",
		DesignRequirement:   "用：露冉常规kt板/中秋/月饼中秋/70*70cm 源文件等比例放大",
		VariantJSON:         json.RawMessage(`{"width":100,"height":100,"area":1}`),
	}
	dimensionText := firstCostDimensionText(
		taskSKUItemVariantCostNotes(item),
		item.ProductNameSnapshot,
		item.ProductShortName,
		billableDesignRequirementDimensionText(item.DesignRequirement),
	)
	width, height, area := taskSKUItemCostPreviewDimensions(&domain.TaskDetail{}, item, dimensionText)
	if width == nil || math.Abs(*width-1) > 0.000001 {
		t.Fatalf("width = %+v, want 1m", width)
	}
	if height == nil || math.Abs(*height-1) > 0.000001 {
		t.Fatalf("height = %+v, want 1m", height)
	}
	if area == nil || math.Abs(*area-1) > 0.000001 {
		t.Fatalf("area = %+v, want 1m2", area)
	}
	if strings.Contains(dimensionText, "70*70") {
		t.Fatalf("dimension text = %q, source-file dimensions must not control billing", dimensionText)
	}
}

func TestPreviewCostRulesFailsClosedForUncertifiedSampleRules(t *testing.T) {
	basePrice := 11.0
	taxMultiplier := 1.1
	result := previewCostRules(domain.CostRulePreviewRequest{Area: testFloat64Ptr(1)}, []*domain.CostRule{{
		RuleID:        1,
		RuleVersion:   1,
		RuleName:      "常规KT板基础单价",
		CategoryCode:  "KT_STANDARD",
		RuleType:      domain.CostRuleTypeFixedUnitPrice,
		BasePrice:     &basePrice,
		TaxMultiplier: &taxMultiplier,
		Priority:      10,
		IsActive:      true,
		Source:        "phase_020_sample",
	}}).Response
	if result == nil || !result.RequiresManualReview {
		t.Fatalf("result = %+v, want manual review", result)
	}
	if result.EstimatedCost != nil {
		t.Fatalf("estimated_cost = %+v, want nil for sample rule", result.EstimatedCost)
	}
	if !strings.Contains(result.Explanation, "未核定样例规则") || !strings.Contains(result.Explanation, "阻止自动写入") {
		t.Fatalf("explanation = %q", result.Explanation)
	}
}

func TestPreviewCostRulesPrefersCertifiedRuleOverSampleRule(t *testing.T) {
	samplePrice := 11.0
	certifiedPrice := 12.0
	result := previewCostRules(domain.CostRulePreviewRequest{Area: testFloat64Ptr(1)}, []*domain.CostRule{
		{RuleID: 1, RuleVersion: 1, RuleName: "样例", RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: &samplePrice, Priority: 1, IsActive: true, Source: "phase_020_sample"},
		{RuleID: 2, RuleVersion: 1, RuleName: "正式", RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: &certifiedPrice, Priority: 10, IsActive: true, Source: "production_verified_20260918"},
	}).Response
	if result == nil || result.RequiresManualReview || result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-12) > 0.000001 {
		t.Fatalf("result = %+v, want certified cost 12", result)
	}
	if result.MatchedRuleID == nil || *result.MatchedRuleID != 2 {
		t.Fatalf("matched_rule_id = %+v, want 2", result.MatchedRuleID)
	}
}

func testFloat64Ptr(value float64) *float64 { return &value }
