package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskResourceWorkflowHandler struct {
	svc service.TaskResourceWorkflowService
}

func (h *TaskResourceWorkflowHandler) ListResourceGroups(c *gin.Context) {
	taskID, _ := strconv.ParseInt(c.Query("task_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	result, appErr := h.svc.ListResourceGroups(c.Request.Context(), requestActor(c), domain.ResourceGroupListParams{
		TaskID: taskID, SKUCode: c.Query("sku_code"), Query: c.Query("q"),
		FormatCategory: domain.AssetFormatCategoryFilter(c.Query("format_category")), BusinessLane: domain.TaskBusinessLane(c.Query("business_lane")),
		Page: page, PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskResourceWorkflowHandler) ResourceGroup(c *gin.Context) {
	groupID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid resource group id", nil))
		return
	}
	result, appErr := h.svc.ResourceGroup(c.Request.Context(), requestActor(c), groupID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskResourceWorkflowHandler) BatchDownloadResourceGroups(c *gin.Context) {
	var request domain.ResourceGroupBatchDownloadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.BatchDownloadResourceGroups(c.Request.Context(), requestActor(c), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func NewTaskResourceWorkflowHandler(svc service.TaskResourceWorkflowService) *TaskResourceWorkflowHandler {
	return &TaskResourceWorkflowHandler{svc: svc}
}

func (h *TaskResourceWorkflowHandler) ResourceBundle(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	result, appErr := h.svc.ResourceBundle(c.Request.Context(), taskID, requestActor(c))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskResourceWorkflowHandler) SubmitDesign(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var request domain.SubmitDesignV2Request
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SubmitDesign(c.Request.Context(), taskID, requestActor(c), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskResourceWorkflowHandler) AuditDecision(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var request domain.AuditDecisionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.AuditDecision(c.Request.Context(), taskID, requestActor(c), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskResourceWorkflowHandler) Reopen(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var request domain.ReopenTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Reopen(c.Request.Context(), taskID, requestActor(c), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
