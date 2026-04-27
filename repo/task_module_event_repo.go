package repo

import (
	"context"

	"workflow/domain"
)

type TaskModuleEventRepo interface {
	Insert(ctx context.Context, tx Tx, event *domain.TaskModuleEvent) (int64, error)
	ListByTaskModule(ctx context.Context, taskModuleID int64, limit int) ([]*domain.TaskModuleEvent, error)
	ListRecentByTask(ctx context.Context, taskID int64, limit int) ([]*domain.TaskModuleEvent, error)
}
