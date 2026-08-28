package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

type rejectingMachineBearerActorResolver struct{}

func (rejectingMachineBearerActorResolver) ResolveRequestActor(context.Context, string) (*domain.RequestActor, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "session token rejected", nil)
}

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

func TestAssetSyncBearerBypassesSessionResolutionAndReachesMachineAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := assetSyncToken
	assetSyncToken = "sync-secret"
	t.Cleanup(func() { assetSyncToken = previous })

	router := gin.New()
	router.Use(injectRequestActor(rejectingMachineBearerActorResolver{}))
	group := router.Group("/v1/integration/asset-sync")
	group.Use(withAssetSyncTokenAuth())
	group.GET("/finalized/manifest", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/integration/asset-sync/finalized/manifest", nil)
		req.Header.Set(authorizationHeader, "Bearer "+token)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp.Code
	}
	if got := request("sync-secret"); got != http.StatusNoContent {
		t.Fatalf("machine bearer status = %d", got)
	}
	if got := request("wrong"); got != http.StatusUnauthorized {
		t.Fatalf("wrong machine bearer status = %d", got)
	}
}

func TestERPBridgeCostTokenIsDedicatedFailClosedAndBypassesSessionResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := erpBridgeCostAPIToken
	t.Cleanup(func() { erpBridgeCostAPIToken = previous })

	request := func(configured, headerName, token string) int {
		erpBridgeCostAPIToken = configured
		router := gin.New()
		router.Use(injectRequestActor(rejectingMachineBearerActorResolver{}))
		group := router.Group("/api/cost")
		group.Use(withERPBridgeCostTokenAuth())
		group.GET("/skus", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		req := httptest.NewRequest(http.MethodGet, "/api/cost/skus", nil)
		req.Header.Set(headerName, token)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp.Code
	}

	if got := request("", erpBridgeCostTokenHeader, "cost-secret"); got != http.StatusUnauthorized {
		t.Fatalf("unset token status = %d", got)
	}
	if got := request("cost-secret", erpBridgeCostTokenHeader, "cost-secret"); got != http.StatusNoContent {
		t.Fatalf("dedicated header status = %d", got)
	}
	if got := request("cost-secret", authorizationHeader, "Bearer cost-secret"); got != http.StatusNoContent {
		t.Fatalf("bearer status = %d", got)
	}
	if got := request("cost-secret", erpBridgeInternalTokenHeader, "cost-secret"); got != http.StatusUnauthorized {
		t.Fatalf("internal token header status = %d", got)
	}
}
