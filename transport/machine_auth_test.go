package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExternalAssetEventTokenAuthIsDedicated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := externalAssetEventToken
	externalAssetEventToken = "event-secret"
	t.Cleanup(func() { externalAssetEventToken = previous })

	router := gin.New()
	router.Use(withExternalAssetEventTokenAuth())
	router.POST("/events", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name       string
		headerName string
		token      string
		wantStatus int
	}{
		{name: "dedicated header", headerName: externalAssetEventTokenHeader, token: "event-secret", wantStatus: http.StatusNoContent},
		{name: "bearer fallback", headerName: authorizationHeader, token: "Bearer event-secret", wantStatus: http.StatusNoContent},
		{name: "legacy agent header rejected", headerName: agentTokenHeader, token: "event-secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong token", headerName: externalAssetEventTokenHeader, token: "wrong", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/events", nil)
			req.Header.Set(tt.headerName, tt.token)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tt.wantStatus, resp.Body.String())
			}
		})
	}
}

func TestAssetSyncTokenAuthIsDedicatedAndFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := assetSyncToken
	t.Cleanup(func() { assetSyncToken = previous })

	request := func(headerName, token string) int {
		router := gin.New()
		router.Use(withAssetSyncTokenAuth())
		router.GET("/manifest", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
		req.Header.Set(headerName, token)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp.Code
	}

	assetSyncToken = ""
	if got := request(assetSyncTokenHeader, "secret"); got != http.StatusUnauthorized {
		t.Fatalf("unset token status = %d", got)
	}
	assetSyncToken = "sync-secret"
	if got := request(assetSyncTokenHeader, "sync-secret"); got != http.StatusNoContent {
		t.Fatalf("dedicated header status = %d", got)
	}
	if got := request(authorizationHeader, "Bearer sync-secret"); got != http.StatusNoContent {
		t.Fatalf("bearer status = %d", got)
	}
	if got := request(externalAssetEventTokenHeader, "sync-secret"); got != http.StatusUnauthorized {
		t.Fatalf("external event header status = %d", got)
	}
}
