package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type CostRuleHandler struct {
	svc service.CostRuleService
}

func NewCostRuleHandler(svc service.CostRuleService) *CostRuleHandler {
	return &CostRuleHandler{svc: svc}
}

type createCostRuleReq struct {
	RuleName              string   `json:"rule_name" binding:"required"`
	RuleVersion           *int     `json:"rule_version"`
	CategoryID            *int64   `json:"category_id"`
	CategoryCode          string   `json:"category_code"`
	ProductFamily         string   `json:"product_family"`
	RuleType              string   `json:"rule_type" binding:"required"`
	BasePrice             *float64 `json:"base_price"`
	TaxMultiplier         *float64 `json:"tax_multiplier"`
	MinArea               *float64 `json:"min_area"`
	AreaThreshold         *float64 `json:"area_threshold"`
	SurchargeAmount       *float64 `json:"surcharge_amount"`
	SpecialProcessKeyword string   `json:"special_process_keyword"`
	SpecialProcessPrice   *float64 `json:"special_process_price"`
	FormulaExpression     string   `json:"formula_expression"`
	Priority              int      `json:"priority"`
	IsActive              *bool    `json:"is_active"`
	EffectiveFrom         *string  `json:"effective_from"`
	EffectiveTo           *string  `json:"effective_to"`
	SupersedesRuleID      *int64   `json:"supersedes_rule_id"`
	GovernanceNote        string   `json:"governance_note"`
	Source                string   `json:"source"`
	Remark                string   `json:"remark"`
}

type patchCostRuleReq struct {
	RuleName              *string  `json:"rule_name"`
	RuleVersion           *int     `json:"rule_version"`
	CategoryID            *int64   `json:"category_id"`
	CategoryCode          *string  `json:"category_code"`
	ProductFamily         *string  `json:"product_family"`
	RuleType              *string  `json:"rule_type"`
	BasePrice             *float64 `json:"base_price"`
	TaxMultiplier         *float64 `json:"tax_multiplier"`
	MinArea               *float64 `json:"min_area"`
	AreaThreshold         *float64 `json:"area_threshold"`
	SurchargeAmount       *float64 `json:"surcharge_amount"`
	SpecialProcessKeyword *string  `json:"special_process_keyword"`
	SpecialProcessPrice   *float64 `json:"special_process_price"`
	FormulaExpression     *string  `json:"formula_expression"`
	Priority              *int     `json:"priority"`
	IsActive              *bool    `json:"is_active"`
	EffectiveFrom         *string  `json:"effective_from"`
	EffectiveTo           *string  `json:"effective_to"`
	SupersedesRuleID      *int64   `json:"supersedes_rule_id"`
	GovernanceNote        *string  `json:"governance_note"`
	Source                *string  `json:"source"`
	Remark                *string  `json:"remark"`
}

func (h *CostRuleHandler) List(c *gin.Context) {
	filter, appErr := parseCostRuleFilterQuery(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	items, pagination, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, pagination)
}

func (h *CostRuleHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule id", nil))
		return
	}
	item, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CostRuleHandler) GetHistory(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule id", nil))
		return
	}
	item, appErr := h.svc.GetHistory(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CostRuleHandler) Create(c *gin.Context) {
	var req createCostRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	effectiveFrom, appErr := parseRFC3339Pointer(req.EffectiveFrom, "effective_from")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	effectiveTo, appErr := parseRFC3339Pointer(req.EffectiveTo, "effective_to")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	item, appErr := h.svc.Create(c.Request.Context(), service.CreateCostRuleParams{
		RuleName:              req.RuleName,
		RuleVersion:           req.RuleVersion,
		CategoryID:            req.CategoryID,
		CategoryCode:          req.CategoryCode,
		ProductFamily:         req.ProductFamily,
		RuleType:              domain.CostRuleType(req.RuleType),
		BasePrice:             req.BasePrice,
		TaxMultiplier:         req.TaxMultiplier,
		MinArea:               req.MinArea,
		AreaThreshold:         req.AreaThreshold,
		SurchargeAmount:       req.SurchargeAmount,
		SpecialProcessKeyword: req.SpecialProcessKeyword,
		SpecialProcessPrice:   req.SpecialProcessPrice,
		FormulaExpression:     req.FormulaExpression,
		Priority:              req.Priority,
		IsActive:              req.IsActive,
		EffectiveFrom:         effectiveFrom,
		EffectiveTo:           effectiveTo,
		SupersedesRuleID:      req.SupersedesRuleID,
		GovernanceNote:        req.GovernanceNote,
		Source:                req.Source,
		Remark:                req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *CostRuleHandler) Patch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule id", nil))
		return
	}
	var req patchCostRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	effectiveFrom, appErr := parseRFC3339Pointer(req.EffectiveFrom, "effective_from")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	effectiveTo, appErr := parseRFC3339Pointer(req.EffectiveTo, "effective_to")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	var ruleType *domain.CostRuleType
	if req.RuleType != nil {
		value := domain.CostRuleType(*req.RuleType)
		ruleType = &value
	}

	item, appErr := h.svc.Patch(c.Request.Context(), service.PatchCostRuleParams{
		RuleID:                id,
		RuleName:              req.RuleName,
		RuleVersion:           req.RuleVersion,
		CategoryID:            req.CategoryID,
		CategoryCode:          req.CategoryCode,
		ProductFamily:         req.ProductFamily,
		RuleType:              ruleType,
		BasePrice:             req.BasePrice,
		TaxMultiplier:         req.TaxMultiplier,
		MinArea:               req.MinArea,
		AreaThreshold:         req.AreaThreshold,
		SurchargeAmount:       req.SurchargeAmount,
		SpecialProcessKeyword: req.SpecialProcessKeyword,
		SpecialProcessPrice:   req.SpecialProcessPrice,
		FormulaExpression:     req.FormulaExpression,
		Priority:              req.Priority,
		IsActive:              req.IsActive,
		EffectiveFrom:         effectiveFrom,
		EffectiveTo:           effectiveTo,
		SupersedesRuleID:      req.SupersedesRuleID,
		GovernanceNote:        req.GovernanceNote,
		Source:                req.Source,
		Remark:                req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CostRuleHandler) Preview(c *gin.Context) {
	var req domain.CostRulePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Preview(c.Request.Context(), req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseRFC3339Pointer(raw *string, field string) (*time.Time, *domain.AppError) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be RFC3339", nil)
	}
	return &t, nil
}
