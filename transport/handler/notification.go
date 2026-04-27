package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	notificationsvc "workflow/service/notification"
)

type NotificationHandler struct {
	svc *notificationsvc.Service
}

func NewNotificationHandler(svc *notificationsvc.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) MyList(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	var isRead *bool
	if raw := c.Query("is_read"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_read must be boolean", nil))
			return
		}
		isRead = &value
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, next, appErr := h.svc.List(c.Request.Context(), actor, notificationsvc.ListFilter{IsRead: isRead, Limit: limit, Cursor: c.Query("cursor")})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": items, "next_cursor": next})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid notification id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.MarkRead(c.Request.Context(), actor, id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.MarkAllRead(c.Request.Context(), actor); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	count, appErr := h.svc.UnreadCount(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"unread_count": count})
}
