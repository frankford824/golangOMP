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
		UserID: actor.ID, Permissions: []domain.PermissionCode{domain.PermissionTaskAuditDecision},
		Assignments: []domain.AccessAssignment{assignment},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionTaskAuditDecision, RoleID: assignment.RoleID, ScopeMode: assignment.ScopeMode}},
	}
	actions := v8AllowedTaskActions(actor, domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAudit, domain.TaskAccessSubject{TaskID: 1, CreatorID: 7, CurrentHandlerID: &actor.ID, OwnerDepartmentID: &departmentID})
	if !slices.Contains(actions, "task.audit.approve") || !slices.Contains(actions, "task.audit.return_to_design") {
		t.Fatalf("allowed actions = %v", actions)
	}
	if !slices.Contains(actions, "task.audit.handover") || slices.Contains(actions, "task.audit.takeover") {
		t.Fatalf("task-level handover actions = %v, want current-handler handover and no item-scoped takeover", actions)
	}
}
