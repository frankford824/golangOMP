package mysqlrepo

import (
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
	if !strings.Contains(spec.fromSQL, "WHEN SUM(CASE WHEN asset_type IN ('delivery'") {
		t.Fatalf("fromSQL missing delivery-priority asset projection: %s", spec.fromSQL)
	}
	if !strings.Contains(spec.fromSQL, "la ON la.task_id = t.id") {
		t.Fatalf("fromSQL missing latest asset alias join: %s", spec.fromSQL)
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
}

func TestScanTaskListItemRowAllowsMissingTaskDetail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Now()
	columns := make([]string, 79)
	for i := range columns {
		columns[i] = fmt.Sprintf("c%d", i)
	}
	values := make([]driver.Value, 79)
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
	values[25] = false                                  // customization_required
	values[26] = ""                                     // customization_source_type
	values[28] = ""                                     // warehouse_reject_reason
	values[29] = ""                                     // warehouse_reject_category
	values[30] = false                                  // is_batch_task
	values[31] = int64(1)                               // batch_item_count
	values[32] = string(domain.TaskBatchModeSingle)     // batch_mode
	values[33] = "SKU-000005"                           // primary_sku_code

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
	if !strings.Contains(spec.whereSQL, "t.customization_required = 1") {
		t.Fatalf("whereSQL missing customization lane clause: %s", spec.whereSQL)
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
	if !strings.Contains(spec.whereSQL, "t.customization_required = 1") {
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
	if !strings.Contains(spec.whereSQL, "t.task_status IN (?) AND t.customization_required = 0") {
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
		"t.owner_department IN (?)",
		"(SELECT department FROM users WHERE id = t.creator_id) IN (?)",
		"(SELECT department FROM users WHERE id = t.designer_id) IN (?)",
		"(SELECT department FROM users WHERE id = t.current_handler_id) IN (?)",
	} {
		if !strings.Contains(spec.whereSQL, want) {
			t.Fatalf("whereSQL missing %q: %s", want, spec.whereSQL)
		}
	}
	if got, want := len(spec.args), 4; got != want {
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

func TestAppendTaskDataScopeWhereKeepsPlainDepartmentScopeNarrow(t *testing.T) {
	spec, err := buildTaskListQuerySpec(repo.TaskListFilter{
		ScopeDepartmentCodes: []string{"设计研发部"},
	}, nil)
	if err != nil {
		t.Fatalf("buildTaskListQuerySpec() error = %v", err)
	}
	if !strings.Contains(spec.whereSQL, "t.owner_department IN (?)") {
		t.Fatalf("whereSQL missing owner department scope: %s", spec.whereSQL)
	}
	if strings.Contains(spec.whereSQL, "SELECT department FROM users") {
		t.Fatalf("plain department scope should not include user department subqueries: %s", spec.whereSQL)
	}
}

func workflowLanePtr(lane domain.WorkflowLane) *domain.WorkflowLane {
	return &lane
}
