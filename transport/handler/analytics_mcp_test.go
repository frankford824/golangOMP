package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	analyticssvc "workflow/service/analytics"
)

type handlerAnalyticsRepoStub struct{}

func (handlerAnalyticsRepoStub) QueryMetric(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsMetricDefinition, domain.AnalyticsMetricQuery) (*domain.AnalyticsMetricResult, error) {
	return &domain.AnalyticsMetricResult{MetricID: "page_views", MetricName: "页面访问", Rows: []domain.AnalyticsMetricRow{}}, nil
}
func (handlerAnalyticsRepoStub) TraceEntity(context.Context, domain.ResourceGroupAccessFilter, domain.AnalyticsTraceQuery) ([]domain.AIRetrievalHit, error) {
	return []domain.AIRetrievalHit{}, nil
}

func TestAnalyticsMCPInitializeAndListTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAIChatHandler(nil, time.Second, analyticssvc.NewService(handlerAnalyticsRepoStub{}, nil))

	initialize := performMCPRequest(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, "")
	if initialize.Code != http.StatusOK || initialize.Header().Get("Mcp-Session-Id") == "" || !strings.Contains(initialize.Body.String(), `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize status=%d headers=%v body=%s", initialize.Code, initialize.Header(), initialize.Body.String())
	}

	tools := performMCPRequest(t, handler, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "https://yongbo.cloud")
	if tools.Code != http.StatusOK || !strings.Contains(tools.Body.String(), `"list_metrics"`) || !strings.Contains(tools.Body.String(), `"query_metric"`) {
		t.Fatalf("tools status=%d body=%s", tools.Code, tools.Body.String())
	}
}

func TestAnalyticsMCPRejectsForeignOrigin(t *testing.T) {
	handler := NewAIChatHandler(nil, time.Second, analyticssvc.NewService(handlerAnalyticsRepoStub{}, nil))
	response := performMCPRequest(t, handler, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "https://attacker.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func performMCPRequest(t *testing.T, handler *AIChatHandler, body, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://yongbo.cloud/v1/analytics/mcp", bytes.NewBufferString(body))
	request.Host = "yongbo.cloud"
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	permissions := []domain.PermissionCode{domain.PermissionReportView, domain.PermissionTaskView}
	actor := domain.RequestActor{ID: 9, Permissions: permissions, EffectiveAccess: &domain.EffectiveAccess{
		UserID: 9, Permissions: permissions,
		Assignments: []domain.AccessAssignment{{UserID: 9, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionReportView, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}, {Permission: domain.PermissionTaskView, RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
	}}
	request = request.WithContext(domain.WithRequestActor(request.Context(), actor))
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = request
	handler.AnalyticsMCPPost(c)
	return response
}
