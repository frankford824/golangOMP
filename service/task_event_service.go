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
	taskEventRepo           repo.TaskEventRepo
	taskRepo                repo.TaskRepo
	userDisplayNameResolver UserDisplayNameResolver
}

type TaskEventServiceOption func(*taskEventService)

func WithTaskEventUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskEventServiceOption {
	return func(s *taskEventService) {
		s.userDisplayNameResolver = resolver
	}
}

func NewTaskEventService(taskEventRepo repo.TaskEventRepo, taskRepo repo.TaskRepo, opts ...TaskEventServiceOption) TaskEventService {
	s := &taskEventService{
		taskEventRepo: taskEventRepo,
		taskRepo:      taskRepo,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
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
	enrichTaskEventsWithActors(ctx, s.userDisplayNameResolver, task, events)
	return events, nil
}

func enrichTaskEventsWithActors(ctx context.Context, resolver UserDisplayNameResolver, task *domain.Task, events []*domain.TaskEvent) {
	if task == nil || len(events) == 0 {
		return
	}
	creatorID := task.CreatorID
	var creatorName string
	if resolver != nil && creatorID > 0 {
		creatorName = resolver.GetDisplayName(ctx, creatorID)
	}
	for _, event := range events {
		if event == nil {
			continue
		}
		if creatorID > 0 {
			event.CreatorID = &creatorID
			event.CreatorName = creatorName
		}
		if resolver != nil && event.OperatorID != nil && *event.OperatorID > 0 {
			event.OperatorName = resolver.GetDisplayName(ctx, *event.OperatorID)
		}
	}
}
