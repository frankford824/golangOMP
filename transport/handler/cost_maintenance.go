package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

func (h *ProductManagementHandler) SetCostMaintenanceService(s service.CostMaintenanceService) {
	h.maintenance = s
}
func maintenanceFilter(c *gin.Context) repo.CostMaintenanceFilter {
	before, _ := strconv.ParseInt(c.Query("before_id"), 10, 64)
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("page_size"))
	return repo.CostMaintenanceFilter{Keyword: strings.TrimSpace(c.Query("keyword")), Status: strings.TrimSpace(c.Query("status")), ReviewState: strings.TrimSpace(c.Query("review_state")), Reason: strings.TrimSpace(c.Query("reason_code")), BeforeID: before, Page: page, PageSize: size}
}
func (h *ProductManagementHandler) ListMaintenance(c *gin.Context) {
	if h.maintenance == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	out, err := h.maintenance.ListCostMaintenance(c.Request.Context(), maintenanceFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *ProductManagementHandler) MaintainCost(c *gin.Context) {
	if h.maintenance == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "无效编码记录", nil))
		return
	}
	var raw json.RawMessage
	var p domain.CostMaintenanceRequest
	if err = c.ShouldBindJSON(&raw); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请核对维护内容", nil))
		return
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(raw, &p) != nil || json.Unmarshal(raw, &fields) != nil || fields["erp_cost"] == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "缺少ERP核对基线，请刷新后重试", nil))
		return
	}
	out, appErr := h.maintenance.MaintainCost(c.Request.Context(), id, p)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, out)
}
func (h *ProductManagementHandler) MaintenanceLogs(c *gin.Context) {
	if h.maintenance == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	rows, meta, err := h.maintenance.CostMaintenanceLogs(c.Request.Context(), maintenanceFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOKWithPagination(c, rows, meta)
}
