package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

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
