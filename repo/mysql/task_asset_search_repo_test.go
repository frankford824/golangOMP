package mysqlrepo

import (
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestBuildTaskAssetSearchWhereKeywordCoversPlannedFields(t *testing.T) {
	query := domain.AssetSearchQuery{
		Keyword: "ABC-123",
	}

	where, args := buildTaskAssetSearchWhere(query)

	for _, expected := range []string{
		"t.sku_code = ?",
		"t.primary_sku_code = ?",
		"ta.scope_sku_code = ?",
		"t.task_no = ?",
		"t.sku_code LIKE ?",
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
	if got, wantMin := len(args), 11; got < wantMin {
		t.Fatalf("args len = %d, want at least %d", got, wantMin)
	}

	wantLike := "%ABC-123%"
	if !containsStringArg(args, wantLike) {
		t.Fatalf("args missing fuzzy fallback %q: %#v", wantLike, args)
	}
	if !containsStringArg(args, "ABC-123") || !containsStringArg(args, "ABC-123%") {
		t.Fatalf("args missing exact/prefix code matches: %#v", args)
	}
}

func TestBuildTaskAssetSearchWhereKeywordWithTaskStatusKeepsArgsAligned(t *testing.T) {
	query := domain.AssetSearchQuery{
		Keyword:    "1001",
		TaskStatus: domain.AssetTaskStatusFilterOpen,
	}

	where, args := buildTaskAssetSearchWhere(query)

	if !strings.Contains(where, "ta.asset_id = ?") {
		t.Fatalf("where clause missing exact asset_id keyword match: %s", where)
	}
	if !strings.Contains(where, "ta.task_id = ?") {
		t.Fatalf("where clause missing exact task_id keyword match: %s", where)
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

func TestBuildTaskAssetSearchWhereExcludesSystemDerivedPreviewAssets(t *testing.T) {
	where, _ := buildTaskAssetSearchWhere(domain.AssetSearchQuery{})

	for _, expected := range []string{
		"da.source_asset_id IS NOT NULL",
		"da.asset_type IN ('preview', 'design_thumb')",
		"async-derived-preview:webp",
	} {
		if !strings.Contains(where, expected) {
			t.Fatalf("where clause missing %q: %s", expected, where)
		}
	}
}

func TestBuildTaskAssetSearchWhereDefaultsToRelevantAssetFormats(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{})

	for _, expected := range []string{
		"LOWER(ta.file_name) LIKE ?",
		"LOWER(COALESCE(ta.original_filename, '')) LIKE ?",
		"LOWER(COALESCE(ta.mime_type, '')) LIKE ?",
	} {
		if !strings.Contains(where, expected) {
			t.Fatalf("where clause missing format condition %q: %s", expected, where)
		}
	}
	for _, ignored := range []string{"%.doc", "%.docx", "%.json"} {
		if containsStringArg(args, ignored) {
			t.Fatalf("default asset search should not include ignored format %q in args: %#v", ignored, args)
		}
	}
}

func TestBuildTaskAssetSearchWhereFiltersUsableState(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		UsableState: domain.AssetUsableStateFilterReadyForUse,
	})

	if !strings.Contains(where, "ta.flow_review_status = ?") {
		t.Fatalf("where clause missing flow review status filter: %s", where)
	}
	if !containsStringArg(args, string(domain.TaskAssetFlowReviewStatusApproved)) {
		t.Fatalf("args missing approved status: %#v", args)
	}
}

func TestBuildTaskAssetSearchWhereFiltersBusinessLane(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		BusinessLane: domain.TaskBusinessLaneCustomization,
	})

	if !strings.Contains(where, "COALESCE(t.business_lane, '') = ?") {
		t.Fatalf("where clause missing customization business lane filter: %s", where)
	}
	if !containsStringArg(args, string(domain.TaskBusinessLaneCustomization)) {
		t.Fatalf("args missing customization business lane: %#v", args)
	}

	normalWhere, normalArgs := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		BusinessLane: domain.TaskBusinessLaneNormal,
	})
	if !strings.Contains(normalWhere, "(COALESCE(t.business_lane, '') = '' OR t.business_lane = ?)") {
		t.Fatalf("where clause missing normal business lane filter: %s", normalWhere)
	}
	if !containsStringArg(normalArgs, string(domain.TaskBusinessLaneNormal)) {
		t.Fatalf("args missing normal business lane: %#v", normalArgs)
	}
}

func TestBuildTaskAssetSearchWhereFiltersAssetTypeWithAliases(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		AssetType: domain.TaskAssetTypeDelivery,
	})

	if !strings.Contains(where, "ta.asset_type IN (?, ?, ?, ?, ?)") {
		t.Fatalf("where clause missing delivery asset type filter: %s", where)
	}
	for _, expected := range []string{"delivery", "draft", "revised", "final", "outsource_return"} {
		if !containsStringArg(args, expected) {
			t.Fatalf("args missing delivery alias %q: %#v", expected, args)
		}
	}

	sourceWhere, sourceArgs := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		AssetType: domain.TaskAssetTypeSource,
	})
	if !strings.Contains(sourceWhere, "ta.asset_type IN (?, ?)") {
		t.Fatalf("where clause missing source asset type filter: %s", sourceWhere)
	}
	for _, expected := range []string{"source", "original"} {
		if !containsStringArg(sourceArgs, expected) {
			t.Fatalf("args missing source alias %q: %#v", expected, sourceArgs)
		}
	}
}

func TestBuildTaskAssetSearchWhereFiltersByUploadedTimeByDefault(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		CreatedFrom: &from,
		CreatedTo:   &to,
	})

	if !strings.Contains(where, "ta.sort_time >= ?") {
		t.Fatalf("where clause missing uploaded time lower bound: %s", where)
	}
	if !strings.Contains(where, "ta.sort_time <= ?") {
		t.Fatalf("where clause missing uploaded time upper bound: %s", where)
	}
	if containsStringArg(args, "t.created_at") {
		t.Fatalf("args should contain values only: %#v", args)
	}
}

func TestBuildTaskAssetSearchWhereFiltersByTaskCreatedTime(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	where, _ := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		TimeBasis:   domain.AssetSearchTimeBasisTaskCreatedAt,
		CreatedFrom: &from,
	})

	if !strings.Contains(where, "t.created_at >= ?") {
		t.Fatalf("where clause missing task created time lower bound: %s", where)
	}
	if strings.Contains(where, "ta.sort_time >= ?") {
		t.Fatalf("where clause should not use uploaded time for task_created_at: %s", where)
	}
}

func TestTaskAssetSearchOrderByFollowsTimeBasis(t *testing.T) {
	if got := taskAssetSearchOrderBy(domain.AssetSearchQuery{}); !strings.Contains(got, "ta.sort_time DESC") {
		t.Fatalf("default order by = %q, want uploaded time", got)
	}
	if got := taskAssetSearchOrderBy(domain.AssetSearchQuery{TimeBasis: domain.AssetSearchTimeBasisTaskCreatedAt}); !strings.Contains(got, "t.created_at DESC") {
		t.Fatalf("task-created order by = %q, want task created time", got)
	}
}

func TestBuildTaskAssetSearchWhereFiltersFormatCategory(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		FormatCategory: domain.AssetFormatCategoryDesign,
	})

	if !strings.Contains(where, "LOWER(ta.file_name) LIKE ?") {
		t.Fatalf("where clause missing file extension format filter: %s", where)
	}
	if !containsStringArg(args, "%.psd") {
		t.Fatalf("args missing psd extension: %#v", args)
	}
	if containsStringArg(args, "%.jpg") {
		t.Fatalf("design format filter should not include image extensions: %#v", args)
	}
}

func TestBuildTaskAssetSearchWhereFiltersEditableAssets(t *testing.T) {
	where, args := buildTaskAssetSearchWhere(domain.AssetSearchQuery{
		UsableState: domain.AssetUsableStateFilterEditable,
	})

	for _, expected := range []string{
		"ta.asset_type IN (?, ?, ?)",
		"COALESCE(ta.flow_review_status, '') NOT IN (?, ?)",
	} {
		if !strings.Contains(where, expected) {
			t.Fatalf("where clause missing %q: %s", expected, where)
		}
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
	if !strings.Contains(query, "ta.id = da.current_version_id") {
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

func TestBuildListCurrentByAssetIDsQueryUsesLegacyCurrentVersionWhenEnabled(t *testing.T) {
	t.Setenv("ASSET_SEARCH_LEGACY_CURRENT_VERSION", "true")
	query, _ := buildListCurrentByAssetIDsQuery([]int64{101})
	if !strings.Contains(query, "ta.id = COALESCE(da.current_version_id") {
		t.Fatalf("query missing legacy current-version fallback: %s", query)
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

func TestTaskAssetSearchProjectionResolvesLegacyModuleLink(t *testing.T) {
	if !strings.Contains(taskAssetSearchSelect, "COALESCE(ta.source_task_module_id, source_tm.id)") {
		t.Fatalf("search projection must resolve missing source_task_module_id from the task module")
	}
	if !strings.Contains(taskAssetSearchFrom, "source_tm.task_id = ta.task_id") ||
		!strings.Contains(taskAssetSearchFrom, "source_tm.module_key = COALESCE(NULLIF(ta.source_module_key, ''), 'design') COLLATE utf8mb4_unicode_ci") {
		t.Fatalf("search join must resolve the task module by task_id and source_module_key")
	}
}

func TestTaskAssetSearchDerivedPreviewRequiresCurrentSourceVersion(t *testing.T) {
	normalized := strings.Join(strings.Fields(taskAssetSearchFrom), " ")
	for _, required := range []string{
		"JOIN task_assets candidate_version ON candidate_version.id = candidate.current_version_id",
		"candidate_version.source_asset_version_id = ta.id",
		"candidate_version.deleted_at IS NULL",
		"candidate_version.cleaned_at IS NULL",
		"COALESCE(candidate_version.storage_key, '') <> ''",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("derived preview selection missing %q: %s", required, normalized)
		}
	}
}

func containsStringArg(args []interface{}, want string) bool {
	for _, arg := range args {
		got, ok := arg.(string)
		if ok && got == want {
			return true
		}
	}
	return false
}
