package task_aggregator

import (
	"context"

	"workflow/domain"
	"workflow/service"
)

type ListService struct {
	fallback service.TaskService
}

func NewListService(fallback service.TaskService) *ListService {
	return &ListService{fallback: fallback}
}

func (s *ListService) List(ctx context.Context, filter service.TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	return s.fallback.List(ctx, filter)
}
