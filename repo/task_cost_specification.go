package repo

import (
	"context"
	"workflow/domain"
)

type TaskCostSpecificationFilter struct {
	TaskID      int64
	CreatorID   int64
	MissingOnly bool
	Access      domain.ResourceGroupAccessFilter
	Page        int
	PageSize    int
}

type TaskCostSpecificationRepo interface {
	List(context.Context, TaskCostSpecificationFilter) ([]*domain.TaskCostSpecification, int64, error)
	Lock(context.Context, Tx, int64, int64) (*domain.TaskCostSpecification, error)
	Save(context.Context, Tx, *domain.TaskCostSpecification, domain.CostInput, float64) error
}
