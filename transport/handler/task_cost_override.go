package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskCostOverrideHandler struct {
	svc service.TaskCostOverrideAuditService
}

func NewTaskCostOverrideHandler(svc service.TaskCostOverrideAuditService) *TaskCostOverrideHandler {
	return &TaskCostOverrideHandler{svc: svc}
}

func (h *TaskCostOverrideHandler) ListByTaskID(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	timeline, appErr := h.svc.ListByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, timeline)
}
