package service

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestCategoryServiceAllowsCodedStyleAsTopLevelCategory(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	svc := NewCategoryService(categoryRepo, noopTxRunner{}).(*categoryService)

	category, appErr := svc.Create(context.Background(), CreateCategoryParams{
		CategoryCode: "HBJ",
		CategoryName: "HBJ",
		CategoryType: domain.CategoryTypeCodedStyle,
		Source:       "test",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if category.CategoryCode != "HBJ" || category.CategoryType != domain.CategoryTypeCodedStyle {
		t.Fatalf("created category = %+v", category)
	}
	if category.Level != 1 {
		t.Fatalf("created category level = %d, want 1", category.Level)
	}
	if category.SearchEntryCode != "HBJ" || !category.IsSearchEntry {
		t.Fatalf("created category search-entry fields = %+v", category)
	}
}

func TestCostRulePreviewAppliesFixedThresholdAndProcessSurcharge(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	now := time.Now()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   1,
		CategoryCode: "KT_STANDARD",
		CategoryName: "常规kt板",
		DisplayName:  "常规kt板",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        1,
			RuleVersion:   1,
			RuleName:      "常规KT板基础单价",
			CategoryCode:  "KT_STANDARD",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(11),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
			EffectiveFrom: &now,
		},
		{
			RuleID:          2,
			RuleVersion:     1,
			RuleName:        "常规KT板小面积附加",
			CategoryCode:    "KT_STANDARD",
			RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
			AreaThreshold:   costRuleFloat64Ptr(0.15),
			SurchargeAmount: costRuleFloat64Ptr(3),
			Priority:        20,
			IsActive:        true,
			Source:          "test",
			EffectiveFrom:   &now,
		},
		{
			RuleID:                3,
			RuleVersion:           1,
			RuleName:              "常规KT板开槽拼接加价",
			CategoryCode:          "KT_STANDARD",
			RuleType:              domain.CostRuleTypeSpecialProcessPrice,
			SpecialProcessKeyword: "开槽拼接",
			SpecialProcessPrice:   costRuleFloat64Ptr(1),
			Priority:              30,
			IsActive:              true,
			Source:                "test",
			EffectiveFrom:         &now,
		},
	}

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Area:         costRuleFloat64Ptr(0.1),
		Process:      "需要开槽拼接",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.RequiresManualReview {
		t.Fatalf("Preview() requires_manual_review = true, want false; result=%+v", result)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleID != 1 {
		t.Fatalf("matched_rule = %+v, want rule_id=1", result.MatchedRule)
	}
	if result.MatchedRuleVersion == nil || *result.MatchedRuleVersion != 1 {
		t.Fatalf("matched_rule_version = %+v, want 1", result.MatchedRuleVersion)
	}
	if result.GovernanceStatus != domain.CostRuleGovernanceStatusEffective {
		t.Fatalf("governance_status = %s, want %s", result.GovernanceStatus, domain.CostRuleGovernanceStatusEffective)
	}
	if len(result.AppliedRules) != 3 {
		t.Fatalf("applied_rules len = %d, want 3", len(result.AppliedRules))
	}
	if result.EstimatedCost == nil || *result.EstimatedCost <= 0 {
		t.Fatalf("estimated_cost = %+v, want > 0", result.EstimatedCost)
	}
	if !strings.Contains(result.Explanation, "固定单价部分") || !strings.Contains(result.Explanation, "特殊工艺附加") {
		t.Fatalf("explanation = %q, want plain Chinese calculation details", result.Explanation)
	}
	if strings.Contains(result.Explanation, " applied ") || strings.Contains(result.Explanation, " requires ") {
		t.Fatalf("explanation leaks technical English: %q", result.Explanation)
	}
}

func TestCostRulePreviewAppliesSurchargeTaxMultiplier(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   11,
		CategoryCode: "PHOTO_CLOTH_STANDARD",
		CategoryName: "常规写真布",
		DisplayName:  "常规写真布",
		CategoryType: domain.CategoryTypeCloth,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       11,
			RuleVersion:  1,
			RuleName:     "常规写真布基础单价",
			CategoryCode: "PHOTO_CLOTH_STANDARD",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    costRuleFloat64Ptr(5),
			Priority:     10,
			IsActive:     true,
			Source:       "test",
		},
		{
			RuleID:          12,
			RuleVersion:     1,
			RuleName:        "常规写真布小面积附加",
			CategoryCode:    "PHOTO_CLOTH_STANDARD",
			RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
			TaxMultiplier:   costRuleFloat64Ptr(1.1),
			AreaThreshold:   costRuleFloat64Ptr(0.15),
			SurchargeAmount: costRuleFloat64Ptr(3),
			Priority:        20,
			IsActive:        true,
			Source:          "test",
		},
	}

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "PHOTO_CLOTH_STANDARD",
		Area:         costRuleFloat64Ptr(0.1),
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-0.83) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 0.83", result.EstimatedCost)
	}
}

func TestCostRulePreviewAppliesSmallAreaSurchargeBySinglePieceArea(t *testing.T) {
	tests := []struct {
		name         string
		categoryCode string
		rules        []*domain.CostRule
		notes        string
		want         float64
	}{
		{
			name:         "kt board piece set",
			categoryCode: "KT_STANDARD",
			notes:        "常规KT板 20*20cm 4件套",
			want:         2.464,
			rules: []*domain.CostRule{
				{
					RuleID:        31,
					RuleVersion:   1,
					RuleName:      "常规KT板基础单价",
					CategoryCode:  "KT_STANDARD",
					RuleType:      domain.CostRuleTypeFixedUnitPrice,
					BasePrice:     costRuleFloat64Ptr(11),
					TaxMultiplier: costRuleFloat64Ptr(1.1),
					Priority:      10,
					IsActive:      true,
					Source:        "test",
				},
				{
					RuleID:          32,
					RuleVersion:     1,
					RuleName:        "常规KT板小面积附加",
					CategoryCode:    "KT_STANDARD",
					RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
					AreaThreshold:   costRuleFloat64Ptr(0.15),
					SurchargeAmount: costRuleFloat64Ptr(3),
					Priority:        20,
					IsActive:        true,
					Source:          "test",
				},
			},
		},
		{
			name:         "photo cloth piece set",
			categoryCode: "PHOTO_CLOTH_STANDARD",
			notes:        "常规写真布 20*20cm 4件套",
			want:         1.408,
			rules: []*domain.CostRule{
				{
					RuleID:        41,
					RuleVersion:   1,
					RuleName:      "常规写真布基础单价",
					CategoryCode:  "PHOTO_CLOTH_STANDARD",
					RuleType:      domain.CostRuleTypeFixedUnitPrice,
					BasePrice:     costRuleFloat64Ptr(5),
					TaxMultiplier: costRuleFloat64Ptr(1.1),
					Priority:      10,
					IsActive:      true,
					Source:        "test",
				},
				{
					RuleID:          42,
					RuleVersion:     1,
					RuleName:        "常规写真布小面积附加",
					CategoryCode:    "PHOTO_CLOTH_STANDARD",
					RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
					TaxMultiplier:   costRuleFloat64Ptr(1.1),
					AreaThreshold:   costRuleFloat64Ptr(0.15),
					SurchargeAmount: costRuleFloat64Ptr(3),
					Priority:        20,
					IsActive:        true,
					Source:          "test",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewCostRules(domain.CostRulePreviewRequest{
				CategoryCode: tt.categoryCode,
				Notes:        tt.notes,
			}, tt.rules).Response
			if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-tt.want) > 0.000001 {
				t.Fatalf("estimated_cost = %+v, want %.3f", result.EstimatedCost, tt.want)
			}
			if len(result.AppliedRules) != 2 {
				t.Fatalf("applied rules = %d, want base + small-area surcharge", len(result.AppliedRules))
			}
			if result.RequiresManualReview {
				t.Fatalf("requires_manual_review = true, want false")
			}
		})
	}
}

func TestCostRulePreviewExtractsSizeFromNotes(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   21,
		CategoryCode: "KT_STANDARD",
		CategoryName: "常规KT板",
		DisplayName:  "常规KT板",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       21,
			RuleVersion:  1,
			RuleName:     "KT板面积单价",
			CategoryCode: "KT_STANDARD",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    costRuleFloat64Ptr(11),
			Priority:     10,
			IsActive:     true,
			Source:       "test",
		},
		{
			RuleID:          22,
			RuleVersion:     1,
			RuleName:        "小面积附加",
			CategoryCode:    "KT_STANDARD",
			RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
			AreaThreshold:   costRuleFloat64Ptr(0.15),
			SurchargeAmount: costRuleFloat64Ptr(3),
			Priority:        20,
			IsActive:        true,
			Source:          "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Notes:        "运营备注：尺寸20x20cm，做常规KT板",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-0.56) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 0.56", result.EstimatedCost)
	}
	if result.RequiresManualReview {
		t.Fatalf("requires_manual_review = true, want false")
	}
}

func TestCostRulePreviewPrefersTextDimensionsOverStaleNumericPayload(t *testing.T) {
	rules := []*domain.CostRule{
		{
			RuleID:       31,
			RuleVersion:  1,
			RuleName:     "常规覆膜KT板小面积保底",
			CategoryCode: "KT_STANDARD_FILM",
			RuleType:     domain.CostRuleTypeMinimumBillableArea,
			MinArea:      costRuleFloat64Ptr(0.15),
			Priority:     5,
			IsActive:     true,
			Source:       "test",
		},
		{
			RuleID:        32,
			RuleVersion:   1,
			RuleName:      "常规覆膜KT板基础单价",
			CategoryCode:  "KT_STANDARD_FILM",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(13),
			TaxMultiplier: costRuleFloat64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
		},
	}

	result := previewCostRules(domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD_FILM",
		Width:        costRuleFloat64Ptr(0.56),
		Height:       costRuleFloat64Ptr(1.2),
		Area:         costRuleFloat64Ptr(1.224),
		Process:      "历史商品名 汪敏/常规KT板/覆膜/帕恰狗黄色圆点/56*120cm",
		Notes:        "汪敏/常规KT板/覆膜/帕恰狗黄色圆点/46*120cm",
	}, rules).Response

	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-7.894) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 7.894", result.EstimatedCost)
	}
}

func TestTaskCostPreviewDimensionsPrefersTextOverStaleDetailArea(t *testing.T) {
	width := 0.56
	height := 1.2
	area := 1.224
	detail := &domain.TaskDetail{
		SpecText: "46*120cm",
		Remark:   "历史备注 汪敏/定制KT板/覆膜/帕恰狗黄色圆点/56*120cm",
		Width:    &width,
		Height:   &height,
		Area:     &area,
	}

	applyTextDerivedCostDimensions(detail, true, false)

	if detail.Width == nil || math.Abs(*detail.Width-46) > 0.000001 {
		t.Fatalf("width = %+v, want 46 cm", detail.Width)
	}
	if detail.Height == nil || math.Abs(*detail.Height-120) > 0.000001 {
		t.Fatalf("height = %+v, want 120 cm", detail.Height)
	}
	if detail.Area == nil || math.Abs(*detail.Area-0.552) > 0.000001 {
		t.Fatalf("area = %+v, want 0.552", detail.Area)
	}
}

func TestTaskCostPreviewDimensionsConvertsStructuredCentimetersToMeters(t *testing.T) {
	widthCM := 180.0
	heightCM := 90.0
	detail := &domain.TaskDetail{Width: &widthCM, Height: &heightCM}

	widthM, heightM, area := taskCostPreviewDimensions(detail, "")
	if widthM == nil || math.Abs(*widthM-1.8) > 0.000001 {
		t.Fatalf("width = %+v, want 1.8 m", widthM)
	}
	if heightM == nil || math.Abs(*heightM-0.9) > 0.000001 {
		t.Fatalf("height = %+v, want 0.9 m", heightM)
	}
	if area != nil {
		t.Fatalf("area = %+v, want nil when only structured width/height exist", area)
	}
}

func TestTaskSKUItemCostPreviewDimensionsConvertsVariantCentimeters(t *testing.T) {
	item := &domain.TaskSKUItem{VariantJSON: json.RawMessage(`{"width":180,"height":90}`)}

	widthM, heightM, area := taskSKUItemCostPreviewDimensions(&domain.TaskDetail{}, item, "")
	if widthM == nil || math.Abs(*widthM-1.8) > 0.000001 {
		t.Fatalf("width = %+v, want 1.8 m", widthM)
	}
	if heightM == nil || math.Abs(*heightM-0.9) > 0.000001 {
		t.Fatalf("height = %+v, want 0.9 m", heightM)
	}
	if area != nil {
		t.Fatalf("area = %+v, want nil when only structured width/height exist", area)
	}
	rules := []*domain.CostRule{{
		RuleID: 10, RuleVersion: 1, RuleName: "常规覆膜KT板基础单价", CategoryCode: "KT_STANDARD_FILM",
		RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: costRuleFloat64Ptr(13), TaxMultiplier: costRuleFloat64Ptr(1.1), IsActive: true,
	}}
	preview := previewCostRules(domain.CostRulePreviewRequest{Width: widthM, Height: heightM}, rules).Response
	if preview.EstimatedCost == nil || math.Abs(*preview.EstimatedCost-23.166) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 23.166", preview.EstimatedCost)
	}
}

func TestCostRulePreviewBlocksSuspiciousAutomaticAmount(t *testing.T) {
	rules := []*domain.CostRule{{
		RuleID: 10, RuleVersion: 1, RuleName: "常规覆膜KT板基础单价", CategoryCode: "KT_STANDARD_FILM",
		RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: costRuleFloat64Ptr(13), TaxMultiplier: costRuleFloat64Ptr(1.1), IsActive: true,
	}}
	result := previewCostRules(domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD_FILM",
		Width:        costRuleFloat64Ptr(180),
		Height:       costRuleFloat64Ptr(90),
	}, rules).Response

	if !result.RequiresManualReview {
		t.Fatalf("requires_manual_review = false, want true")
	}
	if result.EstimatedCost != nil {
		t.Fatalf("estimated_cost = %+v, want nil when amount guard blocks automatic write", result.EstimatedCost)
	}
	if !strings.Contains(result.Explanation, "超过自动写入上限") {
		t.Fatalf("explanation = %q, want amount-guard message", result.Explanation)
	}
}

func TestCostRulePreviewRegularFilmKTSlotSurcharge(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   31,
		CategoryCode: "KT_STANDARD_FILM",
		CategoryName: "常规kt板(覆膜)",
		DisplayName:  "常规kt板(覆膜)",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       31,
			RuleVersion:  1,
			RuleName:     "常规覆膜KT板小面积保底",
			CategoryCode: "KT_STANDARD_FILM",
			RuleType:     domain.CostRuleTypeMinimumBillableArea,
			MinArea:      costRuleFloat64Ptr(0.15),
			Priority:     5,
			IsActive:     true,
			Source:       "test",
		},
		{
			RuleID:        32,
			RuleVersion:   1,
			RuleName:      "常规覆膜KT板基础单价",
			CategoryCode:  "KT_STANDARD_FILM",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(13),
			TaxMultiplier: costRuleFloat64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD_FILM",
		Notes:        "常规kt板(覆膜) 需要开槽 46*120cm",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-8.894) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 8.894", result.EstimatedCost)
	}
	if len(result.AppliedRules) != 2 {
		t.Fatalf("applied_rules len = %d, want 2", len(result.AppliedRules))
	}
	if got := result.AppliedRules[1].RuleName; got != "常规覆膜KT板开槽加价" {
		t.Fatalf("slot surcharge rule = %q", got)
	}

	costRuleRepo.rules = append(costRuleRepo.rules, &domain.CostRule{
		RuleID:                33,
		RuleVersion:           1,
		RuleName:              "数据库常规覆膜KT板开槽加价",
		CategoryCode:          "KT_STANDARD_FILM",
		RuleType:              domain.CostRuleTypeSpecialProcessPrice,
		SpecialProcessKeyword: "开槽",
		SpecialProcessPrice:   costRuleFloat64Ptr(1),
		Priority:              30,
		IsActive:              true,
		Source:                "test",
	})
	result, appErr = svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD_FILM",
		Notes:        "常规kt板(覆膜) 需要开槽 46*120cm",
	})
	if appErr != nil {
		t.Fatalf("Preview(with persisted slot rule) unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-8.894) > 0.000001 {
		t.Fatalf("estimated_cost with persisted rule = %+v, want 8.894", result.EstimatedCost)
	}
	if len(result.AppliedRules) != 2 {
		t.Fatalf("applied_rules with persisted rule len = %d, want 2", len(result.AppliedRules))
	}
}

func TestCostRulePreviewRegularKTUsesTaxedStandardUnitPrice(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   21,
		CategoryCode: "KT_STANDARD",
		CategoryName: "常规KT板",
		DisplayName:  "常规KT板",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        21,
			RuleVersion:   1,
			RuleName:      "KT板面积单价",
			CategoryCode:  "KT_STANDARD",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(11),
			TaxMultiplier: costRuleFloat64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Notes:        "CPT-常规kt板/娜塔莎生日/红色波点裙子大号/150*90cm",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-16.335) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 16.335", result.EstimatedCost)
	}
}

func TestCostRulePreviewTreatsTrailingMultiplierAsBoxFaces(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   21,
		CategoryCode: "KT_STANDARD",
		CategoryName: "常规KT板",
		DisplayName:  "常规KT板",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        21,
			RuleVersion:   1,
			RuleName:      "KT板面积单价",
			CategoryCode:  "KT_STANDARD",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(11),
			TaxMultiplier: costRuleFloat64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
		},
		{
			RuleID:          22,
			RuleVersion:     1,
			RuleName:        "小面积附加",
			CategoryCode:    "KT_STANDARD",
			RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
			AreaThreshold:   costRuleFloat64Ptr(0.15),
			SurchargeAmount: costRuleFloat64Ptr(3),
			Priority:        20,
			IsActive:        true,
			Source:          "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Notes:        "CPT-常规kt板/中高考抽奖箱/高考顺利/30*30cm*6",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-13.068) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 13.068", result.EstimatedCost)
	}
	if len(result.AppliedRules) != 1 {
		t.Fatalf("applied rules = %d, want only base rule without small-area surcharge", len(result.AppliedRules))
	}
}

func TestCostRulePreviewKeepsExplicitBillableAreaWhenNotesContainFaceDimensions(t *testing.T) {
	result := previewCostRules(domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Area:         costRuleFloat64Ptr(0.54),
		Process:      "开槽",
		Notes:        "CPT-常规KT板/教师节抽奖箱（需开槽）/30*30cm",
	}, []*domain.CostRule{
		{
			RuleID:        61,
			RuleVersion:   1,
			RuleName:      "常规KT板基础单价",
			CategoryCode:  "KT_STANDARD",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     costRuleFloat64Ptr(11),
			TaxMultiplier: costRuleFloat64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "test",
		},
		{
			RuleID:          62,
			RuleVersion:     1,
			RuleName:        "常规KT板小面积附加",
			CategoryCode:    "KT_STANDARD",
			RuleType:        domain.CostRuleTypeAreaThresholdSurcharge,
			AreaThreshold:   costRuleFloat64Ptr(0.15),
			SurchargeAmount: costRuleFloat64Ptr(3),
			Priority:        20,
			IsActive:        true,
			Source:          "test",
		},
	}).Response

	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-6.534) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 6.534 from explicit billable area", result.EstimatedCost)
	}
	if len(result.AppliedRules) != 1 {
		t.Fatalf("applied rules = %d, want base rule only", len(result.AppliedRules))
	}
}

func TestCostRulePreviewExtractsLongestSideFromNotes(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   1,
		CategoryCode: "KT_STANDARD",
		CategoryName: "常规kt板",
		DisplayName:  "常规kt板",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       1,
			RuleVersion:  1,
			RuleName:     "常规KT板基础单价",
			CategoryCode: "KT_STANDARD",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    float64Ptr(11),
			Priority:     10,
			IsActive:     true,
			Source:       "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "KT_STANDARD",
		Notes:        "常规kt板 心理手举牌 最长边25cm",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-0.688) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 0.688", result.EstimatedCost)
	}
}

func TestCostRulePreviewCopperPaperSizeLookupUsesNameAndPrintSide(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   22,
		CategoryCode: "COPPER_PAPER",
		CategoryName: "铜版纸",
		DisplayName:  "铜版纸",
		CategoryType: domain.CategoryTypePaper,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:            22,
			RuleVersion:       1,
			RuleName:          "铜版纸尺寸规则骨架",
			CategoryCode:      "COPPER_PAPER",
			RuleType:          domain.CostRuleTypeSizeBasedFormula,
			FormulaExpression: "size_lookup_required",
			Priority:          10,
			IsActive:          true,
			Source:            "test",
		},
	}
	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)

	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "COPPER_PAPER",
		Notes:        "常规250g铜版纸 双面 10*15cm",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.RequiresManualReview {
		t.Fatalf("requires_manual_review = true, want false; result=%+v", result)
	}
	if result.EstimatedCost == nil || math.Abs(*result.EstimatedCost-0.6) > 0.000001 {
		t.Fatalf("estimated_cost = %+v, want 0.6", result.EstimatedCost)
	}
}

func TestCostCategoryAliasesFromTextPrefersOneSpecificNameMatch(t *testing.T) {
	tests := []struct {
		name         string
		categoryCode string
		notes        string
		want         []string
	}{
		{
			name:         "custom film kt does not also add normal kt",
			categoryCode: "GENERAL",
			notes:        "定制覆膜kt板 30*40cm",
			want:         []string{"KT_CUSTOM_FILM"},
		},
		{
			name:         "custom film kt split by slash still keeps film",
			categoryCode: "GENERAL",
			notes:        "汪敏/定制KT板/覆膜/帕恰狗黄色圆点/46*120cm",
			want:         []string{"KT_CUSTOM_FILM"},
		},
		{
			name:         "regular film kt split by slash still keeps film",
			categoryCode: "GENERAL",
			notes:        "汪敏/常规KT板/覆膜/帕恰狗黄色圆点/46*120cm",
			want:         []string{"KT_STANDARD_FILM"},
		},
		{
			name:         "regular kt keeps material when variant name has red",
			categoryCode: "GENERAL",
			notes:        "CPT-常规kt板/娜塔莎生日/红色波点裙子大号/150*90cm",
			want:         []string{"KT_STANDARD"},
		},
		{
			name:         "regular kt keeps material when variant name has gold",
			categoryCode: "GENERAL",
			notes:        "CPT-常规kt板/活动物料/金色奖牌造型/100*60cm",
			want:         []string{"KT_STANDARD"},
		},
		{
			name:         "red kt material still maps to red kt",
			categoryCode: "GENERAL",
			notes:        "CPT-红色kt板/门头装饰/150*90cm",
			want:         []string{"KT_RED"},
		},
		{
			name:         "gold kt material still maps to gold kt",
			categoryCode: "GENERAL",
			notes:        "CPT-金色kt板/门头装饰/150*90cm",
			want:         []string{"KT_GOLD"},
		},
		{
			name:         "copper paper can be found from product name",
			categoryCode: "GENERAL",
			notes:        "常规250g铜版纸 双面 10*15cm",
			want:         []string{"COPPER_PAPER"},
		},
		{
			name:         "plain pp is not mistaken for sticky pp",
			categoryCode: "PP_STICKY",
			notes:        "PP纸无背胶 20*30cm",
			want:         []string{"PP_PLAIN"},
		},
		{
			name:         "legacy chinese acrylic category maps to acrylic rule",
			categoryCode: "亚克力",
			notes:        "CPT紫定制亚克力/教师节/24.5*17cm厚4.5cm",
			want:         []string{"ACRYLIC"},
		},
		{
			name:         "regular poster maps to poster rule",
			categoryCode: "GENERAL",
			notes:        "常规海报 30*40cm",
			want:         []string{"POSTER_STANDARD"},
		},
		{
			name:         "plain banner maps to photo cloth rule",
			categoryCode: "GENERAL",
			notes:        "横幅 40*200cm",
			want:         []string{"PHOTO_CLOTH_STANDARD"},
		},
		{
			name:         "flag cloth is not mistaken for hanging cloth",
			categoryCode: "GENERAL",
			notes:        "露冉常规旗帜布/夏天挂布/特大号夏万物繁盛夏日长100*200cm",
			want:         []string{"FLAG_CLOTH_STANDARD"},
		},
		{
			name:         "regular spray cloth maps to spray cloth rule",
			categoryCode: "GENERAL",
			notes:        "CPT-常规喷绘布/端午保龄球游戏地垫粽子大号/130*240cm",
			want:         []string{"SPRAY_CLOTH_STANDARD"},
		},
		{
			name:         "custom spray cloth maps to custom spray cloth rule",
			categoryCode: "GENERAL",
			notes:        "CPT-定制喷绘布/地垫/80*240cm",
			want:         []string{"SPRAY_CLOTH_CUSTOM"},
		},
		{
			name:         "pp poster is not mistaken for photo cloth",
			categoryCode: "GENERAL",
			notes:        "PP海报背胶 30*40cm",
			want:         []string{"PP_STICKY"},
		},
		{
			name:         "custom poster with happy text is not mistaken for pp material",
			categoryCode: "GENERAL",
			notes:        "露邱/定制海报/4rdhappybirthday白底西瓜彩条/100*150cm",
			want:         []string{"PHOTO_CLOTH_CUSTOM"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := costCategoryAliasesFromText(tt.categoryCode, tt.notes)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("aliases = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCostRulePreviewReturnsManualReviewForManualQuote(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	now := time.Now()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   2,
		CategoryCode: "ACRYLIC",
		CategoryName: "亚克力",
		DisplayName:  "亚克力",
		CategoryType: domain.CategoryTypeMaterial,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        10,
			RuleVersion:   3,
			RuleName:      "亚克力人工报价",
			CategoryCode:  "ACRYLIC",
			RuleType:      domain.CostRuleTypeManualQuote,
			Priority:      10,
			IsActive:      true,
			Source:        "test",
			EffectiveFrom: &now,
		},
	}

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "ACRYLIC",
		Area:         costRuleFloat64Ptr(1),
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if !result.RequiresManualReview {
		t.Fatalf("requires_manual_review = false, want true; result=%+v", result)
	}
	if result.MatchedRule == nil || result.MatchedRule.RuleType != domain.CostRuleTypeManualQuote {
		t.Fatalf("matched_rule = %+v, want manual_quote", result.MatchedRule)
	}
	if result.MatchedRuleVersion == nil || *result.MatchedRuleVersion != 3 {
		t.Fatalf("matched_rule_version = %+v, want 3", result.MatchedRuleVersion)
	}
}

func TestCostRulePreviewCalculatesKeywordAreaUnitPrice(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   2,
		CategoryCode: "ACRYLIC",
		CategoryName: "亚克力",
		DisplayName:  "亚克力",
		CategoryType: domain.CategoryTypeMaterial,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{{
		RuleID:            26,
		RuleVersion:       2,
		RuleName:          "教师节亚克力面积成本",
		CategoryCode:      "ACRYLIC",
		RuleType:          domain.CostRuleTypeSizeBasedFormula,
		FormulaExpression: "keyword_area_unit_price:教师节=264",
		Priority:          10,
		IsActive:          true,
		Source:            "test",
	}}

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "ACRYLIC",
		Area:         costRuleFloat64Ptr(0.035475),
		Notes:        "定制亚克力/教师节/16.5*21.5cm厚4.5mm",
	})
	if appErr != nil {
		t.Fatalf("Preview() unexpected error: %+v", appErr)
	}
	if result.RequiresManualReview {
		t.Fatalf("requires_manual_review = true, want false; result=%+v", result)
	}
	if result.EstimatedCost == nil || *result.EstimatedCost != 9.37 {
		t.Fatalf("estimated_cost = %+v, want 9.37", result.EstimatedCost)
	}
	result, appErr = svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "ACRYLIC",
		Area:         costRuleFloat64Ptr(0.03335),
		Notes:        "定制亚克力/教师节/14.5*23.5cm",
	})
	if appErr != nil {
		t.Fatalf("Preview() with stale structured area unexpected error: %+v", appErr)
	}
	if result.EstimatedCost == nil || *result.EstimatedCost != 9.00 {
		t.Fatalf("text-authoritative estimated_cost = %+v, want 9.00", result.EstimatedCost)
	}
	for _, tt := range []struct {
		name string
		area float64
		want float64
	}{
		{name: "DZA000036", area: 0.03335, want: 8.80},
		{name: "DZA000037", area: 0.03960, want: 10.45},
		{name: "DZA000039", area: 0.03360, want: 8.87},
		{name: "DZA000043", area: 0.034075, want: 9.00},
		{name: "DZA000048", area: 0.04165, want: 11.00},
		{name: "DZA000049", area: 0.03335, want: 8.80},
		{name: "DZA000050", area: 0.034075, want: 9.00},
		{name: "DZA000052", area: 0.028275, want: 7.46},
		{name: "DZA000053", area: 0.034075, want: 9.00},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, appErr := svc.Preview(context.Background(), domain.CostRulePreviewRequest{
				CategoryCode: "ACRYLIC",
				Area:         costRuleFloat64Ptr(tt.area),
				Notes:        "定制亚克力/教师节",
			})
			if appErr != nil {
				t.Fatalf("Preview() unexpected error: %+v", appErr)
			}
			if result.EstimatedCost == nil || *result.EstimatedCost != tt.want {
				t.Fatalf("estimated_cost = %+v, want %.2f", result.EstimatedCost, tt.want)
			}
		})
	}

	result, appErr = svc.Preview(context.Background(), domain.CostRulePreviewRequest{
		CategoryCode: "ACRYLIC",
		Area:         costRuleFloat64Ptr(0.0015),
		Notes:        "定制亚克力/钥匙扣/5*3cm厚2mm",
	})
	if appErr != nil {
		t.Fatalf("Preview() without keyword unexpected error: %+v", appErr)
	}
	if !result.RequiresManualReview || result.EstimatedCost != nil {
		t.Fatalf("non-teacher acrylic result = %+v, want manual review without estimate", result)
	}
}

func TestCostRuleCreateAutoVersionsWhenSupersedingPriorRule(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   3,
		CategoryCode: "KT_CUSTOM",
		CategoryName: "KT Custom",
		DisplayName:  "KT Custom",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       20,
			RuleVersion:  1,
			RuleName:     "KT Custom Base V1",
			CategoryCode: "KT_CUSTOM",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    costRuleFloat64Ptr(10),
			Priority:     10,
			IsActive:     true,
			Source:       "test",
		},
	}
	costRuleRepo.nextID = 21

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	created, appErr := svc.Create(context.Background(), CreateCostRuleParams{
		RuleName:         "KT Custom Base V2",
		CategoryCode:     "KT_CUSTOM",
		RuleType:         domain.CostRuleTypeFixedUnitPrice,
		BasePrice:        costRuleFloat64Ptr(12),
		Priority:         10,
		SupersedesRuleID: int64Ptr(20),
		GovernanceNote:   "new sample price",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if created.RuleVersion != 2 {
		t.Fatalf("created rule_version = %d, want 2", created.RuleVersion)
	}
	if created.SupersedesRuleID == nil || *created.SupersedesRuleID != 20 {
		t.Fatalf("created supersedes_rule_id = %+v, want 20", created.SupersedesRuleID)
	}
}

func TestCostRuleGetHistoryReturnsVersionChainSummary(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   4,
		CategoryCode: "KT_HISTORY",
		CategoryName: "KT History",
		DisplayName:  "KT History",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       30,
			RuleVersion:  1,
			RuleName:     "KT History V1",
			CategoryCode: "KT_HISTORY",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    costRuleFloat64Ptr(10),
			Priority:     10,
			IsActive:     true,
			Source:       "test",
		},
		{
			RuleID:           31,
			RuleVersion:      2,
			RuleName:         "KT History V2",
			CategoryCode:     "KT_HISTORY",
			RuleType:         domain.CostRuleTypeFixedUnitPrice,
			BasePrice:        costRuleFloat64Ptr(11),
			Priority:         10,
			IsActive:         true,
			SupersedesRuleID: int64Ptr(30),
			Source:           "test",
		},
		{
			RuleID:           32,
			RuleVersion:      3,
			RuleName:         "KT History V3",
			CategoryCode:     "KT_HISTORY",
			RuleType:         domain.CostRuleTypeFixedUnitPrice,
			BasePrice:        costRuleFloat64Ptr(12),
			Priority:         10,
			IsActive:         true,
			SupersedesRuleID: int64Ptr(31),
			Source:           "test",
		},
	}

	svc := NewCostRuleService(costRuleRepo, categoryRepo, noopTxRunner{}).(*costRuleService)
	history, appErr := svc.GetHistory(context.Background(), 31)
	if appErr != nil {
		t.Fatalf("GetHistory() unexpected error: %+v", appErr)
	}
	if history == nil || history.Rule == nil {
		t.Fatal("GetHistory() returned nil history")
	}
	if len(history.VersionChain) != 3 {
		t.Fatalf("version_chain len = %d, want 3", len(history.VersionChain))
	}
	if history.Rule.PreviousVersion == nil || history.Rule.PreviousVersion.RuleID != 30 {
		t.Fatalf("previous_version = %+v, want rule_id=30", history.Rule.PreviousVersion)
	}
	if history.Rule.NextVersion == nil || history.Rule.NextVersion.RuleID != 32 {
		t.Fatalf("next_version = %+v, want rule_id=32", history.Rule.NextVersion)
	}
	if history.Rule.VersionChainSummary == nil || history.Rule.VersionChainSummary.TotalVersions != 3 {
		t.Fatalf("version_chain_summary = %+v", history.Rule.VersionChainSummary)
	}
	if history.Rule.SupersessionDepth != 1 {
		t.Fatalf("supersession_depth = %d, want 1", history.Rule.SupersessionDepth)
	}
	if history.CurrentRule == nil || history.CurrentRule.RuleID != 32 || history.CurrentRule.RuleVersion != 3 {
		t.Fatalf("current_rule = %+v, want rule_id=32 version=3", history.CurrentRule)
	}
}

type noopTxRunner struct{}

func (noopTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(noopTx{})
}

type noopTx struct{}

func (noopTx) IsTx() {}

type categoryRepoStub struct {
	byID   map[int64]*domain.Category
	byCode map[string]*domain.Category
	nextID int64
}

func newCategoryRepoStub() *categoryRepoStub {
	return &categoryRepoStub{
		byID:   map[int64]*domain.Category{},
		byCode: map[string]*domain.Category{},
		nextID: 1,
	}
}

func (r *categoryRepoStub) mustCreate(category *domain.Category) {
	copyCategory := *category
	r.byID[category.CategoryID] = &copyCategory
	r.byCode[category.CategoryCode] = &copyCategory
	if category.CategoryID >= r.nextID {
		r.nextID = category.CategoryID + 1
	}
}

func (r *categoryRepoStub) GetByID(_ context.Context, id int64) (*domain.Category, error) {
	item, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *categoryRepoStub) GetByCode(_ context.Context, code string) (*domain.Category, error) {
	item, ok := r.byCode[code]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *categoryRepoStub) List(_ context.Context, _ repo.CategoryListFilter) ([]*domain.Category, int64, error) {
	return nil, 0, nil
}

func (r *categoryRepoStub) Search(_ context.Context, filter repo.CategorySearchFilter) ([]*domain.Category, error) {
	keyword := strings.TrimSpace(filter.Keyword)
	activeOnly := filter.IsActive != nil && *filter.IsActive
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items := make([]*domain.Category, 0)
	for _, item := range r.byID {
		if item == nil {
			continue
		}
		if activeOnly && !item.IsActive {
			continue
		}
		if keyword != "" &&
			!strings.Contains(item.CategoryCode, keyword) &&
			!strings.Contains(item.CategoryName, keyword) &&
			!strings.Contains(item.DisplayName, keyword) {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *categoryRepoStub) Create(_ context.Context, _ repo.Tx, category *domain.Category) (int64, error) {
	copyCategory := *category
	copyCategory.CategoryID = r.nextID
	r.nextID++
	r.byID[copyCategory.CategoryID] = &copyCategory
	r.byCode[copyCategory.CategoryCode] = &copyCategory
	return copyCategory.CategoryID, nil
}

func (r *categoryRepoStub) Update(_ context.Context, _ repo.Tx, category *domain.Category) error {
	copyCategory := *category
	r.byID[copyCategory.CategoryID] = &copyCategory
	r.byCode[copyCategory.CategoryCode] = &copyCategory
	return nil
}

type costRuleRepoStub struct {
	rules  []*domain.CostRule
	nextID int64
}

func newCostRuleRepoStub() *costRuleRepoStub {
	return &costRuleRepoStub{nextID: 1}
}

func (r *costRuleRepoStub) GetByID(_ context.Context, id int64) (*domain.CostRule, error) {
	for _, rule := range r.rules {
		if rule.RuleID == id {
			return r.copyRuleWithDerivedLineage(rule), nil
		}
	}
	return nil, nil
}

func (r *costRuleRepoStub) List(_ context.Context, _ repo.CostRuleListFilter) ([]*domain.CostRule, int64, error) {
	items := make([]*domain.CostRule, 0, len(r.rules))
	for _, rule := range r.rules {
		items = append(items, r.copyRuleWithDerivedLineage(rule))
	}
	return items, int64(len(items)), nil
}

func (r *costRuleRepoStub) ListActiveByCategory(_ context.Context, categoryID *int64, categoryCode string, asOf time.Time) ([]*domain.CostRule, error) {
	items := make([]*domain.CostRule, 0, len(r.rules))
	for _, rule := range r.rules {
		if !rule.IsActive {
			continue
		}
		if rule.CategoryCode != categoryCode && (categoryID == nil || rule.CategoryID == nil || *rule.CategoryID != *categoryID) {
			continue
		}
		if rule.EffectiveFrom != nil && rule.EffectiveFrom.After(asOf) {
			continue
		}
		if rule.EffectiveTo != nil && rule.EffectiveTo.Before(asOf) {
			continue
		}
		items = append(items, r.copyRuleWithDerivedLineage(rule))
	}
	return items, nil
}

func (r *costRuleRepoStub) Create(_ context.Context, _ repo.Tx, rule *domain.CostRule) (int64, error) {
	copyRule := *rule
	copyRule.RuleID = r.nextID
	r.nextID++
	r.rules = append(r.rules, &copyRule)
	return copyRule.RuleID, nil
}

func (r *costRuleRepoStub) Update(_ context.Context, _ repo.Tx, rule *domain.CostRule) error {
	for i, current := range r.rules {
		if current.RuleID == rule.RuleID {
			copyRule := *rule
			r.rules[i] = &copyRule
			return nil
		}
	}
	copyRule := *rule
	r.rules = append(r.rules, &copyRule)
	return nil
}

func (r *costRuleRepoStub) copyRuleWithDerivedLineage(rule *domain.CostRule) *domain.CostRule {
	if rule == nil {
		return nil
	}
	copyRule := *rule
	copyRule.SupersededByRuleID = nil
	for _, candidate := range r.rules {
		if candidate == nil || candidate.SupersedesRuleID == nil {
			continue
		}
		if *candidate.SupersedesRuleID == rule.RuleID {
			id := candidate.RuleID
			copyRule.SupersededByRuleID = &id
			break
		}
	}
	return &copyRule
}

func costRuleFloat64Ptr(v float64) *float64 {
	return &v
}
