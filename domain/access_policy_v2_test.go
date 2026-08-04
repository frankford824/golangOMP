package domain

import "testing"

func TestEffectiveAccessTaskTypeRestrictionFailsClosedWithoutTaskType(t *testing.T) {
	actor := taskTypeRestrictedActor(TaskTypeOriginalProductDevelopment)

	if EffectiveAccessAllowsTask(actor, PermissionTaskCreate, TaskAccessSubject{CreatorID: actor.ID}) {
		t.Fatal("restricted task.create unexpectedly allowed a subject without task type")
	}
	if EffectiveAccessAllowsTask(actor, PermissionTaskCreate, TaskAccessSubject{CreatorID: actor.ID, TaskType: TaskTypeRetouchTask}) {
		t.Fatal("restricted task.create unexpectedly allowed a different task type")
	}
	if !EffectiveAccessAllowsTask(actor, PermissionTaskCreate, TaskAccessSubject{CreatorID: actor.ID, TaskType: TaskTypeOriginalProductDevelopment}) {
		t.Fatal("restricted task.create rejected its configured task type")
	}
}

func TestAccessTaskTypeValidRejectsRetiredPurchaseTask(t *testing.T) {
	if AccessTaskTypeValid(TaskTypePurchaseTask) {
		t.Fatal("purchase_task must not be accepted by new access-policy grants")
	}
	if !AccessTaskTypeValid(TaskTypeCustomerCustomization) || !AccessTaskTypeValid(TaskTypeRegularCustomization) {
		t.Fatal("active customization task types must remain configurable")
	}
}

func TestEffectiveAccessAllowsCurrentAssigneeToReassignAcrossOwnerDepartment(t *testing.T) {
	actorDepartmentID := int64(14)
	ownerDepartmentID := int64(6)
	actor := reassignActor(343, AccessScopeOwnDepartment, &actorDepartmentID, nil)
	subject := TaskAccessSubject{
		TaskID:            2888,
		TaskType:          TaskTypeNewProductDevelopment,
		CreatorID:         341,
		DesignerID:        int64Pointer(343),
		CurrentHandlerID:  int64Pointer(343),
		OwnerDepartmentID: &ownerDepartmentID,
	}

	if EffectiveAccessAllowsTask(actor, PermissionTaskReassign, subject) {
		t.Fatal("ordinary organization scope unexpectedly matched a cross-department task")
	}
	if !EffectiveAccessAllowsTaskReassign(actor, subject) {
		t.Fatal("current assignee with task.reassign grant could not delegate the task")
	}

	subject.DesignerID = int64Pointer(344)
	subject.CurrentHandlerID = int64Pointer(344)
	if EffectiveAccessAllowsTaskReassign(actor, subject) {
		t.Fatal("unrelated actor unexpectedly delegated a cross-department task")
	}
}

func TestEffectiveAccessCurrentAssigneeReassignHonorsTaskTypeRestriction(t *testing.T) {
	actor := reassignActor(343, AccessScopeOwnDepartment, nil, []string{string(TaskTypeRegularCustomization)})
	subject := TaskAccessSubject{
		TaskID:           2888,
		TaskType:         TaskTypeNewProductDevelopment,
		CreatorID:        341,
		DesignerID:       int64Pointer(343),
		CurrentHandlerID: int64Pointer(343),
	}
	if EffectiveAccessAllowsTaskReassign(actor, subject) {
		t.Fatal("current-assignee delegation ignored the grant task-type restriction")
	}
}

func reassignActor(actorID int64, scope AccessScopeMode, departmentID *int64, taskTypes []string) RequestActor {
	const roleID int64 = 14
	assignment := AccessAssignment{
		ID: roleID, UserID: actorID, RoleID: roleID, RoleCode: "design_director",
		ScopeMode: scope, SourceType: "direct",
	}
	effective := &EffectiveAccess{
		UserID: actorID, Permissions: []PermissionCode{PermissionTaskReassign},
		Assignments: []AccessAssignment{assignment},
		Sources: []EffectiveAccessNote{{
			Permission: PermissionTaskReassign, RoleID: roleID, RoleCode: assignment.RoleCode,
			ScopeMode: scope, SourceType: assignment.SourceType, TaskTypes: taskTypes,
		}},
	}
	return RequestActor{
		ID: actorID, DepartmentID: departmentID,
		Permissions: effective.Permissions, EffectiveAccess: effective,
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func taskTypeRestrictedActor(taskType TaskType) RequestActor {
	const actorID int64 = 41
	assignment := AccessAssignment{ID: 9, UserID: actorID, RoleID: 7, RoleCode: "creator", ScopeMode: AccessScopeGlobal, SourceType: "direct"}
	effective := &EffectiveAccess{
		UserID:      actorID,
		Permissions: []PermissionCode{PermissionTaskCreate},
		Assignments: []AccessAssignment{assignment},
		Sources: []EffectiveAccessNote{{
			Permission: PermissionTaskCreate, RoleID: assignment.RoleID, RoleCode: assignment.RoleCode,
			ScopeMode: assignment.ScopeMode, SourceType: assignment.SourceType, TaskTypes: []string{string(taskType)},
		}},
	}
	return RequestActor{ID: actorID, Permissions: effective.Permissions, EffectiveAccess: effective}
}
