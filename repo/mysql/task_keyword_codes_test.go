package mysqlrepo

import (
	"reflect"
	"strings"
	"testing"

	"workflow/repo"
)

func TestTaskListKeywordCodes(t *testing.T) {
	for _, separator := range []string{" ", "\n", "\r\n", "\t", "\u00a0", ",", "，", ";", "；", "、"} {
		t.Run(separator, func(t *testing.T) {
			got := taskListKeywordCodes(" cgk003835" + separator + "CGK004337" + separator + "CGK004340 " + separator + "CGK003835")
			if want := []string{"CGK003835", "CGK004337", "CGK004340"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
	for _, text := range []string{"", "CGK003835", "国庆 立牌", "国庆2026 立牌2个", "summer poster", "CGK003835 国庆", "CGK003835 %", "123 456", "RW-20260922-A-007324 hello"} {
		if got := taskListKeywordCodes(text); got != nil {
			t.Errorf("%q must retain single/phrase search, got %v", text, got)
		}
	}
	if got := taskListKeywordCodes("CGK003835 cgk003835"); !reflect.DeepEqual(got, []string{"CGK003835"}) {
		t.Fatalf("duplicate list: %v", got)
	}
	if got := taskListKeywordCodes("RW-20260908-A-006147，CGK004337"); !reflect.DeepEqual(got, []string{"RW-20260908-A-006147", "CGK004337"}) {
		t.Fatalf("mixed task/SKU codes: %v", got)
	}
}

func TestTaskListMultiCodeRetainsOtherFilters(t *testing.T) {
	creatorID := int64(296)
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		Keyword: "CGK003835 CGK004337", CreatorID: &creatorID,
		ScopeViewAll: true, ExcludePendingAuditHandover: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spec.whereSQL, "t.creator_id = ? AND t.id IN (") ||
		!strings.Contains(spec.whereSQL, ") AND NOT EXISTS (") {
		t.Fatalf("multi-code recall must intersect other filters: %s", spec.whereSQL)
	}
	if len(spec.args) != 10 || spec.args[0] != creatorID || spec.args[9] != "pending_takeover" {
		t.Fatalf("filter argument ordering: %v", spec.args)
	}
}

func TestTaskListMultiCodeQueryPreservesScopeAndFilters(t *testing.T) {
	for _, indexed := range []bool{true, false} {
		filter := repo.TaskListFilter{Keyword: "CGK003835 CGK004337 CGK004340", ScopeUserIDs: []int64{296}}
		spec, err := buildTaskListQuerySpecWithOptions(filter, nil, taskListQueryBuildOptions{UseSearchDocumentKeyword: indexed})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"t.id IN (SELECT id FROM tasks WHERE task_no IN (?,?,?) UNION ALL",
			"SELECT id FROM tasks WHERE sku_code IN (?,?,?)",
			"SELECT id FROM tasks WHERE primary_sku_code IN (?,?,?)",
			"SELECT task_id FROM task_sku_items WHERE sku_code IN (?,?,?)",
			"t.creator_id IN (?)", "t.designer_id IN (?)", "t.current_handler_id IN (?)",
		} {
			if !strings.Contains(spec.whereSQL, want) {
				t.Fatalf("missing %s: %s", want, spec.whereSQL)
			}
		}
		if strings.Contains(spec.whereSQL, "MATCH(") || strings.Contains(spec.whereSQL, "LIKE") {
			t.Fatalf("batch identities must match exactly: %s", spec.whereSQL)
		}
		if strings.Count(spec.whereSQL, "?") != len(spec.args) {
			t.Fatalf("placeholders and args differ: %s / %v", spec.whereSQL, spec.args)
		}
		for i, code := range []string{"CGK003835", "CGK004337", "CGK004340"} {
			for branch := 0; branch < 4; branch++ {
				if spec.args[branch*3+i] != code {
					t.Fatalf("args=%v", spec.args)
				}
			}
		}
	}
}
