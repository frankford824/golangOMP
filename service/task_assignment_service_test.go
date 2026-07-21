package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestResolveTaskAssignmentOperationPendingAssign(t *testing.T) {
	task := &domain.Task{TaskStatus: domain.TaskStatusPendingAssign}
	op := resolveTaskAssignmentOperation(task)
	if op.Action != TaskActionAssign || op.ResultingStatus != domain.TaskStatusInProgress {
		t.Fatalf("operation = %+v, want assign -> InProgress", op)
	}
}

func TestTaskAssignmentTargetUsesExplicitDesignAccessInsteadOfLegacyRole(t *testing.T) {
	departmentID := int64(44)
	designerID := int64(105)
	userRepo := newIdentityUserRepo()
	userRepo.users[designerID] = &domain.User{
		ID: designerID, Username: "candidate", Status: domain.UserStatusActive,
		DepartmentID: &departmentID, Roles: []domain.Role{domain.RoleWarehouse, domain.RoleAuditA},
	}
	taskRepo := &assignmentCASTaskRepo{prdTaskRepo: prdTaskRepo{tasks: map[int64]*domain.Task{
		905: {
			ID: 905, TaskType: domain.TaskTypeOriginalProductDevelopment, TaskStatus: domain.TaskStatusPendingAssign,
			OwnerDepartmentID: &departmentID, CreatorID: 11, CustomizationRequired: true,
		},
	}}}
	reader := assignableAccessReader{byUser: map[int64]*domain.EffectiveAccess{
		designerID: assignableEffectiveAccess(designerID, 71, "designer", domain.PermissionTaskDesignSubmit),
	}}
	svc := NewTaskAssignmentService(taskRepo, &prdTaskEventRepo{}, step04TxRunner{},
		WithTaskAssignmentScopeUserRepo(userRepo),
		WithTaskAssignmentEffectiveAccessReader(reader),
	)

	updated, appErr := svc.Assign(context.Background(), AssignTaskParams{TaskID: 905, DesignerID: &designerID, AssignedBy: 11})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if updated.DesignerID == nil || *updated.DesignerID != designerID {
		t.Fatalf("updated task = %+v", updated)
	}

	legacyOnlyID := int64(106)
	userRepo.users[legacyOnlyID] = &domain.User{ID: legacyOnlyID, Username: "legacy-only", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleDesigner}}
	taskRepo.tasks[905].TaskStatus = domain.TaskStatusPendingAssign
	taskRepo.tasks[905].DesignerID = nil
	taskRepo.tasks[905].CurrentHandlerID = nil
	_, appErr = svc.Assign(context.Background(), AssignTaskParams{TaskID: 905, DesignerID: &legacyOnlyID, AssignedBy: 11})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("legacy role-only target error = %+v", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if details["deny_code"] != "target_assignee_missing_design_access" {
		t.Fatalf("deny details = %+v", details)
	}
}

func TestTaskAssignmentPendingAssignUsesCAS(t *testing.T) {
	taskRepo := &assignmentCASTaskRepo{
		prdTaskRepo: prdTaskRepo{
			tasks: map[int64]*domain.Task{
				901: {
					ID:              901,
					TaskType:        domain.TaskTypeNewProductDevelopment,
					TaskStatus:      domain.TaskStatusPendingAssign,
					OwnerDepartment: string(domain.DepartmentOperations),
					OwnerOrgTeam:    "ops-team-1",
					CreatorID:       11,
				},
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	firstDesigner := int64(101)
	first, appErr := svc.Assign(context.Background(), AssignTaskParams{
		TaskID:     901,
		DesignerID: &firstDesigner,
		AssignedBy: 11,
	})
	if appErr != nil {
		t.Fatalf("first Assign() unexpected error: %+v", appErr)
	}
	if first.DesignerID == nil || *first.DesignerID != firstDesigner || first.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("first assigned task = %+v, want designer 101 in progress", first)
	}

	taskRepo.tasks[901].TaskStatus = domain.TaskStatusPendingAssign
	taskRepo.tasks[901].DesignerID = nil
	taskRepo.tasks[901].CurrentHandlerID = nil
	taskRepo.forceCASConflict = true
	secondDesigner := int64(102)
	_, appErr = svc.Assign(context.Background(), AssignTaskParams{
		TaskID:     901,
		DesignerID: &secondDesigner,
		AssignedBy: 12,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("second Assign() error = %+v, want permission denied", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if details == nil || details["deny_code"] != domain.DenyTaskAlreadyClaimed {
		t.Fatalf("second Assign() details = %+v, want deny_code=%s", appErr.Details, domain.DenyTaskAlreadyClaimed)
	}
	if len(eventRepo.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(eventRepo.events))
	}
}

func TestTaskAssignmentUsesExplicitTaskAssignAndStableScope(t *testing.T) {
	departmentID := int64(44)
	taskRepo := &assignmentCASTaskRepo{
		prdTaskRepo: prdTaskRepo{tasks: map[int64]*domain.Task{
			902: {
				ID: 902, TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusPendingAssign,
				OwnerDepartmentID: &departmentID, CreatorID: 11,
			},
		}},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})
	designerID := int64(101)
	actor := scopedCapabilityActor(51, domain.PermissionTaskAssign, domain.AccessScopeOwnDepartment, &departmentID, nil, nil)

	updated, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{
		TaskID: 902, DesignerID: &designerID, AssignedBy: actor.ID,
	})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if updated == nil || updated.TaskStatus != domain.TaskStatusInProgress || updated.DesignerID == nil || *updated.DesignerID != designerID {
		t.Fatalf("updated task = %+v, want designer %d in progress", updated, designerID)
	}
	if len(eventRepo.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(eventRepo.events))
	}
}

func TestTaskAssignmentExplicitAccessDoesNotFallBackToLegacyRole(t *testing.T) {
	departmentID := int64(44)
	taskRepo := &assignmentCASTaskRepo{
		prdTaskRepo: prdTaskRepo{tasks: map[int64]*domain.Task{
			903: {
				ID: 903, TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusPendingAssign,
				OwnerDepartmentID: &departmentID, CreatorID: 11,
			},
		}},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})
	designerID := int64(102)
	actor := scopedCapabilityActor(52, domain.PermissionTaskView, domain.AccessScopeGlobal, nil, nil, nil)
	actor.Roles = []domain.Role{domain.RoleAdmin, domain.RoleDesignDirector}

	updated, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{
		TaskID: 903, DesignerID: &designerID, AssignedBy: actor.ID,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() task/error = %+v/%+v, want permission denied", updated, appErr)
	}
	if got := taskRepo.tasks[903]; got.DesignerID != nil || got.CurrentHandlerID != nil || got.TaskStatus != domain.TaskStatusPendingAssign {
		t.Fatalf("task mutated after denied assignment: %+v", got)
	}
	if len(eventRepo.events) != 0 {
		t.Fatalf("event count = %d, want 0", len(eventRepo.events))
	}
}

func TestTaskAssignmentRejectsTaskAssignOutsideStableScope(t *testing.T) {
	taskDepartmentID := int64(44)
	actorDepartmentID := int64(45)
	taskRepo := &assignmentCASTaskRepo{
		prdTaskRepo: prdTaskRepo{tasks: map[int64]*domain.Task{
			904: {
				ID: 904, TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusPendingAssign,
				OwnerDepartmentID: &taskDepartmentID, CreatorID: 11,
			},
		}},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})
	designerID := int64(103)
	actor := scopedCapabilityActor(53, domain.PermissionTaskAssign, domain.AccessScopeOwnDepartment, &actorDepartmentID, nil, nil)

	updated, appErr := svc.Assign(domain.WithRequestActor(context.Background(), actor), AssignTaskParams{
		TaskID: 904, DesignerID: &designerID, AssignedBy: actor.ID,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() task/error = %+v/%+v, want scope denial", updated, appErr)
	}
	if got := taskRepo.tasks[904]; got.DesignerID != nil || got.CurrentHandlerID != nil || got.TaskStatus != domain.TaskStatusPendingAssign {
		t.Fatalf("task mutated after scope denial: %+v", got)
	}
	if len(eventRepo.events) != 0 {
		t.Fatalf("event count = %d, want 0", len(eventRepo.events))
	}
}

type assignmentCASTaskRepo struct {
	prdTaskRepo
	forceCASConflict bool
}

func (r *assignmentCASTaskRepo) ClaimPendingAssignment(_ context.Context, _ repo.Tx, id int64, designerID int64, resultingStatus domain.TaskStatus) (bool, error) {
	if r.forceCASConflict {
		return false, nil
	}
	task := r.tasks[id]
	if task == nil || task.TaskStatus != domain.TaskStatusPendingAssign || task.DesignerID != nil || task.CurrentHandlerID != nil {
		return false, nil
	}
	task.DesignerID = &designerID
	task.CurrentHandlerID = &designerID
	task.TaskStatus = resultingStatus
	return true, nil
}
