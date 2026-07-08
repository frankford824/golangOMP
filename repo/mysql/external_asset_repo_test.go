package mysqlrepo

import (
	"strings"
	"testing"

	"workflow/domain"
)

func TestBuildExternalAssetWhereUsesFullTextForKeyword(t *testing.T) {
	where, args, orderBy := buildExternalAssetWhere(domain.ExternalAssetSearchQuery{Keyword: "KT poster", Page: 1, Size: 20})

	if !strings.Contains(where, "MATCH(file_name, origin_path, parent_path, searchable_text) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("where clause should use fulltext: %s", where)
	}
	if strings.Contains(where, "file_name LIKE ? OR origin_path LIKE ?") {
		t.Fatalf("keyword clause should not use contains LIKE in fulltext mode: %s", where)
	}
	if len(args) == 0 || args[0] != "+KT* +poster*" {
		t.Fatalf("args = %#v, want fulltext boolean query", args)
	}
	if !strings.Contains(orderBy, "updated_at DESC") {
		t.Fatalf("orderBy = %q, want updated_at order", orderBy)
	}
}

func TestBuildExternalAssetLikeWhereKeepsFallbackContainsMatch(t *testing.T) {
	where, args, _ := buildExternalAssetLikeWhere(domain.ExternalAssetSearchQuery{Keyword: "海报", Page: 1, Size: 20})

	if strings.Contains(where, "MATCH(") {
		t.Fatalf("fallback where should not use fulltext: %s", where)
	}
	if got := strings.Count(where, "LIKE ?"); got < 4 {
		t.Fatalf("LIKE placeholders = %d, want at least 4 in %s", got, where)
	}
	if len(args) < 4 || args[0] != "%海报%" {
		t.Fatalf("args = %#v, want contains fallback args", args)
	}
}

func TestExternalAssetDirectoryPathsIncludeMountAncestors(t *testing.T) {
	paths := externalAssetDirectoryPaths(`/quark/A06杨玲/2025杨玲`, `/quark`)
	want := []string{`/quark`, `/quark/A06杨玲`, `/quark/A06杨玲/2025杨玲`}

	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q in %#v", i, paths[i], want[i], paths)
		}
	}
}

func TestExternalAssetDirectoryPathsRejectsOutsideMount(t *testing.T) {
	if got := externalAssetDirectoryPaths(`/p3/A`, `/quark`); len(got) != 0 {
		t.Fatalf("paths outside mount = %#v, want empty", got)
	}
}

func TestExternalAssetDirectoryClausesUseParentHash(t *testing.T) {
	clauses, args := externalAssetDirectoryClauses(`/quark/A06杨玲`, []string{`/quark`, `/quark`})

	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(joined, `parent_path_hash = ?`) {
		t.Fatalf("clauses = %#v, want parent_path_hash predicate", clauses)
	}
	if !strings.Contains(joined, `mount_path IN (?)`) {
		t.Fatalf("clauses = %#v, want deduplicated mount predicate", clauses)
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v, want parent hash and one mount", args)
	}
	if args[0] != externalAssetParentPathHash(`/quark/A06杨玲`) || args[1] != `/quark` {
		t.Fatalf("args = %#v, want parent hash and /quark", args)
	}
}

func TestExternalAssetPrepareLimitAllowsServiceBatchSize(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "default", limit: 0, want: 20},
		{name: "normal", limit: 100, want: 100},
		{name: "service max", limit: 200, want: 200},
		{name: "cap", limit: 500, want: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := externalAssetPrepareLimit(tt.limit); got != tt.want {
				t.Fatalf("externalAssetPrepareLimit(%d) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}
