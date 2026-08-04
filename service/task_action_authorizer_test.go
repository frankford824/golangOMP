package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskActionAuthorizerAllowsTrustedInternalCallWithoutActor(t *testing.T) {
	task := &domain.Task{ID: 11, TaskStatus: domain.TaskStatusInProgress}
	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(context.Background(), TaskActionReadDetail, task, "", "")
	if !decision.Allowed || decision.MatchedRule != "trusted_internal_call" {
		t.Fatalf("decision = %+v, want trusted internal allow", decision)
	}
}

func TestTaskActionAuthorizerRejectsLegacyRoleWithoutEffectiveAccess(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 21, Roles: []domain.Role{domain.RoleAdmin}})
	task := &domain.Task{ID: 12, TaskStatus: domain.TaskStatusInProgress, CreatorID: 21}
	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if decision.Allowed || decision.DenyCode != "effective_access_required" {
		t.Fatalf("decision = %+v, want fail-closed without effective access", decision)
	}
}

func TestTaskActionAuthorizerUsesStableDepartmentScope(t *testing.T) {
	departmentID := int64(8)
	actor := taskActionTestActor(31, domain.PermissionTaskView, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &departmentID
	ctx := domain.WithRequestActor(context.Background(), actor)
	task := &domain.Task{ID: 13, TaskStatus: domain.TaskStatusInProgress, CreatorID: 99, OwnerDepartmentID: &departmentID}

	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed || decision.ScopeSource != "explicit_access" {
		t.Fatalf("decision = %+v, want stable department allow", decision)
	}

	otherDepartmentID := int64(9)
	task.OwnerDepartmentID = &otherDepartmentID
	decision = newTaskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if decision.Allowed || decision.DenyCode != "permission_or_scope_denied" {
		t.Fatalf("decision = %+v, want stable department mismatch denied", decision)
	}
}

func TestTaskActionAuthorizerSeparatesAssignAndReassignState(t *testing.T) {
	assignActor := taskActionTestActor(41, domain.PermissionTaskAssign, domain.AccessScopeGlobal)
	assignCtx := domain.WithRequestActor(context.Background(), assignActor)
	pending := &domain.Task{ID: 14, TaskStatus: domain.TaskStatusPendingAssign, CreatorID: 7}
	if decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(assignCtx, TaskActionAssign, pending, "", ""); !decision.Allowed {
		t.Fatalf("pending assign decision = %+v, want allow", decision)
	}
	pending.TaskStatus = domain.TaskStatusInProgress
	if decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(assignCtx, TaskActionAssign, pending, "", ""); decision.Allowed || decision.DenyCode != "task_status_not_actionable" {
		t.Fatalf("in-progress assign decision = %+v, want state denial", decision)
	}

	reassignActor := taskActionTestActor(42, domain.PermissionTaskReassign, domain.AccessScopeGlobal)
	reassignCtx := domain.WithRequestActor(context.Background(), reassignActor)
	if decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(reassignCtx, TaskActionReassign, pending, "", ""); !decision.Allowed {
		t.Fatalf("in-progress reassign decision = %+v, want allow", decision)
	}
}

func TestTaskActionAuthorizerAllowsCurrentAssigneeDelegationAcrossDepartment(t *testing.T) {
	actorDepartmentID := int64(14)
	ownerDepartmentID := int64(6)
	actor := taskActionTestActor(343, domain.PermissionTaskReassign, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &actorDepartmentID
	ctx := domain.WithRequestActor(context.Background(), actor)
	task := &domain.Task{
		ID: 2888, CreatorID: 341, TaskType: domain.TaskTypeNewProductDevelopment,
		TaskStatus: domain.TaskStatusInProgress, OwnerDepartmentID: &ownerDepartmentID,
		DesignerID: authzInt64Ptr(343), CurrentHandlerID: authzInt64Ptr(343),
	}

	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionReassign, task, "", "")
	if !decision.Allowed || decision.ScopeSource != "current_assignment" {
		t.Fatalf("decision = %+v, want current-assignment delegation", decision)
	}
}

func TestTaskActionAuthorizerRequiresCatalogManageForBusinessInfo(t *testing.T) {
	task := &domain.Task{ID: 15, TaskStatus: domain.TaskStatusInProgress, CreatorID: 51}
	viewActor := taskActionTestActor(51, domain.PermissionTaskView, domain.AccessScopeGlobal)
	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(domain.WithRequestActor(context.Background(), viewActor), TaskActionUpdateBusinessInfo, task, "", "")
	if decision.Allowed {
		t.Fatalf("task.view unexpectedly authorized business info update: %+v", decision)
	}
	manageActor := taskActionTestActor(52, domain.PermissionCatalogManage, domain.AccessScopeGlobal)
	decision = newTaskActionAuthorizer().EvaluateTaskActionPolicy(domain.WithRequestActor(context.Background(), manageActor), TaskActionUpdateBusinessInfo, task, "", "")
	if !decision.Allowed {
		t.Fatalf("catalog.manage decision = %+v, want allow", decision)
	}
}

func TestTaskActionAuthorizerDoesNotInferCreateScopeFromNames(t *testing.T) {
	actor := taskActionTestActor(61, domain.PermissionTaskCreate, domain.AccessScopeGlobal)
	ctx := domain.WithRequestActor(context.Background(), actor)
	decision := newTaskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionCreate, nil, "运营部", "运营一组")
	if decision.Allowed || decision.DenyCode != "stable_scope_required" {
		t.Fatalf("decision = %+v, want service create fallback to reject name-only scope", decision)
	}
}

func taskActionTestActor(actorID int64, permission domain.PermissionCode, scope domain.AccessScopeMode) domain.RequestActor {
	const roleID int64 = 701
	assignment := domain.AccessAssignment{ID: 1, UserID: actorID, RoleID: roleID, RoleCode: "test-role", ScopeMode: scope, SourceType: "direct"}
	effective := &domain.EffectiveAccess{
		UserID:      actorID,
		Permissions: []domain.PermissionCode{permission},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{
			Permission: permission,
			RoleID:     roleID,
			RoleCode:   assignment.RoleCode,
			SourceType: assignment.SourceType,
			ScopeMode:  scope,
		}},
	}
	return domain.RequestActor{ID: actorID, Permissions: effective.Permissions, EffectiveAccess: effective}
}
