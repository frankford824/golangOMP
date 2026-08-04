package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type PlanningSKUHandler struct{ svc service.PlanningSKUService }

func NewPlanningSKUHandler(svc service.PlanningSKUService) *PlanningSKUHandler {
	return &PlanningSKUHandler{svc: svc}
}

func (h *PlanningSKUHandler) Template(c *gin.Context) {
	includeERP, _ := strconv.ParseBool(c.Query("erp"))
	content, appErr := h.svc.Template(c.Request.Context(), includeERP)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	filename := "策划SKU导入模板.xlsx"
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h *PlanningSKUHandler) ParseExcel(c *gin.Context) {
	includeERP, _ := strconv.ParseBool(c.PostForm("erp"))
	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required", nil))
		return
	}
	reader, err := file.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file cannot be opened", nil))
		return
	}
	defer reader.Close()
	result, appErr := h.svc.ParseExcel(c.Request.Context(), reader, includeERP)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *PlanningSKUHandler) Update(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	itemID, err := strconv.ParseInt(strings.TrimSpace(c.Param("item_id")), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid planning SKU item id", nil))
		return
	}
	var request domain.UpdatePlanningSKURequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Update(c.Request.Context(), requestActor(c), taskID, itemID, request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *PlanningSKUHandler) GetResult(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	result, appErr := h.svc.GetResult(c.Request.Context(), requestActor(c), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *PlanningSKUHandler) ExportTask(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	h.export(c, domain.PlanningSKUExportRequest{TaskIDs: []int64{taskID}})
}

func (h *PlanningSKUHandler) ExportSelection(c *gin.Context) {
	var request domain.PlanningSKUExportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	h.export(c, request)
}

func (h *PlanningSKUHandler) ERPRetry(c *gin.Context) { h.erpAction(c, false) }

func (h *PlanningSKUHandler) ERPResync(c *gin.Context) { h.erpAction(c, true) }

func (h *PlanningSKUHandler) erpAction(c *gin.Context, resync bool) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	count, appErr := h.svc.RequestERP(c.Request.Context(), requestActor(c), taskID, resync)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"queued": count, "resync": resync})
}

func (h *PlanningSKUHandler) export(c *gin.Context, request domain.PlanningSKUExportRequest) {
	content, filename, appErr := h.svc.Export(c.Request.Context(), requestActor(c), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}
