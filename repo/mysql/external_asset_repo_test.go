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
