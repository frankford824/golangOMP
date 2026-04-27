package repo

import (
	"context"

	"workflow/domain"
)

type ModuleNotificationRepo interface {
	GetTaskModuleByID(ctx context.Context, tx Tx, taskModuleID int64) (*domain.TaskModule, error)
	ListActiveUserIDsByTeam(ctx context.Context, tx Tx, teamCode string, excludeUserID *int64) ([]int64, error)
	ListClaimedUserIDsByTask(ctx context.Context, tx Tx, taskID int64, excludeUserID *int64) ([]int64, error)
}
