package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestPredictionSuggestionEventIDIsPerDisplayAndStableWithinDisplay(t *testing.T) {
	suggestion := domain.PredictionSuggestion{
		ID:         "same-suggestion",
		Type:       "task_next_action",
		Source:     "rules",
		TargetType: "task",
		TargetID:   "42",
	}
	displayedAt := time.Date(2026, 6, 27, 10, 11, 12, 13, time.UTC)

	first := predictionSuggestionEventID("task_next_action", suggestion, displayedAt, 0)
	retry := predictionSuggestionEventID("task_next_action", suggestion, displayedAt, 0)
	nextDisplay := predictionSuggestionEventID("task_next_action", suggestion, displayedAt.Add(time.Second), 0)
	nextOrdinal := predictionSuggestionEventID("task_next_action", suggestion, displayedAt, 1)

	if first != retry {
		t.Fatalf("event id should be stable for the same display: %q != %q", first, retry)
	}
	if first == nextDisplay {
		t.Fatalf("event id should change for a later display")
	}
	if first == nextOrdinal {
		t.Fatalf("event id should change for another item in the same response")
	}
	if len(first) > 191 {
		t.Fatalf("event id length = %d, want <= 191", len(first))
	}
	if !strings.HasPrefix(first, "pred:task_next_action:task_next_action:20260627T101112.000000013Z:00:") {
		t.Fatalf("event id = %q, want stable display prefix", first)
	}
}

func TestPredictionSuggestionStableKeyIgnoresManagementRealtimeCounts(t *testing.T) {
	first := domain.PredictionSuggestion{
		ID:         "management-stale-17",
		Type:       "management",
		ActionType: "open_task_center",
		TargetType: "task_center",
		Source:     "任务更新",
	}
	next := first
	next.ID = "management-stale-23"

	firstKey := predictionSuggestionStableKey("management", first)
	nextKey := predictionSuggestionStableKey("management", next)
	if firstKey == "" {
		t.Fatal("stable key should not be empty")
	}
	if firstKey != nextKey {
		t.Fatalf("management stable key should ignore realtime counts: %q != %q", firstKey, nextKey)
	}
	if predictionAttributionEligible("management", first) {
		t.Fatal("management suggestions should be observation-only in Phase 2")
	}
}

func TestPredictionRecordSuggestionDisplayWaitsForExperienceCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/v1/predictions/tasks/42/next-actions?limit=1", nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	stub := &predictionExperienceServiceStub{
		flags:   domain.ExperienceRuntimeFlags{CaptureEnabled: true},
		entered: entered,
		release: release,
	}
	handler := &PredictionHandler{experienceSvc: stub}
	bundle := &domain.PredictionBundle{
		GeneratedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		Suggestions: []domain.PredictionSuggestion{{
			ID:         "task-next-42",
			Type:       "task_next_action",
			Source:     "rules",
			TargetType: "task",
			TargetID:   "42",
		}},
	}

	go func() {
		handler.recordSuggestionDisplay(c, domain.RequestActor{ID: 291}, "task_next_action", bundle)
		close(done)
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("experience capture was not called")
	}
	select {
	case <-done:
		t.Fatal("recordSuggestionDisplay returned before experience capture finished")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recordSuggestionDisplay did not return after experience capture finished")
	}
	if len(stub.events) != 1 || stub.events[0].SuggestionEventID == "" {
		t.Fatalf("captured events = %+v, want one event with suggestion_event_id", stub.events)
	}
}

type predictionExperienceServiceStub struct {
	experienceHandlerServiceStub
	flags   domain.ExperienceRuntimeFlags
	entered chan struct{}
	release chan struct{}
	events  []domain.AISuggestionEvent
}

func (s *predictionExperienceServiceStub) RuntimeFlags() domain.ExperienceRuntimeFlags {
	return s.flags
}

func (s *predictionExperienceServiceStub) RecordAISuggestionEvent(_ context.Context, event *domain.AISuggestionEvent) *domain.AppError {
	if s.entered != nil {
		close(s.entered)
		s.entered = nil
	}
	if s.release != nil {
		<-s.release
	}
	if event != nil {
		s.events = append(s.events, *event)
	}
	return nil
}
