package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ERPCostAPIHandler struct{ svc service.ERPCostAPIService }

func NewERPCostAPIHandler(svc service.ERPCostAPIService) *ERPCostAPIHandler {
	return &ERPCostAPIHandler{svc: svc}
}

func (h *ERPCostAPIHandler) Feed(c *gin.Context) {
	limit, appErr := parseERPCostLimit(c.Query("limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.Feed(c.Request.Context(), c.Query("updated_since"), c.Query("cursor"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(http.StatusOK, result)
}

type erpCostBatchQueryRequest struct {
	SKUIDs []string `json:"sku_ids"`
}

func (h *ERPCostAPIHandler) BatchQuery(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	var request erpCostBatchQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost batch query payload", nil))
		return
	}
	result, appErr := h.svc.BatchQuery(c.Request.Context(), request.SKUIDs)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ERPCostAPIHandler) History(c *gin.Context) {
	wmsCoIDs, appErr := parseERPCostWMSCoIDs(c.Query("wms_co_ids"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.History(
		c.Request.Context(),
		splitERPCostCSV(c.Query("sku_ids")),
		c.Query("as_of"),
		wmsCoIDs,
	)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ERPCostAPIHandler) Changes(c *gin.Context) {
	limit, appErr := parseERPCostLimit(c.Query("limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.Changes(c.Request.Context(), c.Query("since"), c.Query("cursor"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(http.StatusOK, result)
}

func parseERPCostLimit(raw string) (int, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must be a positive integer", nil)
	}
	if value > 5000 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must not exceed 5000", nil)
	}
	return value, nil
}

func parseERPCostWMSCoIDs(raw string) ([]int64, *domain.AppError) {
	parts := splitERPCostCSV(raw)
	if len(parts) == 0 {
		return nil, nil
	}
	if len(parts) > 100 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "wms_co_ids must contain at most 100 values", nil)
	}
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil || value <= 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "wms_co_ids must contain positive integers", nil)
		}
		values = append(values, value)
	}
	return values, nil
}

func splitERPCostCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}
