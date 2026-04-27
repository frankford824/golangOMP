package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
	"workflow/service/task_aggregator"
)

type TaskDetailHandler struct {
	svc   service.TaskDetailAggregateService
	r3Svc *task_aggregator.DetailService
}

func NewTaskDetailHandler(svc service.TaskDetailAggregateService) *TaskDetailHandler {
	return &TaskDetailHandler{svc: svc}
}

func (h *TaskDetailHandler) SetR3DetailService(svc *task_aggregator.DetailService) {
	h.r3Svc = svc
}

// GetByTaskID handles GET /v1/tasks/:id/detail
func (h *TaskDetailHandler) GetByTaskID(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	if h.r3Svc != nil {
		aggregate, err := h.r3Svc.Get(c.Request.Context(), taskID)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
			return
		}
		if aggregate == nil {
			respondError(c, domain.ErrNotFound)
			return
		}
		respondOK(c, aggregate)
		return
	}
	aggregate, appErr := h.svc.GetByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, aggregate)
}
