package ws

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	gws "github.com/gorilla/websocket"
	"go.uber.org/zap"

	"workflow/domain"
	svcws "workflow/service/websocket"
	"workflow/transport/handler"
)

func TestWebSocketHandshake_SubprotocolTokenFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := svcws.NewHub(zap.NewNop())
	router := gin.New()
	router.GET("/ws/v1", NewHandler(testWSResolver{token: "valid-ws-token"}, hub).Upgrade)
	server := httptest.NewServer(router)
	defer server.Close()

	encodedToken := base64.RawURLEncoding.EncodeToString([]byte("valid-ws-token"))
	dialer := gws.Dialer{
		Subprotocols: []string{"workflow-v1", wsBearerProtocolPrefix + encodedToken},
	}
	header := http.Header{}
	header.Set("Cookie", handler.WSTokenCookie+"=stale-token")

	conn, resp, err := dialer.Dial("ws"+server.URL[len("http"):]+"/ws/v1", header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("websocket dial err=%v status=%d", err, status)
	}
	defer conn.Close()
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("websocket upgrade status=%v want %d", respStatusCode(resp), http.StatusSwitchingProtocols)
	}
	if conn.Subprotocol() != "workflow-v1" {
		t.Fatalf("selected subprotocol=%q want workflow-v1", conn.Subprotocol())
	}
}

func TestBearerTokenFromWSProtocol(t *testing.T) {
	encodedToken := base64.RawURLEncoding.EncodeToString([]byte("token-value"))
	token := bearerTokenFromWSProtocol("workflow-v1, " + wsBearerProtocolPrefix + encodedToken)
	if token != "token-value" {
		t.Fatalf("token=%q want token-value", token)
	}
	if token := bearerTokenFromWSProtocol("workflow-v1, wf-token.not-base64!"); token != "" {
		t.Fatalf("invalid token=%q want empty", token)
	}
}

type testWSResolver struct {
	token string
}

func (r testWSResolver) ResolveRequestActor(_ context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError) {
	if bearerToken != r.token {
		return nil, domain.ErrUnauthorized
	}
	return &domain.RequestActor{
		ID:       991,
		Username: "ws_user",
		Roles:    []domain.Role{domain.RoleMember},
		Team:     "ws-team",
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}, nil
}

func respStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
