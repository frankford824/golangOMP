package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service/task_aggregator"
)

type TaskDetailHandler struct {
	r3Svc *task_aggregator.DetailService
}

func NewTaskDetailHandler(r3Svc *task_aggregator.DetailService) *TaskDetailHandler {
	return &TaskDetailHandler{r3Svc: r3Svc}
}

// GetByTaskID handles GET /v1/tasks/:id/detail
// 返回 V1.1-A1 fast-path 5 段 schema(task / task_detail / modules / events / reference_file_refs).
func (h *TaskDetailHandler) GetByTaskID(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	if h.r3Svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task detail aggregate service not configured", nil))
		return
	}
	aggregate, err := h.r3Svc.Get(c.Request.Context(), taskID)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
		return
	}
	if aggregate == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	var detail *task_aggregator.Detail = aggregate
	respondOK(c, detail)
}
