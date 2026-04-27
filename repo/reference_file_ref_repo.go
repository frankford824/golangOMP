package repo

import (
	"context"

	"workflow/domain"
)

type ReferenceFileRefFlatRepo interface {
	InsertFlat(ctx context.Context, tx Tx, ref *domain.ReferenceFileRefFlat) (int64, error)
	ListByTask(ctx context.Context, taskID int64) ([]*domain.ReferenceFileRefFlat, error)
	DeleteByTaskAndRef(ctx context.Context, tx Tx, taskID int64, refID string) error
}
