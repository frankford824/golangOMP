package repo

import (
	"context"

	"workflow/domain"
)

type TaskRetouchRequirementRepo interface {
	CreateBatch(ctx context.Context, tx Tx, taskID int64, createdBy int64, items []domain.CreateRetouchRequirementItem) error
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskRetouchRequirement, error)
}
