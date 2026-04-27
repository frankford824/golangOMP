//go:build integration

package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"
	"go.uber.org/zap"

	"workflow/domain"
	svcws "workflow/service/websocket"
)

// SA-C-I11 — GET /ws/v1
// Asserts: missing bearer is unauthorized, a valid bearer upgrades to 101, and
// hub BroadcastPoolCountChanged reaches the connected client.
func TestSACI11_WebSocketHandshake_BearerAuthOrUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := svcws.NewHub(zap.NewNop())
	router := gin.New()
	router.GET("/ws/v1", NewHandler(saCWSResolver{token: "valid-sac-token", userID: 40019, team: "sac-ws-team"}, hub).Upgrade)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ws/v1")
	if err != nil {
		t.Fatalf("GET without bearer: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing bearer status=%d want 401", resp.StatusCode)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/v1"
	header := http.Header{"Authorization": []string{"Bearer valid-sac-token"}}
	conn, resp, err := gws.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("websocket dial err=%v status=%d", err, status)
	}
	defer conn.Close()
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("websocket upgrade status=%v want 101", respStatus(resp))
	}

	if err := conn.WriteMessage(gws.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("client write ping: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	time.AfterFunc(50*time.Millisecond, func() {
		hub.BroadcastPoolCountChanged("sac-ws-team", 7)
	})
	var event domain.WebSocketEvent
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read broadcast event: %v", err)
	}
	if event.Type != domain.WebSocketEventTaskPoolCountChanged {
		t.Fatalf("event type=%q want %q payload=%s", event.Type, domain.WebSocketEventTaskPoolCountChanged, string(event.Payload))
	}
}

type saCWSResolver struct {
	token  string
	userID int64
	team   string
}

func (r saCWSResolver) ResolveRequestActor(_ context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError) {
	if bearerToken != r.token {
		return nil, domain.ErrUnauthorized
	}
	return &domain.RequestActor{
		ID:       r.userID,
		Username: "sac_ws_user",
		Roles:    []domain.Role{domain.RoleMember},
		Team:     r.team,
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}, nil
}

func respStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
