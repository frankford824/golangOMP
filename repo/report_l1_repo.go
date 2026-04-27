package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type ReportL1Filter struct {
	From         time.Time
	To           time.Time
	DepartmentID *int64
	TaskType     *string
}

type ReportL1Repo interface {
	GetCards(ctx context.Context) ([]domain.L1Card, error)
	GetThroughput(ctx context.Context, filter ReportL1Filter) ([]domain.L1ThroughputPoint, error)
	GetModuleDwell(ctx context.Context, filter ReportL1Filter) ([]domain.L1ModuleDwellPoint, error)
}
