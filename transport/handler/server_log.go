package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ServerLogHandler struct {
	svc service.ServerLogService
}

func NewServerLogHandler(svc service.ServerLogService) *ServerLogHandler {
	return &ServerLogHandler{svc: svc}
}

func (h *ServerLogHandler) List(c *gin.Context) {
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	filter := service.ServerLogFilter{
		Level:    c.Query("level"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.Since = &t
		}
	}
	if s := c.Query("until"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.Until = &t
		}
	}
	logs, pagination, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, logs, pagination)
}

type cleanServerLogsReq struct {
	OlderThanHours int    `json:"older_than_hours"`
	Reason         string `json:"reason" binding:"required"`
}

func (h *ServerLogHandler) Clean(c *gin.Context) {
	var req cleanServerLogsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actorID := int64(0)
	if actor, ok := domain.RequestActorFromContext(c.Request.Context()); ok && actor.ID > 0 {
		actorID = actor.ID
	}
	deleted, appErr := h.svc.Clean(c.Request.Context(), req.OlderThanHours, req.Reason, actorID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"deleted": deleted})
}

func (h *ServerLogHandler) RecordHTTPError(c *gin.Context, status int, path, method, traceID, clientIP string) {
	details := map[string]interface{}{
		"method":    method,
		"path":      path,
		"status":    status,
		"trace_id":  traceID,
		"client_ip": clientIP,
	}
	_, _ = h.svc.Record(c.Request.Context(), "error", "http_request_5xx", details)
}
