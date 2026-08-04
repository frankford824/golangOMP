package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func TestExperienceServiceClientConfigRequiresCaptureForAIFeedback(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:         true,
		CaptureEnabled:    false,
		AIFeedbackEnabled: true,
		EnabledSurfaces:   []string{"task_detail"},
	}, zap.NewNop())

	config := svc.ClientConfig()
	if config.AIFeedbackEnabled {
		t.Fatalf("client config ai_feedback_enabled = true, want false when capture is disabled")
	}

	svc = NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:         true,
		CaptureEnabled:    true,
		AIFeedbackEnabled: true,
		EnabledSurfaces:   []string{"task_detail"},
	}, zap.NewNop())
	config = svc.ClientConfig()
	if !config.AIFeedbackEnabled {
		t.Fatalf("client config ai_feedback_enabled = false, want true when UI/capture/feedback are enabled")
	}
}

func TestExperienceServiceRuntimeFlagsExposeRuntimeConfigFileStatus(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/experience-flags.json"
	if err := os.WriteFile(path, []byte(`{"experience_ui_enabled":true,"experience_behavior_sample_rate":0}`), 0o600); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}
	svc := NewExperienceService(&experienceRepoStub{}, ExperienceServiceConfig{
		RuntimeConfigFile:  path,
		BehaviorSampleRate: 1,
	}, zap.NewNop())

	flags := svc.RuntimeFlags()
	if !flags.RuntimeConfigLoaded || flags.RuntimeConfigError != "" {
		t.Fatalf("runtime config flags = %+v, want loaded without error", flags)
	}
	if !flags.UIEnabled || flags.BehaviorSampleRate != 0 {
		t.Fatalf("runtime config override flags = %+v, want ui enabled and sample rate 0", flags)
	}
}

func TestExperienceServiceRuntimeFlagsExposeRuntimeConfigFileError(t *testing.T) {
	svc := NewExperienceService(&experienceRepoStub{}, ExperienceServiceConfig{
		RuntimeConfigFile: "/tmp/yongbo-missing-experience-runtime-config.json",
		UIEnabled:         true,
	}, zap.NewNop())

	flags := svc.RuntimeFlags()
	if flags.RuntimeConfigLoaded || flags.RuntimeConfigError == "" {
		t.Fatalf("runtime config flags = %+v, want visible read error", flags)
	}
	if !flags.UIEnabled {
		t.Fatalf("ui flag should fall back to env/default values when runtime file is missing")
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
		EnabledSurfaces:        []string{"task_detail"},
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

func TestExperienceServiceBehaviorEventsRespectEnabledSurfaces(t *testing.T) {
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:              true,
		CaptureEnabled:         true,
		BehaviorCaptureEnabled: true,
		EnabledSurfaces:        []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.RecordBehaviorEvents(context.Background(), domain.RequestActor{ID: 291}, ExperienceBehaviorBatchRequest{
		Events: []ExperienceBehaviorEventRequest{
			{
				ClientEventID:       "client-1",
				Surface:             "task_detail",
				Action:              domain.ExperienceBehaviorActionImpression,
				TargetType:          "task",
				TargetID:            "123",
				SuggestionEventID:   "event-1",
				SuggestionStableKey: "stable-1",
			},
			{
				ClientEventID:       "client-2",
				Surface:             "asset_center",
				Action:              domain.ExperienceBehaviorActionImpression,
				TargetType:          "asset",
				TargetID:            "7001",
				SuggestionEventID:   "event-2",
				SuggestionStableKey: "stable-2",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("RecordBehaviorEvents returned app error: %v", appErr)
	}
	if result.Received != 2 || result.Inserted != 1 {
		t.Fatalf("behavior result = %+v, want received=2 inserted=1", result)
	}
	if len(stub.behaviorEvents) != 1 || stub.behaviorEvents[0].Surface != "task_detail" {
		t.Fatalf("behavior events = %+v, want only task_detail event", stub.behaviorEvents)
	}
}

func TestExperienceServiceClientConfigAllowsZeroBehaviorSampleRate(t *testing.T) {
	svc := NewExperienceService(&experienceRepoStub{}, ExperienceServiceConfig{
		UIEnabled:              true,
		CaptureEnabled:         true,
		BehaviorCaptureEnabled: true,
		BehaviorSampleRate:     0,
	}, zap.NewNop())

	config := svc.ClientConfig()
	if !config.BehaviorCaptureEnabled {
		t.Fatalf("behavior_capture_enabled = false, want enabled")
	}
	if config.BehaviorSampleRate != 0 {
		t.Fatalf("behavior sample rate = %v, want 0", config.BehaviorSampleRate)
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
			TargetType:      "asset",
			TargetID:        "77",
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
	if event.TargetType != "asset" || event.TargetID != "77" {
		t.Fatalf("event target = %s/%s", event.TargetType, event.TargetID)
	}
}

func TestExperienceServiceAssetReviewObserverCapturesArchiveAndCleanOnlyChanges(t *testing.T) {
	tests := []struct {
		name          string
		previousValue json.RawMessage
		currentValue  json.RawMessage
		wantAction    string
		wantOutcome   string
		wantField     string
	}{
		{
			name:          "archive only",
			previousValue: json.RawMessage(`{"approved_at":"2026-06-30T08:00:00Z","archived_at":null,"cleaned_at":null,"flow_review_status":"approved","is_archived":false,"rejected_at":null,"superseded_at":null,"superseded_by_version_id":null}`),
			currentValue:  json.RawMessage(`{"approved_at":"2026-06-30T08:00:00Z","archived_at":"2026-06-30T08:04:00Z","cleaned_at":null,"flow_review_status":"approved","is_archived":true,"rejected_at":null,"superseded_at":null,"superseded_by_version_id":null}`),
			wantAction:    "asset_archive_status_changed",
			wantOutcome:   "archived",
			wantField:     "is_archived",
		},
		{
			name:          "clean only",
			previousValue: json.RawMessage(`{"approved_at":"2026-06-30T08:00:00Z","archived_at":null,"cleaned_at":null,"flow_review_status":"approved","is_archived":false,"rejected_at":null,"superseded_at":null,"superseded_by_version_id":null}`),
			currentValue:  json.RawMessage(`{"approved_at":"2026-06-30T08:00:00Z","archived_at":null,"cleaned_at":"2026-06-30T08:05:00Z","flow_review_status":"approved","is_archived":false,"rejected_at":null,"superseded_at":null,"superseded_by_version_id":null}`),
			wantAction:    "asset_cleaned_at_changed",
			wantOutcome:   "cleaned",
			wantField:     "cleaned_at",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceUpdatedAt := time.Date(2026, 6, 30, 8, 5, 0, 0, time.UTC)
			taskID := int64(42)
			previousValue := canonicalExperienceJSON(tt.previousValue)
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
					ObservedValue:   tt.currentValue,
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
				t.Fatalf("observer result = %+v, want one archive/clean event", result)
			}
			event := stub.enqueuedEvents[0]
			if event.Action != tt.wantAction || event.Outcome != tt.wantOutcome {
				t.Fatalf("event action/outcome = %s/%s, want %s/%s", event.Action, event.Outcome, tt.wantAction, tt.wantOutcome)
			}
			if !strings.Contains(string(event.Payload), `"field":"`+tt.wantField+`"`) {
				t.Fatalf("event payload = %s, want changed field %s", event.Payload, tt.wantField)
			}
		})
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

func TestExperienceServiceOutcomeEventObserverSkipsInvalidPayloadAndAdvancesWatermark(t *testing.T) {
	eventTime := time.Date(2026, 6, 30, 9, 5, 0, 0, time.UTC)
	oversizedPayload := json.RawMessage(`{"blob":"` + strings.Repeat("x", experiencePayloadMaxBytes) + `"}`)
	stub := &experienceRepoStub{
		auditRows: []*domain.ExperienceOutcomeEventRow{
			{
				ID:              1,
				EventKey:        "audit-bad",
				SourceName:      experienceSourceAuditRecords,
				SourceID:        "audit:bad",
				Action:          "audit_payload_changed",
				Outcome:         "invalid",
				EventTime:       eventTime,
				Payload:         oversizedPayload,
				SourceWatermark: "wm-1",
			},
			{
				ID:              2,
				EventKey:        "audit-good",
				SourceName:      experienceSourceAuditRecords,
				SourceID:        "audit:good",
				Action:          "audit_approved",
				Outcome:         "approved",
				EventTime:       eventTime.Add(time.Second),
				Payload:         json.RawMessage(`{"result":"approved"}`),
				SourceWatermark: "wm-2",
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Scanned != 2 || result.Failed != 1 || result.Enqueued != 1 {
		t.Fatalf("observer result = %+v, want scanned=2 failed=1 enqueued=1", result)
	}
	if len(stub.enqueuedEvents) != 1 || stub.enqueuedEvents[0].EventKey != "audit-good" {
		t.Fatalf("enqueued events = %+v, want only valid second row", stub.enqueuedEvents)
	}
	watermark := stub.watermarks[domain.ExperienceWorkerOutcomeObserver+":"+experienceSourceAuditRecords]
	if watermark == nil || watermark.LastSeenID != 2 || watermark.SourceWatermark != "wm-2" {
		t.Fatalf("watermark = %+v, want advanced past poison row", watermark)
	}
	if len(stub.workerRuns) == 0 || stub.workerRuns[0].Status != "partial" {
		t.Fatalf("worker runs = %+v, want partial audit run", stub.workerRuns)
	}
	if stub.workerRuns[0].LastError == "" || !strings.Contains(string(stub.workerRuns[0].Metadata), `"event_key":"audit-bad"`) {
		t.Fatalf("worker run poison details = error %q metadata %s, want event locator", stub.workerRuns[0].LastError, stub.workerRuns[0].Metadata)
	}
}

func TestExperienceServiceOutcomeObserverUsesRunStartCaptureSnapshotForEnqueue(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "experience-runtime.json")
	if err := os.WriteFile(runtimePath, []byte(`{"experience_capture_enabled":true}`), 0o600); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}
	eventTime := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		auditRows: []*domain.ExperienceOutcomeEventRow{{
			ID:              1,
			EventKey:        "audit-1",
			SourceName:      experienceSourceAuditRecords,
			SourceID:        "audit:1",
			Action:          "audit_approved",
			Outcome:         "approved",
			EventTime:       eventTime,
			Payload:         json.RawMessage(`{"result":"approved"}`),
			SourceWatermark: "wm-1",
		}},
		onGetWatermark: func(workerName, sourceName string) {
			if workerName == domain.ExperienceWorkerOutcomeObserver && sourceName == experienceSourceAuditRecords {
				_ = os.WriteFile(runtimePath, []byte(`{"experience_capture_enabled":false}`), 0o600)
			}
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		CaptureEnabled:    true,
		WorkerEnabled:     true,
		RuntimeConfigFile: runtimePath,
	}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Enqueued != 1 || len(stub.enqueuedEvents) != 1 {
		t.Fatalf("observer result=%+v enqueuedEvents=%d, want actual outbox enqueue despite mid-run flag change", result, len(stub.enqueuedEvents))
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

func TestExperienceServiceProcessAttributionsCreatesOnlyBestReviewItemForOutcome(t *testing.T) {
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
		attributionCandidates: []*domain.ExperienceAttributionCandidate{
			{
				SuggestionEventID:   "suggestion-weak",
				SuggestionStableKey: "task|weak|42",
				SuggestionType:      "task_next_action",
				SuggestionID:        "weak",
				TargetType:          "task",
				TargetID:            "42",
				DisplayedAt:         outcomeAt.Add(-2 * time.Hour),
				BehaviorCount:       1,
				BehaviorScore:       2,
			},
			{
				SuggestionEventID:   "suggestion-best",
				SuggestionStableKey: "task|best|42",
				SuggestionType:      "task_next_action",
				SuggestionID:        "best",
				TargetType:          "task",
				TargetID:            "42",
				DisplayedAt:         outcomeAt.Add(-20 * time.Minute),
				BehaviorCount:       1,
				BehaviorScore:       5,
				FeedbackValue:       domain.ExperienceFeedbackAccepted,
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Created != 2 || len(stub.attributions) != 2 {
		t.Fatalf("result=%+v attributions=%d, want two attribution records", result, len(stub.attributions))
	}
	if len(stub.reviewItems) != 1 {
		t.Fatalf("review items = %+v, want exactly one best candidate", stub.reviewItems)
	}
	if stub.reviewItems[0].ItemKey != "attribution:outcome:tasks:42:completed" {
		t.Fatalf("review item key = %s, want outcome-scoped review item", stub.reviewItems[0].ItemKey)
	}
	if !strings.Contains(string(stub.reviewItems[0].EvidenceSummary), "suggestion-best") {
		t.Fatalf("review item evidence = %s, want best suggestion evidence", stub.reviewItems[0].EvidenceSummary)
	}
}

func TestExperienceServiceProcessAttributionsUpdatesSingleReviewItemWhenBestCandidateChanges(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	outcome := &domain.ExperienceAttributionOutcome{
		ID:         9,
		EventKey:   "outcome:tasks:42:completed",
		EventTime:  outcomeAt,
		SourceType: experienceSourceTaskStatusSnapshot,
		Action:     "task_status_changed",
		Outcome:    "Completed",
		TaskID:     experienceInt64Ptr(42),
		TargetType: "task",
		TargetID:   "42",
	}
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{outcome},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "suggestion-a",
			SuggestionStableKey: "task|a|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "a",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         outcomeAt.Add(-30 * time.Minute),
			BehaviorCount:       1,
			BehaviorScore:       5,
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	if _, appErr := svc.ProcessAttributions(context.Background(), 10); appErr != nil {
		t.Fatalf("ProcessAttributions first returned app error: %v", appErr)
	}
	stub.attributionCandidates = []*domain.ExperienceAttributionCandidate{{
		SuggestionEventID:   "suggestion-b",
		SuggestionStableKey: "task|b|42",
		SuggestionType:      "task_next_action",
		SuggestionID:        "b",
		TargetType:          "task",
		TargetID:            "42",
		DisplayedAt:         outcomeAt.Add(-10 * time.Minute),
		BehaviorCount:       1,
		BehaviorScore:       5,
		FeedbackValue:       domain.ExperienceFeedbackAccepted,
	}}
	if _, appErr := svc.ProcessAttributions(context.Background(), 10); appErr != nil {
		t.Fatalf("ProcessAttributions second returned app error: %v", appErr)
	}
	if len(stub.reviewItems) != 1 {
		t.Fatalf("review items = %+v, want one outcome-scoped item after reprocess", stub.reviewItems)
	}
	if stub.reviewItems[0].ItemKey != "attribution:outcome:tasks:42:completed" {
		t.Fatalf("review item key = %s, want stable outcome-scoped key", stub.reviewItems[0].ItemKey)
	}
	evidence := string(stub.reviewItems[0].EvidenceSummary)
	if !strings.Contains(evidence, "suggestion-b") || strings.Contains(evidence, "suggestion-a") {
		t.Fatalf("review evidence = %s, want updated best candidate evidence", evidence)
	}
}

func TestExperienceServiceProcessAttributionsReopensNeedsMoreDataReviewItemWithNewEvidence(t *testing.T) {
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
			SuggestionEventID:   "suggestion-accepted",
			SuggestionStableKey: "task|accepted|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "accepted",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         outcomeAt.Add(-15 * time.Minute),
			BehaviorCount:       1,
			BehaviorScore:       5,
			FeedbackValue:       domain.ExperienceFeedbackAccepted,
		}},
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:         "attribution:outcome:tasks:42:completed",
			ItemType:        "attribution_candidate",
			Status:          domain.ExperienceReviewItemStatusNeedsMoreData,
			Priority:        "medium",
			EvidenceSummary: []byte(`{"suggestion":{"event_id":"old"}}`),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	if _, appErr := svc.ProcessAttributions(context.Background(), 10); appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if len(stub.reviewItems) != 1 {
		t.Fatalf("review items = %+v, want one reopened item", stub.reviewItems)
	}
	if stub.reviewItems[0].Status != domain.ExperienceReviewItemStatusOpen {
		t.Fatalf("review item status = %s, want reopened open", stub.reviewItems[0].Status)
	}
	evidence := string(stub.reviewItems[0].EvidenceSummary)
	if !strings.Contains(evidence, "suggestion-accepted") || strings.Contains(evidence, `"old"`) {
		t.Fatalf("review evidence = %s, want new accepted evidence", evidence)
	}
}

func TestExperienceServiceProcessAttributionsKeepsNeedsMoreDataWhenEvidenceUnchanged(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	outcome := &domain.ExperienceAttributionOutcome{
		ID:         9,
		EventKey:   "outcome:tasks:42:completed",
		EventTime:  outcomeAt,
		SourceType: experienceSourceTaskStatusSnapshot,
		Action:     "task_status_changed",
		Outcome:    "Completed",
		TaskID:     experienceInt64Ptr(42),
		TargetType: "task",
		TargetID:   "42",
	}
	candidate := &domain.ExperienceAttributionCandidate{
		SuggestionEventID:   "suggestion-accepted",
		SuggestionStableKey: "task|accepted|42",
		SuggestionType:      "task_next_action",
		SuggestionID:        "accepted",
		TargetType:          "task",
		TargetID:            "42",
		DisplayedAt:         outcomeAt.Add(-15 * time.Minute),
		BehaviorCount:       1,
		BehaviorScore:       5,
		FeedbackValue:       domain.ExperienceFeedbackAccepted,
	}
	attribution := buildExperienceAttribution(outcome, candidate, outcomeAt.Add(time.Minute))
	reviewItem := buildExperienceReviewItem(attribution)
	if reviewItem == nil {
		t.Fatal("review item = nil, want materializable attribution")
	}
	stub := &experienceRepoStub{
		attributionOutcomes:   []*domain.ExperienceAttributionOutcome{outcome},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{candidate},
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:         reviewItem.ItemKey,
			ItemType:        reviewItem.ItemType,
			Status:          domain.ExperienceReviewItemStatusNeedsMoreData,
			Priority:        reviewItem.Priority,
			EvidenceSummary: append([]byte(nil), reviewItem.EvidenceSummary...),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	if _, appErr := svc.ProcessAttributions(context.Background(), 10); appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if len(stub.reviewItems) != 1 {
		t.Fatalf("review items = %+v, want one item", stub.reviewItems)
	}
	if stub.reviewItems[0].Status != domain.ExperienceReviewItemStatusNeedsMoreData {
		t.Fatalf("review item status = %s, want unchanged needs_more_data", stub.reviewItems[0].Status)
	}
	if string(stub.reviewItems[0].EvidenceSummary) != string(reviewItem.EvidenceSummary) {
		t.Fatalf("review evidence changed = %s, want unchanged %s", stub.reviewItems[0].EvidenceSummary, reviewItem.EvidenceSummary)
	}
}

func TestExperienceServiceProcessAttributionsDoesNotReviewRejectedFeedbackDespiteJump(t *testing.T) {
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
			SuggestionEventID:   "suggestion-rejected",
			SuggestionStableKey: "task|rejected|42",
			SuggestionType:      "task_next_action",
			SuggestionID:        "rejected",
			TargetType:          "task",
			TargetID:            "42",
			DisplayedAt:         outcomeAt.Add(-20 * time.Minute),
			BehaviorCount:       2,
			BehaviorScore:       5,
			BehaviorActions:     []string{domain.ExperienceBehaviorActionJump, domain.ExperienceBehaviorActionDismiss},
			FeedbackValue:       domain.ExperienceFeedbackRejected,
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Created != 1 || len(stub.attributions) != 1 {
		t.Fatalf("result=%+v attributions=%d, want one attribution record", result, len(stub.attributions))
	}
	if stub.attributions[0].Status != domain.ExperienceAttributionStatusRejected {
		t.Fatalf("attribution status=%s score=%f, want rejected attribution for explicit negative feedback", stub.attributions[0].Status, stub.attributions[0].Score)
	}
	if len(stub.reviewItems) != 0 {
		t.Fatalf("review items = %+v, want no review item for rejected feedback", stub.reviewItems)
	}
	if !experienceAttributionSupportsMicroQuestion(stub.attributions[0]) {
		t.Fatalf("attribution evidence = %s, want rejected feedback to support micro question", stub.attributions[0].EvidenceSummary)
	}
}

func TestExperienceServiceProcessAttributionsPreservesBehaviorActionsForMicroQuestion(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	displayedAt := outcomeAt.Add(-30 * time.Minute)
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
			DisplayedAt:         displayedAt,
			BehaviorCount:       2,
			BehaviorScore:       -2,
			BehaviorActions:     []string{domain.ExperienceBehaviorActionIgnoredAfter, domain.ExperienceBehaviorActionVisible},
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Created != 1 || len(stub.attributions) != 1 {
		t.Fatalf("result=%+v attributions=%d, want one attribution", result, len(stub.attributions))
	}
	if stub.attributions[0].Status != domain.ExperienceAttributionStatusRejected || len(stub.reviewItems) != 0 {
		t.Fatalf("attribution=%+v reviewItems=%+v, want rejected micro-question candidate only", stub.attributions[0], stub.reviewItems)
	}
	if !experienceAttributionSupportsMicroQuestion(stub.attributions[0]) {
		t.Fatalf("attribution evidence = %s, want ignored action to support micro question", stub.attributions[0].EvidenceSummary)
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

func TestExperienceServiceProcessAttributionsCreatesReviewItemForCanonicalAssetTarget(t *testing.T) {
	outcomeAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:         9,
			EventKey:   "outcome:asset:77:approved",
			EventTime:  outcomeAt,
			SourceType: experienceSourceTaskAssetReviewSnapshot,
			Action:     "asset_review_status_changed",
			Outcome:    "approved",
			TaskID:     experienceInt64Ptr(42),
			TargetType: "asset",
			TargetID:   "77",
		}},
		attributionCandidates: []*domain.ExperienceAttributionCandidate{{
			SuggestionEventID:   "asset-display-1",
			SuggestionStableKey: "asset|review|workflow|asset|77",
			SuggestionType:      "asset",
			SuggestionID:        "review-asset",
			TargetType:          "asset",
			TargetID:            "77",
			DisplayedAt:         outcomeAt.Add(-30 * time.Minute),
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
		t.Fatalf("result=%+v attributions=%d, want canonical asset attribution", result, len(stub.attributions))
	}
	if len(stub.reviewItems) != 1 || stub.reviewItems[0].ItemKey != "attribution:outcome:asset:77:approved" || !strings.Contains(string(stub.reviewItems[0].EvidenceSummary), "asset-display-1") {
		t.Fatalf("review items = %+v, want materializable asset review item", stub.reviewItems)
	}
}

func TestExperienceServiceProcessOutcomeObserversSkipsWhenWorkerLockHeld(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		workerLockBlocked: true,
		auditRows: []*domain.ExperienceOutcomeEventRow{{
			ID:           1,
			EventKey:     "audit:1",
			SourceName:   experienceSourceAuditRecords,
			SourceID:     "1",
			Action:       "audit_changed",
			Outcome:      "rejected",
			EventTime:    now,
			ObservedFrom: experienceSourceAuditRecords,
			ObservedID:   "1",
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessOutcomeObservers returned app error: %v", appErr)
	}
	if result.Scanned != 0 || len(stub.enqueuedEvents) != 0 {
		t.Fatalf("result=%+v enqueued=%d, want skipped while lock is held", result, len(stub.enqueuedEvents))
	}
	if strings.Join(stub.workerLockNames, ",") != domain.ExperienceWorkerOutcomeObserver {
		t.Fatalf("worker locks = %v, want outcome observer lock", stub.workerLockNames)
	}
	if len(stub.workerRuns) != 1 || stub.workerRuns[0].Status != "locked" || stub.workerRuns[0].SkippedCount != 1 {
		t.Fatalf("worker runs = %+v, want locked skip run", stub.workerRuns)
	}
}

func TestExperienceServiceProcessOutcomeObserversRecordsRunWhenWorkerLockErrors(t *testing.T) {
	stub := &experienceRepoStub{
		workerLockErr: errors.New("lock backend unavailable"),
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessOutcomeObservers(context.Background(), 10)
	if appErr == nil {
		t.Fatalf("ProcessOutcomeObservers appErr = nil, want lock error")
	}
	if result.Failed != 1 {
		t.Fatalf("result=%+v, want failed lock acquisition", result)
	}
	if strings.Join(stub.workerLockNames, ",") != domain.ExperienceWorkerOutcomeObserver {
		t.Fatalf("worker locks = %v, want outcome observer lock", stub.workerLockNames)
	}
	if len(stub.workerRuns) != 1 || stub.workerRuns[0].Status != "failed" || stub.workerRuns[0].FailedCount != 1 {
		t.Fatalf("worker runs = %+v, want failed lock acquisition run", stub.workerRuns)
	}
	if !strings.Contains(stub.workerRuns[0].LastError, "acquire experience outcome observer lock") {
		t.Fatalf("worker run error = %q, want lock context", stub.workerRuns[0].LastError)
	}
}

func TestExperienceServiceProcessAttributionsSkipsWhenWorkerLockHeld(t *testing.T) {
	stub := &experienceRepoStub{
		workerLockBlocked: true,
		attributionOutcomes: []*domain.ExperienceAttributionOutcome{{
			ID:        1,
			EventKey:  "outcome:1",
			EventTime: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{CaptureEnabled: true, WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessAttributions(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("ProcessAttributions returned app error: %v", appErr)
	}
	if result.Scanned != 0 || len(stub.attributions) != 0 {
		t.Fatalf("result=%+v attributions=%d, want skipped while lock is held", result, len(stub.attributions))
	}
	if strings.Join(stub.workerLockNames, ",") != domain.ExperienceWorkerAttribution {
		t.Fatalf("worker locks = %v, want attribution lock", stub.workerLockNames)
	}
	if len(stub.workerRuns) != 1 || stub.workerRuns[0].Status != "locked" || stub.workerRuns[0].SkippedCount != 1 {
		t.Fatalf("worker runs = %+v, want locked skip run", stub.workerRuns)
	}
}

func TestExperienceServiceProcessRetentionSkipsWhenWorkerLockHeld(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	stub := &experienceRepoStub{
		workerLockBlocked: true,
		retentionRun: &domain.ExperienceRetentionRun{
			BehaviorDeleted: 9,
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{WorkerEnabled: true}, zap.NewNop())

	result, appErr := svc.ProcessRetention(context.Background(), now, 10)
	if appErr != nil {
		t.Fatalf("ProcessRetention returned app error: %v", appErr)
	}
	if result.BehaviorDeleted != 0 || !stub.retentionPolicy.BehaviorBefore.IsZero() {
		t.Fatalf("result=%+v policy=%+v, want skipped while lock is held", result, stub.retentionPolicy)
	}
	if strings.Join(stub.workerLockNames, ",") != domain.ExperienceWorkerRetention {
		t.Fatalf("worker locks = %v, want retention lock", stub.workerLockNames)
	}
	if len(stub.workerRuns) != 1 || stub.workerRuns[0].Status != "locked" || stub.workerRuns[0].SkippedCount != 1 {
		t.Fatalf("worker runs = %+v, want locked skip run", stub.workerRuns)
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
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, CaptureEnabled: true, AIFeedbackEnabled: true}, zap.NewNop())

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

func TestExperienceServiceRecordAISuggestionFeedbackRequiresCaptureEnabled(t *testing.T) {
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-1": {
				SuggestionEventID: "display-1",
				TargetType:        "task",
				TargetID:          "42",
				ActorID:           experienceInt64Ptr(291),
			},
		},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, CaptureEnabled: false, AIFeedbackEnabled: true}, zap.NewNop())

	feedback, appErr := svc.RecordAISuggestionFeedback(context.Background(), domain.RequestActor{ID: 291}, AISuggestionFeedbackRequest{
		SuggestionEventID: "display-1",
		FeedbackValue:     domain.ExperienceFeedbackAccepted,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("RecordAISuggestionFeedback appErr = %+v, want permission denied when capture is disabled", appErr)
	}
	if feedback != nil || len(stub.feedbacks) != 0 {
		t.Fatalf("feedback=%+v writes=%d, want no feedback write", feedback, len(stub.feedbacks))
	}
}

func TestExperienceServiceReserveRateLimitUsesUpdatedCount(t *testing.T) {
	periodStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, ReviewMaterializationEnabled: true}, zap.NewNop())

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

func TestExperienceServiceReserveRateLimitRequiresActor(t *testing.T) {
	periodStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	stub := &experienceRepoStub{}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true}, zap.NewNop())

	reservation, appErr := svc.ReserveRateLimit(context.Background(), domain.RequestActor{}, "micro_question_daily", periodStart, periodEnd, 2)
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("ReserveRateLimit appErr = %+v, want invalid request for missing actor", appErr)
	}
	if reservation != nil || len(stub.rateLimits) != 0 {
		t.Fatalf("reservation=%+v rateLimits=%d, want no anonymous bucket", reservation, len(stub.rateLimits))
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

func supportedMicroQuestionAttribution(suggestionEventID string, feedbackValue string) *domain.ExperienceAttribution {
	return &domain.ExperienceAttribution{
		SuggestionEventID: suggestionEventID,
		Status:            domain.ExperienceAttributionStatusPositive,
		Confidence:        "high",
		Score:             0.82,
		ComputedAt:        time.Now().UTC(),
		EvidenceSummary: mustServiceJSON(map[string]interface{}{
			"feedback": map[string]interface{}{"value": feedbackValue},
			"behavior": map[string]interface{}{"count": 1, "score": -2},
			"outcome":  map[string]interface{}{"action": "task_status_changed", "outcome": "Completed"},
		}),
	}
}

func unsupportedMicroQuestionAttribution(suggestionEventID string) *domain.ExperienceAttribution {
	return &domain.ExperienceAttribution{
		SuggestionEventID: suggestionEventID,
		Status:            domain.ExperienceAttributionStatusPositive,
		Confidence:        "high",
		Score:             0.82,
		ComputedAt:        time.Now().UTC(),
		EvidenceSummary: mustServiceJSON(map[string]interface{}{
			"feedback": map[string]interface{}{"value": domain.ExperienceFeedbackAccepted},
			"behavior": map[string]interface{}{"count": 1, "score": 5, "actions": []string{domain.ExperienceBehaviorActionClick}},
			"outcome":  map[string]interface{}{"action": "task_status_changed", "outcome": "Completed"},
		}),
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
		microQuestionAttribution: supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected),
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

func TestExperienceServiceMicroQuestionEligibilityUsesStableAttributionAfterRefresh(t *testing.T) {
	attribution := supportedMicroQuestionAttribution("display-old", domain.ExperienceFeedbackRejected)
	attribution.SuggestionStableKey = "stable-1"
	stub := &experienceRepoStub{
		aiEventsByID: map[string]*domain.AISuggestionEvent{
			"display-new": {
				SuggestionEventID:   "display-new",
				SuggestionStableKey: "stable-1",
				AttributionEligible: true,
				TargetType:          "task",
				TargetID:            "42",
				ActorID:             experienceInt64Ptr(291),
			},
		},
		microQuestionAttribution: attribution,
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{
		UIEnabled:            true,
		CaptureEnabled:       true,
		MicroQuestionEnabled: true,
		EnabledSurfaces:      []string{"task_detail"},
	}, zap.NewNop())

	result, appErr := svc.MicroQuestionEligibility(context.Background(), domain.RequestActor{ID: 291}, ExperienceMicroQuestionEligibilityRequest{
		SuggestionEventID: "display-new",
		Surface:           "task_detail",
		TargetType:        "task",
		TargetID:          "42",
	})
	if appErr != nil {
		t.Fatalf("MicroQuestionEligibility returned app error: %v", appErr)
	}
	if !result.Eligible || !strings.Contains(result.AnswerEventKey, "microq:") {
		t.Fatalf("eligibility = %+v, want refreshed suggestion eligible via stable attribution", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 for non-consuming eligibility", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRequiresSupportedAttribution(t *testing.T) {
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
	if result.Eligible || result.Reason != "no_supported_attribution" {
		t.Fatalf("eligibility = %+v, want no_supported_attribution", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 without supported attribution", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRejectsPositiveOnlyAttribution(t *testing.T) {
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
		microQuestionAttribution: unsupportedMicroQuestionAttribution("display-1"),
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
	if result.Eligible || result.Reason != "no_supported_attribution" {
		t.Fatalf("eligibility = %+v, want no_supported_attribution for positive-only evidence", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 without negative or skipped evidence", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityRejectsWeakPositiveOnlyAttribution(t *testing.T) {
	attribution := unsupportedMicroQuestionAttribution("display-1")
	attribution.Status = domain.ExperienceAttributionStatusWeak
	attribution.Confidence = "medium"
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
		microQuestionAttribution: attribution,
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
	if result.Eligible || result.Reason != "no_supported_attribution" {
		t.Fatalf("eligibility = %+v, want no_supported_attribution for weak positive-only evidence", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want 0 without negative or skipped evidence", stub.reserveCalls)
	}
}

func TestExperienceServiceMicroQuestionEligibilityAllowsRejectedAttributionWithNegativeEvidence(t *testing.T) {
	attribution := supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected)
	attribution.Status = domain.ExperienceAttributionStatusRejected
	attribution.Confidence = "low"
	attribution.Score = 0.3
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
		microQuestionAttribution: attribution,
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
		t.Fatalf("eligibility = %+v, want rejected attribution with negative evidence eligible", result)
	}
	if stub.reserveCalls != 0 {
		t.Fatalf("reserve calls = %d, want eligibility not to reserve", stub.reserveCalls)
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

func TestExperienceServiceRecordMicroQuestionAnswerRequiresSupportedAttributionWithoutReserving(t *testing.T) {
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
		t.Fatalf("reserve calls = %d, want 0 without supported attribution", stub.reserveCalls)
	}
	if len(stub.microAnswers) != 0 {
		t.Fatalf("micro answers = %+v, want no write", stub.microAnswers)
	}
}

func TestExperienceServiceRecordMicroQuestionAnswerRejectsPositiveOnlyAttributionWithoutReserving(t *testing.T) {
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
		microQuestionAttribution: unsupportedMicroQuestionAttribution("display-1"),
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
		t.Fatalf("reserve calls = %d, want 0 without negative or skipped evidence", stub.reserveCalls)
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
		microQuestionAttribution: supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected),
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
		microQuestionAttribution: supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected),
		forceDuplicateAnswer:     true,
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
		microQuestionAttribution: supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected),
		createMicroAnswerErr:     errors.New("insert failed"),
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
		microQuestionAttribution: supportedMicroQuestionAttribution("display-1", domain.ExperienceFeedbackRejected),
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

func TestExperienceMicroQuestionReasonCodesStayInSyncWithStoredSeeds(t *testing.T) {
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
	migrationCodes := extractMicroQuestionReasonCodesFromMigration(t, string(migration))
	for _, code := range codes {
		if !isAllowedExperienceMicroQuestionReason(code) {
			t.Fatalf("service whitelist rejects micro question reason %q", code)
		}
	}
	assertSameStringSet(t, "migration micro-question reasons", codes, migrationCodes)
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
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, ReviewMaterializationEnabled: true}, zap.NewNop())

	decision, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
		Decision:   domain.ExperienceReviewDecisionApprove,
		ReasonCode: "verified",
		Payload:    json.RawMessage(`{"review_confirmation":true}`),
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

func TestExperienceServiceRecordReviewDecisionRequiresMaterializationEnabledForApprove(t *testing.T) {
	stub := &experienceRepoStub{
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:  "review-1",
			ItemType: "attribution_candidate",
			Status:   domain.ExperienceReviewItemStatusOpen,
			Priority: "medium",
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, ReviewMaterializationEnabled: false}, zap.NewNop())

	decision, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
		Decision:   domain.ExperienceReviewDecisionApprove,
		ReasonCode: "verified",
		Payload:    json.RawMessage(`{"review_confirmation":true}`),
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("RecordReviewDecision appErr = %+v, want permission denied when materialization is disabled", appErr)
	}
	if decision != nil || len(stub.reviewDecisions) != 0 {
		t.Fatalf("decision=%+v stored=%d, want no materialization decision", decision, len(stub.reviewDecisions))
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
			svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, ReviewMaterializationEnabled: true}, zap.NewNop())

			_, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
				Decision: domain.ExperienceReviewDecisionReject,
			})
			if appErr == nil || appErr.Code != tt.code {
				t.Fatalf("RecordReviewDecision appErr = %+v, want code %s", appErr, tt.code)
			}
		})
	}
}

func TestExperienceServiceRecordReviewDecisionApproveRequiresConfirmation(t *testing.T) {
	stub := &experienceRepoStub{
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:  "review-1",
			ItemType: "attribution_candidate",
			Status:   domain.ExperienceReviewItemStatusOpen,
			Priority: "high",
		}},
	}
	svc := NewExperienceService(stub, ExperienceServiceConfig{UIEnabled: true, ReviewMaterializationEnabled: true}, zap.NewNop())

	decision, appErr := svc.RecordReviewDecision(context.Background(), domain.RequestActor{ID: 291}, "review-1", ExperienceReviewDecisionRequest{
		Decision:   domain.ExperienceReviewDecisionApprove,
		ReasonCode: "verified",
		Payload:    json.RawMessage(`{"surface":"data_center_experience"}`),
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("RecordReviewDecision appErr = %+v, want invalid request without approval confirmation", appErr)
	}
	if decision != nil || len(stub.reviewDecisions) != 0 {
		t.Fatalf("decision=%+v stored=%d, want no approval write", decision, len(stub.reviewDecisions))
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
	microQuestionAttribution  *domain.ExperienceAttribution
	rateLimits                map[string]*domain.ExperienceRateLimitReservation
	reserveCalls              int
	refundCalls               int
	microAnswers              map[string]*domain.ExperienceMicroQuestionAnswer
	forceDuplicateAnswer      bool
	createMicroAnswerErr      error
	reviewItems               []*domain.ExperienceReviewItem
	reviewDecisions           []*domain.ExperienceReviewDecision
	createReviewDecisionErr   error
	workerLockBlocked         bool
	workerLockErr             error
	workerLockNames           []string
	onGetWatermark            func(workerName, sourceName string)
}

func (s *experienceRepoStub) RunWithExperienceWorkerLock(ctx context.Context, lockName string, _ time.Duration, fn repo.ExperienceWorkerLockFunc) (bool, error) {
	s.workerLockNames = append(s.workerLockNames, lockName)
	if s.workerLockErr != nil {
		return false, s.workerLockErr
	}
	if s.workerLockBlocked {
		return false, nil
	}
	if fn != nil {
		fn(ctx)
	}
	return true, nil
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
	if s.onGetWatermark != nil {
		s.onGetWatermark(workerName, sourceName)
	}
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

func (s *experienceRepoStub) GetLatestExperienceAttributionForSuggestion(_ context.Context, suggestionEventID string) (*domain.ExperienceAttribution, error) {
	return s.GetLatestExperienceAttributionForSuggestionContext(context.Background(), suggestionEventID, "")
}

func (s *experienceRepoStub) GetLatestExperienceAttributionForSuggestionContext(_ context.Context, suggestionEventID string, suggestionStableKey string) (*domain.ExperienceAttribution, error) {
	s.calls++
	if s.microQuestionAttribution == nil {
		return nil, nil
	}
	if strings.TrimSpace(s.microQuestionAttribution.SuggestionEventID) == strings.TrimSpace(suggestionEventID) {
		copied := *s.microQuestionAttribution
		return &copied, nil
	}
	if strings.TrimSpace(suggestionStableKey) == "" || strings.TrimSpace(s.microQuestionAttribution.SuggestionStableKey) != strings.TrimSpace(suggestionStableKey) {
		return nil, nil
	}
	copied := *s.microQuestionAttribution
	return &copied, nil
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
			evidenceChanged := string(existing.EvidenceSummary) != string(item.EvidenceSummary)
			if existing.Status == "" || existing.Status == domain.ExperienceReviewItemStatusOpen || (existing.Status == domain.ExperienceReviewItemStatusNeedsMoreData && evidenceChanged) {
				existing.Priority = item.Priority
				existing.EvidenceSummary = item.EvidenceSummary
				if existing.Status == domain.ExperienceReviewItemStatusNeedsMoreData && evidenceChanged {
					existing.Status = item.Status
				}
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
