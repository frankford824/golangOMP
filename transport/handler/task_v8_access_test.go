package handler

import (
	"slices"
	"testing"

	"workflow/domain"
)

func TestV8AllowedTaskActionsUsesHydratedDepartmentScope(t *testing.T) {
	departmentID := int64(101)
	assignment := domain.AccessAssignment{RoleID: 8, ScopeMode: domain.AccessScopeOwnDepartment}
	actor := domain.RequestActor{ID: 9, DepartmentID: &departmentID}
	actor.EffectiveAccess = &domain.EffectiveAccess{
		UserID: actor.ID, Permissions: []domain.PermissionCode{domain.PermissionTaskAuditDecision, domain.PermissionTaskAuditHandover},
		Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionTaskAuditDecision, RoleID: assignment.RoleID, ScopeMode: assignment.ScopeMode},
			{Permission: domain.PermissionTaskAuditHandover, RoleID: assignment.RoleID, ScopeMode: assignment.ScopeMode},
		},
	}
	actions := v8AllowedTaskActions(actor, domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAudit, domain.TaskAccessSubject{TaskID: 1, CreatorID: 7, CurrentHandlerID: &actor.ID, OwnerDepartmentID: &departmentID})
	if !slices.Contains(actions, "task.audit.approve") || !slices.Contains(actions, "task.audit.return_to_design") {
		t.Fatalf("allowed actions = %v", actions)
	}
	if !slices.Contains(actions, "task.audit.handover") || slices.Contains(actions, "task.audit.takeover") {
		t.Fatalf("task-level handover actions = %v, want current-handler handover and no item-scoped takeover", actions)
	}
}

func TestV8AllowedTaskActionsUsesSeparateTaskOperations(t *testing.T) {
	departmentID := int64(101)
	subject := domain.TaskAccessSubject{TaskID: 2, CreatorID: 7, OwnerDepartmentID: &departmentID}
	actorFor := func(permissions ...domain.PermissionCode) domain.RequestActor {
		assignment := domain.AccessAssignment{ID: 9, RoleID: 8, ScopeMode: domain.AccessScopeOwnDepartment}
		sources := make([]domain.EffectiveAccessNote, 0, len(permissions))
		for _, permission := range permissions {
			sources = append(sources, domain.EffectiveAccessNote{Permission: permission, RoleID: assignment.RoleID, ScopeMode: assignment.ScopeMode})
		}
		return domain.RequestActor{
			ID: 9, DepartmentID: &departmentID,
			EffectiveAccess: &domain.EffectiveAccess{
				UserID:      9,
				Permissions: permissions,
				Assignments: []domain.AccessAssignment{assignment},
				Sources:     sources,
			},
		}
	}

	managed := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, subject)
	if !slices.Contains(managed, "task.reference.append") || !slices.Contains(managed, "task.assign") || slices.Contains(managed, "task.design.submit") {
		t.Fatalf("task create/assign actions = %v", managed)
	}
	pendingAssign := v8AllowedTaskActions(actorFor(domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAssign, subject)
	if !slices.Contains(pendingAssign, "task.assign") {
		t.Fatalf("pending-assign actions = %v, want task.assign", pendingAssign)
	}
	designer := v8AllowedTaskActions(actorFor(domain.PermissionTaskDesignSubmit), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, subject)
	if slices.Contains(designer, "task.reference.append") || slices.Contains(designer, "task.assign") || !slices.Contains(designer, "task.design.submit") {
		t.Fatalf("task.design.submit actions = %v", designer)
	}
	completed := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusCompleted, subject)
	if slices.Contains(completed, "task.reference.append") || slices.Contains(completed, "task.assign") {
		t.Fatalf("completed actions = %v", completed)
	}
	blocked := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusBlocked, subject)
	if slices.Contains(blocked, "task.reference.append") || slices.Contains(blocked, "task.assign") {
		t.Fatalf("blocked actions = %v", blocked)
	}
}
