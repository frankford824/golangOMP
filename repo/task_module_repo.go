package repo

import (
	"context"
	"encoding/json"

	"workflow/domain"
)

type TaskModuleRepo interface {
	GetByTaskAndKey(ctx context.Context, taskID int64, moduleKey string) (*domain.TaskModule, error)
	ListByTask(ctx context.Context, taskID int64) ([]*domain.TaskModule, error)
	ClaimCAS(ctx context.Context, tx Tx, taskID int64, moduleKey, poolTeamCode string, actorID int64, claimedTeamCode string, actorSnapshot json.RawMessage) (bool, error)
	Enter(ctx context.Context, tx Tx, taskID int64, moduleKey string, state domain.ModuleState, poolTeamCode *string, data json.RawMessage) (*domain.TaskModule, error)
	UpdateState(ctx context.Context, tx Tx, taskID int64, moduleKey string, state domain.ModuleState, terminal bool, data json.RawMessage) error
	Reassign(ctx context.Context, tx Tx, taskID int64, moduleKey string, actorID int64, claimedTeamCode string, actorSnapshot json.RawMessage) error
	PoolReassign(ctx context.Context, tx Tx, taskID int64, moduleKey, poolTeamCode string) error
	CloseOpenModules(ctx context.Context, tx Tx, taskID int64, state domain.ModuleState) ([]*domain.TaskModule, error)
	InsertPlaceholder(ctx context.Context, tx Tx, taskID int64, moduleKey string) (*domain.TaskModule, error)
}
