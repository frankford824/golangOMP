package mysqlrepo

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestScanTaskHydratesStableOrganizationIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "task_no", "source_mode", "product_id", "sku_code", "product_name_snapshot",
		"task_type", "operator_group_id", "owner_team", "owner_department", "owner_org_team", "owner_department_id", "owner_team_id", "creator_id", "requester_id", "designer_id", "current_handler_id",
		"task_status", "workflow_revision", "priority", "deadline_at", "need_outsource", "is_outsource", "business_lane", "customization_required", "customization_source_type",
		"last_customization_operator_id", "warehouse_reject_reason", "warehouse_reject_category",
		"is_batch_task", "batch_item_count", "batch_mode", "primary_sku_code", "sku_generation_status", "created_at", "updated_at",
	}).AddRow(
		int64(1), "RW-1", "existing_product", nil, "SKU-1", "Product",
		"original_product_development", nil, "display team", "display department", "display org team", int64(101), int64(202), int64(7), nil, nil, nil,
		"PendingAudit", int64(3), "normal", nil, false, false, "normal", false, "",
		nil, "", "", false, int64(1), "single", "SKU-1", "not_applicable", now, now,
	)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	task, err := scanTask(db.QueryRow("SELECT"))
	if err != nil {
		t.Fatalf("scanTask() error = %v", err)
	}
	if task.OwnerDepartmentID == nil || *task.OwnerDepartmentID != 101 || task.OwnerTeamID == nil || *task.OwnerTeamID != 202 {
		t.Fatalf("stable organization ids = department:%v team:%v", task.OwnerDepartmentID, task.OwnerTeamID)
	}
	if subject := task.AccessSubject(); subject.OwnerDepartmentID == nil || *subject.OwnerDepartmentID != 101 || subject.OwnerTeamID == nil || *subject.OwnerTeamID != 202 {
		t.Fatalf("access subject = %+v", subject)
	}
}

func TestAccessSubjectOwnDepartmentAndTeamUseStableIDs(t *testing.T) {
	departmentID, teamID := int64(101), int64(202)
	subject := domain.TaskAccessSubject{TaskID: 1, CreatorID: 7, OwnerDepartmentID: &departmentID, OwnerTeamID: &teamID}
	for _, tc := range []struct {
		name      string
		scope     domain.AccessScopeMode
		actorDept *int64
		actorTeam *int64
	}{
		{name: "department", scope: domain.AccessScopeOwnDepartment, actorDept: &departmentID},
		{name: "team", scope: domain.AccessScopeOwnTeam, actorTeam: &teamID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := domain.RequestActor{ID: 9, DepartmentID: tc.actorDept, TeamID: tc.actorTeam}
			assignment := domain.AccessAssignment{RoleID: 8, ScopeMode: tc.scope}
			actor.EffectiveAccess = &domain.EffectiveAccess{
				UserID: actor.ID, Permissions: []domain.PermissionCode{domain.PermissionTaskAuditDecision},
				Assignments: []domain.AccessAssignment{assignment},
				Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionTaskAuditDecision, RoleID: assignment.RoleID, ScopeMode: assignment.ScopeMode}},
			}
			if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, subject) {
				t.Fatalf("stable %s scope denied", tc.name)
			}
		})
	}
}
