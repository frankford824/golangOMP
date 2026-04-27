package ws

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"

	"workflow/domain"
	svcws "workflow/service/websocket"
)

type RequestActorResolver interface {
	ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError)
}

type Handler struct {
	resolver RequestActorResolver
	hub      *svcws.Hub
	upgrader gws.Upgrader
}

func NewHandler(resolver RequestActorResolver, hub *svcws.Hub) *Handler {
	return &Handler{
		resolver: resolver,
		hub:      hub,
		upgrader: gws.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func (h *Handler) Upgrade(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		token = strings.TrimSpace(c.Query("access_token"))
	}
	if token == "" || h.resolver == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIErrorResponse{Error: domain.ErrUnauthorized})
		return
	}
	actor, appErr := h.resolver.ResolveRequestActor(c.Request.Context(), token)
	if appErr != nil || actor == nil || actor.ID <= 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIErrorResponse{Error: domain.ErrUnauthorized})
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := svcws.NewConnection(h.hub, conn, actor.ID, actor.Team)
	h.hub.Register(client)
	go client.WritePump()
	go client.ReadPump()
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) < 7 || !strings.EqualFold(header[:6], "Bearer") {
		return ""
	}
	return strings.TrimSpace(header[6:])
}
