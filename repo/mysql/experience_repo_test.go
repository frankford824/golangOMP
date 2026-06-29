package mysqlrepo

import (
	"context"
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
				"task_next_action",
				"displayed",
				`{"actor_id":291,"actor_type":"user","surface":"ai_suggestion"}`,
				`{"target_type":"task","target_id":"1867"}`,
				`{"source":"workflow-module"}`,
				"ai_suggestion",
				"displayed",
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
		"action",
		"outcome",
		"actor_snapshot_json",
		"business_snapshot_json",
		"payload_json",
		"data_classification",
		"ground_truth_status",
		"created_at",
	}
}
