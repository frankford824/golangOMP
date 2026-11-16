package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"workflow/domain"
)

func TestUsageHTTPFailureClassificationAndPrivacy(t *testing.T) {
	for _, status := range []int{200, 403, 409, 422, 500} {
		stub := &workflowTraceRecorderStub{}
		router := gin.New()
		router.Use(requestLogger(zap.NewNop(), nil, stub))
		router.POST("/v1/tasks/:id/audit/decision", func(c *gin.Context) { c.Set("usage_error_code", "permission_denied"); c.Status(status) })
		req := httptest.NewRequest(http.MethodPost, "/v1/tasks/12/audit/decision?token=secret&q=private", nil)
		req.Header.Set("Origin", "https://assets.yongbo.cloud")
		req.Header.Set("X-Usage-App", "main-ops")
		req.Header.Set("X-Usage-Session", "session")
		req.Header.Set("X-Usage-Page", "page")
		req.Header.Set("Referer", "https://assets.yongbo.cloud/drive?sig=secret#private")
		router.ServeHTTP(httptest.NewRecorder(), req)
		if stub.event == nil {
			t.Fatal("missing trace")
		}
		want := domain.WorkflowTraceOutcomeSucceeded
		if status >= 400 {
			want = domain.WorkflowTraceOutcomeFailed
		}
		if stub.event.Outcome != want {
			t.Fatal(status, stub.event.Outcome)
		}
		if strings.Contains(string(stub.event.Payload), "secret") || strings.Contains(string(stub.event.Payload), "private") {
			t.Fatal(string(stub.event.Payload))
		}
		var payload map[string]string
		_ = json.Unmarshal(stub.event.Payload, &payload)
		if payload["app_id"] != "asset-workbench" || payload["session_id"] != "session" || payload["request_kind"] != "mutation_request" {
			t.Fatal(payload)
		}
		if status >= 400 && payload["error_code"] != "permission_denied" {
			t.Fatal(payload)
		}
	}
}
