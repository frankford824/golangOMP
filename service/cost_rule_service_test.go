package service

import (
	"context"
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

func (r *categoryRepoStub) Search(_ context.Context, _ repo.CategorySearchFilter) ([]*domain.Category, error) {
	return nil, nil
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
