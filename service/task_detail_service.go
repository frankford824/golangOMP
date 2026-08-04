package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

// TaskDetailAggregateService loads the task and its editable business detail.
// The public V8 detail response is owned by task_aggregator.DetailService; this
// smaller service exists only for the product/cost edit endpoints that must
// preserve fields omitted by a partial PATCH.
type TaskDetailAggregateService interface {
	GetByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetailAggregate, *domain.AppError)
}

type taskDetailAggregateService struct {
	taskRepo repo.TaskRepo
}

type TaskDetailAggregateServiceOption func(*taskDetailAggregateService)

func NewTaskDetailAggregateService(taskRepo repo.TaskRepo, opts ...TaskDetailAggregateServiceOption) TaskDetailAggregateService {
	svc := &taskDetailAggregateService{taskRepo: taskRepo}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *taskDetailAggregateService) GetByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetailAggregate, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task business detail task", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := newTaskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionReadDetail, task); appErr != nil {
		return nil, appErr
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task business detail", err)
	}
	if detail == nil {
		detail = &domain.TaskDetail{TaskID: taskID}
	}
	attachTaskProductSelection(detail, task)
	hydrateTaskDetailFilingProjection(task, detail)

	return &domain.TaskDetailAggregate{Task: task, TaskDetail: detail}, nil
}
