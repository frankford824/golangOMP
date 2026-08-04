package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/service"
)

type TaskBoardHandler struct {
	svc service.TaskBoardService
}

func NewTaskBoardHandler(svc service.TaskBoardService) *TaskBoardHandler {
	return &TaskBoardHandler{svc: svc}
}

func (h *TaskBoardHandler) OperationalOverview(c *gin.Context) {
	result, appErr := h.svc.GetOperationalOverview(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
