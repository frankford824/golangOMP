package mysqlrepo

import (
	"strings"
	"testing"

	"workflow/domain"
)

func TestBuildTaskAssetSearchWhereKeywordCoversPlannedFields(t *testing.T) {
	query := domain.AssetSearchQuery{
		Keyword: "ABC-123",
	}

	where, args := buildTaskAssetSearchWhere(query)

	for _, expected := range []string{
		"CAST(ta.asset_id AS CHAR) LIKE ?",
		"CAST(ta.task_id AS CHAR) LIKE ?",
		"t.sku_code LIKE ?",
		"t.primary_sku_code LIKE ?",
		"ta.scope_sku_code LIKE ?",
		"ta.file_name LIKE ?",
		"ta.original_filename LIKE ?",
	} {
		if !strings.Contains(where, expected) {
			t.Fatalf("where clause missing %q: %s", expected, where)
		}
	}

	if got, want := strings.Count(where, "?"), len(args); got != want {
		t.Fatalf("placeholder count = %d, args count = %d", got, want)
	}
	if got, want := len(args), 9; got != want {
		t.Fatalf("args len = %d, want %d", got, want)
	}

	wantLike := "%ABC-123%"
	for i, arg := range args {
		got, ok := arg.(string)
		if !ok {
			t.Fatalf("args[%d] type = %T, want string", i, arg)
		}
		if got != wantLike {
			t.Fatalf("args[%d] = %q, want %q", i, got, wantLike)
		}
	}
}

func TestBuildTaskAssetSearchWhereKeywordWithTaskStatusKeepsArgsAligned(t *testing.T) {
	query := domain.AssetSearchQuery{
		Keyword:    "1001",
		TaskStatus: domain.AssetTaskStatusFilterOpen,
	}

	where, args := buildTaskAssetSearchWhere(query)

	if !strings.Contains(where, "CAST(ta.asset_id AS CHAR) LIKE ?") {
		t.Fatalf("where clause missing asset_id keyword match: %s", where)
	}
	if !strings.Contains(where, "CAST(ta.task_id AS CHAR) LIKE ?") {
		t.Fatalf("where clause missing task_id keyword match: %s", where)
	}
	if !strings.Contains(where, "t.sku_code LIKE ?") {
		t.Fatalf("where clause missing sku_code keyword match: %s", where)
	}
	if !strings.Contains(where, "ta.original_filename LIKE ?") {
		t.Fatalf("where clause missing original_filename keyword match: %s", where)
	}
	if !strings.Contains(where, "t.task_status NOT IN (?, ?, ?)") {
		t.Fatalf("where clause missing open task status filter: %s", where)
	}

	if got, want := strings.Count(where, "?"), len(args); got != want {
		t.Fatalf("placeholder count = %d, args count = %d", got, want)
	}
}

func TestBuildListCurrentByAssetIDsQueryBuildsParameterizedINClause(t *testing.T) {
	query, args := buildListCurrentByAssetIDsQuery([]int64{101, 202, 303})
	if query == "" {
		t.Fatal("query is empty")
	}
	if !strings.Contains(query, "WHERE da.id IN (?, ?, ?)") {
		t.Fatalf("query missing IN placeholders: %s", query)
	}
	if strings.Contains(query, "IN (101") {
		t.Fatalf("query should be parameterized, got: %s", query)
	}
	if !strings.Contains(query, "ta.id = COALESCE(da.current_version_id") {
		t.Fatalf("query missing current-version guard: %s", query)
	}
	if got, want := strings.Count(query, "?"), len(args); got != want {
		t.Fatalf("placeholder count = %d, args count = %d", got, want)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("args len = %d, want %d", got, want)
	}
	if args[0] != int64(101) || args[1] != int64(202) || args[2] != int64(303) {
		t.Fatalf("args = %#v, want [101 202 303]", args)
	}
}

func TestBuildListCurrentByAssetIDsQueryEmptyIDsReturnsNoQuery(t *testing.T) {
	query, args := buildListCurrentByAssetIDsQuery([]int64{})
	if query != "" {
		t.Fatalf("query = %q, want empty", query)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, want empty", args)
	}
}

func TestBuildListCurrentByAssetIDsQueryUsesCurrentReadModelSelect(t *testing.T) {
	query, _ := buildListCurrentByAssetIDsQuery([]int64{77})
	if !strings.Contains(query, taskAssetSearchSelect) {
		t.Fatalf("query should reuse taskAssetSearchSelect")
	}
	if !strings.Contains(query, taskAssetSearchFrom) {
		t.Fatalf("query should reuse taskAssetSearchFrom")
	}
	if !strings.Contains(query, "ta.original_filename") || !strings.Contains(query, "ta.storage_key") {
		t.Fatalf("query missing key ZIP fields: %s", query)
	}
}
