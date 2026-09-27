package repo

import (
	"context"
	"workflow/domain"
)

type CostSyncRepo interface {
	Coverage(context.Context) (*domain.CostSyncCoverage, error)
	Get(context.Context, string) (*domain.CostSyncState, error)
	List(context.Context, string, string, int, int) ([]*domain.CostSyncState, int64, error)
	Lock(context.Context, string) (func(), error)
	Observe(context.Context, string, *float64) error
	Acknowledge(context.Context, string, int64, *float64) error
	Fail(context.Context, string, int64, string) error
	Collect(context.Context) error
	Due(context.Context, int) ([]string, error)
	BaselineDue(context.Context, int) ([]string, error)
	Resolve(context.Context, domain.CostSyncResolution) error
}

type ManualCostSyncRepo interface {
	LockManualRevision(context.Context, Tx, string, int64) error
	StageManualCost(context.Context, Tx, string, *float64, int64, string) error
}
