package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type CostRuleFilter struct {
	CategoryID    *int64
	CategoryCode  string
	ProductFamily string
	RuleType      *domain.CostRuleType
	IsActive      *bool
	Page          int
	PageSize      int
}

type CreateCostRuleParams struct {
	RuleName              string
	RuleVersion           *int
	CategoryID            *int64
	CategoryCode          string
	ProductFamily         string
	RuleType              domain.CostRuleType
	BasePrice             *float64
	TaxMultiplier         *float64
	MinArea               *float64
	AreaThreshold         *float64
	SurchargeAmount       *float64
	SpecialProcessKeyword string
	SpecialProcessPrice   *float64
	FormulaExpression     string
	Priority              int
	IsActive              *bool
	EffectiveFrom         *time.Time
	EffectiveTo           *time.Time
	SupersedesRuleID      *int64
	GovernanceNote        string
	Source                string
	Remark                string
}

type PatchCostRuleParams struct {
	RuleID                int64
	RuleName              *string
	RuleVersion           *int
	CategoryID            *int64
	CategoryCode          *string
	ProductFamily         *string
	RuleType              *domain.CostRuleType
	BasePrice             *float64
	TaxMultiplier         *float64
	MinArea               *float64
	AreaThreshold         *float64
	SurchargeAmount       *float64
	SpecialProcessKeyword *string
	SpecialProcessPrice   *float64
	FormulaExpression     *string
	Priority              *int
	IsActive              *bool
	EffectiveFrom         *time.Time
	EffectiveTo           *time.Time
	SupersedesRuleID      *int64
	GovernanceNote        *string
	Source                *string
	Remark                *string
}

type CostRuleService interface {
	List(ctx context.Context, filter CostRuleFilter) ([]*domain.CostRule, domain.PaginationMeta, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.CostRule, *domain.AppError)
	GetHistory(ctx context.Context, id int64) (*domain.CostRuleHistoryReadModel, *domain.AppError)
	Create(ctx context.Context, p CreateCostRuleParams) (*domain.CostRule, *domain.AppError)
	Patch(ctx context.Context, p PatchCostRuleParams) (*domain.CostRule, *domain.AppError)
	Preview(ctx context.Context, req domain.CostRulePreviewRequest) (*domain.CostRulePreviewResponse, *domain.AppError)
}

type costRuleService struct {
	costRuleRepo repo.CostRuleRepo
	categoryRepo repo.CategoryRepo
	txRunner     repo.TxRunner
}

func NewCostRuleService(costRuleRepo repo.CostRuleRepo, categoryRepo repo.CategoryRepo, txRunner repo.TxRunner) CostRuleService {
	return &costRuleService{costRuleRepo: costRuleRepo, categoryRepo: categoryRepo, txRunner: txRunner}
}

func (s *costRuleService) List(ctx context.Context, filter CostRuleFilter) ([]*domain.CostRule, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.costRuleRepo.List(ctx, repo.CostRuleListFilter{
		CategoryID:    filter.CategoryID,
		CategoryCode:  strings.TrimSpace(filter.CategoryCode),
		ProductFamily: strings.TrimSpace(filter.ProductFamily),
		RuleType:      filter.RuleType,
		IsActive:      filter.IsActive,
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list cost rules", err)
	}
	loader := newCostRuleLineageLoader(s.costRuleRepo, time.Now().UTC())
	for _, item := range items {
		if _, err := decorateCostRuleLineage(ctx, loader, item); err != nil {
			return nil, domain.PaginationMeta{}, infraError("decorate cost rule lineage", err)
		}
	}
	return items, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *costRuleService) GetByID(ctx context.Context, id int64) (*domain.CostRule, *domain.AppError) {
	rule, err := s.costRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get cost rule", err)
	}
	if rule == nil {
		return nil, domain.ErrNotFound
	}
	if _, err := decorateCostRuleLineage(ctx, newCostRuleLineageLoader(s.costRuleRepo, time.Now().UTC()), rule); err != nil {
		return nil, infraError("decorate cost rule detail lineage", err)
	}
	return rule, nil
}

func (s *costRuleService) GetHistory(ctx context.Context, id int64) (*domain.CostRuleHistoryReadModel, *domain.AppError) {
	rule, err := s.costRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get cost rule history", err)
	}
	if rule == nil {
		return nil, domain.ErrNotFound
	}
	history, err := decorateCostRuleLineage(ctx, newCostRuleLineageLoader(s.costRuleRepo, time.Now().UTC()), rule)
	if err != nil {
		return nil, infraError("decorate cost rule history lineage", err)
	}
	return history, nil
}

func (s *costRuleService) Create(ctx context.Context, p CreateCostRuleParams) (*domain.CostRule, *domain.AppError) {
	rule, appErr := s.buildCostRuleDraft(ctx, 0, p.RuleName, p.RuleVersion, p.CategoryID, p.CategoryCode, p.ProductFamily, p.RuleType, p.BasePrice, p.TaxMultiplier, p.MinArea, p.AreaThreshold, p.SurchargeAmount, p.SpecialProcessKeyword, p.SpecialProcessPrice, p.FormulaExpression, p.Priority, p.IsActive, p.EffectiveFrom, p.EffectiveTo, p.SupersedesRuleID, p.GovernanceNote, p.Source, p.Remark)
	if appErr != nil {
		return nil, appErr
	}

	var id int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdID, err := s.costRuleRepo.Create(ctx, tx, rule)
		if err != nil {
			return err
		}
		id = createdID
		return nil
	}); err != nil {
		return nil, infraError("create cost rule tx", err)
	}
	return s.GetByID(ctx, id)
}

func (s *costRuleService) Patch(ctx context.Context, p PatchCostRuleParams) (*domain.CostRule, *domain.AppError) {
	current, err := s.costRuleRepo.GetByID(ctx, p.RuleID)
	if err != nil {
		return nil, infraError("get cost rule for patch", err)
	}
	if current == nil {
		return nil, domain.ErrNotFound
	}

	ruleName := current.RuleName
	if p.RuleName != nil {
		ruleName = *p.RuleName
	}
	categoryID := current.CategoryID
	if p.CategoryID != nil {
		categoryID = p.CategoryID
	}
	categoryCode := current.CategoryCode
	if p.CategoryCode != nil {
		categoryCode = *p.CategoryCode
	}
	productFamily := current.ProductFamily
	if p.ProductFamily != nil {
		productFamily = *p.ProductFamily
	}
	ruleType := current.RuleType
	if p.RuleType != nil {
		ruleType = *p.RuleType
	}
	basePrice := current.BasePrice
	if p.BasePrice != nil {
		basePrice = p.BasePrice
	}
	taxMultiplier := current.TaxMultiplier
	if p.TaxMultiplier != nil {
		taxMultiplier = p.TaxMultiplier
	}
	minArea := current.MinArea
	if p.MinArea != nil {
		minArea = p.MinArea
	}
	areaThreshold := current.AreaThreshold
	if p.AreaThreshold != nil {
		areaThreshold = p.AreaThreshold
	}
	surchargeAmount := current.SurchargeAmount
	if p.SurchargeAmount != nil {
		surchargeAmount = p.SurchargeAmount
	}
	specialProcessKeyword := current.SpecialProcessKeyword
	if p.SpecialProcessKeyword != nil {
		specialProcessKeyword = *p.SpecialProcessKeyword
	}
	specialProcessPrice := current.SpecialProcessPrice
	if p.SpecialProcessPrice != nil {
		specialProcessPrice = p.SpecialProcessPrice
	}
	formulaExpression := current.FormulaExpression
	if p.FormulaExpression != nil {
		formulaExpression = *p.FormulaExpression
	}
	priority := current.Priority
	if p.Priority != nil {
		priority = *p.Priority
	}
	ruleVersion := current.RuleVersion
	if p.RuleVersion != nil {
		ruleVersion = *p.RuleVersion
	}
	isActive := current.IsActive
	if p.IsActive != nil {
		isActive = *p.IsActive
	}
	effectiveFrom := current.EffectiveFrom
	if p.EffectiveFrom != nil {
		effectiveFrom = p.EffectiveFrom
	}
	effectiveTo := current.EffectiveTo
	if p.EffectiveTo != nil {
		effectiveTo = p.EffectiveTo
	}
	supersedesRuleID := current.SupersedesRuleID
	if p.SupersedesRuleID != nil {
		supersedesRuleID = p.SupersedesRuleID
	}
	governanceNote := current.GovernanceNote
	if p.GovernanceNote != nil {
		governanceNote = *p.GovernanceNote
	}
	source := current.Source
	if p.Source != nil {
		source = *p.Source
	}
	remark := current.Remark
	if p.Remark != nil {
		remark = *p.Remark
	}

	rule, appErr := s.buildCostRuleDraft(ctx, p.RuleID, ruleName, &ruleVersion, categoryID, categoryCode, productFamily, ruleType, basePrice, taxMultiplier, minArea, areaThreshold, surchargeAmount, specialProcessKeyword, specialProcessPrice, formulaExpression, priority, &isActive, effectiveFrom, effectiveTo, supersedesRuleID, governanceNote, source, remark)
	if appErr != nil {
		return nil, appErr
	}
	rule.RuleID = p.RuleID

	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.costRuleRepo.Update(ctx, tx, rule)
	}); err != nil {
		return nil, infraError("patch cost rule tx", err)
	}
	return s.GetByID(ctx, p.RuleID)
}

func (s *costRuleService) Preview(ctx context.Context, req domain.CostRulePreviewRequest) (*domain.CostRulePreviewResponse, *domain.AppError) {
	category, categoryCode, appErr := s.resolveCategoryLink(ctx, req.CategoryID, req.CategoryCode)
	if appErr != nil {
		return nil, appErr
	}
	if categoryCode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id or category_code is required", nil)
	}

	rules, err := s.costRuleRepo.ListActiveByCategory(ctx, categoryIDValue(category), categoryCode, time.Now())
	if err != nil {
		return nil, infraError("list preview cost rules", err)
	}
	return previewCostRules(req, rules).Response, nil
}

func (s *costRuleService) buildCostRuleDraft(ctx context.Context, ruleID int64, ruleName string, ruleVersion *int, categoryID *int64, categoryCode, productFamily string, ruleType domain.CostRuleType, basePrice, taxMultiplier, minArea, areaThreshold, surchargeAmount *float64, specialProcessKeyword string, specialProcessPrice *float64, formulaExpression string, priority int, isActive *bool, effectiveFrom, effectiveTo *time.Time, supersedesRuleID *int64, governanceNote, source, remark string) (*domain.CostRule, *domain.AppError) {
	ruleName = strings.TrimSpace(ruleName)
	categoryCode = strings.ToUpper(strings.TrimSpace(categoryCode))
	productFamily = strings.TrimSpace(productFamily)
	specialProcessKeyword = strings.TrimSpace(specialProcessKeyword)
	formulaExpression = strings.TrimSpace(formulaExpression)
	governanceNote = strings.TrimSpace(governanceNote)
	source = strings.TrimSpace(source)
	remark = strings.TrimSpace(remark)

	if ruleName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_name is required", nil)
	}
	if !ruleType.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_type is required and must be supported", nil)
	}

	category, normalizedCode, appErr := s.resolveCategoryLink(ctx, categoryID, categoryCode)
	if appErr != nil {
		return nil, appErr
	}
	if normalizedCode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id or category_code is required", nil)
	}
	if effectiveFrom != nil && effectiveTo != nil && effectiveTo.Before(*effectiveFrom) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_to must be >= effective_from", nil)
	}
	if priority == 0 {
		priority = 100
	}
	resolvedRuleVersion := 0
	if ruleVersion != nil {
		resolvedRuleVersion = *ruleVersion
		if resolvedRuleVersion < 1 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_version must be >= 1", nil)
		}
	}
	if supersedesRuleID != nil {
		if ruleID != 0 && *supersedesRuleID == ruleID {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supersedes_rule_id cannot point to the same rule", nil)
		}
		priorRule, err := s.costRuleRepo.GetByID(ctx, *supersedesRuleID)
		if err != nil {
			return nil, infraError("get superseded cost rule", err)
		}
		if priorRule == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supersedes_rule_id does not exist", nil)
		}
		if normalizedCode != "" && priorRule.CategoryCode != "" && priorRule.CategoryCode != normalizedCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supersedes_rule_id must stay within the same category_code", nil)
		}
		if resolvedRuleVersion == 0 {
			resolvedRuleVersion = priorRule.RuleVersion + 1
		}
		if resolvedRuleVersion <= priorRule.RuleVersion {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_version must be greater than the superseded rule version", nil)
		}
	}
	if resolvedRuleVersion <= 0 {
		resolvedRuleVersion = 1
	}

	active := true
	if isActive != nil {
		active = *isActive
	}
	if source == "" {
		source = "admin_manual"
	}
	if productFamily == "" && category != nil {
		productFamily = string(category.CategoryType)
	}
	if ruleType == domain.CostRuleTypeManualQuote {
		formulaExpression = ""
	}

	return &domain.CostRule{
		RuleID:                ruleID,
		RuleName:              ruleName,
		RuleVersion:           resolvedRuleVersion,
		CategoryID:            categoryIDValue(category),
		CategoryCode:          normalizedCode,
		ProductFamily:         productFamily,
		RuleType:              ruleType,
		BasePrice:             basePrice,
		TaxMultiplier:         taxMultiplier,
		MinArea:               minArea,
		AreaThreshold:         areaThreshold,
		SurchargeAmount:       surchargeAmount,
		SpecialProcessKeyword: specialProcessKeyword,
		SpecialProcessPrice:   specialProcessPrice,
		FormulaExpression:     formulaExpression,
		Priority:              priority,
		IsActive:              active,
		EffectiveFrom:         effectiveFrom,
		EffectiveTo:           effectiveTo,
		SupersedesRuleID:      supersedesRuleID,
		GovernanceNote:        governanceNote,
		Source:                source,
		Remark:                remark,
	}, nil
}

func (s *costRuleService) resolveCategoryLink(ctx context.Context, categoryID *int64, categoryCode string) (*domain.Category, string, *domain.AppError) {
	categoryCode = strings.ToUpper(strings.TrimSpace(categoryCode))
	if categoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *categoryID)
		if err != nil {
			return nil, "", infraError("get category for cost rule", err)
		}
		if category == nil {
			return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id does not exist", nil)
		}
		if categoryCode != "" && category.CategoryCode != categoryCode {
			return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id and category_code do not refer to the same category", nil)
		}
		return category, category.CategoryCode, nil
	}
	if categoryCode == "" {
		return nil, "", nil
	}
	category, err := s.categoryRepo.GetByCode(ctx, categoryCode)
	if err != nil {
		return nil, "", infraError("get category by code for cost rule", err)
	}
	if category == nil {
		return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code does not exist", nil)
	}
	return category, category.CategoryCode, nil
}

func applyFixedUnitPrice(rule *domain.CostRule, area float64, quantity int64) (*float64, bool) {
	if rule.BasePrice == nil {
		return nil, false
	}
	effectiveQuantity := float64(quantity)
	if effectiveQuantity <= 0 {
		effectiveQuantity = 1
	}
	if area <= 0 {
		return nil, false
	}
	total := (*rule.BasePrice) * area * effectiveQuantity
	if rule.TaxMultiplier != nil && *rule.TaxMultiplier > 0 {
		total = total * (*rule.TaxMultiplier)
	}
	return &total, true
}

func applySizeBasedFormula(rule *domain.CostRule, quantity int64, process, notes string) (float64, string, bool) {
	expr := strings.TrimSpace(rule.FormulaExpression)
	if strings.HasPrefix(expr, "print_side:") {
		side := detectPrintSide(process, notes)
		prices := parsePrintSideFormula(expr)
		price, ok := prices[side]
		if !ok {
			if fallback, ok := prices["single"]; ok {
				price = fallback
				ok = true
			}
		}
		if !ok {
			return 0, "", false
		}
		total := price * float64(quantity)
		return total, fmt.Sprintf("%s applied %s print-side price %.2f", rule.RuleName, side, total), true
	}
	return 0, "", false
}

func parsePrintSideFormula(expr string) map[string]float64 {
	out := map[string]float64{}
	parts := strings.Split(strings.TrimPrefix(expr, "print_side:"), ",")
	for _, part := range parts {
		pieces := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pieces) != 2 {
			continue
		}
		var value float64
		if _, err := fmt.Sscanf(strings.TrimSpace(pieces[1]), "%f", &value); err != nil {
			continue
		}
		out[strings.TrimSpace(pieces[0])] = value
	}
	return out
}

func detectPrintSide(process, notes string) string {
	combined := strings.ToLower(strings.TrimSpace(process + " " + notes))
	if strings.Contains(combined, "双面") || strings.Contains(combined, "double") {
		return "double"
	}
	return "single"
}

func containsProcessKeyword(process, notes, keyword string) bool {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return false
	}
	combined := strings.ToLower(process + " " + notes)
	return strings.Contains(combined, strings.ToLower(keyword))
}

func previewArea(req domain.CostRulePreviewRequest) float64 {
	if req.Area != nil && *req.Area > 0 {
		return *req.Area
	}
	if req.Width != nil && req.Height != nil && *req.Width > 0 && *req.Height > 0 {
		return *req.Width * *req.Height
	}
	return 0
}

func previewQuantity(quantity *int64) int64 {
	if quantity == nil || *quantity <= 0 {
		return 1
	}
	return *quantity
}

func categoryIDValue(category *domain.Category) *int64 {
	if category == nil {
		return nil
	}
	return &category.CategoryID
}

func hydrateCostRuleGovernanceStatuses(rules []*domain.CostRule, asOf time.Time) {
	for _, rule := range rules {
		hydrateCostRuleGovernanceStatus(rule, asOf)
	}
}

func hydrateCostRuleGovernanceStatus(rule *domain.CostRule, asOf time.Time) {
	if rule == nil {
		return
	}
	rule.GovernanceStatus = rule.GovernanceStatusAt(asOf)
}
