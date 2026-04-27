package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskServiceGetByIDIncludesRequesterAndActorNames(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			501: {
				ID:               501,
				TaskNo:           "T-501",
				TaskType:         domain.TaskTypeNewProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				CreatorID:        7,
				RequesterID:      int64Ptr(8),
				DesignerID:       int64Ptr(9),
				CurrentHandlerID: int64Ptr(10),
			},
		},
		details: map[int64]*domain.TaskDetail{
			501: {TaskID: 501},
		},
	}

	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		nil,
		step04TxRunner{},
		WithUserDisplayNameResolver(actorNameResolverStub{names: map[int64]string{
			7:  "Creator Seven",
			8:  "Requester Eight",
			9:  "Designer Nine",
			10: "Auditor Ten",
		}}),
	)

	readModel, appErr := svc.GetByID(context.Background(), 501)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.RequesterID == nil || *readModel.RequesterID != 8 {
		t.Fatalf("requester_id = %+v, want 8", readModel.RequesterID)
	}
	if readModel.RequesterName != "Requester Eight" {
		t.Fatalf("requester_name = %q, want Requester Eight", readModel.RequesterName)
	}
	if readModel.CreatorName != "Creator Seven" {
		t.Fatalf("creator_name = %q, want Creator Seven", readModel.CreatorName)
	}
	if readModel.DesignerName != "Designer Nine" {
		t.Fatalf("designer_name = %q, want Designer Nine", readModel.DesignerName)
	}
	if readModel.AssigneeName != "Designer Nine" {
		t.Fatalf("assignee_name = %q, want Designer Nine", readModel.AssigneeName)
	}
	if readModel.CurrentHandlerName != "Auditor Ten" {
		t.Fatalf("current_handler_name = %q, want Auditor Ten", readModel.CurrentHandlerName)
	}
}

func TestTaskDetailAggregateIncludesActorProjections(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			502: {
				ID:               502,
				TaskNo:           "T-502",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusInProgress,
				CreatorID:        17,
				RequesterID:      int64Ptr(18),
				DesignerID:       int64Ptr(19),
				CurrentHandlerID: int64Ptr(19),
			},
		},
		details: map[int64]*domain.TaskDetail{
			502: {TaskID: 502},
		},
	}

	svc := NewTaskDetailAggregateService(
		taskRepo,
		&prdProcurementRepo{},
		nil,
		nil,
		&auditV7RepoStub{},
		&taskDetailOutsourceRepoStub{},
		&prdTaskAssetRepo{},
		&prdWarehouseRepo{},
		&prdTaskEventRepo{},
		nil,
		nil,
		nil,
		WithTaskDetailUserDisplayNameResolver(actorNameResolverStub{names: map[int64]string{
			17: "Creator Seventeen",
			18: "Requester Eighteen",
			19: "Designer Nineteen",
		}}),
	)

	aggregate, appErr := svc.GetByTaskID(context.Background(), 502)
	if appErr != nil {
		t.Fatalf("GetByTaskID() unexpected error: %+v", appErr)
	}
	if aggregate.RequesterID == nil || *aggregate.RequesterID != 18 {
		t.Fatalf("requester_id = %+v, want 18", aggregate.RequesterID)
	}
	if aggregate.CreatorName != "Creator Seventeen" {
		t.Fatalf("creator_name = %q, want Creator Seventeen", aggregate.CreatorName)
	}
	if aggregate.RequesterName != "Requester Eighteen" {
		t.Fatalf("requester_name = %q, want Requester Eighteen", aggregate.RequesterName)
	}
	if aggregate.DesignerName != "Designer Nineteen" {
		t.Fatalf("designer_name = %q, want Designer Nineteen", aggregate.DesignerName)
	}
	if aggregate.AssigneeName != "Designer Nineteen" {
		t.Fatalf("assignee_name = %q, want Designer Nineteen", aggregate.AssigneeName)
	}
	if aggregate.CurrentHandlerName != "Designer Nineteen" {
		t.Fatalf("current_handler_name = %q, want Designer Nineteen", aggregate.CurrentHandlerName)
	}
}

type actorNameResolverStub struct {
	names map[int64]string
}

func (r actorNameResolverStub) GetDisplayName(_ context.Context, userID int64) string {
	if r.names == nil {
		return ""
	}
	return r.names[userID]
}
