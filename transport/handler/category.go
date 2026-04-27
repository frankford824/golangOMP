package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type CategoryHandler struct {
	svc service.CategoryService
}

func NewCategoryHandler(svc service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

type createCategoryReq struct {
	CategoryCode    string `json:"category_code" binding:"required"`
	CategoryName    string `json:"category_name" binding:"required"`
	DisplayName     string `json:"display_name"`
	ParentID        *int64 `json:"parent_id"`
	Level           int    `json:"level"`
	SearchEntryCode string `json:"search_entry_code"`
	IsSearchEntry   *bool  `json:"is_search_entry"`
	CategoryType    string `json:"category_type" binding:"required"`
	IsActive        *bool  `json:"is_active"`
	SortOrder       int    `json:"sort_order"`
	Source          string `json:"source"`
	Remark          string `json:"remark"`
}

type patchCategoryReq struct {
	CategoryCode    *string `json:"category_code"`
	CategoryName    *string `json:"category_name"`
	DisplayName     *string `json:"display_name"`
	ParentID        *int64  `json:"parent_id"`
	Level           *int    `json:"level"`
	SearchEntryCode *string `json:"search_entry_code"`
	IsSearchEntry   *bool   `json:"is_search_entry"`
	CategoryType    *string `json:"category_type"`
	IsActive        *bool   `json:"is_active"`
	SortOrder       *int    `json:"sort_order"`
	Source          *string `json:"source"`
	Remark          *string `json:"remark"`
}

func (h *CategoryHandler) List(c *gin.Context) {
	filter, appErr := parseCategoryFilterQuery(c)
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

func (h *CategoryHandler) Search(c *gin.Context) {
	filter, appErr := parseCategorySearchQuery(c)
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

func (h *CategoryHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid category id", nil))
		return
	}
	item, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	var req createCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.Create(c.Request.Context(), service.CreateCategoryParams{
		CategoryCode:    req.CategoryCode,
		CategoryName:    req.CategoryName,
		DisplayName:     req.DisplayName,
		ParentID:        req.ParentID,
		Level:           req.Level,
		SearchEntryCode: req.SearchEntryCode,
		IsSearchEntry:   req.IsSearchEntry,
		CategoryType:    domain.CategoryType(req.CategoryType),
		IsActive:        req.IsActive,
		SortOrder:       req.SortOrder,
		Source:          req.Source,
		Remark:          req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *CategoryHandler) Patch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid category id", nil))
		return
	}
	var req patchCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	var categoryType *domain.CategoryType
	if req.CategoryType != nil {
		value := domain.CategoryType(*req.CategoryType)
		categoryType = &value
	}

	item, appErr := h.svc.Patch(c.Request.Context(), service.PatchCategoryParams{
		CategoryID:      id,
		CategoryCode:    req.CategoryCode,
		CategoryName:    req.CategoryName,
		DisplayName:     req.DisplayName,
		ParentID:        req.ParentID,
		Level:           req.Level,
		SearchEntryCode: req.SearchEntryCode,
		IsSearchEntry:   req.IsSearchEntry,
		CategoryType:    categoryType,
		IsActive:        req.IsActive,
		SortOrder:       req.SortOrder,
		Source:          req.Source,
		Remark:          req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}
