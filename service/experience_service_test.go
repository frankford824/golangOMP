package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
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

func TestExperienceServiceClientConfigRequiresCaptureForMicroQuestion(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       false,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	config := svc.ClientConfig()
	if config.MicroQuestionEnabled {
		t.Fatalf("client config micro_question_enabled = true, want false when capture is disabled")
	}

	svc = NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())
	config = svc.ClientConfig()
	if !config.MicroQuestionEnabled {
		t.Fatalf("client config micro_question_enabled = false, want true when UI/capture/micro are enabled")
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

func TestExperienceServiceObserverContinuesAfterSourceFailureAndRecordsRuns(t *testing.T) {
	taskID := int64(42)
	sourceUpdatedAt := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		auditErr: errors.New("audit source unavailable"),
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
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Failed != 1 || result.Baselines != 1 {
		t.Fatalf("observer result = %+v, want failed source plus later baseline", result)
	}
	if got := stub.observedState(experienceSourceTaskStatusSnapshot, "task", "42"); got == nil {
		t.Fatal("task snapshot source did not continue after audit source failure")
	}
	if len(stub.workerRuns) < 2 {
		t.Fatalf("worker runs = %d, want failed and success source runs", len(stub.workerRuns))
	}
	if stub.workerRuns[0].Status != "failed" || stub.workerRuns[0].SourceName != experienceSourceAuditRecords {
		t.Fatalf("first worker run = %+v, want failed audit source", stub.workerRuns[0])
	}
}

func TestExperienceServiceProcessAttributionsCreatesWeightedCandidate(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	displayedAt := outcomeAt.Add(-2 * time.Hour)
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:         9,
			EventKey:   "outcome:tasks:42:completed",
			EventTime:  outcomeAt,
			SourceType: experienceSourceTaskStatusSnapshot,
			Action:     "task_status_changed",
			Outcome:    "Completed",
			TaskID:     experienceInt64Ptr(42),
			TargetType: "task",
			TargetID:   "42",
			Payload:    json.RawMessage(`{"changed_fields":[{"field":"task_status","from":"InProgress","to":"Completed"}]}`),
		}},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "suggestion-display-1",
			SuggestionStableKey: "task_next|asset_ready|workflow|task|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "asset_ready",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         displayedAt,
			BehaviorCount:       1,
			BehaviorScore:       5,
			FeedbackValue:       domain.ExperienceFeedbackAccepted,
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Scanned != 1 || result.Created != 1 || result.Failed != 0 {
		t.Fatalf("attribution result = %+v, want one created", result)
	}
	if len(stub.attributions) != 1 {
		t.Fatalf("attributions = %d, want 1", len(stub.attributions))
	}
	attribution := stub.attributions[0]
	if attribution.Status != domain.ExperienceAttributionStatusPositive || attribution.Confidence != "high" {
		t.Fatalf("attribution status/confidence = %s/%s", attribution.Status, attribution.Confidence)
	}
	if attribution.Score < 0.75 {
		t.Fatalf("attribution score = %f, want high confidence score", attribution.Score)
	}
	if len(stub.reviewItems) != 1 || stub.reviewItems[0].Status != domain.ExperienceReviewItemStatusOpen {
		t.Fatalf("review items = %+v, want one open attribution candidate", stub.reviewItems)
	}
	watermark := stub.watermarks[domain.ExperienceWorkerAttribution+":experience_events"]
	if watermark == nil || watermark.LastSeenID != 9 {
		t.Fatalf("attribution watermark = %+v, want outcome id 9", watermark)
	}
}

func TestExperienceServiceProcessAttributionsSkipsCandidateWithoutBehaviorOrFeedback(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:         9,
			EventKey:   "outcome:tasks:42:completed",
			EventTime:  outcomeAt,
			SourceType: experienceSourceTaskStatusSnapshot,
			Action:     "task_status_changed",
			Outcome:    "Completed",
			TaskID:     experienceInt64Ptr(42),
			TargetType: "task",
			TargetID:   "42",
		}},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "suggestion-display-1",
			SuggestionStableKey: "task_next|asset_ready|workflow|task|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "asset_ready",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         outcomeAt.Add(-2 * time.Hour),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Created != 0 || len(stub.attributions) != 0 || len(stub.reviewItems) != 0 {
		t.Fatalf("result=%+v attributions=%d review_items=%d, want no attribution without behavior or feedback", result, len(stub.attributions), len(stub.reviewItems))
	}
}

func TestExperienceServiceProcessAttributionsReprocessesRecentOutcomesForLateFeedback(t *testing.T) {
	outcomeAt := time.Now().UTC().Add(-2 * time.Hour)
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{},
		recentAttributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:         9,
			EventKey:   "outcome:tasks:42:completed",
			EventTime:  outcomeAt,
			SourceType: experienceSourceTaskStatusSnapshot,
			Action:     "task_status_changed",
			Outcome:    "Completed",
			TaskID:     experienceInt64Ptr(42),
			TargetType: "task",
			TargetID:   "42",
		}},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "suggestion-display-1",
			SuggestionStableKey: "task_next|asset_ready|workflow|task|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "asset_ready",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         outcomeAt.Add(-2 * time.Hour),
			FeedbackValue:       domain.ExperienceFeedbackAccepted,
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Scanned != 1 || result.Created != 1 {
		t.Fatalf("attribution result = %+v, want recent outcome reprocessed into attribution", result)
	}
	if len(stub.attributions) != 1 || len(stub.reviewItems) != 1 {
		t.Fatalf("attributions/reviewItems = %d/%d, want 1/1", len(stub.attributions), len(stub.reviewItems))
	}
	if watermark := stub.watermarks[domain.ExperienceWorkerAttribution+":experience_events"]; watermark != nil {
		t.Fatalf("watermark = %+v, want recent reprocess not to advance watermark", watermark)
	}
	if watermark := stub.watermarks[domain.ExperienceWorkerAttribution+":"+experienceSourceAttributionRecentReprocess]; watermark == nil || watermark.LastSeenID != 9 {
		t.Fatalf("recent watermark = %+v, want recent reprocess cursor to advance", watermark)
	}
}

func TestExperienceServiceProcessAttributionsSkipsReviewItemForNonMaterializableTarget(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:         9,
			EventKey:   "outcome:task_asset:7001:approved",
			EventTime:  outcomeAt,
			SourceType: experienceSourceTaskAssetReviewSnapshot,
			Action:     "asset_review_status_changed",
			Outcome:    "approved",
			TaskID:     experienceInt64Ptr(42),
			TargetType: "task_asset",
			TargetID:   "7001",
		}},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "suggestion-display-1",
			SuggestionStableKey: "asset|review|workflow|task_asset|7001",
			SuggestionType:      "asset",
			SuggestionID:        "review-asset",
			TargetType:          "task_asset",
			TargetID:            "7001",
			DisplayedAt:         outcomeAt.Add(-2 * time.Hour),
			BehaviorCount:       1,
			BehaviorScore:       5,
			FeedbackValue:       domain.ExperienceFeedbackAccepted,
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Created != 1 || len(stub.attributions) != 1 {
		t.Fatalf("result=%+v attributions=%d, want attribution created", result, len(stub.attributions))
	}
	if len(stub.reviewItems) != 0 {
		t.Fatalf("review items = %+v, want non-materializable target skipped", stub.reviewItems)
	}
}

func TestExperienceServiceRecordAISuggestionFeedbackRequiresOwnedSuggestion(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID: "display-1",
				TargetType:        "task",
				TargetID:          "42",
				ActorID:           experienceInt64Ptr(291),
			},
			"display-2": {
				SuggestionEventID: "display-2",
				TargetType:        "task",
				TargetID:          "43",
				ActorID:           experienceInt64Ptr(999),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{AIFeedbackEnabled: true}, zap.NewNop())

	feedback, appErr := svc.RecordAISuggestionFeedback(context.Background(), domain.RequestActor{ID: 291}, AISuggestionFeedbackRequest{
		SuggestionEventID: "display-1",
		FeedbackValue:     domain.ExperienceFeedbackAccepted,
	})
	if appErr != nil {
		t.Fatalf("RecordAISuggestionFeedback owned suggestion returned app error: %v", appErr)
	}
	if feedback == nil || feedback.ID != 1 {
		t.Fatalf("feedback = %+v, want created", feedback)
	}

	feedback, appErr = svc.RecordAISuggestionFeedback(context.Background(), domain.RequestActor{ID: 291}, AISuggestionFeedbackRequest{
		SuggestionEventID: "display-2",
		FeedbackValue:     domain.ExperienceFeedbackAccepted,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordAISuggestionFeedback cross-actor appErr = %+v, want invalid request", appErr)
	}
	if feedback != nil {
		t.Fatalf("feedback = %+v, want nil for cross-actor suggestion", feedback)
	}
	if len(stub.feedbacks) != 1 {
		t.Fatalf("feedback writes = %d, want only owned write", len(stub.feedbacks))
	}
}

func TestExperienceServiceReserveRateLimitUsesUpdatedCount(t *testing.T) {
	periodStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true}, zap.NewNop())

	first, appErr := svc.ReserveRateLimit(context.Background(), domain.RequestActor{ID: 291}, "micro_question_daily", periodStart, periodEnd, 2)
	if appErr != nil {
		t.Fatalf("ReserveRateLimit first returned app error: %v", appErr)
	}
	second, appErr := svc.ReserveRateLimit(context.Background(), domain.RequestActor{ID: 291}, "micro_question_daily", periodStart, periodEnd, 2)
	if appErr != nil {
		t.Fatalf("ReserveRateLimit second returned app error: %v", appErr)
	}
	third, appErr := svc.ReserveRateLimit(context.Background(), domain.RequestActor{ID: 291}, "micro_question_daily", periodStart, periodEnd, 2)
	if appErr != nil {
		t.Fatalf("ReserveRateLimit third returned app error: %v", appErr)
	}
	if !first.Allowed || !second.Allowed || third.Allowed {
		t.Fatalf("allowed sequence = %v/%v/%v, want true/true/false", first.Allowed, second.Allowed, third.Allowed)
	}
	if third.Count != 3 || third.HardCap != 20 {
		t.Fatalf("third reservation = %+v, want count=3 hard_cap=20", third)
	}
}

func TestExperienceServiceProcessRetentionIncludesWorkerRunRetention(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		retentionRun: &domain.ExperienceRetentionRun{
			BehaviorDeleted:    2,
			RateLimitDeleted:   3,
			ObservedTombstoned: 4,
			WorkerRunDeleted:   5,
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{WorkerEnabled: true, RetentionDays: 120}, zap.NewNop())

	result, appErr := svc.ProcessRetention(context.Background(), now, 50)
	if appErr != nil {
		t.Fatalf("ProcessRetention returned app error: %v", appErr)
	}
	if result.WorkerRunDeleted != 5 {
		t.Fatalf("retention result = %+v, want worker run deletions", result)
	}
	if !stub.retentionPolicy.WorkerRunBefore.Equal(now.AddDate(0, 0, -30)) {
		t.Fatalf("worker run retention cutoff = %s, want %s", stub.retentionPolicy.WorkerRunBefore, now.AddDate(0, 0, -30))
	}
	if !stub.retentionPolicy.ObservedTerminalBefore.Equal(now.AddDate(0, 0, -120)) {
		t.Fatalf("observed retention cutoff = %s, want %s", stub.retentionPolicy.ObservedTerminalBefore, now.AddDate(0, 0, -120))
	}
	if len(stub.workerRuns) != 1 || stub.workerRuns[0].SkippedCount != 10 {
		t.Fatalf("worker run record = %+v, want deleted rows counted as skipped", stub.workerRuns)
	}
}

func TestExperienceServiceMicroQuestionEligibilityDoesNotConsumeRateLimit(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.MicroQuestionEligibility(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionEligibilityRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
	})
	if appErr != nil {
		t.Fatalf("MicroQuestionEligibility returned app error: %v", appErr)
	}
	if !result.Eligible || result.AnswerEventKey == "" {
		t.Fatalf("eligibility = %+v, want eligible with answer key", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for non-consuming eligibility", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRequiresCaptureEnabled(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       false,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.MicroQuestionEligibility(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionEligibilityRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
	})
	if appErr != nil {
		t.Fatalf("MicroQuestionEligibility returned app error: %v", appErr)
	}
	if result.Eligible || result.Reason != "disabled" {
		t.Fatalf("eligibility = %+v, want disabled when capture is disabled", result)
	}
	if stub.calls != 0 {
		t.Fatalf("repo calls = %d, want 0 when capture is disabled", stub.calls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRejectsActorMismatch(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(999),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.MicroQuestionEligibility(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionEligibilityRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
	})
	if appErr != nil {
		t.Fatalf("MicroQuestionEligibility returned app error: %v", appErr)
	}
	if result.Eligible || result.Reason != "suggestion_not_found" {
		t.Fatalf("eligibility = %+v, want non-eligible suggestion_not_found", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for actor mismatch", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRejectsTargetMismatch(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.MicroQuestionEligibility(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionEligibilityRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "asset",
		TargetID:          "42",
	})
	if appErr != nil {
		t.Fatalf("MicroQuestionEligibility returned app error: %v", appErr)
	}
	if result.Eligible || result.Reason != "target_mismatch" {
		t.Fatalf("eligibility = %+v, want non-eligible target_mismatch", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for target mismatch", stub.reserveCalls)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerIdempotentDoesNotReserveTwice(t *testing.T) {
	answerKey := buildExperienceMicroQuestionAnswerEventKey(291, "display-1", "stable-1", "task_detail", "task", "42")
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
		microAnswers: map[string]*domain.ExperienceMicroQuestionAnswer{
			answerKey: {AnswerEventKey: answerKey},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	answer, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		AnswerEventKey:      answerKey,
		SuggestionEventID:   "display-1",
		SuggestionStableKey: "stable-1",
		Surface:             "task_detail",
		TargetType:          "task",
		TargetID:            "42",
		AnswerValue:         domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:          "missing_context",
	})
	if appErr != nil {
		t.Fatalf("RecordMicroQuestionAnswer returned app error: %v", appErr)
	}
	if answer.AnswerEventKey != answerKey {
		t.Fatalf("answer key = %s, want %s", answer.AnswerEventKey, answerKey)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for idempotent already-answered request", stub.reserveCalls)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRejectsActorMismatchWithoutReserving(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(999),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	_, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want invalid request", appErr)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for actor mismatch", stub.reserveCalls)
	}
	if len(stub.microAnswers) != 0 {
		t.Fatalf("micro answers = %+v, want no write", stub.microAnswers)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRejectsUnknownSuggestionWithoutReserving(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	answer, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID:   "missing-display",
		SuggestionStableKey: "stable-1",
		Surface:             "task_detail",
		TargetType:          "task",
		TargetID:            "42",
		AnswerValue:         domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:          "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want invalid request", appErr)
	}
	if answer != nil {
		t.Fatalf("answer = %+v, want nil", answer)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 when suggestion is invalid", stub.reserveCalls)
	}
	if len(stub.microAnswers) != 0 {
		t.Fatalf("micro answers = %+v, want no write", stub.microAnswers)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRejectsIneligibleSuggestionWithoutReserving(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-1",
				AttributionEligible: false,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	_, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID:   "display-1",
		SuggestionStableKey: "stable-1",
		Surface:             "task_detail",
		TargetType:          "task",
		TargetID:            "42",
		AnswerValue:         domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:          "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want invalid request", appErr)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 when suggestion is ineligible", stub.reserveCalls)
	}
	if len(stub.microAnswers) != 0 {
		t.Fatalf("micro answers = %+v, want no write", stub.microAnswers)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerValidatesScopeAndCreatesAnswer(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-from-backend",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	answer, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr != nil {
		t.Fatalf("RecordMicroQuestionAnswer returned app error: %v", appErr)
	}
	expectedKey := buildExperienceMicroQuestionAnswerEventKey(291, "display-1", "stable-from-backend", "task_detail", "task", "42")
	if answer.AnswerEventKey != expectedKey || answer.SuggestionStableKey != "stable-from-backend" {
		t.Fatalf("answer = %+v, want backend stable key and deterministic answer key", answer)
	}
	if stub.reserveCalls != 1 {
		t.Fatalf("reserve calls = %d, want 1", stub.reserveCalls)
	}
	if got := stub.microAnswers[expectedKey]; got == nil || got.ReasonCode != "missing_context" {
		t.Fatalf("stored answer = %+v, want created answer", got)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRefundsQuotaOnDuplicateInsert(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-from-backend",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
		forceDuplicateAnswer: true,
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	answer, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr != nil {
		t.Fatalf("RecordMicroQuestionAnswer returned app error: %v", appErr)
	}
	if answer == nil || answer.AnswerEventKey == "" {
		t.Fatalf("answer = %+v, want idempotent answer", answer)
	}
	if stub.reserveCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("reserve/refund calls = %d/%d, want 1/1", stub.reserveCalls, stub.refundCalls)
	}
	periodStart, _ := experienceBeijingDayWindow(time.Now())
	limitKey := buildExperienceRateLimitKey(291, "micro_question_daily", periodStart)
	if got := stub.rateLimits[limitKey]; got == nil || got.Count != 0 {
		t.Fatalf("rate limit = %+v, want refunded to 0", got)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRefundsQuotaOnInsertError(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-from-backend",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
		createMicroAnswerErr: errors.New("insert failed"),
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	_, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want internal error", appErr)
	}
	if stub.reserveCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("reserve/refund calls = %d/%d, want 1/1", stub.reserveCalls, stub.refundCalls)
	}
	periodStart, _ := experienceBeijingDayWindow(time.Now())
	limitKey := buildExperienceRateLimitKey(291, "micro_question_daily", periodStart)
	if got := stub.rateLimits[limitKey]; got == nil || got.Count != 0 {
		t.Fatalf("rate limit = %+v, want refunded to 0", got)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRefundsQuotaOnRateLimited(t *testing.T) {
	periodStart, periodEnd := experienceBeijingDayWindow(time.Now())
	limitKey := buildExperienceRateLimitKey(291, "micro_question_daily", periodStart)
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-from-backend",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
		rateLimits: map[string]*domain.ExperienceRateLimitReservation{
			limitKey: {
				LimitKey:    limitKey,
				ActorID:     experienceInt64Ptr(291),
				BucketName:  "micro_question_daily",
				PeriodStart: periodStart,
				PeriodEnd:   periodEnd,
				Limit:       experienceMicroQuestionDailyLimit,
				HardCap:     experienceMicroQuestionDailyLimit * 10,
				Count:       experienceMicroQuestionDailyLimit,
				Allowed:     true,
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	_, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want invalid request", appErr)
	}
	if stub.reserveCalls != 1 || stub.refundCalls != 1 {
		t.Fatalf("reserve/refund calls = %d/%d, want 1/1", stub.reserveCalls, stub.refundCalls)
	}
	if got := stub.rateLimits[limitKey]; got == nil || got.Count != experienceMicroQuestionDailyLimit {
		t.Fatalf("rate limit = %+v, want denied reservation refunded to limit", got)
	}
	if len(stub.microAnswers) != 0 {
		t.Fatalf("micro answers = %+v, want no write when rate limited", stub.microAnswers)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRequiresCaptureEnabled(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID:   "display-1",
				SuggestionStableKey: "stable-from-backend",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       false,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	_, appErr := svc.RecordMicroQuestionAnswer(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionAnswerRequest{
		SuggestionEventID: "display-1",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
		AnswerValue:       domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:        "missing_context",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("RecordMicroQuestionAnswer appErr = %+v, want permission denied", appErr)
	}
	if stub.reserveCalls != 0 || len(stub.microAnswers) != 0 {
		t.Fatalf("reserveCalls=%d microAnswers=%d, want no side effects", stub.reserveCalls, len(stub.microAnswers))
	}
}

func TestExperienceMicroQuestionReasonCodesStayInSync(t *testing.T) {
	codes := []string{
		"temporarily_not_needed",
		"will_handle_later",
		"already_handled",
		"not_relevant",
		"missing_context",
		"stage_not_applicable",
		"customer_special_case",
		"suggestion_outdated",
	}
	migration, err := os.ReadFile("../db/migrations/100_v1_14_experience_phase2_closed_loop.sql")
	if err != nil {
		t.Fatalf("read experience migration: %v", err)
	}
	openapi, err := os.ReadFile("../docs/api/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	migrationCodes := extractMicroQuestionReasonCodesFromMigration(t, string(migration))
	openAPICodes := extractMicroQuestionReasonCodesFromOpenAPI(t, string(openapi))
	for _, code := range codes {
		if !isAllowedExperienceMicroQuestionReason(code) {
			t.Fatalf("service whitelist rejects micro question reason %q", code)
		}
	}
	assertSameStringSet(t, "migration micro-question reasons", codes, migrationCodes)
	assertSameStringSet(t, "openapi micro-question reasons", codes, openAPICodes)
}

func extractMicroQuestionReasonCodesFromMigration(t *testing.T, content string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?m)(?:SELECT|UNION ALL SELECT)\s+'ai_suggestion_micro_question'(?:\s+AS\s+scene)?,\s+'([^']+)'`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		t.Fatal("migration micro-question reason seed not found")
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func extractMicroQuestionReasonCodesFromOpenAPI(t *testing.T, content string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?s)reason_code:\s*\n\s*type:\s*string\s*\n\s*description:[^\n]*\n\s*enum:\s*\[([^\]]+)\]`)
	match := re.FindStringSubmatch(content)
	if len(match) != 2 {
		t.Fatal("openapi micro-question reason_code enum not found")
	}
	parts := strings.Split(match[1], ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func assertSameStringSet(t *testing.T, label string, want []string, got []string) {
	t.Helper()
	want = append([]string(nil), want...)
	got = append([]string(nil), got...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, "\x00") != strings.Join(got, "\x00") {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestExperienceServiceRecordReviewDecisionUpdatesCandidateStatus(t *testing.T) {
	stub := &experienceRepoStub{
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:  "review-1",
			ItemType: "attribution_candidate",
			Status:   domain.ExperienceReviewItemStatusOpen,
			Priority: "medium",
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true}, zap.NewNop())

	decision, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
		Decision:   domain.ExperienceReviewDecisionApprove,
		ReasonCode: "verified",
	})
	if appErr != nil {
		t.Fatalf("RecordReviewDecision returned app error: %v", appErr)
	}
	if decision.Decision != domain.ExperienceReviewDecisionApprove {
		t.Fatalf("decision = %+v, want approve", decision)
	}
	if len(stub.reviewDecisions) != 1 {
		t.Fatalf("review decisions = %d, want 1", len(stub.reviewDecisions))
	}
	if stub.reviewItems[0].Status != domain.ExperienceReviewItemStatusApproved {
		t.Fatalf("review item status = %s, want approved", stub.reviewItems[0].Status)
	}
}

func TestExperienceServiceRecordReviewDecisionMapsValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{
			name: "not open",
			err:  errors.New("experience review item is not open"),
			code: domain.ErrCodeConflict,
		},
		{
			name: "not materializable",
			err:  errors.New("experience review target is not materializable"),
			code: domain.ErrCodeInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &experienceRepoStub{createReviewDecisionErr: tt.err}
			svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true}, zap.NewNop())

			_, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
				Decision: domain.ExperienceReviewDecisionApprove,
			})
			if appErr == nil || appErr.Code != tt.code {
				t.Fatalf("RecordReviewDecision appErr = %+v, want code %s", appErr, tt.code)
			}
		})
	}
}

type experienceRepoStub struct {
	calls                     int
	enqueued                  *domain.ExperienceOutboxEvent
	enqueuedEvents            []*domain.ExperienceOutboxEvent
	aiEvent                   *domain.AISuggestionEvent
	aiEventsByID              map[string]*domain.AISuggestionEvent
	feedbacks                 []*domain.AISuggestionFeedback
	behaviorEvents            []*domain.ExperienceBehaviorEvent
	watermarks                map[string]*domain.ExperienceWorkerWatermark
	auditRows                 []*domain.ExperienceOutcomeEventRow
	moduleRows                []*domain.ExperienceOutcomeEventRow
	auditErr                  error
	taskSnapshots             []*domain.ExperienceOutcomeSnapshotRow
	assetSnapshots            []*domain.ExperienceOutcomeSnapshotRow
	detailSnapshots           []*domain.ExperienceOutcomeSnapshotRow
	skuItemSnapshots          []*domain.ExperienceOutcomeSnapshotRow
	observed                  map[string]*domain.ExperienceObservedEntityState
	retentionRun              *domain.ExperienceRetentionRun
	retentionPolicy           repo.ExperienceRetentionPolicy
	workerRuns                []*domain.ExperienceWorkerRunRecord
	attributionOutcomes       []*domain.ExperienceAttributionOutcome
	recentAttributionOutcomes []*domain.ExperienceAttributionOutcome
	attributionCandidates     []*domain.ExperienceAttributionCandidate
	attributions              []*domain.ExperienceAttribution
	rateLimits                map[string]*domain.ExperienceRateLimitReservation
	reserveCalls              int
	refundCalls               int
	microAnswers              map[string]*domain.ExperienceMicroQuestionAnswer
	forceDuplicateAnswer      bool
	createMicroAnswerErr      error
	reviewItems               []*domain.ExperienceReviewItem
	reviewDecisions           []*domain.ExperienceReviewDecision
	createReviewDecisionErr   error
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
	if event != nil && strings.TrimSpace(event.SuggestionEventID) != "" {
		if s.aiEventsByID == nil {
			s.aiEventsByID = make(map[string]*domain.AISuggestionEvent)
		}
		copied := *event
		s.aiEventsByID[event.SuggestionEventID] = &copied
	}
	return nil
}

func (s *experienceRepoStub) GetAISuggestionEventByEventID(_ context.Context, suggestionEventID string) (*domain.AISuggestionEvent, error) {
	s.calls++
	if s.aiEventsByID != nil {
		if event := s.aiEventsByID[strings.TrimSpace(suggestionEventID)]; event != nil {
			copied := *event
			return &copied, nil
		}
	}
	if s.aiEvent != nil && s.aiEvent.SuggestionEventID == strings.TrimSpace(suggestionEventID) {
		copied := *s.aiEvent
		return &copied, nil
	}
	return nil, nil
}

func (s *experienceRepoStub) CreateExperienceBehaviorEvents(_ context.Context, events []*domain.ExperienceBehaviorEvent) (int, error) {
	s.calls++
	s.behaviorEvents = append(s.behaviorEvents, events...)
	return len(events), nil
}

func (s *experienceRepoStub) CreateAISuggestionFeedback(_ context.Context, feedback *domain.AISuggestionFeedback) (int64, error) {
	s.calls++
	copied := *feedback
	copied.ID = int64(len(s.feedbacks) + 1)
	s.feedbacks = append(s.feedbacks, &copied)
	return copied.ID, nil
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
	if s.auditErr != nil {
		return nil, s.auditErr
	}
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

func (s *experienceRepoStub) RunExperienceRetention(_ context.Context, policy repo.ExperienceRetentionPolicy) (*domain.ExperienceRetentionRun, error) {
	s.calls++
	s.retentionPolicy = policy
	if s.retentionRun != nil {
		return s.retentionRun, nil
	}
	return &domain.ExperienceRetentionRun{}, nil
}

func (s *experienceRepoStub) CreateExperienceWorkerRun(_ context.Context, run *domain.ExperienceWorkerRunRecord) error {
	s.calls++
	if run != nil {
		copied := *run
		s.workerRuns = append(s.workerRuns, &copied)
	}
	return nil
}

func (s *experienceRepoStub) ListRecentExperienceWorkerRuns(context.Context, int) ([]*domain.ExperienceWorkerRunRecord, error) {
	s.calls++
	return s.workerRuns, nil
}

func (s *experienceRepoStub) ListExperienceAttributionOutcomes(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceAttributionOutcome, error) {
	s.calls++
	return s.attributionOutcomes, nil
}

func (s *experienceRepoStub) ListRecentExperienceAttributionOutcomes(context.Context, time.Time, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceAttributionOutcome, error) {
	s.calls++
	return s.recentAttributionOutcomes, nil
}

func (s *experienceRepoStub) ListExperienceAttributionCandidates(context.Context, *domain.ExperienceAttributionOutcome, time.Duration, int) ([]*domain.ExperienceAttributionCandidate, error) {
	s.calls++
	return s.attributionCandidates, nil
}

func (s *experienceRepoStub) CreateExperienceAttribution(_ context.Context, attribution *domain.ExperienceAttribution) error {
	s.calls++
	if attribution != nil {
		copied := *attribution
		s.attributions = append(s.attributions, &copied)
	}
	return nil
}

func (s *experienceRepoStub) ReserveExperienceRateLimit(_ context.Context, req repo.ExperienceRateLimitRequest) (*domain.ExperienceRateLimitReservation, error) {
	s.calls++
	s.reserveCalls++
	if s.rateLimits == nil {
		s.rateLimits = make(map[string]*domain.ExperienceRateLimitReservation)
	}
	item := s.rateLimits[req.LimitKey]
	if item == nil {
		item = &domain.ExperienceRateLimitReservation{
			LimitKey:    req.LimitKey,
			ActorID:     req.ActorID,
			BucketName:  req.BucketName,
			PeriodStart: req.PeriodStart,
			PeriodEnd:   req.PeriodEnd,
			Limit:       req.Limit,
			HardCap:     req.HardCap,
		}
		s.rateLimits[req.LimitKey] = item
	}
	if item.Count < item.HardCap {
		item.Count++
	}
	item.Allowed = item.Count <= req.Limit
	copied := *item
	return &copied, nil
}

func (s *experienceRepoStub) RefundExperienceRateLimit(_ context.Context, limitKey string) error {
	s.calls++
	s.refundCalls++
	if s.rateLimits == nil {
		return nil
	}
	item := s.rateLimits[strings.TrimSpace(limitKey)]
	if item != nil && item.Count > 0 {
		item.Count--
		item.Allowed = item.Limit <= 0 || item.Count <= item.Limit
	}
	return nil
}

func (s *experienceRepoStub) GetExperienceRateLimit(_ context.Context, limitKey string, limit int) (*domain.ExperienceRateLimitReservation, error) {
	s.calls++
	if s.rateLimits == nil {
		return nil, nil
	}
	item := s.rateLimits[strings.TrimSpace(limitKey)]
	if item == nil {
		return nil, nil
	}
	copied := *item
	copied.Limit = limit
	copied.Allowed = limit <= 0 || copied.Count < limit
	return &copied, nil
}

func (s *experienceRepoStub) CreateExperienceMicroQuestionAnswer(_ context.Context, answer *domain.ExperienceMicroQuestionAnswer) (bool, error) {
	s.calls++
	if s.createMicroAnswerErr != nil {
		return false, s.createMicroAnswerErr
	}
	if s.microAnswers == nil {
		s.microAnswers = make(map[string]*domain.ExperienceMicroQuestionAnswer)
	}
	if answer != nil {
		if s.forceDuplicateAnswer {
			return false, nil
		}
		if s.microAnswers[strings.TrimSpace(answer.AnswerEventKey)] != nil {
			return false, nil
		}
		copied := *answer
		s.microAnswers[answer.AnswerEventKey] = &copied
		return true, nil
	}
	return false, nil
}

func (s *experienceRepoStub) HasExperienceMicroQuestionAnswer(_ context.Context, answerEventKey string) (bool, error) {
	s.calls++
	if s.microAnswers == nil {
		return false, nil
	}
	return s.microAnswers[strings.TrimSpace(answerEventKey)] != nil, nil
}

func (s *experienceRepoStub) CreateExperienceReviewItem(_ context.Context, item *domain.ExperienceReviewItem) error {
	s.calls++
	if item == nil {
		return nil
	}
	for _, existing := range s.reviewItems {
		if existing.ItemKey == item.ItemKey {
			if existing.Status == "" || existing.Status == domain.ExperienceReviewItemStatusOpen {
				existing.Priority = item.Priority
				existing.EvidenceSummary = item.EvidenceSummary
			}
			return nil
		}
	}
	copied := *item
	s.reviewItems = append(s.reviewItems, &copied)
	return nil
}

func (s *experienceRepoStub) ListExperienceReviewItems(_ context.Context, filter repo.ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, int64, error) {
	s.calls++
	items := make([]*domain.ExperienceReviewItem, 0, len(s.reviewItems))
	for _, item := range s.reviewItems {
		if item == nil {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.ItemType != "" && item.ItemType != filter.ItemType {
			continue
		}
		copied := *item
		items = append(items, &copied)
	}
	return items, int64(len(items)), nil
}

func (s *experienceRepoStub) CreateExperienceReviewDecision(_ context.Context, decision *domain.ExperienceReviewDecision, nextStatus string) error {
	s.calls++
	if s.createReviewDecisionErr != nil {
		return s.createReviewDecisionErr
	}
	if decision != nil {
		copied := *decision
		s.reviewDecisions = append(s.reviewDecisions, &copied)
		for _, item := range s.reviewItems {
			if item != nil && item.ItemKey == decision.ReviewItemKey {
				item.Status = nextStatus
				return nil
			}
		}
	}
	return nil
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
