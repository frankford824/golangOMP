package mysqlrepo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	corerepo "workflow/repo"
)

func TestExperienceRepoListExperienceEventsIncludesAISuggestionSamples(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "experience-sample-count":
			for _, fragment := range []string{"SELECT COUNT(*) FROM", "FROM experience_events", "UNION ALL", "FROM ai_suggestion_events"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("sample count SQL missing %q: %s", fragment, normalized)
				}
			}
		case "experience-sample-list":
			for _, fragment := range []string{"FROM experience_events", "UNION ALL", "FROM ai_suggestion_events", "ORDER BY event_time DESC, id DESC"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("sample list SQL missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	displayedAt := time.Date(2026, 6, 29, 7, 20, 8, 0, time.UTC)
	mock.ExpectQuery("experience-sample-count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("experience-sample-list").
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows(experienceEventColumns()).
			AddRow(
				int64(-7),
				"pred:task_next_action:1",
				1,
				displayedAt,
				"ai_suggestion",
				"workflow-module",
				int64(1867),
				"task",
				"1867",
				"stable-task-1867",
				"ai_suggestion_events",
				"pred:task_next_action:1",
				"task_next_action",
				"displayed",
				`{"actor_id":291,"actor_type":"user","surface":"ai_suggestion"}`,
				`{"target_type":"task","target_id":"1867"}`,
				`{"source":"workflow-module"}`,
				"ai_suggestion",
				"displayed",
				"rejected",
				"missing_context",
				displayedAt,
				3,
				displayedAt,
			))

	repo := NewExperienceRepo(New(db))
	events, total, err := repo.ListExperienceEvents(context.Background(), corerepo.ExperienceEventListFilter{
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExperienceEvents() error = %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(events))
	}
	got := events[0]
	if got.SourceType != "ai_suggestion" || got.SourceID != "workflow-module" || got.Action != "task_next_action" || got.Outcome != "displayed" {
		t.Fatalf("event mapping = %+v", got)
	}
	if got.TaskID == nil || *got.TaskID != 1867 {
		t.Fatalf("task_id = %v, want 1867", got.TaskID)
	}
	if got.TargetType != "task" || got.TargetID != "1867" || got.SourceWatermark != "stable-task-1867" || got.ObservedFrom != "ai_suggestion_events" {
		t.Fatalf("target/observed fields = %+v", got)
	}
	if got.EvidenceLevel != "L3" || got.FeedbackValue != "rejected" || got.FeedbackReasonCode != "missing_context" {
		t.Fatalf("feedback evidence = level:%s value:%s reason:%s", got.EvidenceLevel, got.FeedbackValue, got.FeedbackReasonCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoStatsIncludesAttributionAndReviewCounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, _ string) error {
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	expectCount := func(value int64, args ...driver.Value) {
		q := mock.ExpectQuery("stats-count")
		if len(args) > 0 {
			q.WithArgs(args...)
		}
		q.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(value))
	}

	expectCount(11) // total events
	expectCount(0)  // outbox queued
	expectCount(0)  // outbox processing
	expectCount(0)  // outbox dead-letter
	expectCount(7, sqlmock.AnyArg())
	expectCount(1, sqlmock.AnyArg())
	expectCount(8)  // tags total
	expectCount(8)  // tags enabled
	expectCount(12) // AI suggestions
	mock.ExpectQuery("feedback-grouped").
		WillReturnRows(sqlmock.NewRows([]string{"feedback_value", "total", "reasoned"}).
			AddRow(domain.ExperienceFeedbackAccepted, int64(1), int64(0)).
			AddRow(domain.ExperienceFeedbackPartiallyAccepted, int64(1), int64(1)).
			AddRow(domain.ExperienceFeedbackRejected, int64(1), int64(1)))
	mock.ExpectQuery("attribution-grouped").
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(domain.ExperienceAttributionStatusPositive, int64(4)).
			AddRow(domain.ExperienceAttributionStatusWeak, int64(3)).
			AddRow(domain.ExperienceAttributionStatusRejected, int64(2)))
	mock.ExpectQuery("review-grouped").
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(domain.ExperienceReviewItemStatusOpen, int64(5)).
			AddRow(domain.ExperienceReviewItemStatusApproved, int64(6)).
			AddRow(domain.ExperienceReviewItemStatusRejected, int64(1)).
			AddRow(domain.ExperienceReviewItemStatusNeedsMoreData, int64(2)))
	mock.ExpectQuery("micro-question-grouped").
		WillReturnRows(sqlmock.NewRows([]string{"answer_value", "count"}).
			AddRow(domain.ExperienceMicroQuestionAnswerAnswered, int64(5)).
			AddRow(domain.ExperienceMicroQuestionAnswerDismissed, int64(1)).
			AddRow("other", int64(1)))
	expectCount(2, sqlmock.AnyArg(), domain.ExperienceMicroQuestionDailyLimit)
	expectCount(4) // task profiles
	expectCount(2) // asset quality labels
	expectCount(10)
	expectCount(12)
	expectCount(5)
	mock.ExpectQuery("latest-profile-rebuilt").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)))
	mock.ExpectQuery("worker-runs").
		WithArgs(12).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "worker_name", "source_name", "started_at", "finished_at", "status",
			"scanned_count", "enqueued_count", "skipped_count", "failed_count", "last_error", "metadata_json", "created_at",
		}))

	repo := NewExperienceRepo(New(db))
	stats, err := repo.ExperienceStats(context.Background())
	if err != nil {
		t.Fatalf("ExperienceStats() error = %v", err)
	}
	if stats.AttributionTotal != 9 ||
		stats.AttributionPositive != 4 ||
		stats.AttributionWeak != 3 ||
		stats.AttributionRejected != 2 {
		t.Fatalf("attribution stats = %+v, want 9 total / 4 positive / 3 weak / 2 rejected", stats)
	}
	if stats.ReviewItemsOpen != 5 ||
		stats.ReviewItemsApproved != 6 ||
		stats.ReviewItemsRejected != 1 ||
		stats.ReviewItemsNeedsMoreData != 2 {
		t.Fatalf("review stats = %+v, want open/approved/rejected/needs_more_data counts", stats)
	}
	if stats.MicroQuestionAnswers != 7 ||
		stats.MicroQuestionAnswered != 5 ||
		stats.MicroQuestionDismissed != 1 ||
		stats.MicroQuestionRateLimited != 2 {
		t.Fatalf("micro-question stats = %+v, want answer and limit counts", stats)
	}
	if stats.LocatableSamples != 22 || stats.LocatableDisplayedEvents != 12 {
		t.Fatalf("locatable stats = %+v, want total 22 and displayed 12", stats)
	}
	if stats.AIFeedbackEvents != 3 ||
		stats.FeedbackSamples != 3 ||
		stats.FeedbackAccepted != 1 ||
		stats.FeedbackPartiallyAccepted != 1 ||
		stats.FeedbackRejected != 1 ||
		stats.ReasonedFeedbackSamples != 2 {
		t.Fatalf("feedback stats = %+v, want grouped latest feedback distribution", stats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListRecentAttributionOutcomesUsesRecentWindow(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "recent-attribution-outcomes" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"FROM experience_events",
			"WHERE event_time >= ?",
			"AND ((event_time > ?) OR (event_time = ? AND id > ?))",
			"AND (target_type <> '' OR task_id IS NOT NULL)",
			"ORDER BY event_time ASC, id ASC",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("recent attribution SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	eventTime := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	since := eventTime.Add(-24 * time.Hour)
	cursorAt := since.Add(2 * time.Hour)
	mock.ExpectQuery("recent-attribution-outcomes").
		WithArgs(since, cursorAt, cursorAt, int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_key", "event_time", "source_type", "action", "outcome", "task_id",
			"target_type", "target_id", "payload_json",
		}).AddRow(
			int64(9),
			"outcome:tasks:42:completed",
			eventTime,
			"task_status_snapshot",
			"task_status_changed",
			"Completed",
			int64(42),
			"task",
			"42",
			`{"changed_fields":[]}`,
		))

	repo := NewExperienceRepo(New(db))
	outcomes, err := repo.ListRecentExperienceAttributionOutcomes(context.Background(), since, corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 7,
	}, 20)
	if err != nil {
		t.Fatalf("ListRecentExperienceAttributionOutcomes() error = %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].ID != 9 || outcomes[0].TargetType != "task" {
		t.Fatalf("outcomes = %+v, want mapped recent outcome", outcomes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListAttributionCandidatesOnlyFallsBackForTaskOutcomes(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "attribution-candidates" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"FROM ai_suggestion_events a",
			"GROUP_CONCAT(DISTINCT b.action",
			"(a.target_type = ? AND a.target_id = ?)",
			"OR (? = 'task' AND ? <> '' AND a.target_type = 'task' AND a.target_id = ?)",
			"HAVING behavior_count > 0 OR COALESCE(lf.feedback_value, '') <> ''",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("attribution candidates SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	outcomeAt := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	from := outcomeAt.Add(-7 * 24 * time.Hour)
	taskID := int64(42)
	mock.ExpectQuery("attribution-candidates").
		WithArgs(
			outcomeAt,
			from,
			outcomeAt,
			"task_asset",
			"7001",
			"task_asset",
			"42",
			"42",
			20,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"suggestion_event_id", "suggestion_stable_key", "suggestion_type", "suggestion_id",
			"source", "target_type", "target_id", "displayed_at", "behavior_count", "behavior_score",
			"latest_behavior_at", "behavior_actions", "feedback_value", "reason_code", "created_at",
		}))

	repo := NewExperienceRepo(New(db))
	candidates, err := repo.ListExperienceAttributionCandidates(context.Background(), &domain.ExperienceAttributionOutcome{
		EventTime:  outcomeAt,
		TaskID:     &taskID,
		TargetType: "task_asset",
		TargetID:   "7001",
	}, 7*24*time.Hour, 20)
	if err != nil {
		t.Fatalf("ListExperienceAttributionCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListAttributionCandidatesScansBehaviorActions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	outcomeAt := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	displayedAt := outcomeAt.Add(-30 * time.Minute)
	latestBehaviorAt := outcomeAt.Add(-5 * time.Minute)
	mock.ExpectQuery("SELECT a.suggestion_event_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"suggestion_event_id", "suggestion_stable_key", "suggestion_type", "suggestion_id",
			"source", "target_type", "target_id", "displayed_at", "behavior_count", "behavior_score",
			"latest_behavior_at", "behavior_actions", "feedback_value", "reason_code", "created_at",
		}).AddRow(
			"display-1",
			"stable-1",
			"task_next_action",
			"asset_ready",
			"workflow-module",
			"task",
			"42",
			displayedAt,
			2,
			1,
			latestBehaviorAt,
			"ignored_after_timeout,visible",
			"",
			"",
			nil,
		))

	repo := NewExperienceRepo(New(db))
	candidates, err := repo.ListExperienceAttributionCandidates(context.Background(), &domain.ExperienceAttributionOutcome{
		EventTime:  outcomeAt,
		TargetType: "task",
		TargetID:   "42",
	}, time.Hour, 20)
	if err != nil {
		t.Fatalf("ListExperienceAttributionCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	if strings.Join(candidates[0].BehaviorActions, ",") != "ignored_after_timeout,visible" {
		t.Fatalf("behavior actions = %v, want ignored_after_timeout and visible", candidates[0].BehaviorActions)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoGetLatestExperienceAttributionForSuggestion(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "latest-attribution" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"FROM experience_attributions",
			"WHERE suggestion_event_id = ?",
			"AND status IN (?, ?, ?)",
			"ORDER BY computed_at DESC, id DESC",
			"LIMIT 1",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("latest attribution SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	computedAt := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery("latest-attribution").
		WithArgs(
			"display-1",
			domain.ExperienceAttributionStatusPositive,
			domain.ExperienceAttributionStatusWeak,
			domain.ExperienceAttributionStatusRejected,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "suggestion_event_id", "suggestion_stable_key", "candidate_event_key", "outcome_event_key",
			"status", "confidence", "score", "computed_at", "evidence_summary_json", "created_at", "updated_at",
		}).AddRow(
			int64(11),
			"display-1",
			"stable-1",
			"candidate:display-1",
			"outcome-1",
			domain.ExperienceAttributionStatusWeak,
			"medium",
			0.62,
			computedAt,
			`{"behavior":{"actions":["dismiss"]}}`,
			computedAt,
			computedAt,
		))

	repo := NewExperienceRepo(New(db))
	attribution, err := repo.GetLatestExperienceAttributionForSuggestion(context.Background(), " display-1 ")
	if err != nil {
		t.Fatalf("GetLatestExperienceAttributionForSuggestion() error = %v", err)
	}
	if attribution == nil || attribution.ID != 11 || attribution.SuggestionEventID != "display-1" || attribution.Status != domain.ExperienceAttributionStatusWeak {
		t.Fatalf("attribution = %+v, want latest weak attribution", attribution)
	}
	if string(attribution.EvidenceSummary) != `{"behavior":{"actions":["dismiss"]}}` {
		t.Fatalf("evidence summary = %s, want stored JSON", attribution.EvidenceSummary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoGetLatestExperienceAttributionForSuggestionReturnsNilWhenMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, suggestion_event_id").
		WithArgs(
			"display-unknown",
			domain.ExperienceAttributionStatusPositive,
			domain.ExperienceAttributionStatusWeak,
			domain.ExperienceAttributionStatusRejected,
		).
		WillReturnError(sql.ErrNoRows)

	repo := NewExperienceRepo(New(db))
	attribution, err := repo.GetLatestExperienceAttributionForSuggestion(context.Background(), "display-unknown")
	if err != nil {
		t.Fatalf("GetLatestExperienceAttributionForSuggestion() error = %v", err)
	}
	if attribution != nil {
		t.Fatalf("attribution = %+v, want nil", attribution)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewItemDoesNotOverwriteResolvedEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"INSERT INTO experience_review_items",
			"ON DUPLICATE KEY UPDATE",
			"priority = IF(status = 'open', VALUES(priority), priority)",
			"evidence_summary_json = IF(status = 'open', VALUES(evidence_summary_json), evidence_summary_json)",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("review item upsert SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("review-item-upsert").
		WithArgs("review-1", "attribution_candidate", "open", "high", `{"score":0.91}`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewExperienceRepo(New(db))
	if err := repo.CreateExperienceReviewItem(context.Background(), &domain.ExperienceReviewItem{
		ItemKey:         "review-1",
		ItemType:        "attribution_candidate",
		Status:          domain.ExperienceReviewItemStatusOpen,
		Priority:        "high",
		EvidenceSummary: []byte(`{"score":0.91}`),
	}); err != nil {
		t.Fatalf("CreateExperienceReviewItem() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionUpdatesItemBeforeInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	actorID := int64(291)
	evidence := `{"suggestion":{"target_type":"task","target_id":"42","type":"task_next_action"},"outcome":{"action":"task_status_changed","outcome":"Completed"}}`
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-1").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusOpen, evidence))
	mock.ExpectExec("UPDATE experience_review_items").
		WithArgs(domain.ExperienceReviewItemStatusApproved, "review-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO experience_review_decisions").
		WithArgs(
			"review-1",
			domain.ExperienceReviewDecisionApprove,
			"verified",
			toNullInt64(&actorID),
			toNullJSONString([]byte(`{"surface":"data_center"}`)),
		).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery("SELECT 1 FROM tasks").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectExec("INSERT INTO task_experience_profiles").
		WithArgs(
			int64(42),
			"task_next_action",
			"Completed",
			"task_status_changed",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewExperienceRepo(New(db))
	if err := repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-1",
		Decision:      domain.ExperienceReviewDecisionApprove,
		ReasonCode:    "verified",
		ActorID:       &actorID,
		Payload:       []byte(`{"surface":"data_center"}`),
	}, domain.ExperienceReviewItemStatusApproved); err != nil {
		t.Fatalf("CreateExperienceReviewDecision() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionRejectsMissingTaskTarget(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	evidence := `{"suggestion":{"target_type":"task","target_id":"42","type":"task_next_action"},"outcome":{"action":"task_status_changed","outcome":"Completed"}}`
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-1").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusOpen, evidence))
	mock.ExpectExec("UPDATE experience_review_items").
		WithArgs(domain.ExperienceReviewItemStatusApproved, "review-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO experience_review_decisions").
		WithArgs("review-1", domain.ExperienceReviewDecisionApprove, "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery("SELECT 1 FROM tasks").
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	repo := NewExperienceRepo(New(db))
	err = repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-1",
		Decision:      domain.ExperienceReviewDecisionApprove,
	}, domain.ExperienceReviewItemStatusApproved)
	if err == nil || !strings.Contains(err.Error(), "task target not found") {
		t.Fatalf("CreateExperienceReviewDecision() error = %v, want missing task target", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionMissingItemDoesNotInsertDecision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("missing-review").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	repo := NewExperienceRepo(New(db))
	err = repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "missing-review",
		Decision:      domain.ExperienceReviewDecisionApprove,
	}, domain.ExperienceReviewItemStatusApproved)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("CreateExperienceReviewDecision() error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionRejectsResolvedItem(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-1").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusApproved, `{"suggestion":{"target_type":"task","target_id":"42"}}`))
	mock.ExpectRollback()

	repo := NewExperienceRepo(New(db))
	err = repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-1",
		Decision:      domain.ExperienceReviewDecisionReject,
	}, domain.ExperienceReviewItemStatusRejected)
	if err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("CreateExperienceReviewDecision() error = %v, want not open", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionRequiresMaterializableApprovalEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-1").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusOpen, `{"suggestion":{"target_type":"management","target_id":"dashboard"},"outcome":{"action":"dashboard_signal_changed","outcome":"observed"}}`))
	mock.ExpectExec("UPDATE experience_review_items").
		WithArgs(domain.ExperienceReviewItemStatusApproved, "review-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO experience_review_decisions").
		WithArgs("review-1", domain.ExperienceReviewDecisionApprove, "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	repo := NewExperienceRepo(New(db))
	err = repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-1",
		Decision:      domain.ExperienceReviewDecisionApprove,
	}, domain.ExperienceReviewItemStatusApproved)
	if err == nil || !strings.Contains(err.Error(), "not materializable") {
		t.Fatalf("CreateExperienceReviewDecision() error = %v, want not materializable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionRequiresSuggestionAndOutcomeEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-1").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusOpen, `{"suggestion":{"target_type":"task","target_id":"42"}}`))
	mock.ExpectExec("UPDATE experience_review_items").
		WithArgs(domain.ExperienceReviewItemStatusApproved, "review-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO experience_review_decisions").
		WithArgs("review-1", domain.ExperienceReviewDecisionApprove, "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	repo := NewExperienceRepo(New(db))
	err = repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-1",
		Decision:      domain.ExperienceReviewDecisionApprove,
	}, domain.ExperienceReviewItemStatusApproved)
	if err == nil || !strings.Contains(err.Error(), "requires suggestion and outcome evidence") {
		t.Fatalf("CreateExperienceReviewDecision() error = %v, want missing evidence", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateReviewDecisionMaterializesAssetQualityLabel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	actorID := int64(291)
	evidence := `{"suggestion":{"target_type":"asset","target_id":"77","type":"asset_quality"},"feedback":{"reason_code":"asset_mismatch"},"outcome":{"action":"asset_review_status_changed","outcome":"approved"}}`
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT item_type, status, evidence_summary_json").
		WithArgs("review-asset").
		WillReturnRows(sqlmock.NewRows([]string{"item_type", "status", "evidence_summary_json"}).
			AddRow("attribution_candidate", domain.ExperienceReviewItemStatusOpen, evidence))
	mock.ExpectExec("UPDATE experience_review_items").
		WithArgs(domain.ExperienceReviewItemStatusApproved, "review-asset").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO experience_review_decisions").
		WithArgs(
			"review-asset",
			domain.ExperienceReviewDecisionApprove,
			"",
			toNullInt64(&actorID),
			toNullJSONString(nil),
		).
		WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectQuery("SELECT 1 FROM design_assets").
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectExec("INSERT INTO asset_quality_labels").
		WithArgs(
			int64(77),
			"asset_mismatch",
			"review-asset",
			toNullInt64(&actorID),
			sqlmock.AnyArg(),
			"review-asset",
		).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	repo := NewExperienceRepo(New(db))
	if err := repo.CreateExperienceReviewDecision(context.Background(), &domain.ExperienceReviewDecision{
		ReviewItemKey: "review-asset",
		Decision:      domain.ExperienceReviewDecisionApprove,
		ActorID:       &actorID,
	}, domain.ExperienceReviewItemStatusApproved); err != nil {
		t.Fatalf("CreateExperienceReviewDecision() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListTaskStatusSnapshotsUsesTupleCursor(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"WHERE (updated_at > ?) OR (updated_at = ? AND id > ?)",
			"ORDER BY updated_at ASC, id ASC",
			"LIMIT ?",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("task snapshot SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	cursorAt := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("task-snapshot").
		WithArgs(cursorAt, cursorAt, int64(10), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_status", "updated_at"}).
			AddRow(int64(11), "Completed", updatedAt))

	repo := NewExperienceRepo(New(db))
	rows, err := repo.ListExperienceTaskStatusSnapshots(context.Background(), corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 10,
	}, 20)
	if err != nil {
		t.Fatalf("ListExperienceTaskStatusSnapshots() error = %v", err)
	}
	if len(rows) != 1 || rows[0].EntityID != "11" || rows[0].TerminalState != "Completed" {
		t.Fatalf("snapshot rows = %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoUpsertObservedStateDoesNotOverwriteNewerSourceState(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"ON DUPLICATE KEY UPDATE",
			"observed_value_json = IF(source_updated_at IS NULL OR VALUES(source_updated_at) >= source_updated_at, VALUES(observed_value_json), observed_value_json)",
			"observed_hash = IF(source_updated_at IS NULL OR VALUES(source_updated_at) >= source_updated_at, VALUES(observed_hash), observed_hash)",
			"source_updated_at = IF(source_updated_at IS NULL OR VALUES(source_updated_at) >= source_updated_at, VALUES(source_updated_at), source_updated_at)",
			"last_seen_at = VALUES(last_seen_at)",
			"tombstoned = IF(source_updated_at IS NULL OR VALUES(source_updated_at) >= source_updated_at, VALUES(tombstoned), tombstoned)",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("observed state upsert SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	sourceUpdatedAt := now.Add(-time.Minute)
	mock.ExpectExec("observed-upsert").
		WithArgs(
			"tasks_status_snapshot",
			"task",
			"42",
			sqlmock.AnyArg(),
			"hash-newer",
			"Completed",
			sourceUpdatedAt,
			sourceUpdatedAt,
			now,
			false,
			nil,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewExperienceRepo(New(db))
	err = repo.UpsertExperienceObservedEntityState(context.Background(), &domain.ExperienceObservedEntityState{
		SourceName:         "tasks_status_snapshot",
		EntityType:         "task",
		EntityID:           "42",
		ObservedValue:      []byte(`{"task_status":"Completed"}`),
		ObservedHash:       "hash-newer",
		TerminalState:      "Completed",
		TerminalObservedAt: &sourceUpdatedAt,
		SourceUpdatedAt:    &sourceUpdatedAt,
		LastSeenAt:         now,
		Tombstoned:         false,
	})
	if err != nil {
		t.Fatalf("UpsertExperienceObservedEntityState() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListTaskAssetReviewSnapshotsUsesBusinessTimestampCursor(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"GREATEST( created_at, COALESCE(approved_at, created_at), COALESCE(rejected_at, created_at), COALESCE(superseded_at, created_at), COALESCE(archived_at, created_at), COALESCE(cleaned_at, created_at) ) AS source_updated_at",
			"(created_at > ?) OR (created_at = ? AND id > ?)",
			"(approved_at > ?) OR (approved_at = ? AND id > ?)",
			"(rejected_at > ?) OR (rejected_at = ? AND id > ?)",
			"(superseded_at > ?) OR (superseded_at = ? AND id > ?)",
			"(archived_at > ?) OR (archived_at = ? AND id > ?)",
			"(cleaned_at > ?) OR (cleaned_at = ? AND id > ?)",
			"ORDER BY source_updated_at ASC, id ASC",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("task asset snapshot SQL missing %q: %s", fragment, normalized)
			}
		}
		if strings.Contains(normalized, "task_assets WHERE (updated_at > ?") {
			return fmt.Errorf("task asset observer must not use missing updated_at column: %s", normalized)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	cursorAt := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, 6, 30, 8, 1, 0, 0, time.UTC)
	args := []driver.Value{}
	for i := 0; i < 6; i++ {
		args = append(args, cursorAt, cursorAt, int64(10))
	}
	args = append(args, 20)
	mock.ExpectQuery("task-asset-snapshot").
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "flow_review_status", "approved_at", "rejected_at", "superseded_at", "superseded_by_version_id", "is_archived", "archived_at", "source_updated_at",
		}).AddRow(int64(11), int64(42), "superseded", nil, nil, approvedAt, int64(12), false, nil, approvedAt))

	repo := NewExperienceRepo(New(db))
	rows, err := repo.ListExperienceTaskAssetReviewSnapshots(context.Background(), corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 10,
	}, 20)
	if err != nil {
		t.Fatalf("ListExperienceTaskAssetReviewSnapshots() error = %v", err)
	}
	if len(rows) != 1 || rows[0].EntityID != "11" || rows[0].TerminalState != "superseded" {
		t.Fatalf("snapshot rows = %+v", rows)
	}
	if !strings.Contains(string(rows[0].ObservedValue), `"superseded_by_version_id":12`) {
		t.Fatalf("observed value = %s, want superseded version marker", rows[0].ObservedValue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoListFilingSnapshotsUseTupleCursor(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"WHERE (updated_at > ?) OR (updated_at = ? AND id > ?)",
			"ORDER BY updated_at ASC, id ASC",
			"LIMIT ?",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("filing snapshot SQL missing %q: %s", fragment, normalized)
			}
		}
		switch expectedSQL {
		case "task-detail-filing-snapshot":
			if !strings.Contains(normalized, "FROM task_details") {
				return fmt.Errorf("expected task_details query: %s", normalized)
			}
		case "task-sku-item-filing-snapshot":
			if !strings.Contains(normalized, "FROM task_sku_items") {
				return fmt.Errorf("expected task_sku_items query: %s", normalized)
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	cursorAt := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 30, 8, 0, 1, 0, time.UTC)
	mock.ExpectQuery("task-detail-filing-snapshot").
		WithArgs(cursorAt, cursorAt, int64(10), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "filing_status", "erp_sync_required", "last_filed_at", "updated_at"}).
			AddRow(int64(11), int64(42), "filed", false, updatedAt, updatedAt))
	mock.ExpectQuery("task-sku-item-filing-snapshot").
		WithArgs(cursorAt, cursorAt, int64(10), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "filing_status", "erp_sync_status", "erp_sync_required", "last_filed_at", "updated_at"}).
			AddRow(int64(12), int64(42), "filed", "filed", false, updatedAt, updatedAt))

	repo := NewExperienceRepo(New(db))
	details, err := repo.ListExperienceTaskDetailFilingSnapshots(context.Background(), corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 10,
	}, 20)
	if err != nil {
		t.Fatalf("ListExperienceTaskDetailFilingSnapshots() error = %v", err)
	}
	if len(details) != 1 || details[0].EntityID != "11" || details[0].TerminalState != "filed" {
		t.Fatalf("detail snapshots = %+v", details)
	}
	skus, err := repo.ListExperienceTaskSKUItemFilingSnapshots(context.Background(), corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 10,
	}, 20)
	if err != nil {
		t.Fatalf("ListExperienceTaskSKUItemFilingSnapshots() error = %v", err)
	}
	if len(skus) != 1 || skus[0].EntityID != "12" || skus[0].TerminalState != "filed" {
		t.Fatalf("sku snapshots = %+v", skus)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoReserveRateLimitUsesAtomicUpsertThenReadsCount(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "rate-limit-upsert":
			for _, fragment := range []string{
				"INSERT INTO experience_rate_limits",
				"ON DUPLICATE KEY UPDATE",
				"count = LEAST(count + 1, GREATEST(hard_cap, VALUES(hard_cap)))",
				"hard_cap = GREATEST(hard_cap, VALUES(hard_cap))",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("rate limit upsert SQL missing %q: %s", fragment, normalized)
				}
			}
		case "rate-limit-select":
			for _, fragment := range []string{
				"SELECT limit_key, actor_id, bucket_name, period_start, period_end, count, hard_cap",
				"FROM experience_rate_limits",
				"WHERE limit_key = ?",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("rate limit select SQL missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	actorID := int64(291)
	periodStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)
	mock.ExpectBegin()
	mock.ExpectExec("rate-limit-upsert").
		WithArgs("291:micro_question_daily:20260630", sqlmock.AnyArg(), "micro_question_daily", periodStart, periodEnd, 20).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("rate-limit-select").
		WithArgs("291:micro_question_daily:20260630").
		WillReturnRows(sqlmock.NewRows([]string{"limit_key", "actor_id", "bucket_name", "period_start", "period_end", "count", "hard_cap"}).
			AddRow("291:micro_question_daily:20260630", actorID, "micro_question_daily", periodStart, periodEnd, 3, 20))
	mock.ExpectCommit()

	repo := NewExperienceRepo(New(db))
	reservation, err := repo.ReserveExperienceRateLimit(context.Background(), corerepo.ExperienceRateLimitRequest{
		LimitKey:    "291:micro_question_daily:20260630",
		ActorID:     &actorID,
		BucketName:  "micro_question_daily",
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Limit:       2,
		HardCap:     20,
	})
	if err != nil {
		t.Fatalf("ReserveExperienceRateLimit() error = %v", err)
	}
	if reservation.Allowed || reservation.Count != 3 || reservation.HardCap != 20 {
		t.Fatalf("reservation = %+v, want count=3 hard_cap=20 allowed=false", reservation)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoRefundRateLimitDecrementsCount(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "rate-limit-refund" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"UPDATE experience_rate_limits",
			"count = GREATEST(count - 1, 0)",
			"WHERE limit_key = ?",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("rate limit refund SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("rate-limit-refund").
		WithArgs("291:micro_question_daily:20260701T080000Z").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewExperienceRepo(New(db))
	if err := repo.RefundExperienceRateLimit(context.Background(), "291:micro_question_daily:20260701T080000Z"); err != nil {
		t.Fatalf("RefundExperienceRateLimit() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestExperienceRepoCreateMicroQuestionAnswerReturnsInsertedFlag(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "micro-answer-insert" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"INSERT IGNORE INTO experience_micro_question_answers",
			"answer_event_key, suggestion_event_id, suggestion_stable_key, actor_id",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("micro answer insert SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	actorID := int64(291)
	answer := &domain.ExperienceMicroQuestionAnswer{
		AnswerEventKey:      "microq:1",
		SuggestionEventID:   "display-1",
		SuggestionStableKey: "stable-1",
		ActorID:             &actorID,
		Surface:             "task_detail",
		TargetType:          "task",
		TargetID:            "42",
		AnswerValue:         domain.ExperienceMicroQuestionAnswerAnswered,
		ReasonCode:          "missing_context",
	}
	mock.ExpectExec("micro-answer-insert").
		WithArgs("microq:1", "display-1", "stable-1", sqlmock.AnyArg(), "task_detail", "task", "42", domain.ExperienceMicroQuestionAnswerAnswered, "missing_context", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("micro-answer-insert").
		WithArgs("microq:1", "display-1", "stable-1", sqlmock.AnyArg(), "task_detail", "task", "42", domain.ExperienceMicroQuestionAnswerAnswered, "missing_context", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewExperienceRepo(New(db))
	inserted, err := repo.CreateExperienceMicroQuestionAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("CreateExperienceMicroQuestionAnswer insert error = %v", err)
	}
	if !inserted {
		t.Fatalf("inserted = false, want true for first insert")
	}
	inserted, err = repo.CreateExperienceMicroQuestionAnswer(context.Background(), answer)
	if err != nil {
		t.Fatalf("CreateExperienceMicroQuestionAnswer duplicate error = %v", err)
	}
	if inserted {
		t.Fatalf("inserted = true, want false for duplicate insert")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func experienceEventColumns() []string {
	return []string{
		"id",
		"event_key",
		"schema_version",
		"event_time",
		"source_type",
		"source_id",
		"task_id",
		"target_type",
		"target_id",
		"source_watermark",
		"observed_from",
		"observed_id",
		"action",
		"outcome",
		"actor_snapshot_json",
		"business_snapshot_json",
		"payload_json",
		"data_classification",
		"ground_truth_status",
		"feedback_value",
		"feedback_reason_code",
		"feedback_created_at",
		"evidence_rank",
		"created_at",
	}
}
