package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type ProductionPackageJobRepo interface {
	Create(ctx context.Context, job *domain.ProductionPackageJob) error
	Get(ctx context.Context, jobID string) (*domain.ProductionPackageJob, error)
	Claim(ctx context.Context, workerID string, limit int, leaseUntil time.Time) ([]*domain.ProductionPackageJob, error)
	UpdateProgress(ctx context.Context, jobID, workerID string, processedCount, failedCount int) error
	Complete(ctx context.Context, jobID, workerID string, result []byte, failedCount int, finishedAt time.Time) error
	Fail(ctx context.Context, jobID, workerID, message string, finishedAt time.Time) error
}
