package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type CategoryERPMappingHandler struct {
	svc service.CategoryERPMappingService
}

func NewCategoryERPMappingHandler(svc service.CategoryERPMappingService) *CategoryERPMappingHandler {
	return &CategoryERPMappingHandler{svc: svc}
}

type createCategoryERPMappingReq struct {
	CategoryID              *int64 `json:"category_id"`
	CategoryCode            string `json:"category_code"`
	SearchEntryCode         string `json:"search_entry_code"`
	ERPMatchType            string `json:"erp_match_type" binding:"required"`
	ERPMatchValue           string `json:"erp_match_value" binding:"required"`
	SecondaryConditionKey   string `json:"secondary_condition_key"`
	SecondaryConditionValue string `json:"secondary_condition_value"`
	TertiaryConditionKey    string `json:"tertiary_condition_key"`
	TertiaryConditionValue  string `json:"tertiary_condition_value"`
	IsPrimary               *bool  `json:"is_primary"`
	IsActive                *bool  `json:"is_active"`
	Priority                int    `json:"priority"`
	Source                  string `json:"source"`
	Remark                  string `json:"remark"`
}

type patchCategoryERPMappingReq struct {
	CategoryID              *int64  `json:"category_id"`
	CategoryCode            *string `json:"category_code"`
	SearchEntryCode         *string `json:"search_entry_code"`
	ERPMatchType            *string `json:"erp_match_type"`
	ERPMatchValue           *string `json:"erp_match_value"`
	SecondaryConditionKey   *string `json:"secondary_condition_key"`
	SecondaryConditionValue *string `json:"secondary_condition_value"`
	TertiaryConditionKey    *string `json:"tertiary_condition_key"`
	TertiaryConditionValue  *string `json:"tertiary_condition_value"`
	IsPrimary               *bool   `json:"is_primary"`
	IsActive                *bool   `json:"is_active"`
	Priority                *int    `json:"priority"`
	Source                  *string `json:"source"`
	Remark                  *string `json:"remark"`
}

func (h *CategoryERPMappingHandler) List(c *gin.Context) {
	filter, appErr := parseCategoryERPMappingFilterQuery(c)
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

func (h *CategoryERPMappingHandler) Search(c *gin.Context) {
	filter, appErr := parseCategoryERPMappingSearchQuery(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	items, appErr := h.svc.Search(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *CategoryERPMappingHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid category mapping id", nil))
		return
	}
	item, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CategoryERPMappingHandler) Create(c *gin.Context) {
	var req createCategoryERPMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.Create(c.Request.Context(), service.CreateCategoryERPMappingParams{
		CategoryID:              req.CategoryID,
		CategoryCode:            req.CategoryCode,
		SearchEntryCode:         req.SearchEntryCode,
		ERPMatchType:            domain.CategoryERPMatchType(req.ERPMatchType),
		ERPMatchValue:           req.ERPMatchValue,
		SecondaryConditionKey:   req.SecondaryConditionKey,
		SecondaryConditionValue: req.SecondaryConditionValue,
		TertiaryConditionKey:    req.TertiaryConditionKey,
		TertiaryConditionValue:  req.TertiaryConditionValue,
		IsPrimary:               req.IsPrimary,
		IsActive:                req.IsActive,
		Priority:                req.Priority,
		Source:                  req.Source,
		Remark:                  req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *CategoryERPMappingHandler) Patch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid category mapping id", nil))
		return
	}
	var req patchCategoryERPMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	var matchType *domain.CategoryERPMatchType
	if req.ERPMatchType != nil {
		value := domain.CategoryERPMatchType(*req.ERPMatchType)
		matchType = &value
	}

	item, appErr := h.svc.Patch(c.Request.Context(), service.PatchCategoryERPMappingParams{
		MappingID:               id,
		CategoryID:              req.CategoryID,
		CategoryCode:            req.CategoryCode,
		SearchEntryCode:         req.SearchEntryCode,
		ERPMatchType:            matchType,
		ERPMatchValue:           req.ERPMatchValue,
		SecondaryConditionKey:   req.SecondaryConditionKey,
		SecondaryConditionValue: req.SecondaryConditionValue,
		TertiaryConditionKey:    req.TertiaryConditionKey,
		TertiaryConditionValue:  req.TertiaryConditionValue,
		IsPrimary:               req.IsPrimary,
		IsActive:                req.IsActive,
		Priority:                req.Priority,
		Source:                  req.Source,
		Remark:                  req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}
