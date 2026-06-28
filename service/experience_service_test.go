package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

func TestExperienceServiceDisabledDoesNotTouchRepo(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{}, zap.NewNop())

	stats, appErr := svc.Stats(context.Background())
	if appErr != nil {
		t.Fatalf("Stats returned app error: %v", appErr)
	}
	if stats == nil || stats.Flags.UIEnabled {
		t.Fatalf("Stats flags = %#v, want disabled empty stats", stats)
	}
	if err := svc.EnqueueEvent(context.Background(), &domain.ExperienceOutboxEvent{SourceType: "task", Action: "audit_reject"}); err != nil {
		t.Fatalf("EnqueueEvent disabled returned error: %v", err)
	}
	if err := svc.RecordAISuggestionEvent(context.Background(), &domain.AISuggestionEvent{SuggestionType: "task"}); err != nil {
		t.Fatalf("RecordAISuggestionEvent disabled returned error: %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("repo calls = %d, want 0 while all switches disabled", stub.calls)
	}
}

func TestExperienceServiceEnqueueSanitizesAndBuildsEventKey(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true}, zap.NewNop())
	payload := json.RawMessage(`{"reason_code":"bad_asset","customer_phone":"13800000000","nested":{"alipay_account":"secret"}}`)

	appErr := svc.EnqueueEvent(context.Background(), &domain.ExperienceOutboxEvent{
		SourceType: "audit",
		SourceID:   "task-42",
		TaskID:     experienceInt64Ptr(42),
		Action:     "reject",
		Outcome:    "failed",
		EventTime:  time.Date(2026, 6, 27, 8, 1, 2, 0, time.FixedZone("CST", 8*3600)),
		Payload:    payload,
	})
	if appErr != nil {
		t.Fatalf("EnqueueEvent returned app error: %v", appErr)
	}
	if stub.enqueued == nil {
		t.Fatal("expected enqueued event")
	}
	if stub.enqueued.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", stub.enqueued.SchemaVersion)
	}
	if stub.enqueued.EventKey == "" || !strings.Contains(stub.enqueued.EventKey, "audit:task-42:reject") {
		t.Fatalf("event_key = %q, want deterministic source/action key", stub.enqueued.EventKey)
	}
	var sanitized map[string]interface{}
	if err := json.Unmarshal(stub.enqueued.Payload, &sanitized); err != nil {
		t.Fatalf("payload is not valid json: %v", err)
	}
	if sanitized["customer_phone"] != "[REDACTED]" {
		t.Fatalf("customer_phone = %#v, want redacted", sanitized["customer_phone"])
	}
	nested := sanitized["nested"].(map[string]interface{})
	if nested["alipay_account"] != "[REDACTED]" {
		t.Fatalf("alipay_account = %#v, want redacted", nested["alipay_account"])
	}
}

func TestExperienceServiceRecordsAISuggestionEventWhenCaptureEnabled(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, AIFeedbackEnabled: false}, zap.NewNop())

	appErr := svc.RecordAISuggestionEvent(context.Background(), &domain.AISuggestionEvent{
		SuggestionEventID: "display-1",
		SuggestionType:    "task",
		DisplayedAt:       time.Date(2026, 6, 28, 8, 0, 0, 0, time.UTC),
	})
	if appErr != nil {
		t.Fatalf("RecordAISuggestionEvent returned app error: %v", appErr)
	}
	if stub.aiEvent == nil || stub.aiEvent.SuggestionEventID != "display-1" {
		t.Fatalf("ai event = %#v, want captured even when AI feedback is disabled", stub.aiEvent)
	}
}

type experienceRepoStub struct {
	calls    int
	enqueued *domain.ExperienceOutboxEvent
	aiEvent  *domain.AISuggestionEvent
}

func (s *experienceRepoStub) ListReasonTags(context.Context, string) ([]*domain.ExperienceReasonTag, error) {
	s.calls++
	return nil, nil
}

func (s *experienceRepoStub) ListExperienceEvents(context.Context, repo.ExperienceEventListFilter) ([]*domain.ExperienceEvent, int64, error) {
	s.calls++
	return nil, 0, nil
}

func (s *experienceRepoStub) ExperienceStats(context.Context) (*domain.ExperienceStats, error) {
	s.calls++
	return &domain.ExperienceStats{}, nil
}

func (s *experienceRepoStub) EnqueueExperienceEvent(_ context.Context, event *domain.ExperienceOutboxEvent) error {
	s.calls++
	s.enqueued = event
	return nil
}

func (s *experienceRepoStub) ClaimExperienceOutbox(context.Context, int, string, time.Time, time.Duration) ([]*domain.ExperienceOutboxEvent, error) {
	s.calls++
	return nil, nil
}

func (s *experienceRepoStub) CreateExperienceEventFromOutbox(context.Context, *domain.ExperienceOutboxEvent) error {
	s.calls++
	return nil
}

func (s *experienceRepoStub) MarkExperienceOutboxProcessed(context.Context, int64, time.Time) error {
	s.calls++
	return nil
}

func (s *experienceRepoStub) MarkExperienceOutboxFailed(context.Context, int64, int, int, string, time.Time) (bool, error) {
	s.calls++
	return false, nil
}

func (s *experienceRepoStub) CreateAISuggestionEvent(_ context.Context, event *domain.AISuggestionEvent) error {
	s.calls++
	s.aiEvent = event
	return nil
}

func (s *experienceRepoStub) CreateAISuggestionFeedback(context.Context, *domain.AISuggestionFeedback) (int64, error) {
	s.calls++
	return 1, nil
}

func experienceInt64Ptr(value int64) *int64 {
	return &value
}
