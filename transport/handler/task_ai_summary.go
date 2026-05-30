package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service/aiagent"
)

type TaskAISummaryService interface {
	Generate(ctx context.Context, taskID int64) (*aiagent.TaskSummary, *domain.AppError)
}

type TaskAISummaryHandler struct {
	service TaskAISummaryService
}

func NewTaskAISummaryHandler(service TaskAISummaryService) *TaskAISummaryHandler {
	return &TaskAISummaryHandler{service: service}
}

func (h *TaskAISummaryHandler) Generate(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	if h == nil || h.service == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task AI summary handler is not configured", nil))
		return
	}
	summary, appErr := h.service.Generate(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, summary)
}
