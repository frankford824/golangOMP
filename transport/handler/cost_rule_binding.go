package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type CostRuleBindingHandler struct {
	svc      service.CostRuleBindingService
	costSync *service.CostSyncService
}

func (h *CostRuleBindingHandler) SetCostSyncService(s *service.CostSyncService) { h.costSync = s }
func (h *CostRuleBindingHandler) ListCostSyncStates(c *gin.Context) {
	if h.costSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "成本同步服务未启用", nil))
		return
	}
	f := parseCostRuleBindingFilter(c)
	rows, meta, err := h.costSync.List(c.Request.Context(), strings.TrimSpace(c.Query("status")), f.Keyword, f.Page, f.PageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	coverage, appErr := h.costSync.Coverage(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, domain.CostSyncListResult{Data: rows, Pagination: meta, Coverage: coverage})
}
func (h *CostRuleBindingHandler) ResolveCostSync(c *gin.Context) {
	if h.costSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "成本同步服务未启用", nil))
		return
	}
	var p domain.CostSyncResolution
	if err := c.ShouldBindJSON(&p); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请填写确认内容", nil))
		return
	}
	p.SKUCode = strings.TrimSpace(c.Param("sku"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	p.ActorID = actor.ID
	if err := h.costSync.Resolve(c.Request.Context(), p); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, domain.CostSyncDecisionResult{Accepted: true, SKUCode: p.SKUCode})
}

func NewCostRuleBindingHandler(svc service.CostRuleBindingService) *CostRuleBindingHandler {
	return &CostRuleBindingHandler{svc: svc}
}

func (h *CostRuleBindingHandler) List(c *gin.Context) {
	items, meta, appErr := h.svc.List(c.Request.Context(), parseCostRuleBindingFilter(c))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

func (h *CostRuleBindingHandler) ListBoundSKUs(c *gin.Context) {
	items, meta, appErr := h.svc.ListBoundSKUs(c.Request.Context(), parseCostRuleBindingFilter(c))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

type createCostRuleBindingReq struct {
	IIDRaw      string `json:"i_id_raw"`
	RuleGroup   string `json:"rule_group"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
	IsActive    *bool  `json:"is_active"`
}

func (h *CostRuleBindingHandler) Create(c *gin.Context) {
	var req createCostRuleBindingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule binding payload", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.svc.Create(c.Request.Context(), service.CreateCostRuleBindingParams{
		IIDRaw:      req.IIDRaw,
		RuleGroup:   req.RuleGroup,
		DisplayName: req.DisplayName,
		Source:      req.Source,
		IsActive:    req.IsActive,
		ActorID:     actor.ID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

type patchCostRuleBindingReq struct {
	IIDRaw      *string `json:"i_id_raw"`
	RuleGroup   *string `json:"rule_group"`
	DisplayName *string `json:"display_name"`
	Source      *string `json:"source"`
	IsActive    *bool   `json:"is_active"`
}

func (h *CostRuleBindingHandler) Patch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule binding id", nil))
		return
	}
	var req patchCostRuleBindingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost rule binding payload", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.svc.Patch(c.Request.Context(), service.PatchCostRuleBindingParams{
		ID:          id,
		IIDRaw:      req.IIDRaw,
		RuleGroup:   req.RuleGroup,
		DisplayName: req.DisplayName,
		Source:      req.Source,
		IsActive:    req.IsActive,
		ActorID:     actor.ID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CostRuleBindingHandler) ListUnboundCandidates(c *gin.Context) {
	filter := service.UnboundCostRuleCandidateFilter{
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Page:     1,
		PageSize: 20,
	}
	if filter.Keyword == "" {
		filter.Keyword = strings.TrimSpace(c.Query("q"))
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must be an integer", nil))
			return
		}
		filter.Limit = limit
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil))
			return
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil))
			return
		}
		filter.PageSize = pageSize
	}
	items, meta, appErr := h.svc.ListUnboundCandidates(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

func parseCostRuleBindingFilter(c *gin.Context) service.CostRuleBindingFilter {
	filter := service.CostRuleBindingFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		RuleGroup: strings.TrimSpace(c.Query("rule_group")),
		Page:      1,
		PageSize:  20,
	}
	if filter.Keyword == "" {
		filter.Keyword = strings.TrimSpace(c.Query("q"))
	}
	if raw := strings.TrimSpace(c.Query("is_active")); raw != "" {
		active := raw == "1" || strings.EqualFold(raw, "true")
		filter.IsActive = &active
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if page, err := parseInt(raw); err == nil {
			filter.Page = page
		}
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if pageSize, err := parseInt(raw); err == nil {
			filter.PageSize = pageSize
		}
	}
	return filter
}
