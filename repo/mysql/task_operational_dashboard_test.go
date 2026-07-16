package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTaskOperationalBoundariesUseBeijingCalendarWeek(t *testing.T) {
	location := taskOperationalLocation()
	now := time.Date(2026, 7, 12, 18, 30, 0, 0, time.UTC) // Monday 02:30 in Beijing.
	today, tomorrow, week, trend := taskOperationalBoundaries(now, location)
	if got, want := today, time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("today = %s, want %s", got, want)
	}
	if got, want := tomorrow, time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("tomorrow = %s, want %s", got, want)
	}
	if !week.Equal(today) {
		t.Fatalf("week = %s, want Monday start %s", week, today)
	}
	if got, want := trend, time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("trend = %s, want %s", got, want)
	}
}

func TestTaskOperationalOverviewUsesFullAggregateAndCompletionEvents(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "operational-counts":
			for _, fragment := range []string{"WITH exact_completion AS", "task.closed", "task.design.submitted", "FROM task_facts", "audit_records"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("counts SQL missing %q", fragment)
				}
			}
		case "operational-trend":
			for _, fragment := range []string{"WITH exact_completion AS", "UNION ALL", "DATE_ADD", "completed_count", "due_count"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("trend SQL missing %q", fragment)
				}
			}
		case "operational-distribution":
			for _, fragment := range []string{"PendingAudit", "Blocked", "customization_required", "GROUP BY bucket"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("distribution SQL missing %q", fragment)
				}
			}
		case "operational-recent-tasks":
			for _, fragment := range []string{"FROM tasks t", "LEFT JOIN users designer", "ORDER BY t.updated_at DESC", "LIMIT 8"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("recent tasks SQL missing %q", fragment)
				}
			}
		case "operational-events":
			for _, fragment := range []string{"task_event_logs", "task.audit.rejected", "ORDER BY tel.created_at DESC", "LIMIT 20"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("events SQL missing %q", fragment)
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

	countArgs := make([]driver.Value, 25)
	for index := range countArgs {
		countArgs[index] = sqlmock.AnyArg()
	}
	mock.ExpectQuery("operational-counts").WithArgs(countArgs...).WillReturnRows(sqlmock.NewRows([]string{
		"total_tasks", "active_tasks", "design_pending", "pending_audit", "handover",
		"customization_in_progress", "overdue", "due_today",
		"today_created", "today_completed", "week_created", "week_created_completed",
		"week_completed", "average_processing_hours", "average_processing_sample_count",
		"exact_completion_sample_count", "fallback_completion_sample_count",
		"week_audit_decisions", "week_audit_rejected",
	}).AddRow(1817, 978, 303, 457, 0, 54, 938, 29, 5, 5, 5, 1, 5, 8.25, 5, 4, 1, 2, 1))
	mock.ExpectQuery("operational-trend").WillReturnRows(sqlmock.NewRows([]string{"day", "created_count", "completed_count", "due_count"}).
		AddRow("2026-07-12", 31, 14, 34).
		AddRow("2026-07-13", 5, 5, 29))
	mock.ExpectQuery("operational-distribution").WillReturnRows(sqlmock.NewRows([]string{"bucket", "count"}).
		AddRow("design_ops", 303).
		AddRow("audit", 457).
		AddRow("customization", 52).
		AddRow("blocked", 2).
		AddRow("completed", 839))
	mock.ExpectQuery("operational-recent-tasks").WillReturnRows(sqlmock.NewRows([]string{"id", "task_no", "product_name", "owner_name", "task_status", "deadline_at"}).
		AddRow(2317, "RW-20260713-A-002314", "测试产品", "设计甲", "InProgress", time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)))
	mock.ExpectQuery("operational-events").WillReturnRows(sqlmock.NewRows([]string{"id", "event_type", "task_id", "task_no", "actor_name", "created_at"}).
		AddRow("event-1", "task.closed", 2311, "RW-20260712-A-002308", "系统", time.Date(2026, 7, 13, 0, 55, 0, 0, time.UTC)))

	repository := NewTaskOperationalDashboardRepo(New(db))
	overview, err := repository.GetTaskOperationalOverview(context.Background(), time.Date(2026, 7, 13, 1, 17, 22, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetTaskOperationalOverview() error = %v", err)
	}
	if overview.Counts.TotalTasks != 1817 || overview.Counts.TodayCompleted != 5 {
		t.Fatalf("counts = %+v", overview.Counts)
	}
	if overview.KPIs.WeekCompletionRate != 20 || overview.KPIs.WeekRejectRate != 50 || overview.KPIs.CompletionEventCoverageRate != 80 {
		t.Fatalf("kpis = %+v", overview.KPIs)
	}
	if len(overview.Trend) != 7 || overview.Trend[5].Date != "2026-07-12" || overview.Trend[5].Created != 31 || overview.Trend[6].Completed != 5 {
		t.Fatalf("trend = %+v", overview.Trend)
	}
	if len(overview.StatusDistribution) != 5 || overview.StatusDistribution[4].Count != 839 {
		t.Fatalf("distribution = %+v", overview.StatusDistribution)
	}
	if len(overview.RecentTasks) != 1 || overview.RecentTasks[0].OwnerName != "设计甲" {
		t.Fatalf("recent_tasks = %+v", overview.RecentTasks)
	}
	if len(overview.RecentEvents) != 1 || overview.RecentEvents[0].Title != "任务结单" {
		t.Fatalf("recent_events = %+v", overview.RecentEvents)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
