package handler

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	notificationsvc "workflow/service/notification"
)

type NotificationHandler struct {
	svc *notificationsvc.Service
}

type broadcastNotificationReq struct {
	Audience string  `json:"audience"`
	UserIDs  []int64 `json:"user_ids"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
}

type deleteWebPushSubscriptionReq struct {
	Endpoint string `json:"endpoint"`
}

func NewNotificationHandler(svc *notificationsvc.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) MyList(c *gin.Context) {
	filter, appErr := notificationListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, next, appErr := h.svc.List(c.Request.Context(), actor, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": items, "next_cursor": next})
}

func (h *NotificationHandler) AssetWorkbenchList(c *gin.Context) {
	filter, appErr := notificationListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, next, appErr := h.svc.ListAssetWorkbench(c.Request.Context(), actor, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": items, "next_cursor": next})
}

func notificationListFilter(c *gin.Context) (notificationsvc.ListFilter, *domain.AppError) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	var isRead *bool
	if raw := c.Query("is_read"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return notificationsvc.ListFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "is_read must be boolean", nil)
		}
		isRead = &value
	}
	return notificationsvc.ListFilter{IsRead: isRead, Limit: limit, Cursor: c.Query("cursor")}, nil
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	h.markRead(c, false)
}

func (h *NotificationHandler) MarkAssetWorkbenchRead(c *gin.Context) {
	h.markRead(c, true)
}

func (h *NotificationHandler) markRead(c *gin.Context, assetWorkbench bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid notification id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var appErr *domain.AppError
	if assetWorkbench {
		appErr = h.svc.MarkAssetWorkbenchRead(c.Request.Context(), actor, id)
	} else {
		appErr = h.svc.MarkRead(c.Request.Context(), actor, id)
	}
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	h.markAllRead(c, false)
}

func (h *NotificationHandler) MarkAllAssetWorkbenchRead(c *gin.Context) {
	h.markAllRead(c, true)
}

func (h *NotificationHandler) markAllRead(c *gin.Context, assetWorkbench bool) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var appErr *domain.AppError
	if assetWorkbench {
		appErr = h.svc.MarkAllAssetWorkbenchRead(c.Request.Context(), actor)
	} else {
		appErr = h.svc.MarkAllRead(c.Request.Context(), actor)
	}
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	h.unreadCount(c, false)
}

func (h *NotificationHandler) AssetWorkbenchUnreadCount(c *gin.Context) {
	h.unreadCount(c, true)
}

func (h *NotificationHandler) unreadCount(c *gin.Context, assetWorkbench bool) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var count int
	var appErr *domain.AppError
	if assetWorkbench {
		count, appErr = h.svc.AssetWorkbenchUnreadCount(c.Request.Context(), actor)
	} else {
		count, appErr = h.svc.UnreadCount(c.Request.Context(), actor)
	}
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"unread_count": count})
}

func (h *NotificationHandler) WebPushConfig(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var result *notificationsvc.WebPushConfigView
	result, appErr := h.svc.WebPushConfig(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *NotificationHandler) RegisterWebPushSubscription(c *gin.Context) {
	var req notificationsvc.WebPushSubscriptionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var result *notificationsvc.WebPushSubscriptionView
	result, appErr := h.svc.RegisterWebPushSubscription(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *NotificationHandler) DeleteCurrentWebPushSubscription(c *gin.Context) {
	var req deleteWebPushSubscriptionReq
	if c.Request.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 4096))
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
				return
			}
		}
	}
	if endpoint := strings.TrimSpace(c.Query("endpoint")); endpoint != "" && req.Endpoint == "" {
		req.Endpoint = endpoint
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.DeleteCurrentWebPushSubscription(c.Request.Context(), actor, req.Endpoint); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(204)
}

func (h *NotificationHandler) SendWebPushTest(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var result *notificationsvc.WebPushTestResult
	result, appErr := h.svc.SendWebPushTest(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var result *notificationsvc.NotificationPreferencesView
	result, appErr := h.svc.GetPreferences(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *NotificationHandler) PatchPreferences(c *gin.Context) {
	var req notificationsvc.NotificationPreferencesPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	var result *notificationsvc.NotificationPreferencesView
	result, appErr := h.svc.PatchPreferences(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *NotificationHandler) Broadcast(c *gin.Context) {
	var req broadcastNotificationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.svc.Broadcast(c.Request.Context(), actor, notificationsvc.BroadcastParams{
		Audience: notificationsvc.BroadcastAudience(req.Audience),
		UserIDs:  req.UserIDs,
		Title:    req.Title,
		Content:  req.Content,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
