package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL == "report-daily-table-exists" {
			normalized := strings.Join(strings.Fields(actualSQL), " ")
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table existence SQL unexpected: %s", normalized)
			}
			return nil
		}
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
	mock.ExpectQuery("report-daily-table-exists").
		WithArgs("report_task_daily").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
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

func TestReportL1ThroughputUsesDailyAggregateWhenPresent(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "report-daily-table-exists":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table existence SQL unexpected: %s", normalized)
			}
		case "report-daily-freshness":
			if !strings.Contains(normalized, "SELECT MAX(updated_at) FROM report_task_daily") {
				return fmt.Errorf("daily freshness SQL unexpected: %s", normalized)
			}
		case "report-throughput-daily":
			for _, fragment := range []string{"FROM report_task_daily r", "SUM(r.created_count)", "SUM(r.completed_count)", "r.owner_department = CAST(? AS CHAR)", "r.task_type = ?"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("daily throughput SQL missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "task_event_logs") || strings.Contains(normalized, "FROM tasks t") {
				return fmt.Errorf("daily throughput should not use realtime task tables: %s", normalized)
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

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	end := to.AddDate(0, 0, 1)
	deptID := int64(12)
	taskType := "normal"
	mock.ExpectQuery("report-daily-table-exists").
		WithArgs("report_task_daily").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("report-daily-freshness").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(time.Now()))
	mock.ExpectQuery("report-throughput-daily").
		WithArgs(from, end, deptID, taskType).
		WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "archived_count"}).
			AddRow("2026-06-01", int64(10), int64(3), int64(3)))

	l1Repo := NewReportL1Repo(New(db))
	points, err := l1Repo.GetThroughput(context.Background(), repo.ReportL1Filter{
		From:         from,
		To:           to,
		DepartmentID: &deptID,
		TaskType:     &taskType,
	})
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

func TestReportL1ThroughputUsesRealtimeTailForCurrentDay(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "report-daily-table-exists":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table existence SQL unexpected: %s", normalized)
			}
		case "report-daily-freshness":
			if !strings.Contains(normalized, "SELECT MAX(updated_at) FROM report_task_daily") {
				return fmt.Errorf("daily freshness SQL unexpected: %s", normalized)
			}
		case "report-throughput-daily":
			if !strings.Contains(normalized, "FROM report_task_daily r") || strings.Contains(normalized, "WITH dates AS") {
				return fmt.Errorf("historical throughput should use daily aggregate only: %s", normalized)
			}
		case "report-throughput-realtime":
			for _, fragment := range []string{"WITH dates AS", "FROM tasks t", "task_event_logs"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("current-day throughput SQL missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "FROM report_task_daily r") {
				return fmt.Errorf("current-day throughput must not use stale daily aggregate: %s", normalized)
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

	today := startOfUTCDay(time.Now())
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)
	mock.ExpectQuery("report-daily-table-exists").
		WithArgs("report_task_daily").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("report-daily-freshness").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(time.Now()))
	mock.ExpectQuery("report-throughput-daily").
		WithArgs(yesterday, today).
		WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "archived_count"}).
			AddRow(yesterday.Format("2006-01-02"), int64(8), int64(2), int64(2)))
	mock.ExpectQuery("report-throughput-realtime").
		WithArgs(today, tomorrow, today, tomorrow, today, tomorrow, today, tomorrow).
		WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "archived_count"}).
			AddRow(today.Format("2006-01-02"), int64(3), int64(1), int64(1)))

	l1Repo := NewReportL1Repo(New(db))
	points, err := l1Repo.GetThroughput(context.Background(), reportL1TestFilter(yesterday, today))
	if err != nil {
		t.Fatalf("GetThroughput() error = %v", err)
	}
	if len(points) != 2 || points[0].Date != yesterday.Format("2006-01-02") || points[1].Date != today.Format("2006-01-02") {
		t.Fatalf("points=%+v", points)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReportL1ThroughputFallsBackWhenDailyAggregateIsStale(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "report-daily-table-exists":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table existence SQL unexpected: %s", normalized)
			}
		case "report-daily-freshness":
			if !strings.Contains(normalized, "SELECT MAX(updated_at) FROM report_task_daily") {
				return fmt.Errorf("daily freshness SQL unexpected: %s", normalized)
			}
		case "report-throughput-realtime":
			for _, fragment := range []string{"WITH dates AS", "FROM tasks t", "task_event_logs"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("stale aggregate fallback SQL missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "FROM report_task_daily r") {
				return fmt.Errorf("stale aggregate must not use daily table: %s", normalized)
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

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	end := to.AddDate(0, 0, 1)
	mock.ExpectQuery("report-daily-table-exists").
		WithArgs("report_task_daily").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("report-daily-freshness").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(time.Now().Add(-3 * time.Hour)))
	mock.ExpectQuery("report-throughput-realtime").
		WithArgs(from, end, from, end, from, end, from, end).
		WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "archived_count"}).
			AddRow("2026-06-01", int64(10), int64(3), int64(3)))

	l1Repo := NewReportL1Repo(New(db))
	points, err := l1Repo.GetThroughput(context.Background(), reportL1TestFilter(from, to))
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

func TestReportL1RefreshDailyAggregatesRebuildsDateRange(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "report-daily-table-exists":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table existence SQL unexpected: %s", normalized)
			}
		case "report-daily-delete":
			if !strings.Contains(normalized, "DELETE FROM report_task_daily") || !strings.Contains(normalized, "day >= DATE(?)") || !strings.Contains(normalized, "day < DATE(?)") {
				return fmt.Errorf("delete SQL unexpected: %s", normalized)
			}
		case "report-daily-insert":
			for _, fragment := range []string{"INSERT INTO report_task_daily", "FROM tasks t", "task_event_logs tel", "COUNT(DISTINCT tel.task_id)", "ON DUPLICATE KEY UPDATE"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("refresh SQL missing %q: %s", fragment, normalized)
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

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	end := to.AddDate(0, 0, 1)
	mock.ExpectQuery("report-daily-table-exists").
		WithArgs("report_task_daily").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("report-daily-delete").
		WithArgs(from, end).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("report-daily-insert").
		WithArgs(from, end, from, end).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repo := NewReportL1Repo(New(db))
	if err := repo.RefreshDailyAggregates(context.Background(), from, to); err != nil {
		t.Fatalf("RefreshDailyAggregates() error = %v", err)
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
		for _, fragment := range []string{"event_sequence AS", "normalized_events AS", "ROWS BETWEEN 1 FOLLOWING", "task_event_logs", "'customization' AS module_key", "task.customization.reviewed"} {
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
		WithArgs(from, end, from, end, end, end, end, end).
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
