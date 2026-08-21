package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestAnalyticsRepoCompilesAllowListedScopedMetric(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actual string) error {
		normalized := strings.Join(strings.Fields(actual), " ")
		for _, fragment := range []string{"FROM task_event_logs tel", "tel.event_type IN (?,?)", "t.owner_department_id IN (?)", "GROUP BY row_key, row_label"} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("missing %q in %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	from := time.Date(2026, 8, 15, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	to := from.AddDate(0, 0, 7)
	mock.ExpectQuery("metric").WithArgs(from, to, "task.design_submitted", "task.design.submitted", int64(3), 20).
		WillReturnRows(sqlmock.NewRows([]string{"key", "label", "events", "tasks", "actors", "latency"}).
			AddRow("2026-08-15", "2026-08-15", 65, 64, 9, 0))
	definition := domain.AnalyticsMetricDefinition{ID: "task_design_submitted", Name: "设计提交", Source: domain.AnalyticsMetricSourceTaskEvent,
		EventTypes: []string{"task.design_submitted", "task.design.submitted"}, AllowedGroupBys: []string{"day"}}
	result, err := NewAnalyticsRepo(New(db)).QueryMetric(context.Background(), domain.ResourceGroupAccessFilter{DepartmentIDs: []int64{3}}, definition,
		domain.AnalyticsMetricQuery{MetricID: definition.ID, From: from, To: to, GroupBy: "day", Limit: 20})
	if err != nil || len(result.Rows) != 1 || result.Rows[0].TaskCount != 64 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyticsRepoRejectsUnlistedGroupingBeforeSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	definition := domain.AnalyticsMetricDefinition{ID: "page_views", Source: domain.AnalyticsMetricSourceWorkflowTrace, AllowedGroupBys: []string{"day"}}
	_, err = NewAnalyticsRepo(New(db)).QueryMetric(context.Background(), domain.ResourceGroupAccessFilter{Global: true}, definition,
		domain.AnalyticsMetricQuery{From: time.Now(), To: time.Now().Add(time.Hour), GroupBy: "raw_sql", Limit: 20})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
