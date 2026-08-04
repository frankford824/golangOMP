package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

// TaskCostOverrideAuditService exposes the immutable cost-override timeline.
// The retired review/finance placeholder writes are intentionally absent.
type TaskCostOverrideAuditService interface {
	ListByTaskID(ctx context.Context, taskID int64) (*domain.TaskCostOverrideAuditTimeline, *domain.AppError)
}

type taskCostOverrideAuditService struct {
	taskRepo              repo.TaskRepo
	costOverrideEventRepo repo.TaskCostOverrideEventRepo
	taskEventRepo         repo.TaskEventRepo
}

func NewTaskCostOverrideAuditService(
	taskRepo repo.TaskRepo,
	costOverrideEventRepo repo.TaskCostOverrideEventRepo,
	taskEventRepo repo.TaskEventRepo,
) TaskCostOverrideAuditService {
	return &taskCostOverrideAuditService{
		taskRepo:              taskRepo,
		costOverrideEventRepo: costOverrideEventRepo,
		taskEventRepo:         taskEventRepo,
	}
}

func (s *taskCostOverrideAuditService) ListByTaskID(ctx context.Context, taskID int64) (*domain.TaskCostOverrideAuditTimeline, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for cost override audit", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail for cost override audit", err)
	}

	events := []*domain.TaskCostOverrideAuditEvent{}
	if s.costOverrideEventRepo != nil {
		events, err = s.costOverrideEventRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list cost override audit events", err)
		}
		if events == nil {
			events = []*domain.TaskCostOverrideAuditEvent{}
		}
	}

	var taskEvents []*domain.TaskEvent
	if len(events) == 0 && s.taskEventRepo != nil {
		taskEvents, err = s.taskEventRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list fallback task events for cost override audit", err)
		}
	}

	overrideSummary, governanceAuditSummary := buildTaskCostOverrideReadModels(detail, taskEvents, events)
	if governanceAuditSummary == nil && overrideSummary != nil {
		governanceAuditSummary = buildTaskGovernanceAuditSummary(detail, overrideSummary, len(events))
	}
	return &domain.TaskCostOverrideAuditTimeline{
		TaskID:                 taskID,
		Events:                 events,
		GovernanceAuditSummary: governanceAuditSummary,
	}, nil
}
