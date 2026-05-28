package repo

import (
	"context"

	"workflow/domain"
)

type TaskRetouchRequirementRepo interface {
	CreateBatch(ctx context.Context, tx Tx, taskID int64, createdBy int64, items []domain.CreateRetouchRequirementItem) error
	GetByID(ctx context.Context, id int64) (*domain.TaskRetouchRequirement, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskRetouchRequirement, error)
}
