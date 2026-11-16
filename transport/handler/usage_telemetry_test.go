package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"workflow/domain"
	"workflow/service"
)

type usageTraceStub struct{ event *domain.WorkflowTraceEvent }

func (s *usageTraceStub) RecordTraceEvent(_ context.Context, e *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError) {
	s.event = e
	return e, nil
}
func (s *usageTraceStub) ListTraceEvents(context.Context, service.WorkflowTraceEventFilter) ([]*domain.WorkflowTraceEvent, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func TestUsageHandlerDerivesActorAndRemovesSecrets(t *testing.T) {
	stub := &usageTraceStub{}
	h := &UserAdminHandler{traceEventSvc: stub}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/trace-events", strings.NewReader(`{"event_type":"page_view","actor_id":999,"page_url":"https://assets.yongbo.cloud/drive?token=secret#private","occurred_at":"2099-01-01T00:00:00Z","payload":{"telemetry_schema":"usage.v1","app_id":"main-ops","token":"secret","file_name":"private.psd","visible_ms":120000}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Origin", "https://assets.yongbo.cloud")
	c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), domain.RequestActor{ID: 7, Username: "employee"}))
	h.RecordWorkflowTraceEvent(c)
	if w.Code != 200 || stub.event == nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if stub.event.ActorID == nil || *stub.event.ActorID != 7 {
		t.Fatal("client spoofed actor")
	}
	if stub.event.PageURL != "https://assets.yongbo.cloud/drive" || strings.Contains(string(stub.event.Payload), "secret") || strings.Contains(string(stub.event.Payload), "private") {
		t.Fatal(stub.event)
	}
	if !strings.Contains(string(stub.event.Payload), `"app_id":"asset-workbench"`) {
		t.Fatal(string(stub.event.Payload))
	}
	if time.Since(stub.event.OccurredAt) > time.Second || stub.event.OccurredAt.After(time.Now().Add(time.Second)) {
		t.Fatal("client clock accepted")
	}
}

func TestUsageHandlerRejectsOversizedOrWrongEventType(t *testing.T) {
	for _, body := range []string{`{"event_type":"api_request","payload":{"telemetry_schema":"usage.v1","app_id":"main-ops"}}`, `{"event_type":"page_view","payload":{"padding":"` + strings.Repeat("x", 40000) + `"}}`} {
		stub := &usageTraceStub{}
		h := &UserAdminHandler{traceEventSvc: stub}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/trace-events", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.RecordWorkflowTraceEvent(c)
		if w.Code != 400 || stub.event != nil {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
