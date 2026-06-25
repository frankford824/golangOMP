package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestParseTaskFilterQueryPriorityCritical(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks?priority=critical", nil)

	filter, appErr := parseTaskFilterQuery(c)
	if appErr != nil {
		t.Fatalf("parseTaskFilterQuery() error = %+v", appErr)
	}
	if len(filter.Priorities) != 1 || filter.Priorities[0] != domain.TaskPriorityCritical {
		t.Fatalf("Priorities = %#v, want [critical]", filter.Priorities)
	}
}

func TestParseTaskFilterQueryPriorityMultiValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks?priority=critical,high", nil)

	filter, appErr := parseTaskFilterQuery(c)
	if appErr != nil {
		t.Fatalf("parseTaskFilterQuery() error = %+v", appErr)
	}
	if len(filter.Priorities) != 2 {
		t.Fatalf("Priorities len = %d, want 2", len(filter.Priorities))
	}
	if filter.Priorities[0] != domain.TaskPriorityCritical || filter.Priorities[1] != domain.TaskPriorityHigh {
		t.Fatalf("Priorities = %#v, want [critical high]", filter.Priorities)
	}
}

func TestParseTaskFilterQueryPriorityInvalidReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks?priority=urgent", nil)

	_, appErr := parseTaskFilterQuery(c)
	if appErr == nil {
		t.Fatal("parseTaskFilterQuery() expected error for invalid priority")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("AppError.Code = %q, want %q", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if appErr.Message != "task_priority_invalid" {
		t.Fatalf("AppError.Message = %q, want task_priority_invalid", appErr.Message)
	}
}
