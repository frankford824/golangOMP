package repo

import (
	"context"
	"time"

	"workflow/domain"
)

// ExternalAssetSyncRow is the machine-sync projection of one source path.
// Missing rows are explicit tombstones; indexed rows must already have a
// durable OSS original before they can enter the current snapshot.
type ExternalAssetSyncRow struct {
	ID               int64
	Provider         string
	Kind             domain.ExternalAssetKind
	MountPath        string
	OriginPathHash   string
	OriginPath       string
	ParentPath       string
	FileName         string
	MimeType         string
	FileSize         int64
	Status           domain.ExternalAssetStatus
	OSSSyncStatus    domain.ExternalAssetOSSStatus
	OSSOriginalKey   string
	SourceModifiedAt *time.Time
	UpdatedAt        time.Time
}

type ExternalAssetSyncRepo interface {
	ListExternalAssetSyncSnapshot(context.Context, []ExternalAssetOriginPrefix) ([]ExternalAssetSyncRow, error)
	ListCurrentExternalAssetsForSyncByIDs(context.Context, []int64, []ExternalAssetOriginPrefix) ([]ExternalAssetSyncRow, error)
	GetExternalAssetSyncHead(context.Context, []ExternalAssetOriginPrefix) (ExternalAssetSyncCursor, error)
	ListExternalAssetSyncChanges(context.Context, []ExternalAssetOriginPrefix, ExternalAssetSyncCursor, int) ([]ExternalAssetSyncRow, bool, error)
}

type ExternalAssetSyncCursor struct {
	UpdatedAt time.Time
	ID        int64
}
