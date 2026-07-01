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

func TestExperienceRepoListRecentAttributionOutcomesUsesRecentWindow(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "recent-attribution-outcomes" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"FROM experience_events",
			"WHERE event_time >= ?",
			"AND (target_type <> '' OR task_id IS NOT NULL)",
			"ORDER BY event_time DESC, id DESC",
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
	mock.ExpectQuery("recent-attribution-outcomes").
		WithArgs(since, 20).
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
	outcomes, err := repo.ListRecentExperienceAttributionOutcomes(context.Background(), since, 20)
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

func TestExperienceRepoListTaskAssetReviewSnapshotsUsesBusinessTimestampCursor(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{
			"GREATEST( created_at, COALESCE(approved_at, created_at), COALESCE(rejected_at, created_at), COALESCE(archived_at, created_at), COALESCE(cleaned_at, created_at) ) AS source_updated_at",
			"(created_at > ?) OR (created_at = ? AND id > ?)",
			"(approved_at > ?) OR (approved_at = ? AND id > ?)",
			"(rejected_at > ?) OR (rejected_at = ? AND id > ?)",
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
	for i := 0; i < 5; i++ {
		args = append(args, cursorAt, cursorAt, int64(10))
	}
	args = append(args, 20)
	mock.ExpectQuery("task-asset-snapshot").
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "flow_review_status", "approved_at", "rejected_at", "is_archived", "archived_at", "source_updated_at",
		}).AddRow(int64(11), int64(42), "approved", approvedAt, nil, false, nil, approvedAt))

	repo := NewExperienceRepo(New(db))
	rows, err := repo.ListExperienceTaskAssetReviewSnapshots(context.Background(), corerepo.ExperienceSourceCursor{
		LastSeenAt: &cursorAt,
		LastSeenID: 10,
	}, 20)
	if err != nil {
		t.Fatalf("ListExperienceTaskAssetReviewSnapshots() error = %v", err)
	}
	if len(rows) != 1 || rows[0].EntityID != "11" || rows[0].TerminalState != "approved" {
		t.Fatalf("snapshot rows = %+v", rows)
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
