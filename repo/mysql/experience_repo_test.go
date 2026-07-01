package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

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
