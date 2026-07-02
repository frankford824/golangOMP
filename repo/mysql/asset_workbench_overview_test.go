package mysqlrepo

import (
	"strings"
	"testing"

	"workflow/repo"
)

func TestBuildAssetWorkbenchOverviewQueryAlignsStringCollations(t *testing.T) {
	query, _ := buildAssetWorkbenchOverviewQuery(repo.AssetWorkbenchOverviewSearchFilter{
		Keyword: "CGP000071",
		Creator: "交付同学",
	})

	if got := strings.Count(query, "COLLATE utf8mb4_0900_ai_ci"); got < 20 {
		t.Fatalf("overview query COLLATE count = %d, want >= 20\n%s", got, query)
	}
	for _, expected := range []string{
		"CAST((s.submission_no) AS CHAR CHARACTER SET utf8mb4) COLLATE utf8mb4_0900_ai_ci AS title",
		"CAST((i.order_no) AS CHAR CHARACTER SET utf8mb4) COLLATE utf8mb4_0900_ai_ci AS primary_code",
		"CAST(('submission_file') AS CHAR CHARACTER SET utf8mb4) COLLATE utf8mb4_0900_ai_ci AS source",
		"CAST((u.display_name) AS CHAR CHARACTER SET utf8mb4) COLLATE utf8mb4_0900_ai_ci",
		"CONCAT('/drive?file_id=', f.id)",
		"CAST((JSON_OBJECT",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("overview query missing %q\n%s", expected, query)
		}
	}
}
