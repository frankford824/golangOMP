package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestAuthorizeTaskBusinessInfoUpdateAllowsOwnCreatorWithoutGovernedFields(t *testing.T) {
	task := &domain.Task{ID: 81, CreatorID: 901, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(901, domain.PermissionTaskCreate, domain.AccessScopeGlobal)

	access, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task)
	if appErr != nil {
		t.Fatalf("own creator unexpectedly denied: %+v", appErr)
	}
	if access.CanManageGovernedFields {
		t.Fatal("task creator unexpectedly received governed cost and filing access")
	}
}

func TestAuthorizeTaskBusinessInfoUpdateRejectsDifferentCreator(t *testing.T) {
	task := &domain.Task{ID: 82, CreatorID: 902, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(903, domain.PermissionTaskCreate, domain.AccessScopeGlobal)

	if _, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr == nil {
		t.Fatal("different creator unexpectedly authorized")
	}
}

func TestAuthorizeTaskBusinessInfoUpdateAllowsScopedTaskManagerForTeamMember(t *testing.T) {
	departmentID := int64(44)
	task := &domain.Task{
		ID: 85, CreatorID: 902, TaskStatus: domain.TaskStatusInProgress,
		OwnerDepartmentID: &departmentID,
	}
	actor := taskActionTestActor(903, domain.PermissionTaskCreate, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &departmentID
	actor.EffectiveAccess.Permissions = append(actor.EffectiveAccess.Permissions, domain.PermissionTaskReassign)
	actor.EffectiveAccess.Sources = append(actor.EffectiveAccess.Sources, domain.EffectiveAccessNote{
		Permission: domain.PermissionTaskReassign,
		RoleID:     actor.EffectiveAccess.Assignments[0].RoleID,
		ScopeMode:  domain.AccessScopeOwnDepartment,
	})

	access, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task)
	if appErr != nil {
		t.Fatalf("scoped task manager unexpectedly denied: %+v", appErr)
	}
	if access.CanManageGovernedFields {
		t.Fatal("task manager unexpectedly received governed cost and filing access")
	}
}

func TestAuthorizeTaskBusinessInfoUpdateUsesManagerScopeInsteadOfCreateScope(t *testing.T) {
	departmentID := int64(44)
	task := &domain.Task{
		ID: 87, CreatorID: 902, TaskStatus: domain.TaskStatusInProgress,
		OwnerDepartmentID: &departmentID,
	}
	actor := domain.RequestActor{
		ID:           903,
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

	if _, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr != nil {
		t.Fatalf("scoped task manager unexpectedly inherited the narrower create scope: %+v", appErr)
	}
}

func TestAuthorizeTaskBusinessInfoUpdateRejectsTaskManagerOutsideScope(t *testing.T) {
	actorDepartmentID := int64(44)
	taskDepartmentID := int64(45)
	task := &domain.Task{
		ID: 86, CreatorID: 902, TaskStatus: domain.TaskStatusInProgress,
		OwnerDepartmentID: &taskDepartmentID,
	}
	actor := taskActionTestActor(903, domain.PermissionTaskCreate, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &actorDepartmentID
	actor.EffectiveAccess.Permissions = append(actor.EffectiveAccess.Permissions, domain.PermissionTaskReassign)
	actor.EffectiveAccess.Sources = append(actor.EffectiveAccess.Sources, domain.EffectiveAccessNote{
		Permission: domain.PermissionTaskReassign,
		RoleID:     actor.EffectiveAccess.Assignments[0].RoleID,
		ScopeMode:  domain.AccessScopeOwnDepartment,
	})

	if _, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr == nil {
		t.Fatal("out-of-scope task manager unexpectedly authorized")
	}
}

func TestAuthorizeTaskBusinessInfoUpdateKeepsCatalogManagerGovernedAccess(t *testing.T) {
	task := &domain.Task{ID: 83, CreatorID: 904, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(905, domain.PermissionCatalogManage, domain.AccessScopeGlobal)

	access, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task)
	if appErr != nil {
		t.Fatalf("catalog manager unexpectedly denied: %+v", appErr)
	}
	if !access.CanManageGovernedFields {
		t.Fatal("catalog manager lost governed cost and filing access")
	}
}

func TestAuthorizeTaskBusinessInfoUpdateRejectsTerminalTaskForCreatorAndManager(t *testing.T) {
	for _, status := range []domain.TaskStatus{
		domain.TaskStatusCompleted,
		domain.TaskStatusArchived,
		domain.TaskStatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			task := &domain.Task{ID: 84, CreatorID: 906, TaskStatus: status}
			creator := taskActionTestActor(906, domain.PermissionTaskCreate, domain.AccessScopeGlobal)
			if _, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), creator), task); appErr == nil {
				t.Fatal("creator unexpectedly authorized terminal task")
			}

			manager := taskActionTestActor(907, domain.PermissionCatalogManage, domain.AccessScopeGlobal)
			if _, appErr := authorizeTaskBusinessInfoUpdate(domain.WithRequestActor(context.Background(), manager), task); appErr == nil {
				t.Fatal("catalog manager unexpectedly authorized terminal task")
			}
		})
	}
}
