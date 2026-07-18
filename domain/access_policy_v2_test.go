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
