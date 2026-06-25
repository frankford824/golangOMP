package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestReportL1CardsUseCurrentTaskStatusesAndTaskEvents(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "report-card-in-progress":
			if strings.Contains(normalized, "task_module_events") {
				return fmt.Errorf("in-progress card must not use legacy task_module_events")
			}
			for _, fragment := range []string{"FROM tasks", "task_status NOT IN ('Draft', 'Completed', 'Archived', 'Cancelled')"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("in-progress SQL missing %q", fragment)
				}
			}
		case "report-card-completed-today":
			for _, fragment := range []string{"FROM tasks t", "task_status IN ('Completed', 'Archived')", "task_event_logs", "task.warehouse.completed"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("completed-today SQL missing %q", fragment)
				}
			}
		case "report-card-archived-total":
			if strings.Contains(normalized, "task_module_events") {
				return fmt.Errorf("archived card must not use legacy task_module_events")
			}
			if !strings.Contains(normalized, "task_status IN ('Completed', 'Archived')") {
				return fmt.Errorf("archived SQL uses stale status names: %s", normalized)
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

	mock.ExpectQuery("report-card-in-progress").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(740)))
	mock.ExpectQuery("report-card-completed-today").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(12)))
	mock.ExpectQuery("report-card-archived-total").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(37)))

	repo := NewReportL1Repo(New(db))
	cards, err := repo.GetCards(context.Background())
	if err != nil {
		t.Fatalf("GetCards() error = %v", err)
	}
	if len(cards) != 3 || cards[0].Value != 740 || cards[1].Value != 12 || cards[2].Value != 37 {
		t.Fatalf("cards=%+v", cards)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReportL1ThroughputUsesTaskEvents(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "report-throughput" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{"WITH dates AS", "FROM tasks t", "task_event_logs", "task.audit.approved", "task.warehouse.completed"} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("throughput SQL missing %q", fragment)
			}
		}
		if strings.Contains(normalized, "task_module_events") {
			return fmt.Errorf("throughput must not use legacy task_module_events")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	end := to.AddDate(0, 0, 1)
	mock.ExpectQuery("report-throughput").
		WithArgs(from, end, from, end, from, end, from, end).
		WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "archived_count"}).
			AddRow("2026-06-01", int64(10), int64(3), int64(3)))

	repo := NewReportL1Repo(New(db))
	points, err := repo.GetThroughput(context.Background(), reportL1TestFilter(from, to))
	if err != nil {
		t.Fatalf("GetThroughput() error = %v", err)
	}
	if len(points) != 1 || points[0].Created != 10 || points[0].Completed != 3 {
		t.Fatalf("points=%+v", points)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReportL1ModuleDwellUsesTaskEventsAndCustomizationSamples(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "report-module-dwell" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, fragment := range []string{"normalized_events AS", "task_event_logs", "'customization' AS module_key", "task.customization.reviewed"} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("module dwell SQL missing %q", fragment)
			}
		}
		if strings.Contains(normalized, "task_module_events") {
			return fmt.Errorf("module dwell must not use legacy task_module_events")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	end := to.AddDate(0, 0, 1)
	mock.ExpectQuery("report-module-dwell").
		WithArgs(from, end, from, end, from, end, from, end, from, end).
		WillReturnRows(sqlmock.NewRows([]string{"module_key", "avg_dwell", "p95_dwell", "samples"}).
			AddRow("design", float64(3600), float64(7200), int64(2)).
			AddRow("customization", float64(1800), float64(1800), int64(1)))

	repo := NewReportL1Repo(New(db))
	points, err := repo.GetModuleDwell(context.Background(), reportL1TestFilter(from, to))
	if err != nil {
		t.Fatalf("GetModuleDwell() error = %v", err)
	}
	seen := map[string]bool{}
	for _, point := range points {
		if point.Samples > 0 {
			seen[point.ModuleKey] = true
		}
	}
	if !seen["design"] || !seen["customization"] {
		t.Fatalf("points=%+v", points)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func reportL1TestFilter(from, to time.Time) repo.ReportL1Filter {
	return repo.ReportL1Filter{From: from, To: to}
}
