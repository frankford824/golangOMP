package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service/aichat"
	analyticssvc "workflow/service/analytics"
)

type AIChatHandler struct {
	service   *aichat.Service
	analytics *analyticssvc.Service
	heartbeat time.Duration
}

func NewAIChatHandler(service *aichat.Service, heartbeat time.Duration, analyticsServices ...*analyticssvc.Service) *AIChatHandler {
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}
	var analytics *analyticssvc.Service
	if len(analyticsServices) > 0 {
		analytics = analyticsServices[0]
	}
	return &AIChatHandler{service: service, analytics: analytics, heartbeat: heartbeat}
}

func (h *AIChatHandler) Config(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.service.Config(actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *AIChatHandler) ListConversations(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.service.ListConversations(c.Request.Context(), actor, positiveQueryInt(c, "page", 1), positiveQueryInt(c, "page_size", 20))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *AIChatHandler) CreateConversation(c *gin.Context) {
	var request domain.AICreateConversationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请求格式无效", err))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.service.CreateConversation(c.Request.Context(), actor, request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *AIChatHandler) GetConversation(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.service.GetConversation(c.Request.Context(), actor, c.Param("conversation_id"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *AIChatHandler) DeleteConversation(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.service.DeleteConversation(c.Request.Context(), actor, c.Param("conversation_id")); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AIChatHandler) AdminListConversations(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	filter := domain.AIAdminConversationFilter{
		Status: strings.TrimSpace(c.Query("status")), Page: positiveQueryInt(c, "page", 1), PageSize: positiveQueryInt(c, "page_size", 20),
	}
	if raw := strings.TrimSpace(c.Query("owner_user_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			filter.OwnerUserID = &value
		}
	}
	filter.From = optionalDate(c.Query("from"))
	filter.To = optionalDate(c.Query("to"))
	item, appErr := h.service.AdminListConversations(c.Request.Context(), actor, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *AIChatHandler) AdminGetConversation(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.service.AdminGetConversation(c.Request.Context(), actor, c.Param("conversation_id"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *AIChatHandler) StreamMessage(c *gin.Context) {
	var request domain.AIStreamMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请求格式无效", err))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	events := make(chan domain.AISSEEvent, 32)
	result := make(chan *domain.AppError, 1)
	go func() {
		result <- h.service.StreamMessage(c.Request.Context(), actor, c.Param("conversation_id"), request, func(event domain.AISSEEvent) error {
			select {
			case events <- event:
				return nil
			case <-c.Request.Context().Done():
				return c.Request.Context().Err()
			}
		})
	}()
	ticker := time.NewTicker(h.heartbeat)
	defer ticker.Stop()
	started := false
	start := func() {
		if started {
			return
		}
		started = true
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-store")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
	}
	writeEvent := func(event domain.AISSEEvent) bool {
		start()
		raw, err := json.Marshal(event.Data)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, raw); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	for {
		select {
		case event := <-events:
			if !writeEvent(event) {
				return
			}
		case appErr := <-result:
			if appErr != nil {
				if !started {
					respondError(c, appErr)
					return
				}
				_ = writeEvent(domain.AISSEEvent{Type: "error", Data: map[string]any{"code": appErr.Code, "message": appErr.Message}})
			}
			return
		case <-ticker.C:
			start()
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			// The provider receives the same cancellation signal, but StreamMessage
			// persists the cancelled assistant message with a detached, bounded
			// context before reporting completion. Drain that result so the handler
			// does not return while the persistence transaction is still running.
			select {
			case <-result:
			case <-time.After(6 * time.Second):
			}
			return
		}
	}
}

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func optionalDate(value string) *time.Time {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &parsed
}
