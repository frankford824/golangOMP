package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskBoardHandler struct {
	svc service.TaskBoardService
}

func NewTaskBoardHandler(svc service.TaskBoardService) *TaskBoardHandler {
	return &TaskBoardHandler{svc: svc}
}

func (h *TaskBoardHandler) Summary(c *gin.Context) {
	filter, appErr := parseTaskBoardFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	result, appErr := h.svc.GetSummary(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskBoardHandler) Queues(c *gin.Context) {
	filter, appErr := parseTaskBoardFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	result, appErr := h.svc.GetQueues(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseTaskBoardFilter(c *gin.Context) (service.TaskBoardFilter, *domain.AppError) {
	taskFilter, appErr := parseTaskFilterQuery(c)
	if appErr != nil {
		return service.TaskBoardFilter{}, appErr
	}
	filter := service.TaskBoardFilter{
		BoardView:   domain.TaskBoardView(c.Query("board_view")),
		QueueKey:    c.Query("queue_key"),
		TaskFilter:  taskFilter,
		PreviewSize: 3,
	}
	if raw := c.Query("preview_size"); raw != "" {
		previewSize, err := parseInt(raw)
		if err != nil {
			return service.TaskBoardFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "preview_size must be an integer", nil)
		}
		filter.PreviewSize = previewSize
	}

	return filter, nil
}
