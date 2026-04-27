package handler

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	taskdraftsvc "workflow/service/task_draft"
)

type TaskDraftHandler struct {
	svc *taskdraftsvc.Service
}

func NewTaskDraftHandler(svc *taskdraftsvc.Service) *TaskDraftHandler {
	return &TaskDraftHandler{svc: svc}
}

func (h *TaskDraftHandler) CreateOrUpdate(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil || !json.Valid(raw) {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid draft payload", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	draft, appErr := h.svc.CreateOrUpdate(c.Request.Context(), actor, raw)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, draft)
}

func (h *TaskDraftHandler) MyList(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, next, appErr := h.svc.List(c.Request.Context(), actor, taskdraftsvc.ListDraftFilter{
		TaskType: c.Query("task_type"),
		Limit:    limit,
		Cursor:   c.Query("cursor"),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": items, "next_cursor": next})
}

func (h *TaskDraftHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("draft_id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid draft id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	draft, appErr := h.svc.Get(c.Request.Context(), actor, id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, draft)
}

func (h *TaskDraftHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("draft_id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid draft id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.Delete(c.Request.Context(), actor, id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}
