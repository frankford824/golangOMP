package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestBuildTaskListQuerySpecUsesJoinedLatestAssetProjection(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}

	if !strings.Contains(spec.latestAssetExpr, "ELSE la.asset_type") {
		t.Fatalf("latestAssetExpr = %q, want canonicalized la.asset_type expression", spec.latestAssetExpr)
	}
	if !strings.Contains(spec.fromSQL, "LEFT JOIN (") {
		t.Fatalf("fromSQL missing latest asset join: %s", spec.fromSQL)
	}
	if !strings.Contains(spec.fromSQL, "WHEN SUM(CASE WHEN ta.asset_type IN ('delivery'") {
		t.Fatalf("fromSQL missing delivery-priority asset projection: %s", spec.fromSQL)
	}
	if !strings.Contains(spec.fromSQL, "ta.id = da.current_version_id") {
		t.Fatalf("fromSQL missing current-version guard: %s", spec.fromSQL)
	}
	if !strings.Contains(spec.fromSQL, "la ON la.task_id = t.id") {
		t.Fatalf("fromSQL missing latest asset alias join: %s", spec.fromSQL)
	}
	if strings.Contains(spec.fromSQL, "t.idLEFT") {
		t.Fatalf("fromSQL concatenated warehouse and latest asset joins without whitespace: %s", spec.fromSQL)
	}
	if strings.Contains(spec.countFromSQL, "task_assets ta") {
		t.Fatalf("countFromSQL should not include latest asset join for plain list: %s", spec.countFromSQL)
	}
}

func TestBuildTaskListQuerySpecWarehouseBlockingUsesJoinedLatestAssetAlias(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			WarehouseBlockingReasonCodes: []domain.WorkflowReasonCode{
				domain.WorkflowReasonMissingFinalAsset,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}

	if !strings.Contains(spec.whereSQL, "COALESCE(CASE") || !strings.Contains(spec.whereSQL, "<> 'delivery'") {
		t.Fatalf("whereSQL = %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "SELECT ta.asset_type") {
		t.Fatalf("whereSQL should not contain repeated latest-asset scalar subquery: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.countFromSQL, "task_assets ta") {
		t.Fatalf("countFromSQL should keep latest asset join when where depends on la: %s", spec.countFromSQL)
	}
}

func TestBuildTaskListQuerySpecUsesSearchDocumentsForKeywordWhenEnabled(t *testing.T) {
	spec, err := buildTaskListQuerySpecWithOptions(repo.TaskListFilter{Keyword: "海报"}, nil, taskListQueryBuildOptions{UseSearchDocumentKeyword: true})
	if err != nil {
		t.Fatalf("buildTaskListQuerySpecWithOptions() error = %v", err)
	}
	if strings.Contains(spec.fromSQL, "task_search_documents") {
		t.Fatalf("fromSQL should recall via IN subquery, not a document join: %s", spec.fromSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.id IN (SELECT task_id FROM task_search_documents WHERE MATCH(search_text) AGAINST (? IN NATURAL LANGUAGE MODE))") {
		t.Fatalf("whereSQL missing task_search_documents fulltext recall: %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "task_keyword_actor") || strings.Contains(spec.whereSQL, "tsi_text") {
		t.Fatalf("whereSQL should not use legacy keyword EXISTS on document path: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecUsesUnionRecallForCodeKeyword(t *testing.T) {
	spec, err := buildTaskListQuerySpecWithOptions(repo.TaskListFilter{Keyword: "CGK000181"}, nil, taskListQueryBuildOptions{UseSearchDocumentKeyword: true})
	if err != nil {
		t.Fatalf("buildTaskListQuerySpecWithOptions() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.id IN (SELECT task_id FROM (") || !strings.Contains(spec.whereSQL, "UNION ALL") {
		t.Fatalf("whereSQL missing union recall for code keyword: %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "MATCH(") {
		t.Fatalf("code keyword recall must not use fulltext match: %s", spec.whereSQL)
	}
}

func TestScanTaskListItemRowAllowsMissingTaskDetail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Now()
	columns := make([]string, 81)
	for i := range columns {
		columns[i] = fmt.Sprintf("c%d", i)
	}
	values := make([]driver.Value, 81)
	values[0] = int64(26)                               // id
	values[1] = "RW-20260313-A-000022"                  // task_no
	values[3] = "SKU-000005"                            // sku_code
	values[4] = "v0.3 regression retry 1773368404"      // product_name_snapshot
	values[5] = string(domain.TaskTypePurchaseTask)     // task_type
	values[6] = string(domain.TaskSourceModeNewProduct) // source_mode
	values[7] = "team-a"                                // owner_team
	values[8] = ""                                      // owner_department
	values[9] = ""                                      // owner_org_team
	values[10] = string(domain.TaskPriorityLow)         // priority
	values[11] = int64(7)                               // creator_id
	values[12] = int64(8)                               // requester_id
	values[15] = "Requester 8"                          // requester_name
	values[16] = "Creator 7"                            // creator_name
	values[19] = string(domain.TaskStatusPendingAssign) // task_status
	values[20] = now                                    // created_at
	values[21] = now                                    // updated_at
	values[23] = false                                  // need_outsource
	values[24] = false                                  // is_outsource
	values[25] = string(domain.TaskBusinessLaneNormal)  // business_lane
	values[26] = false                                  // customization_required
	values[27] = ""                                     // customization_source_type
	values[29] = ""                                     // warehouse_reject_reason
	values[30] = ""                                     // warehouse_reject_category
	values[31] = false                                  // is_batch_task
	values[32] = int64(1)                               // batch_item_count
	values[33] = string(domain.TaskBatchModeSingle)     // batch_mode
	values[34] = "SKU-000005"                           // primary_sku_code
	values[35] = string(domain.TaskSKUCodeTypeRegular)  // sku_code_type

	rows := sqlmock.NewRows(columns).AddRow(values...)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	sqlRows, err := db.Query("SELECT")
	if err != nil {
		t.Fatalf("db.Query() error = %v", err)
	}
	defer sqlRows.Close()
	if !sqlRows.Next() {
		t.Fatal("sqlRows.Next() = false, want true")
	}

	item, err := scanTaskListItemRow(sqlRows)
	if err != nil {
		t.Fatalf("scanTaskListItemRow() error = %v", err)
	}
	if item == nil {
		t.Fatal("scanTaskListItemRow() = nil")
	}
	if item.ID != 26 {
		t.Fatalf("item.ID = %d, want 26", item.ID)
	}
	if item.RequesterID == nil || *item.RequesterID != 8 || item.RequesterName != "Requester 8" {
		t.Fatalf("requester projection = (%v, %q), want (8, Requester 8)", item.RequesterID, item.RequesterName)
	}
	if item.CreatorName != "Creator 7" {
		t.Fatalf("item.CreatorName = %q, want Creator 7", item.CreatorName)
	}
	if item.Category != "" || item.SpecText != "" || item.Material != "" || item.SizeText != "" || item.CraftText != "" {
		t.Fatalf("nullable task_detail strings should stay empty: %+v", item)
	}
	if item.ProcurementStatus != nil || item.WarehouseStatus != nil || item.LatestAssetType != nil {
		t.Fatalf("nullable joins should remain nil-backed: %+v", item)
	}
	if err := sqlRows.Err(); err != nil {
		t.Fatalf("sqlRows.Err() = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}

func TestBuildTaskListQuerySpecSupportsCanonicalOwnerFiltersAndScope(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			OwnerDepartments: []string{"运营部"},
			OwnerOrgTeams:    []string{"运营三组"},
		},
		ScopeDepartmentCodes: []string{"设计部"},
		ScopeTeamCodes:       []string{"设计审核组"},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}

	if !strings.Contains(spec.whereSQL, "t.owner_department IN") {
		t.Fatalf("whereSQL missing owner_department filter: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.owner_org_team IN") {
		t.Fatalf("whereSQL missing owner_org_team filter: %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "t.owner_team IN") {
		t.Fatalf("whereSQL should not use legacy owner_team for scoped canonical filters: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecSupportsWorkflowLaneFilter(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "COALESCE(t.business_lane, '') = 'customization'") {
		t.Fatalf("whereSQL missing customization lane clause: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecSupportsPriorityFilter(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Priorities: []domain.TaskPriority{domain.TaskPriorityCritical},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.priority IN (?)") {
		t.Fatalf("whereSQL missing t.priority IN clause: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecSupportsPriorityMultiValueFilter(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Priorities: []domain.TaskPriority{
				domain.TaskPriorityCritical,
				domain.TaskPriorityHigh,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.priority IN (?, ?)") {
		t.Fatalf("whereSQL missing multi-value t.priority IN clause: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecPriorityDoesNotBreakStatusFilter(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Priorities: []domain.TaskPriority{domain.TaskPriorityCritical},
			Statuses:   []domain.TaskStatus{domain.TaskStatusInProgress},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.priority IN (?)") {
		t.Fatalf("whereSQL missing t.priority IN clause: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.task_status IN (?)") {
		t.Fatalf("whereSQL missing t.task_status IN clause: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecSupportsStageVisibilityScope(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeStageVisibilities: []repo.ScopeStageVisibility{
			{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingCustomizationReview,
					domain.TaskStatusPendingEffectReview,
				},
				Lane: workflowLanePtr(domain.WorkflowLaneCustomization),
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if strings.Contains(spec.whereSQL, "1=0") {
		t.Fatalf("whereSQL should not collapse to 1=0 when stage visibility exists: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.task_status IN (?, ?)") {
		t.Fatalf("whereSQL missing stage status IN clause: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "COALESCE(t.business_lane, '') = 'customization'") {
		t.Fatalf("whereSQL missing stage lane clause: %s", spec.whereSQL)
	}
}

func TestAppendTaskDataScopeWhereOrsExistingAndStageClauses(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeDepartmentCodes: []string{string(domain.DepartmentOperations)},
		ScopeStageVisibilities: []repo.ScopeStageVisibility{
			{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingAuditA,
				},
				Lane: workflowLanePtr(domain.WorkflowLaneNormal),
			},
			{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingWarehouseReceive,
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.owner_department IN (?)") {
		t.Fatalf("whereSQL missing department scope clause: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.task_status IN (?) AND (COALESCE(t.business_lane, '') = '' OR t.business_lane = 'normal')") {
		t.Fatalf("whereSQL missing normal-lane stage clause: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "t.task_status IN (?)") {
		t.Fatalf("whereSQL missing unrestricted stage clause: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, " OR ") {
		t.Fatalf("whereSQL missing OR-joined scope block: %s", spec.whereSQL)
	}
}

func TestAppendTaskDataScopeWhereIncludesManagedDepartmentUserTies(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeManagedDepartmentCodes: []string{"设计研发部"},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	for _, want := range []string{
		"t.owner_department IN (?, ?, ?)",
		"(SELECT department FROM users WHERE id = t.creator_id) IN (?, ?, ?)",
		"(SELECT department FROM users WHERE id = t.designer_id) IN (?, ?, ?)",
		"(SELECT department FROM users WHERE id = t.current_handler_id) IN (?, ?, ?)",
	} {
		if !strings.Contains(spec.whereSQL, want) {
			t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
		}
	}
	if got, want := len(spec.args), 12; got != want {
		t.Fatalf("args len = %d, want %d; args=%+v", got, want, spec.args)
	}
}

func TestAppendTaskDataScopeWhereIncludesManagedTeamUserTies(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeManagedTeamCodes: []string{"默认组"},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	for _, want := range []string{
		"t.owner_org_team IN (?)",
		"(SELECT team FROM users WHERE id = t.creator_id) IN (?)",
		"(SELECT team FROM users WHERE id = t.designer_id) IN (?)",
		"(SELECT team FROM users WHERE id = t.current_handler_id) IN (?)",
	} {
		if !strings.Contains(spec.whereSQL, want) {
			t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
		}
	}
}

func TestBuildTaskListQuerySpecMineActorOwnership(t *testing.T) {
	actorID := int64(88)
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{MineActorID: &actorID}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	want := "(t.creator_id = ? OR t.designer_id = ? OR t.current_handler_id = ?)"
	if !strings.Contains(spec.whereSQL, want) {
		t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
	}
	if len(spec.args) < 3 {
		t.Fatalf("args len = %d, want at least 3 mine actor placeholders", len(spec.args))
	}
	for i := len(spec.args) - 3; i < len(spec.args); i++ {
		if spec.args[i] != actorID {
			t.Fatalf("mine actor arg[%d] = %v, want %d", i, spec.args[i], actorID)
		}
	}
}

func TestBuildTaskListQuerySpecCurrentHandlerOwnership(t *testing.T) {
	handlerID := int64(331)
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{CurrentHandlerID: &handlerID}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	want := "t.current_handler_id = ?"
	if !strings.Contains(spec.whereSQL, want) {
		t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
	}
	if len(spec.args) == 0 || spec.args[len(spec.args)-1] != handlerID {
		t.Fatalf("last arg = %v, want %d", spec.args, handlerID)
	}
}

func TestBuildTaskListQuerySpecExcludesPendingAuditHandover(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{ExcludePendingAuditHandover: true}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	for _, want := range []string{"NOT EXISTS", "audit_handovers ah_pending", "ah_pending.status = ?"} {
		if !strings.Contains(spec.whereSQL, want) {
			t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
		}
	}
	if len(spec.args) == 0 || spec.args[len(spec.args)-1] != string(domain.HandoverStatusPendingTakeover) {
		t.Fatalf("last arg = %v, want pending_takeover", spec.args)
	}
}

func TestBuildTaskListQuerySpecDesignerEmpty(t *testing.T) {
	designerEmpty := true
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{DesignerEmpty: &designerEmpty}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	want := "(t.designer_id IS NULL OR t.designer_id = 0)"
	if !strings.Contains(spec.whereSQL, want) {
		t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecKeywordIncludesTaskSkuItems(t *testing.T) {
	keyword := "CGG000026"
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{Keyword: keyword}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}

	if !strings.Contains(spec.whereSQL, "EXISTS") {
		t.Fatalf("whereSQL missing EXISTS: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "task_sku_items tsi") {
		t.Fatalf("whereSQL missing task_sku_items: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "tsi.sku_code = ?") {
		t.Fatalf("whereSQL missing exact batch sku item match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "tsi.sku_code LIKE ?") {
		t.Fatalf("whereSQL missing prefix batch sku item match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "tsi_text.product_name_snapshot LIKE ?") {
		t.Fatalf("whereSQL missing batch sku item product name match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "tsi_text.product_short_name LIKE ?") {
		t.Fatalf("whereSQL missing batch sku item short name match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "tsi_text.design_requirement LIKE ?") {
		t.Fatalf("whereSQL missing batch sku item design requirement match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "users task_keyword_actor") {
		t.Fatalf("whereSQL missing task actor keyword user lookup: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "task_keyword_actor.display_name LIKE ?") {
		t.Fatalf("whereSQL missing actor display_name match: %s", spec.whereSQL)
	}
	if !strings.Contains(spec.whereSQL, "task_keyword_actor.username LIKE ?") {
		t.Fatalf("whereSQL missing actor username match: %s", spec.whereSQL)
	}
	if strings.Contains(spec.fromSQL, "JOIN task_sku_items") {
		t.Fatalf("fromSQL must not JOIN task_sku_items (would duplicate rows): %s", spec.fromSQL)
	}

	like := "%" + keyword + "%"
	for _, want := range []interface{}{like, keyword, keyword + "%"} {
		if !containsInterfaceArg(spec.args, want) {
			t.Fatalf("args missing %#v: %#v", want, spec.args)
		}
	}
}

func TestBuildTaskListQuerySpecKeywordOmitsTaskSkuItemsWhenEmpty(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if strings.Contains(spec.whereSQL, "task_sku_items") {
		t.Fatalf("whereSQL should not reference task_sku_items without keyword: %s", spec.whereSQL)
	}
}

func TestTaskRepoListKeywordBatchSkuItemSearch(t *testing.T) {
	t.Run("primary batch sku", func(t *testing.T) {
		runTaskRepoListKeywordSearchCase(t, "CGG000025", 1, 1)
	})
	t.Run("non-primary batch sku", func(t *testing.T) {
		runTaskRepoListKeywordSearchCase(t, "CGG000026", 1, 1)
	})
	t.Run("missing sku", func(t *testing.T) {
		runTaskRepoListKeywordSearchCase(t, "CGG000999", 0, 0)
	})
}

func runTaskRepoListKeywordSearchCase(t *testing.T, keyword string, wantTotal int64, wantItems int) {
	t.Helper()

	mysqlSchemaPresenceCache.Store(
		mysqlSchemaCacheKey{kind: "table", table: "task_search_documents"},
		mysqlSchemaCacheEntry{exists: false, expiresAt: time.Now().Add(time.Minute)},
	)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	taskRepo := NewTaskRepo(&DB{db: db})
	like := "%" + keyword + "%"
	prefix := keyword + "%"
	countArgs := []driver.Value{
		like, like, like, like, like, like, like, like, like,
		keyword, keyword, keyword,
		prefix, prefix, prefix,
		keyword, prefix,
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(wantTotal))

	if wantItems > 0 {
		mock.ExpectQuery("SELECT t.id").
			WithArgs(append(countArgs, 20, 0)...).
			WillReturnRows(newTaskListItemSQLMockRow(t))
		mock.ExpectQuery("SELECT id, task_id, sequence_no, sku_code").
			WithArgs(int64(100)).
			WillReturnRows(newTaskListSKUItemSQLMockRows())
	} else {
		mock.ExpectQuery("SELECT t.id").
			WithArgs(append(countArgs, 20, 0)...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}

	items, total, err := taskRepo.List(context.Background(), repo.TaskListFilter{
		Keyword:      keyword,
		Page:         1,
		PageSize:     20,
		ScopeViewAll: true,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != wantTotal {
		t.Fatalf("total = %d, want %d", total, wantTotal)
	}
	if len(items) != wantItems {
		t.Fatalf("len(items) = %d, want %d", len(items), wantItems)
	}
	if wantItems > 0 && len(items[0].SKUItems) != 2 {
		t.Fatalf("len(items[0].SKUItems) = %d, want 2", len(items[0].SKUItems))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}

func containsInterfaceArg(args []interface{}, want interface{}) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func newTaskListItemSQLMockRow(t *testing.T) *sqlmock.Rows {
	t.Helper()

	now := time.Now()
	columns := make([]string, 81)
	for i := range columns {
		columns[i] = fmt.Sprintf("c%d", i)
	}
	values := make([]driver.Value, 81)
	values[0] = int64(100)
	values[1] = "T-BATCH-001"
	values[3] = "CGG000025"
	values[4] = "batch new product task"
	values[5] = string(domain.TaskTypeNewProductDevelopment)
	values[6] = string(domain.TaskSourceModeNewProduct)
	values[7] = "team-a"
	values[8] = ""
	values[9] = ""
	values[10] = string(domain.TaskPriorityLow)
	values[16] = "Creator"
	values[19] = string(domain.TaskStatusPendingAssign)
	values[11] = int64(1)
	values[20] = now
	values[21] = now
	values[23] = false
	values[24] = false
	values[25] = string(domain.TaskBusinessLaneNormal)
	values[26] = false
	values[31] = true
	values[32] = int64(5)
	values[33] = string(domain.TaskBatchModeMultiSKU)
	values[34] = "CGG000025"
	values[35] = string(domain.TaskSKUCodeTypeRegular)
	return sqlmock.NewRows(columns).AddRow(values...)
}

func newTaskListSKUItemSQLMockRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id",
		"task_id",
		"sequence_no",
		"sku_code",
		"sku_status",
		"product_name_snapshot",
		"product_short_name",
		"design_requirement",
		"filing_status",
		"erp_sync_status",
		"erp_sync_required",
		"erp_sync_version",
		"last_filed_at",
		"filing_error_message",
		"created_at",
		"updated_at",
	}).
		AddRow(int64(1001), int64(100), 1, "CGG000025", string(domain.TaskSKUStatusGenerated), "寿比南山 A", "寿比南山 A", "升学宴主视觉", string(domain.FilingStatusFiled), string(domain.FilingStatusFiled), false, int64(1), now, "", now, now).
		AddRow(int64(1002), int64(100), 2, "CGG000026", string(domain.TaskSKUStatusGenerated), "寿比南山 B", "寿比南山 B", "升学宴副视觉", string(domain.FilingStatusFilingFailed), string(domain.FilingStatusFilingFailed), true, int64(2), nil, "ERP失败", now, now)
}

func TestAppendTaskDataScopeWhereExpandsPlainDepartmentAliasesWithoutUserTies(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeDepartmentCodes: []string{"设计研发部"},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.owner_department IN (?, ?, ?)") {
		t.Fatalf("whereSQL missing owner department scope: %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "SELECT department FROM users") {
		t.Fatalf("plain department scope should not include user department subqueries: %s", spec.whereSQL)
	}
}

func TestBuildTaskListQuerySpecExpandsRenamedOwnerOrgTeamFilter(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			OwnerOrgTeams: []string{"淘系运营三部"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.owner_org_team IN (?, ?)") {
		t.Fatalf("whereSQL missing expanded owner org team filter: %s", spec.whereSQL)
	}
	if got, want := len(spec.args), 2; got != want {
		t.Fatalf("args len = %d, want %d; args=%+v", got, want, spec.args)
	}
	if !taskTestArgsContain(spec.args, "淘系运营三部") || !taskTestArgsContain(spec.args, "淘系三组") {
		t.Fatalf("args = %+v, want current and historical org team aliases", spec.args)
	}
}

func workflowLanePtr(lane domain.WorkflowLane) *domain.WorkflowLane {
	return &lane
}

func taskTestArgsContain(args []interface{}, target string) bool {
	for _, arg := range args {
		if value, ok := arg.(string); ok && value == target {
			return true
		}
	}
	return false
}
