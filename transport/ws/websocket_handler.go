package ws

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"

	"workflow/domain"
	svcws "workflow/service/websocket"
	"workflow/transport/handler"
)

type RequestActorResolver interface {
	ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError)
}

type Handler struct {
	resolver RequestActorResolver
	hub      *svcws.Hub
	upgrader gws.Upgrader
}

// wsAllowedOrigins lists extra cross-origin sources allowed to open WebSocket
// connections (comma-separated full origins, e.g. "https://app.example.com").
// Same-host origins and non-browser clients (no Origin header) are always allowed.
var wsAllowedOrigins = parseAllowedOrigins(os.Getenv("WS_ALLOWED_ORIGINS"))

func parseAllowedOrigins(raw string) []string {
	var origins []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimRight(strings.TrimSpace(item), "/")
		if item != "" {
			origins = append(origins, item)
		}
	}
	return origins
}

func checkWSOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	normalized := strings.TrimRight(origin, "/")
	for _, allowed := range wsAllowedOrigins {
		if strings.EqualFold(normalized, allowed) {
			return true
		}
	}
	return false
}

func NewHandler(resolver RequestActorResolver, hub *svcws.Hub) *Handler {
	return &Handler{
		resolver: resolver,
		hub:      hub,
		upgrader: gws.Upgrader{CheckOrigin: checkWSOrigin},
	}
}

func (h *Handler) Upgrade(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		if cookie, err := c.Cookie(handler.WSTokenCookie); err == nil {
			token = strings.TrimSpace(cookie)
		}
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
