package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"workflow/domain"
	"workflow/service"
)

type TaskAssignmentHandler struct {
	svc service.TaskAssignmentService
}

func NewTaskAssignmentHandler(svc service.TaskAssignmentService) *TaskAssignmentHandler {
	return &TaskAssignmentHandler{svc: svc}
}

type assignTaskReq struct {
	DesignerID     *int64 `json:"designer_id"`
	AssignedBy     *int64 `json:"assigned_by"`
	Remark         string `json:"remark"`
	BatchRequestID string `json:"batch_request_id"`
}

func (h *TaskAssignmentHandler) Assign(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req assignTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if req.DesignerID != nil && *req.DesignerID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid_designer_id", map[string]interface{}{"deny_code": "invalid_designer_id"}))
		return
	}

	assignedBy, appErr := actorIDOrRequestValue(c, req.AssignedBy, "assigned_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	task, appErr := h.svc.Assign(c.Request.Context(), service.AssignTaskParams{
		TaskID:         taskID,
		DesignerID:     req.DesignerID,
		AssignedBy:     assignedBy,
		Remark:         req.Remark,
		BatchRequestID: strings.TrimSpace(req.BatchRequestID),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, task)
}

type batchAssignTaskReq struct {
	TaskIDs        []int64 `json:"task_ids" binding:"required"`
	DesignerID     int64   `json:"designer_id" binding:"required"`
	AssignedBy     *int64  `json:"assigned_by"`
	Remark         string  `json:"remark"`
	BatchRequestID string  `json:"batch_request_id"`
}

type batchRemindTaskReq struct {
	TaskIDs        []int64 `json:"task_ids" binding:"required"`
	ActorID        *int64  `json:"actor_id"`
	Reason         string  `json:"reason" binding:"required"`
	RemindChannel  string  `json:"remind_channel"`
	BatchRequestID string  `json:"batch_request_id"`
}

func (h *TaskAssignmentHandler) BatchAssign(c *gin.Context) {
	var req batchAssignTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	assignedBy, appErr := actorIDOrRequestValue(c, req.AssignedBy, "assigned_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	batchRequestID := strings.TrimSpace(req.BatchRequestID)
	if batchRequestID == "" {
		batchRequestID = uuid.NewString()
	}
	result, appErr := h.svc.BatchAssign(c.Request.Context(), service.BatchAssignTasksParams{
		TaskIDs:        req.TaskIDs,
		DesignerID:     req.DesignerID,
		AssignedBy:     assignedBy,
		Remark:         req.Remark,
		BatchRequestID: batchRequestID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskAssignmentHandler) BatchRemind(c *gin.Context) {
	var req batchRemindTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actorID, appErr := actorIDOrRequestValue(c, req.ActorID, "actor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	batchRequestID := strings.TrimSpace(req.BatchRequestID)
	if batchRequestID == "" {
		batchRequestID = uuid.NewString()
	}
	result, appErr := h.svc.BatchRemind(c.Request.Context(), service.BatchRemindTasksParams{
		TaskIDs:        req.TaskIDs,
		ActorID:        actorID,
		Reason:         req.Reason,
		RemindChannel:  req.RemindChannel,
		BatchRequestID: batchRequestID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
