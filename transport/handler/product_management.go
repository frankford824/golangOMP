package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

type ProductManagementHandler struct {
	svc service.ProductManagementService
}

func NewProductManagementHandler(svc service.ProductManagementService) *ProductManagementHandler {
	return &ProductManagementHandler{svc: svc}
}

func (h *ProductManagementHandler) List(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "product management service is not configured", nil))
		return
	}
	filter, appErr := parseProductManagementListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	items, meta, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

func (h *ProductManagementHandler) ListByTaskID(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "product management service is not configured", nil))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	items, appErr := h.svc.GetByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *ProductManagementHandler) ListImageCandidates(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, appErr := h.svc.ListImageCandidates(c.Request.Context(), actor, recordID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *ProductManagementHandler) ReparseImage(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.ReparseImage(c.Request.Context(), actor, recordID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

type setProductManagementImageReq struct {
	AssetID int64 `json:"asset_id"`
}

func (h *ProductManagementHandler) SetManualImage(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	var req setProductManagementImageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid image payload", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.SetManualImage(c.Request.Context(), actor, recordID, req.AssetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

type requestProductManagementSyncReq struct {
	Force bool `json:"force"`
}

func (h *ProductManagementHandler) RequestSync(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	var req requestProductManagementSyncReq
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.RequestSync(c.Request.Context(), actor, recordID, req.Force)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

func parseProductManagementListFilter(c *gin.Context) (repo.ProductManagementListFilter, *domain.AppError) {
	filter := repo.ProductManagementListFilter{
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		ImageSource: strings.TrimSpace(c.Query("image_source")),
		SyncStatus:  strings.TrimSpace(c.Query("sync_status")),
		CostStatus:  strings.TrimSpace(c.Query("cost_status")),
		IssueScope:  strings.TrimSpace(c.DefaultQuery("issue_scope", "attention")),
		Page:        1,
		PageSize:    20,
	}
	if filter.Keyword == "" {
		filter.Keyword = strings.TrimSpace(c.Query("q"))
	}
	if raw := strings.TrimSpace(c.Query("creator_id")); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "creator_id must be an integer", nil)
		}
		filter.CreatorID = &id
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = pageSize
	}
	return filter, nil
}

func productManagementRecordID(c *gin.Context) (int64, bool) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid product management record id", nil))
		return 0, false
	}
	return id, true
}
