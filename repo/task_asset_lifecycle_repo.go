package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type TaskAssetLifecycleUpdate struct {
	AssetID      int64
	TaskAssetIDs []int64
	ActorID      int64
	Reason       string
	Now          time.Time
}

type TaskAssetDeleteGuard struct {
	DesignAssetIDs       []int64
	TaskAssetIDs         []int64
	AllStagedUnbound     bool
	RevisionReferenceIDs []int64
	PublicationPinIDs    []int64
}

func (g TaskAssetDeleteGuard) Referenced() bool {
	return len(g.RevisionReferenceIDs) > 0 || len(g.PublicationPinIDs) > 0
}

type AssetObjectDeletionOutboxItem struct {
	ID                   int64
	TaskAssetID          *int64
	StorageRefID         *string
	StorageAdapter       domain.AssetStorageAdapter
	StorageIsPlaceholder bool
	StorageKey           string
	Attempt              int
}

type AssetObjectDeletionOutboxRepo interface {
	ClaimObjectDeletions(ctx context.Context, tx Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]AssetObjectDeletionOutboxItem, error)
	MarkObjectDeletionSucceeded(ctx context.Context, tx Tx, item AssetObjectDeletionOutboxItem, leaseToken string, deletedAt time.Time) error
	MarkObjectDeletionRetry(ctx context.Context, tx Tx, item AssetObjectDeletionOutboxItem, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error
}

type TaskAssetCleanupCandidate struct {
	AssetID            int64
	VersionID          int64
	TaskID             int64
	SourceTaskModuleID *int64
	StorageKey         string
	RelatedStorageKeys []string
	SourceModuleKey    string
	TaskUpdatedAt      time.Time
	CleanupReason      string
}

type TaskAssetLifecycleRepo interface {
	Archive(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	Restore(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	LockGenericDeleteGuard(ctx context.Context, tx Tx, assetID int64) (*TaskAssetDeleteGuard, error)
	LockCleanupObjectIDs(ctx context.Context, tx Tx, versionID int64) ([]int64, error)
	EnqueueObjectDeletions(ctx context.Context, tx Tx, taskAssetIDs []int64) error
	SoftDelete(ctx context.Context, tx Tx, update TaskAssetLifecycleUpdate) error
	MarkAutoCleaned(ctx context.Context, tx Tx, versionID int64, cleanedAt time.Time) error
	ListEligibleForCleanup(ctx context.Context, cutoff time.Time, limit int) ([]*TaskAssetCleanupCandidate, error)
	GetCurrentForUpdate(ctx context.Context, tx Tx, assetID int64) (*TaskAssetSearchRow, error)
	InsertLifecycleEvent(ctx context.Context, tx Tx, moduleID int64, eventType domain.ModuleEventType, actorID *int64, payload interface{}) error
}

// TaskAssetLifecycleObjectDeletionRepo is the runtime lifecycle repository
// contract. Keeping the durable outbox methods in the constructor's static
// return type prevents the deletion worker from being wired as an optional,
// silently disabled capability.
type TaskAssetLifecycleObjectDeletionRepo interface {
	TaskAssetLifecycleRepo
	AssetObjectDeletionOutboxRepo
}
