package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

// TaskEventService provides read access to task_event_logs (V7 §8).
// Write access is performed directly via TaskEventRepo inside service transactions
// (audit, outsource, etc.) to guarantee atomicity with business state changes.
type TaskEventService interface {
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskEvent, *domain.AppError)
}

type taskEventService struct {
	taskEventRepo repo.TaskEventRepo
	taskRepo      repo.TaskRepo
}

func NewTaskEventService(taskEventRepo repo.TaskEventRepo, taskRepo repo.TaskRepo) TaskEventService {
	return &taskEventService{
		taskEventRepo: taskEventRepo,
		taskRepo:      taskRepo,
	}
}

func (s *taskEventService) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskEvent, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for task events", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}

	events, err := s.taskEventRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task events", err)
	}
	return events, nil
}
