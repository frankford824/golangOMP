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
	params := domain.ResourceGroupListParams{
		TaskID: taskID, SKUCode: c.Query("sku_code"), TaskNo: c.Query("task_no"), Query: c.Query("q"),
		ResourceRole:   domain.ResourceRoleFilter(c.Query("resource_role")),
		FormatCategory: domain.AssetFormatCategoryFilter(c.Query("format_category")),
		BusinessLane:   domain.TaskBusinessLane(c.Query("business_lane")),
		Page:           page, PageSize: pageSize,
	}
	if !params.ResourceRole.Valid() {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource_role must be reference, source, or final", nil))
		return
	}
	if raw := c.Query("creator_id"); raw != "" {
		creatorID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || creatorID <= 0 {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "creator_id must be a positive integer", nil))
			return
		}
		params.CreatorID = &creatorID
	}
	if params.BusinessLane != "" && !params.BusinessLane.Valid() {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_lane must be normal or customization", nil))
		return
	}
	result, appErr := h.svc.ListResourceGroups(c.Request.Context(), requestActor(c), params)
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
