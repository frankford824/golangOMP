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

func costRecalculationRunID(c *gin.Context) (int64, bool) {
	id, err := parseInt64(strings.TrimSpace(c.Param("run_id")))
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost recalculation run id", nil))
		return 0, false
	}
	return id, true
}
