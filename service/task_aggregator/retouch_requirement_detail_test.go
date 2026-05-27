package task_aggregator

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type retouchRequirementRepoStub struct {
	rows []*domain.TaskRetouchRequirement
}

func (s *retouchRequirementRepoStub) CreateBatch(context.Context, repo.Tx, int64, int64, []domain.CreateRetouchRequirementItem) error {
	return nil
}

func (s *retouchRequirementRepoStub) ListByTaskID(context.Context, int64) ([]*domain.TaskRetouchRequirement, error) {
	if len(s.rows) == 0 {
		return []*domain.TaskRetouchRequirement{}, nil
	}
	out := make([]*domain.TaskRetouchRequirement, len(s.rows))
	copy(out, s.rows)
	return out, nil
}

func TestDetailServiceGetReturnsRetouchRequirements(t *testing.T) {
	taskID := int64(701)
	now := time.Now().UTC()
	retouchRepo := &retouchRequirementRepoStub{rows: []*domain.TaskRetouchRequirement{
		{ID: 1, TaskID: taskID, Description: "需求一", SortOrder: 1, CreatedAt: now, UpdatedAt: now},
		{ID: 2, TaskID: taskID, Description: "需求二", SortOrder: 2, CreatedAt: now, UpdatedAt: now},
	}}

	svc := NewDetailService(
		detailTaskRepoStub{
			task: &domain.Task{
				ID:       taskID,
				TaskNo:   "RT-701",
				TaskType: domain.TaskTypeRetouchTask,
			},
			detail: &domain.TaskDetail{TaskID: taskID},
		},
		detailModuleRepoStub{},
		detailModuleEventRepoStub{},
		detailReferenceRepoStub{},
		WithTaskRetouchRequirementRepo(retouchRepo),
	)

	detail, err := svc.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if len(detail.RetouchRequirements) != 2 {
		t.Fatalf("retouch_requirements len = %d, want 2", len(detail.RetouchRequirements))
	}
	if detail.RetouchRequirements[0].Description != "需求一" {
		t.Fatalf("retouch_requirements[0] = %+v", detail.RetouchRequirements[0])
	}
}

func TestDetailServiceGetLegacyRetouchTaskReturnsEmptyRequirements(t *testing.T) {
	taskID := int64(702)
	svc := NewDetailService(
		detailTaskRepoStub{
			task: &domain.Task{
				ID:       taskID,
				TaskNo:   "RT-702",
				TaskType: domain.TaskTypeRetouchTask,
			},
			detail: &domain.TaskDetail{TaskID: taskID},
		},
		detailModuleRepoStub{},
		detailModuleEventRepoStub{},
		detailReferenceRepoStub{},
		WithTaskRetouchRequirementRepo(&retouchRequirementRepoStub{}),
	)

	detail, err := svc.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if detail.RetouchRequirements == nil || len(detail.RetouchRequirements) != 0 {
		t.Fatalf("retouch_requirements = %#v, want empty slice", detail.RetouchRequirements)
	}
}
