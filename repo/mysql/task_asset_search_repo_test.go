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
