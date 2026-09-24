package repo

import (
	"context"
	"workflow/domain"
)

type ExternalAssetMediaRepo interface {
	GetMediaFingerprint(context.Context, int64) (*domain.ExternalMediaFingerprint, error)
	ApplyMediaDelete(context.Context, string, domain.ExternalAssetFilesystemEvent) error
	CommitMediaResult(context.Context, *domain.AssetMediaJob, domain.ExternalMediaResult) error
	StartMediaScan(context.Context, domain.ExternalMediaScan) error
	PutMediaScanPart(context.Context, domain.ExternalMediaScanPart) error
	GetMediaScan(context.Context, string) (domain.ExternalMediaScan, []domain.ExternalMediaScanPart, error)
	FinishMediaScan(context.Context, domain.ExternalMediaScan) error
	PendingRequiredMediaIDs(context.Context, []string, int) ([]int64, error)
}
