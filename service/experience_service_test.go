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

func TestExperienceServiceRecordsBehaviorEventsWhenEnabled(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:              true,
		CaptureEnabled:         true,
		BehaviorCaptureEnabled: true,
	}, zap.NewNop())

	result, appErr := svc.RecordBehaviorEvents(context.Background(), domain.RequestActor{ID: 291}, ExperienceBehaviorBatchRequest{
		Events: []ExperienceBehaviorEventRequest{{
			ClientEventID:       "client-1",
			Surface:             "task_detail",
			Action:              domain.ExperienceBehaviorActionImpression,
			TargetType:          "task",
			TargetID:            "123",
			SuggestionEventID:   "event-1",
			SuggestionStableKey: "stable-1",
		}},
	})
	if appErr != nil {
		t.Fatalf("RecordBehaviorEvents returned app error: %v", appErr)
	}
	if result.Received != 1 || result.Inserted != 1 {
		t.Fatalf("behavior result = %+v, want 1/1", result)
	}
	if len(stub.behaviorEvents) != 1 {
		t.Fatalf("behavior events = %d, want 1", len(stub.behaviorEvents))
	}
	event := stub.behaviorEvents[0]
	if event.EventKey != "291:client-1" || event.ActorID == nil || *event.ActorID != 291 {
		t.Fatalf("behavior identity = %+v", event)
	}
}

func TestExperienceServiceTaskStatusObserverCreatesBaselineOnlyOnFirstScan(t *testing.T) {
	sourceUpdatedAt := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	taskID := int64(42)
	stub := &experienceRepoStub{
		taskSnapshots: []*domain.ExperienceOutcomeSnapshotRow{{
			SourceName:      experienceSourceTaskStatusSnapshot,
			EntityType:      "task",
			EntityID:        "42",
			TaskID:          &taskID,
			TargetType:      "task",
			TargetID:        "42",
			SourceUpdatedAt: sourceUpdatedAt,
			ObservedValue:   json.RawMessage(`{"task_status":"InProgress"}`),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		CaptureEnabled: true,
		WorkerEnabled:  true,
	}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Scanned != 1 || result.Baselines != 1 || result.Enqueued != 0 {
		t.Fatalf("observer result = %+v, want scanned=1 baselines=1 enqueued=0", result)
	}
	if len(stub.enqueuedEvents) != 0 {
		t.Fatalf("enqueued events = %d, want 0 for first baseline", len(stub.enqueuedEvents))
	}
	if got := stub.observedState(experienceSourceTaskStatusSnapshot, "task", "42"); got == nil || got.ObservedHash == "" {
		t.Fatalf("observed state = %+v, want baseline with hash", got)
	}
}

func TestExperienceServiceTaskStatusObserverEnqueuesFieldDiffOnChange(t *testing.T) {
	sourceUpdatedAt := time.Date(2026, 6, 30, 8, 1, 0, 0, time.UTC)
	taskID := int64(42)
	previousValue := canonicalExperienceJSON(json.RawMessage(`{"task_status":"InProgress"}`))
	stub := &experienceRepoStub{
		observed: map[string]*domain.ExperienceObservedEntityState{
			experienceObservedKey(experienceSourceTaskStatusSnapshot, "task", "42"): {
				SourceName:    experienceSourceTaskStatusSnapshot,
				EntityType:    "task",
				EntityID:      "42",
				ObservedValue: previousValue,
				ObservedHash:  hashObservedValue(previousValue),
			},
		},
		taskSnapshots: []*domain.ExperienceOutcomeSnapshotRow{{
			SourceName:      experienceSourceTaskStatusSnapshot,
			EntityType:      "task",
			EntityID:        "42",
			TaskID:          &taskID,
			TargetType:      "task",
			TargetID:        "42",
			SourceUpdatedAt: sourceUpdatedAt,
			ObservedValue:   json.RawMessage(`{"task_status":"Completed"}`),
			TerminalState:   string(domain.TaskStatusCompleted),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		CaptureEnabled: true,
		WorkerEnabled:  true,
	}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Scanned != 1 || result.Changed != 1 || result.Enqueued != 1 {
		t.Fatalf("observer result = %+v, want scanned=1 changed=1 enqueued=1", result)
	}
	if len(stub.enqueuedEvents) != 1 {
		t.Fatalf("enqueued events = %d, want 1", len(stub.enqueuedEvents))
	}
	event := stub.enqueuedEvents[0]
	if event.Action != "task_status_changed" || event.Outcome != string(domain.TaskStatusCompleted) {
		t.Fatalf("event action/outcome = %s/%s", event.Action, event.Outcome)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	changed, ok := payload["changed_fields"].([]interface{})
	if !ok || len(changed) != 1 {
		t.Fatalf("changed_fields = %#v, want one field diff", payload["changed_fields"])
	}
	state := stub.observedState(experienceSourceTaskStatusSnapshot, "task", "42")
	if state == nil || state.TerminalState != string(domain.TaskStatusCompleted) || state.TerminalObservedAt == nil {
		t.Fatalf("observed state after change = %+v", state)
	}
}

func TestExperienceServiceAssetReviewObserverUsesSemanticOutcome(t *testing.T) {
	sourceUpdatedAt := time.Date(2026, 6, 30, 8, 2, 0, 0, time.UTC)
	taskID := int64(42)
	previousValue := canonicalExperienceJSON(json.RawMessage(`{"approved_at":null,"flow_review_status":"pending_review","rejected_at":null}`))
	stub := &experienceRepoStub{
		observed: map[string]*domain.ExperienceObservedEntityState{
			experienceObservedKey(experienceSourceTaskAssetReviewSnapshot, "task_asset", "7001"): {
				SourceName:    experienceSourceTaskAssetReviewSnapshot,
				EntityType:    "task_asset",
				EntityID:      "7001",
				ObservedValue: previousValue,
				ObservedHash:  hashObservedValue(previousValue),
			},
		},
		assetSnapshots: []*domain.ExperienceOutcomeSnapshotRow{{
			SourceName:      experienceSourceTaskAssetReviewSnapshot,
			EntityType:      "task_asset",
			EntityID:        "7001",
			TaskID:          &taskID,
			TargetType:      "task_asset",
			TargetID:        "7001",
			SourceUpdatedAt: sourceUpdatedAt,
			ObservedValue:   json.RawMessage(`{"approved_at":"2026-06-30T08:02:00Z","flow_review_status":"approved","rejected_at":null}`),
			TerminalState:   string(domain.TaskAssetFlowReviewStatusApproved),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		CaptureEnabled: true,
		WorkerEnabled:  true,
	}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Changed != 1 || result.Enqueued != 1 {
		t.Fatalf("observer result = %+v, want one semantic asset review event", result)
	}
	event := stub.enqueuedEvents[0]
	if event.Action != "asset_review_status_changed" || event.Outcome != "approved" {
		t.Fatalf("event action/outcome = %s/%s", event.Action, event.Outcome)
	}
	if event.TargetType != "task_asset" || event.TargetID != "7001" {
		t.Fatalf("event target = %s/%s", event.TargetType, event.TargetID)
	}
}

func TestExperienceServiceFilingObserverUsesFieldSpecificOutcome(t *testing.T) {
	sourceUpdatedAt := time.Date(2026, 6, 30, 8, 3, 0, 0, time.UTC)
	taskID := int64(42)
	previousValue := canonicalExperienceJSON(json.RawMessage(`{"erp_sync_required":true,"filing_status":"filed","last_filed_at":"2026-06-30T08:00:00Z"}`))
	stub := &experienceRepoStub{
		observed: map[string]*domain.ExperienceObservedEntityState{
			experienceObservedKey(experienceSourceTaskDetailFilingSnapshot, "task_detail", "9001"): {
				SourceName:    experienceSourceTaskDetailFilingSnapshot,
				EntityType:    "task_detail",
				EntityID:      "9001",
				ObservedValue: previousValue,
				ObservedHash:  hashObservedValue(previousValue),
			},
		},
		detailSnapshots: []*domain.ExperienceOutcomeSnapshotRow{{
			SourceName:      experienceSourceTaskDetailFilingSnapshot,
			EntityType:      "task_detail",
			EntityID:        "9001",
			TaskID:          &taskID,
			TargetType:      "task",
			TargetID:        "42",
			SourceUpdatedAt: sourceUpdatedAt,
			ObservedValue:   json.RawMessage(`{"erp_sync_required":false,"filing_status":"filed","last_filed_at":"2026-06-30T08:00:00Z"}`),
			TerminalState:   string(domain.FilingStatusFiled),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		CaptureEnabled: true,
		WorkerEnabled:  true,
	}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Changed != 1 || result.Enqueued != 1 {
		t.Fatalf("observer result = %+v, want one filing sync event", result)
	}
	event := stub.enqueuedEvents[0]
	if event.Action != "erp_sync_required_changed" || event.Outcome != "false" {
		t.Fatalf("event action/outcome = %s/%s", event.Action, event.Outcome)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	changed, ok := payload["changed_fields"].([]interface{})
	if !ok || len(changed) != 1 {
		t.Fatalf("changed_fields = %#v, want only erp_sync_required diff", payload["changed_fields"])
	}
}

type experienceRepoStub struct {
	calls            int
	enqueued         *domain.ExperienceOutboxEvent
	enqueuedEvents   []*domain.ExperienceOutboxEvent
	aiEvent          *domain.AISuggestionEvent
	behaviorEvents   []*domain.ExperienceBehaviorEvent
	watermarks       map[string]*domain.ExperienceWorkerWatermark
	auditRows        []*domain.ExperienceOutcomeEventRow
	moduleRows       []*domain.ExperienceOutcomeEventRow
	taskSnapshots    []*domain.ExperienceOutcomeSnapshotRow
	assetSnapshots   []*domain.ExperienceOutcomeSnapshotRow
	detailSnapshots  []*domain.ExperienceOutcomeSnapshotRow
	skuItemSnapshots []*domain.ExperienceOutcomeSnapshotRow
	observed         map[string]*domain.ExperienceObservedEntityState
	retentionRun     *domain.ExperienceRetentionRun
}

func (s *experienceRepoStub) ListReasonTags(context.Context, string) ([]*domain.ExperienceReasonTag, error) {
	s.calls++
	return nil, nil
}

func (s *experienceRepoStub) ListClientReasonTags(context.Context, string, []string) ([]*domain.ExperienceClientReasonTag, error) {
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
	if event != nil {
		copied := *event
		s.enqueuedEvents = append(s.enqueuedEvents, &copied)
	}
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

func (s *experienceRepoStub) CreateExperienceBehaviorEvents(_ context.Context, events []*domain.ExperienceBehaviorEvent) (int, error) {
	s.calls++
	s.behaviorEvents = append(s.behaviorEvents, events...)
	return len(events), nil
}

func (s *experienceRepoStub) CreateAISuggestionFeedback(context.Context, *domain.AISuggestionFeedback) (int64, error) {
	s.calls++
	return 1, nil
}

func (s *experienceRepoStub) GetExperienceWorkerWatermark(_ context.Context, workerName, sourceName string) (*domain.ExperienceWorkerWatermark, error) {
	s.calls++
	if s.watermarks == nil {
		return nil, nil
	}
	if watermark := s.watermarks[workerName+":"+sourceName]; watermark != nil {
		copied := *watermark
		return &copied, nil
	}
	return nil, nil
}

func (s *experienceRepoStub) SaveExperienceWorkerWatermark(_ context.Context, watermark *domain.ExperienceWorkerWatermark) error {
	s.calls++
	if s.watermarks == nil {
		s.watermarks = make(map[string]*domain.ExperienceWorkerWatermark)
	}
	copied := *watermark
	s.watermarks[watermark.WorkerName+":"+watermark.SourceName] = &copied
	return nil
}

func (s *experienceRepoStub) ListExperienceAuditOutcomeRows(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeEventRow, error) {
	s.calls++
	return s.auditRows, nil
}

func (s *experienceRepoStub) ListExperienceModuleOutcomeRows(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeEventRow, error) {
	s.calls++
	return s.moduleRows, nil
}

func (s *experienceRepoStub) ListExperienceTaskStatusSnapshots(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	s.calls++
	return s.taskSnapshots, nil
}

func (s *experienceRepoStub) ListExperienceTaskAssetReviewSnapshots(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	s.calls++
	return s.assetSnapshots, nil
}

func (s *experienceRepoStub) ListExperienceTaskDetailFilingSnapshots(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	s.calls++
	return s.detailSnapshots, nil
}

func (s *experienceRepoStub) ListExperienceTaskSKUItemFilingSnapshots(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	s.calls++
	return s.skuItemSnapshots, nil
}

func (s *experienceRepoStub) GetExperienceObservedEntityState(_ context.Context, sourceName, entityType, entityID string) (*domain.ExperienceObservedEntityState, error) {
	s.calls++
	state := s.observedState(sourceName, entityType, entityID)
	if state == nil {
		return nil, nil
	}
	copied := *state
	return &copied, nil
}

func (s *experienceRepoStub) UpsertExperienceObservedEntityState(_ context.Context, state *domain.ExperienceObservedEntityState) error {
	s.calls++
	if s.observed == nil {
		s.observed = make(map[string]*domain.ExperienceObservedEntityState)
	}
	copied := *state
	s.observed[experienceObservedKey(state.SourceName, state.EntityType, state.EntityID)] = &copied
	return nil
}

func (s *experienceRepoStub) RunExperienceRetention(context.Context, repo.ExperienceRetentionPolicy) (*domain.ExperienceRetentionRun, error) {
	s.calls++
	if s.retentionRun != nil {
		return s.retentionRun, nil
	}
	return &domain.ExperienceRetentionRun{}, nil
}

func (s *experienceRepoStub) observedState(sourceName, entityType, entityID string) *domain.ExperienceObservedEntityState {
	if s.observed == nil {
		return nil
	}
	return s.observed[experienceObservedKey(sourceName, entityType, entityID)]
}

func experienceObservedKey(sourceName, entityType, entityID string) string {
	return sourceName + ":" + entityType + ":" + entityID
}

func experienceInt64Ptr(value int64) *int64 {
	return &value
}
