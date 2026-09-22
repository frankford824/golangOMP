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
