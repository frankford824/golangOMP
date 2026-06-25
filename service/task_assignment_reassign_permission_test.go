package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskActionAuthorizerReassignAllowsOpsCreatorDespiteDepartmentScope(t *testing.T) {
	designerID := int64(203)
	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         305,
		Roles:      []domain.Role{domain.RoleOps, domain.RoleMember},
		Department: string(domain.DepartmentOperations),
		Team:       "天猫一组",
	})
	task := &domain.Task{
		ID:               905,
		CreatorID:        305,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
	}

	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReassign, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true, decision=%+v", decision)
	}
	if decision.DenyCode != "" {
		t.Fatalf("DenyCode = %q, want empty", decision.DenyCode)
	}
}

func TestTaskActionAuthorizerReassignAllowsOpsRequesterDespiteDepartmentScope(t *testing.T) {
	designerID := int64(203)
	requesterID := int64(305)
	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         305,
		Roles:      []domain.Role{domain.RoleOps, domain.RoleMember},
		Department: string(domain.DepartmentOperations),
	})
	task := &domain.Task{
		ID:               906,
		CreatorID:        999,
		RequesterID:      &requesterID,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
	}

	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReassign, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true, decision=%+v", decision)
	}
}

func TestTaskAssignmentServiceOpsCreatorReassignInProgress(t *testing.T) {
	userRepo := newIdentityUserRepo()
	seedTaskAssignmentUser(userRepo, 203, domain.DepartmentDesignRD, "默认组", domain.UserStatusActive, domain.RoleDesigner)

	currentDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               905,
		CreatorID:        305,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         305,
		Roles:      []domain.Role{domain.RoleOps, domain.RoleMember},
		Department: string(domain.DepartmentOperations),
		Team:       "天猫一组",
	})
	newDesignerID := int64(204)
	seedTaskAssignmentUser(userRepo, newDesignerID, domain.DepartmentDesignRD, "默认组", domain.UserStatusActive, domain.RoleDesigner)

	task, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     905,
		DesignerID: authzInt64Ptr(newDesignerID),
		AssignedBy: 305,
	})
	if appErr != nil {
		t.Fatalf("Assign(ops creator reassign) unexpected error: %+v", appErr)
	}
	if task.DesignerID == nil || *task.DesignerID != newDesignerID {
		t.Fatalf("designer_id = %+v, want %d", task.DesignerID, newDesignerID)
	}
}

func TestTaskAssignmentServiceOpsNonCreatorReassignStillDenied(t *testing.T) {
	userRepo := newIdentityUserRepo()
	seedTaskAssignmentUser(userRepo, 203, domain.DepartmentDesignRD, "默认组", domain.UserStatusActive, domain.RoleDesigner)

	currentDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               907,
		CreatorID:        999,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         305,
		Roles:      []domain.Role{domain.RoleOps, domain.RoleMember},
		Department: string(domain.DepartmentOperations),
		Team:       "天猫一组",
	})
	_, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     907,
		DesignerID: authzInt64Ptr(204),
		AssignedBy: 305,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() appErr = %+v, want permission denied", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got, _ := details["deny_code"].(string); got != "task_reassign_requires_requester_or_manager" {
		t.Fatalf("deny_code = %v, want task_reassign_requires_requester_or_manager", details["deny_code"])
	}
}

func TestTaskAssignmentServiceDesignManagerReassignToSelf(t *testing.T) {
	userRepo := newIdentityUserRepo()
	seedTaskAssignmentUser(userRepo, 228, domain.DepartmentDesignRD, "默认组", domain.UserStatusActive, domain.RoleDesigner)

	otherDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               908,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &otherDesignerID,
		CurrentHandlerID: &otherDesignerID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:                 228,
		Roles:              []domain.Role{domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesigner, domain.RoleMember},
		Department:         string(domain.DepartmentDesignRD),
		Team:               "默认组",
		ManagedDepartments: []string{string(domain.DepartmentDesignRD)},
	})
	task, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     908,
		DesignerID: authzInt64Ptr(228),
		AssignedBy: 228,
	})
	if appErr != nil {
		t.Fatalf("Assign(manager reassign to self) unexpected error: %+v", appErr)
	}
	if task.DesignerID == nil || *task.DesignerID != 228 {
		t.Fatalf("designer_id = %+v, want 228", task.DesignerID)
	}
	if task.CurrentHandlerID == nil || *task.CurrentHandlerID != 228 {
		t.Fatalf("current_handler_id = %+v, want 228", task.CurrentHandlerID)
	}
}

func TestTaskAssignmentServicePlainDesignerReassignToSelfStillDenied(t *testing.T) {
	userRepo := newIdentityUserRepo()
	seedTaskAssignmentUser(userRepo, 101, domain.DepartmentDesignRD, "默认组", domain.UserStatusActive, domain.RoleDesigner)

	otherDesignerID := int64(203)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               909,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "天猫一组",
		DesignerID:       &otherDesignerID,
		CurrentHandlerID: &otherDesignerID,
	})
	svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         101,
		Roles:      []domain.Role{domain.RoleDesigner, domain.RoleMember},
		Department: string(domain.DepartmentDesignRD),
		Team:       "默认组",
	})
	_, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     909,
		DesignerID: authzInt64Ptr(101),
		AssignedBy: 101,
	})
	if appErr == nil {
		t.Fatal("Assign(designer self takeover) expected error")
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got, _ := details["deny_code"].(string); got != domain.DenyTaskAlreadyClaimed {
		t.Fatalf("deny_code = %v, want %s", got, domain.DenyTaskAlreadyClaimed)
	}
}
