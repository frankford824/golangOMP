package handler

import (
	"slices"
	"testing"

	"workflow/domain"
)

func TestV8AllowedTaskActionsUsesHydratedDepartmentScope(t *testing.T) {
	departmentID := int64(101)
	assignment := domain.AccessAssignment{RoleID: 8, ScopeMode: domain.AccessScopeOwnDepartment}
	actor := domain.RequestActor{
		ID:           9,
		DepartmentID: &departmentID,
		Permissions:  []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskReassign},
	}
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
	subject := domain.TaskAccessSubject{TaskID: 2, CreatorID: 9, OwnerDepartmentID: &departmentID}
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

	managed := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskReassign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, subject)
	if !slices.Contains(managed, "task.reference.append") || !slices.Contains(managed, "task.assign") || !slices.Contains(managed, "task.business_info.edit") || slices.Contains(managed, "task.design.submit") {
		t.Fatalf("task create/reassign actions = %v", managed)
	}
	otherCreatorsTask := subject
	otherCreatorsTask.CreatorID = 7
	ordinaryOperations := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, otherCreatorsTask)
	if slices.Contains(ordinaryOperations, "task.reference.append") || slices.Contains(ordinaryOperations, "task.business_info.edit") {
		t.Fatalf("same-department non-creator actions = %v, want no task.reference.append or business edit", ordinaryOperations)
	}
	manager := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskReassign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, otherCreatorsTask)
	if !slices.Contains(manager, "task.business_info.edit") || !slices.Contains(manager, "task.reference.append") {
		t.Fatalf("manager actions on another creator's task = %v, want both business edit and task.reference.append", manager)
	}
	assetManager := v8AllowedTaskActions(actorFor(domain.PermissionAssetManage), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, otherCreatorsTask)
	if !slices.Contains(assetManager, "task.reference.append") {
		t.Fatalf("asset manager actions = %v, want task.reference.append", assetManager)
	}
	pendingAssign := v8AllowedTaskActions(actorFor(domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAssign, subject)
	if !slices.Contains(pendingAssign, "task.assign") {
		t.Fatalf("pending-assign actions = %v, want task.assign", pendingAssign)
	}
	designerSubject := subject
	designerSubject.DesignerID = handlerInt64Ptr(9)
	designerSubject.CurrentHandlerID = handlerInt64Ptr(9)
	designer := v8AllowedTaskActions(actorFor(domain.PermissionTaskDesignSubmit), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, designerSubject)
	if slices.Contains(designer, "task.reference.append") || slices.Contains(designer, "task.assign") || !slices.Contains(designer, "task.design.submit") {
		t.Fatalf("task.design.submit actions = %v", designer)
	}
	unclaimed := domain.TaskAccessSubject{TaskID: 4, CreatorID: 7, OwnerDepartmentID: &departmentID}
	operationsOnPool := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAssign, unclaimed)
	if slices.Contains(operationsOnPool, "task.claim") {
		t.Fatalf("operations actions on an unclaimed task = %v, want no task.claim", operationsOnPool)
	}
	designerOnPool := v8AllowedTaskActions(actorFor(domain.PermissionTaskUploadSource), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusPendingAssign, unclaimed)
	if !slices.Contains(designerOnPool, "task.claim") {
		t.Fatalf("designer actions on an unclaimed task = %v, want task.claim", designerOnPool)
	}
	takenByOther := unclaimed
	takenByOther.DesignerID = handlerInt64Ptr(77)
	designerOnTaken := v8AllowedTaskActions(actorFor(domain.PermissionTaskUploadSource), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, takenByOther)
	if slices.Contains(designerOnTaken, "task.claim") || slices.Contains(designerOnTaken, "task.design.submit") {
		t.Fatalf("designer actions on another handler's task = %v, want no task.claim or task.design.submit", designerOnTaken)
	}
	planningPool := v8AllowedTaskActions(actorFor(domain.PermissionTaskUploadSource), domain.TaskTypeSKUPlanning, domain.TaskStatusPendingAssign, unclaimed)
	if slices.Contains(planningPool, "task.claim") {
		t.Fatalf("planning actions = %v, want no task.claim", planningPool)
	}

	completed := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusCompleted, subject)
	if slices.Contains(completed, "task.reference.append") || slices.Contains(completed, "task.assign") {
		t.Fatalf("completed actions = %v", completed)
	}
	blocked := v8AllowedTaskActions(actorFor(domain.PermissionTaskCreate, domain.PermissionTaskAssign), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusBlocked, subject)
	if slices.Contains(blocked, "task.reference.append") || slices.Contains(blocked, "task.assign") {
		t.Fatalf("blocked actions = %v", blocked)
	}

	terminator := v8AllowedTaskActions(actorFor(domain.PermissionTaskTerminate), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, subject)
	if !slices.Contains(terminator, "task.terminate") {
		t.Fatalf("terminator actions = %v, want task.terminate", terminator)
	}
	terminal := v8AllowedTaskActions(actorFor(domain.PermissionTaskTerminate), domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusCompleted, subject)
	if slices.Contains(terminal, "task.terminate") {
		t.Fatalf("terminal actions = %v, want no task.terminate", terminal)
	}
}

func TestV8AllowedTaskActionsUsesManagerScopeInsteadOfCreateScope(t *testing.T) {
	departmentID := int64(101)
	actor := domain.RequestActor{
		ID:           9,
		DepartmentID: &departmentID,
		Permissions:  []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskReassign},
	}
	actor.EffectiveAccess = &domain.EffectiveAccess{
		UserID:      actor.ID,
		Permissions: []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskReassign},
		Assignments: []domain.AccessAssignment{
			{ID: 1, RoleID: 10, ScopeMode: domain.AccessScopeSelf},
			{ID: 2, RoleID: 11, ScopeMode: domain.AccessScopeOwnDepartment},
		},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionTaskCreate, RoleID: 10, ScopeMode: domain.AccessScopeSelf},
			{Permission: domain.PermissionTaskReassign, RoleID: 11, ScopeMode: domain.AccessScopeOwnDepartment},
		},
	}
	subject := domain.TaskAccessSubject{
		TaskID: 3, CreatorID: 7, OwnerDepartmentID: &departmentID,
	}

	actions := v8AllowedTaskActions(actor, domain.TaskTypeOriginalProductDevelopment, domain.TaskStatusInProgress, subject)
	if !slices.Contains(actions, "task.business_info.edit") {
		t.Fatalf("manager actions = %v, want task.business_info.edit", actions)
	}
}

func TestV8AllowedTaskActionsExposesCurrentAssigneeDelegation(t *testing.T) {
	actorDepartmentID := int64(14)
	ownerDepartmentID := int64(6)
	const roleID int64 = 14
	assignment := domain.AccessAssignment{
		ID: roleID, UserID: 343, RoleID: roleID, RoleCode: "design_director",
		ScopeMode: domain.AccessScopeOwnDepartment, SourceType: "direct",
	}
	actor := domain.RequestActor{
		ID: 343,
		EffectiveAccess: &domain.EffectiveAccess{
			UserID: 343, Permissions: []domain.PermissionCode{domain.PermissionTaskReassign},
			Assignments: []domain.AccessAssignment{assignment},
			Sources: []domain.EffectiveAccessNote{{
				Permission: domain.PermissionTaskReassign, RoleID: roleID, RoleCode: assignment.RoleCode,
				ScopeMode: assignment.ScopeMode, SourceType: assignment.SourceType,
			}},
		},
	}
	actor.DepartmentID = &actorDepartmentID
	subject := domain.TaskAccessSubject{
		TaskID: 2888, CreatorID: 341, DesignerID: handlerInt64Ptr(343),
		CurrentHandlerID: handlerInt64Ptr(343), OwnerDepartmentID: &ownerDepartmentID,
	}

	actions := v8AllowedTaskActions(actor, domain.TaskTypeNewProductDevelopment, domain.TaskStatusInProgress, subject)
	if !slices.Contains(actions, "task.assign") {
		t.Fatalf("allowed actions = %v, want task.assign for current-assignee delegation", actions)
	}

	subject.DesignerID = handlerInt64Ptr(344)
	subject.CurrentHandlerID = handlerInt64Ptr(344)
	actions = v8AllowedTaskActions(actor, domain.TaskTypeNewProductDevelopment, domain.TaskStatusInProgress, subject)
	if slices.Contains(actions, "task.assign") {
		t.Fatalf("allowed actions = %v, want no cross-department delegation for unrelated actor", actions)
	}
}

func handlerInt64Ptr(value int64) *int64 {
	return &value
}
