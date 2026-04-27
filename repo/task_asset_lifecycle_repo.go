package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type TaskAssetLifecycleUpdate struct {
	AssetID int64
	ActorID int64
	Reason  string
	Now     time.Time
}

type TaskAssetCleanupCandidate struct {
	AssetID            int64
	VersionID          int64
	TaskID             int64
	SourceTaskModuleID *int64
	StorageKey         string
	SourceModuleKey    string
	TaskUpdatedAt      time.Time
}

type TaskAssetLifecycleRepo interface {
	Archive(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	Restore(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	SoftDelete(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	MarkAutoCleaned(ctx context.Context, tx Tx, versionID int64, cleanedAt time.Time) error
	ListEligibleForCleanup(ctx context.Context, cutoff time.Time, limit int) ([]*TaskAssetCleanupCandidate, error)
	GetCurrentForUpdate(ctx context.Context, tx Tx, assetID int64) (*TaskAssetSearchRow, error)
	InsertLifecycleEvent(ctx context.Context, tx Tx, moduleID int64, eventType domain.ModuleEventType, actorID *int64, payload interface{}) error
}
