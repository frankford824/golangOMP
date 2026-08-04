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

func TestAIAnalysisEvidenceQueriesApplySameStableScope(t *testing.T) {
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		for _, fragment := range strings.Split(expected, "|") {
			if !strings.Contains(normalizeAIAnalysisSQL(actual), normalizeAIAnalysisSQL(fragment)) {
				return fmt.Errorf("query missing %q: %s", fragment, actual)
			}
		}
		return nil
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewAIAnalysisRepo(New(db))
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 10)
	access := domain.ResourceGroupAccessFilter{ActorID: 9, Self: true, DepartmentIDs: []int64{3}, TeamIDs: []int64{7}}
	mock.ExpectQuery("FROM tasks t|JOIN task_search_documents d|t.id = ?|t.creator_id = ?|t.owner_department_id IN (?)|t.owner_team_id IN (?)").
		WithArgs(int64(11), int64(9), int64(9), int64(9), int64(9), int64(3), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_no", "sku", "product", "search_text", "version"}).
			AddRow(11, "T-11", "SKU-11", "产品", "任务完整检索文本", "2"))
	task, err := repository.GetTaskDetailEvidence(context.Background(), access, 11)
	if err != nil || len(task) != 1 || task[0].EntityType != "task" || task[0].EntityID != "11" {
		t.Fatalf("task=%+v err=%v", task, err)
	}

	mock.ExpectQuery("FROM task_asset_groups g|JOIN task_asset_group_search_documents d|g.id = ?|t.creator_id = ?|t.owner_department_id IN (?)|t.owner_team_id IN (?)").
		WithArgs(int64(21), int64(9), int64(9), int64(9), int64(9), int64(3), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_no", "sku", "internal_text", "version"}).
			AddRow(21, "T-11", "SKU-11", "参考图 源文件 最终成品图", "8"))
	group, err := repository.GetResourceGroupDetailEvidence(context.Background(), access, 21)
	if err != nil || len(group) != 1 || group[0].EntityType != "task_resource_group" || group[0].EntityID != "21" {
		t.Fatalf("group=%+v err=%v", group, err)
	}

	mock.ExpectQuery("FROM task_event_logs tel|t.creator_id = ?|t.owner_department_id IN (?)|t.owner_team_id IN (?)").
		WithArgs(from, to, int64(9), int64(9), int64(9), int64(9), int64(3), int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "task_no", "sku", "product", "event_type", "operator", "created_at", "version"}).
			AddRow("event-1", 11, "T-11", "SKU-11", "产品", "task.closed", "审核员", from, "2"))
	kpi, err := repository.ListKPIEvidence(context.Background(), access, from, to, 20)
	if err != nil || len(kpi) != 1 || kpi[0].EntityType != "task_kpi" {
		t.Fatalf("kpi=%+v err=%v", kpi, err)
	}

	mock.ExpectQuery("FROM tasks t LEFT JOIN task_details td|t.creator_id = ?|t.owner_department_id IN (?)|t.owner_team_id IN (?)").
		WithArgs(from, to, int64(9), int64(9), int64(9), int64(9), int64(3), int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_no", "sku", "product", "category", "demand", "copy", "requirement", "material", "size", "updated_at", "version"}).
			AddRow(11, "T-11", "SKU-11", "产品", "挂饰", "夏季", "轻盈", "白底", "金属", "10cm", from, "2"))
	trends, err := repository.ListBusinessTrendEvidence(context.Background(), access, from, to, 20)
	if err != nil || len(trends) != 1 || trends[0].EntityType != "business_trend" {
		t.Fatalf("trends=%+v err=%v", trends, err)
	}

	mock.ExpectQuery("FROM experience_events e JOIN tasks t|e.task_id IS NOT NULL|t.creator_id = ?|t.owner_department_id IN (?)|t.owner_team_id IN (?)").
		WithArgs(from, to, int64(9), int64(9), int64(9), int64(9), int64(3), int64(7), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "task_no", "action", "outcome", "feedback", "reason", "payload", "event_time", "version"}).
			AddRow(91, 11, "T-11", "audit", "accepted", "helpful", "clear", `{}`, from, "w1"))
	experience, err := repository.ListExperienceEvidence(context.Background(), access, from, to, 20)
	if err != nil || len(experience) != 1 || experience[0].EntityType != "experience_summary" {
		t.Fatalf("experience=%+v err=%v", experience, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAIAnalysisEvidenceFailsClosedForEmptyScope(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actual string) error {
		if !strings.Contains(normalizeAIAnalysisSQL(actual), "AND 1 = 0") {
			return fmt.Errorf("missing fail-closed predicate: %s", actual)
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	mock.ExpectQuery("fail-closed").WithArgs(from, to, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "task_no", "sku", "product", "event_type", "operator", "created_at", "version"}))
	items, err := NewAIAnalysisRepo(New(db)).ListKPIEvidence(context.Background(), domain.ResourceGroupAccessFilter{ActorID: 9}, from, to, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func normalizeAIAnalysisSQL(value string) string { return strings.Join(strings.Fields(value), " ") }
