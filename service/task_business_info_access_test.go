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
