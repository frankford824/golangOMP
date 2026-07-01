package handler

import (
	"strings"
	"testing"
	"time"

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
