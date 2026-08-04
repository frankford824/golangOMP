package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestAuthorizeTaskSKUItemBusinessInfoUpdateAllowsOwnCreator(t *testing.T) {
	task := &domain.Task{ID: 71, CreatorID: 801, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(801, domain.PermissionTaskCreate, domain.AccessScopeGlobal)

	if appErr := authorizeTaskSKUItemBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr != nil {
		t.Fatalf("own creator unexpectedly denied: %+v", appErr)
	}
}

func TestAuthorizeTaskSKUItemBusinessInfoUpdateRejectsDifferentCreator(t *testing.T) {
	task := &domain.Task{ID: 72, CreatorID: 802, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(803, domain.PermissionTaskCreate, domain.AccessScopeGlobal)

	if appErr := authorizeTaskSKUItemBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr == nil {
		t.Fatal("different creator unexpectedly authorized")
	}
}

func TestAuthorizeTaskSKUItemBusinessInfoUpdateKeepsCatalogManagerAccess(t *testing.T) {
	task := &domain.Task{ID: 73, CreatorID: 804, TaskStatus: domain.TaskStatusInProgress}
	actor := taskActionTestActor(805, domain.PermissionCatalogManage, domain.AccessScopeGlobal)

	if appErr := authorizeTaskSKUItemBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr != nil {
		t.Fatalf("catalog manager unexpectedly denied: %+v", appErr)
	}
}

func TestAuthorizeTaskSKUItemBusinessInfoUpdateRejectsTerminalTask(t *testing.T) {
	task := &domain.Task{ID: 74, CreatorID: 806, TaskStatus: domain.TaskStatusCompleted}
	actor := taskActionTestActor(806, domain.PermissionTaskCreate, domain.AccessScopeGlobal)

	if appErr := authorizeTaskSKUItemBusinessInfoUpdate(domain.WithRequestActor(context.Background(), actor), task); appErr == nil {
		t.Fatal("creator unexpectedly authorized completed task")
	}
}
