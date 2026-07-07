package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

type ProductManagementHandler struct {
	svc     service.ProductManagementService
	costRun service.CostRecalculationService
}

func NewProductManagementHandler(svc service.ProductManagementService, costRun ...service.CostRecalculationService) *ProductManagementHandler {
	h := &ProductManagementHandler{svc: svc}
	if len(costRun) > 0 {
		h.costRun = costRun[0]
	}
	return h
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

func (h *ProductManagementHandler) ListComboTree(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "product management service is not configured", nil))
		return
	}
	filter, appErr := parseProductManagementListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.ListComboTree(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, result)
}

func (h *ProductManagementHandler) CostDashboard(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "product management service is not configured", nil))
		return
	}
	result, appErr := h.svc.CostDashboard(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ProductManagementHandler) CreateCostRecalculationRun(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	var req domain.CreateCostRecalculationRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost recalculation payload", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.costRun.Create(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ProductManagementHandler) ListCostRecalculationRuns(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	filter, appErr := parseCostRecalculationRunFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	items, meta, appErr := h.costRun.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

func (h *ProductManagementHandler) GetCostRecalculationRun(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	runID, ok := costRecalculationRunID(c)
	if !ok {
		return
	}
	filter, appErr := parseCostRecalculationRunItemFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, meta, appErr := h.costRun.Get(c.Request.Context(), runID, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result, meta)
}

func (h *ProductManagementHandler) ApplyCostRecalculationRun(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	runID, ok := costRecalculationRunID(c)
	if !ok {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.costRun.Apply(c.Request.Context(), actor, runID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ProductManagementHandler) SyncERPCostRecalculationRun(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	runID, ok := costRecalculationRunID(c)
	if !ok {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.costRun.SyncERP(c.Request.Context(), actor, runID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ProductManagementHandler) CancelCostRecalculationRun(c *gin.Context) {
	if h == nil || h.costRun == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil))
		return
	}
	runID, ok := costRecalculationRunID(c)
	if !ok {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.costRun.Cancel(c.Request.Context(), actor, runID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
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
	req := parseProductManagementSyncRequest(c)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.RequestSync(c.Request.Context(), actor, recordID, req.Force)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

func (h *ProductManagementHandler) RequestBaseSync(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	req := parseProductManagementSyncRequest(c)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.RequestBaseSync(c.Request.Context(), actor, recordID, req.Force)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

func (h *ProductManagementHandler) RequestImageSync(c *gin.Context) {
	recordID, ok := productManagementRecordID(c)
	if !ok {
		return
	}
	req := parseProductManagementSyncRequest(c)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	record, appErr := h.svc.RequestImageSync(c.Request.Context(), actor, recordID, req.Force)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

func parseProductManagementSyncRequest(c *gin.Context) requestProductManagementSyncReq {
	var req requestProductManagementSyncReq
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	return req
}

func parseProductManagementListFilter(c *gin.Context) (repo.ProductManagementListFilter, *domain.AppError) {
	filter := repo.ProductManagementListFilter{
		Keyword:         strings.TrimSpace(c.Query("keyword")),
		DisplayScope:    strings.TrimSpace(c.Query("display_scope")),
		ImageSource:     strings.TrimSpace(c.Query("image_source")),
		SyncStatus:      strings.TrimSpace(c.Query("sync_status")),
		BaseSyncStatus:  strings.TrimSpace(c.Query("base_sync_status")),
		ImageSyncStatus: strings.TrimSpace(c.Query("image_sync_status")),
		CostStatus:      strings.TrimSpace(c.Query("cost_status")),
		IssueScope:      strings.TrimSpace(c.DefaultQuery("issue_scope", "attention")),
		Page:            1,
		PageSize:        20,
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

func parseCostRecalculationRunFilter(c *gin.Context) (repo.CostRecalculationRunFilter, *domain.AppError) {
	filter := repo.CostRecalculationRunFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		Mode:     strings.TrimSpace(c.Query("mode")),
		Page:     1,
		PageSize: 20,
	}
	if raw := strings.TrimSpace(c.Query("created_by")); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "created_by must be an integer", nil)
		}
		filter.CreatedBy = &id
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

func parseCostRecalculationRunItemFilter(c *gin.Context) (repo.CostRecalculationRunItemFilter, *domain.AppError) {
	filter := repo.CostRecalculationRunItemFilter{
		Status:   strings.TrimSpace(c.Query("item_status")),
		Page:     1,
		PageSize: 50,
	}
	if filter.Status == "" {
		filter.Status = strings.TrimSpace(c.Query("status"))
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

func costRecalculationRunID(c *gin.Context) (int64, bool) {
	id, err := parseInt64(strings.TrimSpace(c.Param("run_id")))
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost recalculation run id", nil))
		return 0, false
	}
	return id, true
}
