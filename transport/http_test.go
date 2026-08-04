package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"workflow/domain"
)

func TestRegisterOperationalRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	registerOperationalRoutes(router)

	for _, path := range []string{"/health", "/healthz", "/ping"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

func TestRequestLoggerRecordsWorkflowTraceEvent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	traceStub := &workflowTraceRecorderStub{}
	router := gin.New()
	router.Use(injectTraceID())
	router.Use(func(c *gin.Context) {
		actor := domain.RequestActor{
			ID:         7,
			Username:   "ops-user",
			Roles:      []domain.Role{domain.RoleOps},
			Department: string(domain.DepartmentOperations),
			Team:       "运营一组",
			Source:     domain.RequestActorSourceSessionToken,
			AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
		}
		c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
		c.Next()
	})
	router.Use(requestLogger(zap.NewNop(), nil, traceStub))
	router.GET("/v1/tasks/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/733?tab=audit", nil)
	req.Header.Set("User-Agent", "trace-test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	event := traceStub.event
	if event == nil {
		t.Fatal("workflow trace event was not recorded")
	}
	if event.EventSource != domain.WorkflowTraceSourceAPI || event.EventType != domain.WorkflowTraceEventAPIRequest {
		t.Fatalf("event source/type = %q/%q", event.EventSource, event.EventType)
	}
	if event.ActorID == nil || *event.ActorID != 7 {
		t.Fatalf("actor id = %v, want 7", event.ActorID)
	}
	if event.TaskID == nil || *event.TaskID != 733 {
		t.Fatalf("task id = %v, want 733", event.TaskID)
	}
	if event.RoutePath != "/v1/tasks/:id" {
		t.Fatalf("route path = %q, want /v1/tasks/:id", event.RoutePath)
	}
	if event.HTTPStatus == nil || *event.HTTPStatus != http.StatusNoContent {
		t.Fatalf("http status = %v", event.HTTPStatus)
	}
	var payload map[string]string
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload json err=%v raw=%s", err, string(event.Payload))
	}
	if payload["query"] != "tab=audit" {
		t.Fatalf("payload query = %q, want tab=audit", payload["query"])
	}
}

func TestV1R1_OpenAPI_Lint(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "run", "./cmd/tools/openapi-validate", "docs/api/openapi.yaml")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("openapi validation failed: %v\n%s", err, string(out))
	}
}

type workflowTraceRecorderStub struct {
	event *domain.WorkflowTraceEvent
}

func (s *workflowTraceRecorderStub) RecordTraceEvent(_ context.Context, event *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError) {
	s.event = event
	return event, nil
}

func hasRole(roles []domain.Role, target domain.Role) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller path")
	}
	return filepath.Dir(filepath.Dir(filename))
}
