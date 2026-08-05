package handler

import (
	"strconv"
	"strings"

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
		FileFormat:     strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Query("file_format")), ".")),
		TaskType:       domain.TaskType(strings.TrimSpace(c.Query("task_type"))),
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
	if raw := c.Query("resource_owner_id"); raw != "" {
		ownerID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || ownerID <= 0 {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource_owner_id must be a positive integer", nil))
			return
		}
		params.ResourceOwnerID = &ownerID
	}
	if params.FileFormat != "" && !validResourceFileFormat(params.FileFormat) {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_format must be an extension such as jpg, psd, or tif", nil))
		return
	}
	var appErr *domain.AppError
	params.ResourceCreatedFrom, appErr = parseTaskCreatedDateBoundary(c.Query("created_from"), false)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	params.ResourceCreatedTo, appErr = parseTaskCreatedDateBoundary(c.Query("created_to"), true)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if params.ResourceCreatedFrom != nil && params.ResourceCreatedTo != nil && params.ResourceCreatedFrom.After(*params.ResourceCreatedTo) {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "created_from must not be after created_to", nil))
		return
	}
	if params.TaskType != "" && !params.TaskType.Valid() {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_type is invalid", nil))
		return
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

func validResourceFileFormat(value string) bool {
	if len(value) > 16 {
		return false
	}
	for _, ch := range value {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return value != ""
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

func (h *TaskResourceWorkflowHandler) ResourceGroupRevisions(c *gin.Context) {
	groupID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid resource group id", nil))
		return
	}
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, pageSizeErr := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageErr != nil || pageSizeErr != nil || page <= 0 || pageSize <= 0 || pageSize > 200 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be positive and page_size must be between 1 and 200", map[string]interface{}{"page_size_limit": 200}))
		return
	}
	result, appErr := h.svc.ResourceGroupRevisions(c.Request.Context(), requestActor(c), groupID, page, pageSize)
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
