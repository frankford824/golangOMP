package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ERPBridgeHandler struct {
	svc service.ERPBridgeService
}

func NewERPBridgeHandler(svc service.ERPBridgeService) *ERPBridgeHandler {
	return &ERPBridgeHandler{svc: svc}
}

func (h *ERPBridgeHandler) SearchProducts(c *gin.Context) {
	filter, appErr := parseERPBridgeSearchFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	items, appErr := h.svc.SearchProducts(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{
		"data":               items.Items,
		"pagination":         items.Pagination,
		"normalized_filters": items.NormalizedFilters,
	})
}

func (h *ERPBridgeHandler) ListIIDs(c *gin.Context) {
	filter, appErr := parseERPIIDListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.ListIIDs(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{
		"data":               result.Items,
		"pagination":         result.Pagination,
		"normalized_filters": result.NormalizedFilters,
	})
}

func (h *ERPBridgeHandler) GetProductByID(c *gin.Context) {
	id := strings.TrimSpace(strings.TrimPrefix(c.Param("id"), "/"))
	if id == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid erp product id", nil))
		return
	}
	product, appErr := h.svc.GetProductByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, product)
}

func parseERPIIDListFilter(c *gin.Context) (domain.ERPIIDListFilter, *domain.AppError) {
	filter := domain.ERPIIDListFilter{
		Q:        strings.TrimSpace(c.Query("q")),
		Page:     1,
		PageSize: 50,
	}
	if filter.Q == "" {
		filter.Q = strings.TrimSpace(c.Query("keyword"))
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		n, err := parseInt(raw)
		if err != nil {
			return domain.ERPIIDListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = n
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		n, err := parseInt(raw)
		if err != nil {
			return domain.ERPIIDListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = n
	}
	return filter, nil
}

func (h *ERPBridgeHandler) ListCategories(c *gin.Context) {
	items, appErr := h.svc.ListCategories(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *ERPBridgeHandler) ListWarehouses(c *gin.Context) {
	items, appErr := h.svc.ListWarehouses(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *ERPBridgeHandler) UpsertProduct(c *gin.Context) {
	var payload domain.ERPProductUpsertPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid upsert payload", nil))
		return
	}
	if strings.TrimSpace(payload.SKUID) == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_id is required", nil))
		return
	}
	result, appErr := h.svc.UpsertProduct(c.Request.Context(), payload)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) UpdateItemStyle(c *gin.Context) {
	var payload domain.ERPItemStyleUpdatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid item style update payload", nil))
		return
	}
	if strings.TrimSpace(payload.IID) == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "i_id is required", nil))
		return
	}
	result, appErr := h.svc.UpdateItemStyle(c.Request.Context(), payload)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) ListSyncLogs(c *gin.Context) {
	filter, appErr := parseERPBridgeSyncLogFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.ListSyncLogs(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result.Items, result.Pagination)
}

func (h *ERPBridgeHandler) GetSyncLogByID(c *gin.Context) {
	id := strings.TrimSpace(strings.TrimPrefix(c.Param("id"), "/"))
	if id == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid erp sync log id", nil))
		return
	}
	result, appErr := h.svc.GetSyncLogByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) ShelveProductsBatch(c *gin.Context) {
	var payload domain.ERPProductBatchMutationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid shelve batch payload", nil))
		return
	}
	result, appErr := h.svc.ShelveProductsBatch(c.Request.Context(), payload)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) UnshelveProductsBatch(c *gin.Context) {
	var payload domain.ERPProductBatchMutationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid unshelve batch payload", nil))
		return
	}
	result, appErr := h.svc.UnshelveProductsBatch(c.Request.Context(), payload)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) UpdateVirtualInventory(c *gin.Context) {
	var payload domain.ERPVirtualInventoryUpdatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid virtual inventory payload", nil))
		return
	}
	result, appErr := h.svc.UpdateVirtualInventory(c.Request.Context(), payload)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *ERPBridgeHandler) ListJSTUsers(c *gin.Context) {
	filter, appErr := parseJSTUserListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.ListJSTUsers(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseJSTUserListFilter(c *gin.Context) (domain.JSTUserListFilter, *domain.AppError) {
	filter := domain.JSTUserListFilter{
		CurrentPage: 1,
		PageSize:    50,
		PageAction:  0,
		Version:     2,
	}
	if raw := strings.TrimSpace(c.Query("current_page")); raw != "" {
		n, err := parseInt(raw)
		if err != nil {
			return domain.JSTUserListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "current_page must be an integer", nil)
		}
		filter.CurrentPage = n
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		n, err := parseInt(raw)
		if err != nil {
			return domain.JSTUserListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = n
	}
	if raw := strings.TrimSpace(c.Query("page_action")); raw != "" {
		n, err := parseInt(raw)
		if err != nil {
			return domain.JSTUserListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_action must be an integer", nil)
		}
		filter.PageAction = n
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		b := strings.EqualFold(raw, "true") || raw == "1"
		filter.Enabled = &b
	}
	if raw := strings.TrimSpace(c.Query("version")); raw != "" {
		n, err := parseInt(raw)
		if err == nil {
			filter.Version = n
		}
	}
	filter.LoginID = strings.TrimSpace(c.Query("loginId"))
	filter.CreatedBegin = strings.TrimSpace(c.Query("creatd_begin"))
	filter.CreatedEnd = strings.TrimSpace(c.Query("creatd_end"))
	return filter, nil
}

func parseERPBridgeSearchFilter(c *gin.Context) (domain.ERPProductSearchFilter, *domain.AppError) {
	filter := domain.ERPProductSearchFilter{
		Q:            strings.TrimSpace(c.Query("q")),
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		SKUCode:      strings.TrimSpace(firstNonEmptyQueryValue(c, "sku_code", "sku")),
		CategoryID:   strings.TrimSpace(c.Query("category_id")),
		CategoryName: strings.TrimSpace(firstNonEmptyQueryValue(c, "category_name", "category")),
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			return domain.ERPProductSearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			return domain.ERPProductSearchFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = pageSize
	}
	return filter, nil
}

func parseERPBridgeSyncLogFilter(c *gin.Context) (domain.ERPSyncLogFilter, *domain.AppError) {
	filter := domain.ERPSyncLogFilter{
		Status:       strings.TrimSpace(c.Query("status")),
		Connector:    strings.TrimSpace(c.Query("connector")),
		Operation:    strings.TrimSpace(c.Query("operation")),
		ResourceType: strings.TrimSpace(c.Query("resource_type")),
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			return domain.ERPSyncLogFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			return domain.ERPSyncLogFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = pageSize
	}
	return filter, nil
}

func firstNonEmptyQueryValue(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}
