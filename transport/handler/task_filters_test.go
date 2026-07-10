package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestParseTaskFilterQueryCreatedDateRangeUsesShanghaiDayBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks?date_from=2026-07-01&date_to=2026-07-01", nil)

	filter, appErr := parseTaskFilterQuery(c)
	if appErr != nil {
		t.Fatalf("parseTaskFilterQuery() error = %+v", appErr)
	}
	if filter.CreatedFrom == nil || filter.CreatedTo == nil {
		t.Fatalf("created range = %+v / %+v, want both boundaries", filter.CreatedFrom, filter.CreatedTo)
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	wantFrom := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	wantTo := time.Date(2026, 7, 1, 23, 59, 59, int(time.Second-time.Nanosecond), location)
	if !filter.CreatedFrom.Equal(wantFrom) || !filter.CreatedTo.Equal(wantTo) {
		t.Fatalf("created range = %s / %s, want %s / %s", filter.CreatedFrom, filter.CreatedTo, wantFrom, wantTo)
	}
}

func TestParseTaskFilterQueryRejectsInvertedCreatedDateRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks?date_from=2026-07-02&date_to=2026-07-01", nil)

	_, appErr := parseTaskFilterQuery(c)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("parseTaskFilterQuery() error = %+v, want INVALID_REQUEST", appErr)
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
