package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskAssignmentServiceExplicitGlobalReassignsInProgressTask(t *testing.T) {
	currentDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               905,
		CreatorID:        305,
		TaskStatus:       domain.TaskStatusInProgress,
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{})
	actor := taskActionTestActor(305, domain.PermissionTaskReassign, domain.AccessScopeGlobal)

	updated, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{
		TaskID: 905, DesignerID: authzInt64Ptr(204), AssignedBy: actor.ID,
	})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if updated.DesignerID == nil || *updated.DesignerID != 204 || updated.CurrentHandlerID == nil || *updated.CurrentHandlerID != 204 {
		t.Fatalf("assignment = designer:%v handler:%v, want 204/204", updated.DesignerID, updated.CurrentHandlerID)
	}
}

func TestTaskAssignmentServiceLegacyRoleOnlyCannotReassign(t *testing.T) {
	currentDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 906, CreatorID: 305, TaskStatus: domain.TaskStatusInProgress,
		DesignerID: &currentDesignerID, CurrentHandlerID: &currentDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{})
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 305, Roles: []domain.Role{domain.RoleAdmin}})

	_, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 906, DesignerID: authzInt64Ptr(204), AssignedBy: 305})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() error = %+v, want explicit permission denial", appErr)
	}
}

func TestTaskAssignmentServiceExplicitScopeMismatchCannotReassign(t *testing.T) {
	actorDepartmentID := int64(10)
	taskDepartmentID := int64(11)
	currentDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 907, CreatorID: 999, TaskStatus: domain.TaskStatusInProgress,
		OwnerDepartmentID: &taskDepartmentID, DesignerID: &currentDesignerID, CurrentHandlerID: &currentDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{})
	actor := taskActionTestActor(305, domain.PermissionTaskReassign, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &actorDepartmentID

	_, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{TaskID: 907, DesignerID: authzInt64Ptr(204), AssignedBy: actor.ID})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() error = %+v, want scope mismatch denial", appErr)
	}
}

func TestTaskAssignmentServiceCurrentAssigneeCanDelegateAcrossOwnerDepartment(t *testing.T) {
	actorDepartmentID := int64(14)
	taskDepartmentID := int64(6)
	currentDesignerID := int64(343)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 2888, CreatorID: 341, TaskType: domain.TaskTypeNewProductDevelopment,
		TaskStatus: domain.TaskStatusInProgress, OwnerDepartmentID: &taskDepartmentID,
		DesignerID: &currentDesignerID, CurrentHandlerID: &currentDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{})
	actor := taskActionTestActor(currentDesignerID, domain.PermissionTaskReassign, domain.AccessScopeOwnDepartment)
	actor.DepartmentID = &actorDepartmentID

	updated, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{
		TaskID: 2888, DesignerID: authzInt64Ptr(344), AssignedBy: actor.ID,
	})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if updated.DesignerID == nil || *updated.DesignerID != 344 || updated.CurrentHandlerID == nil || *updated.CurrentHandlerID != 344 {
		t.Fatalf("assignment = designer:%v handler:%v, want 344/344", updated.DesignerID, updated.CurrentHandlerID)
	}
}
