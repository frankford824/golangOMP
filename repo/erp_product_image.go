package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type ERPProductImageReference struct {
	VersionID int64
	ExpiresAt *time.Time
}

type ERPProductImageTarget struct {
	TaskID    int64
	TaskNo    string
	VersionID int64
	SingleSKU bool
}

type ERPProductImageRequest struct {
	TaskID int64
	RefID  string
}

type ERPProductImageRepo interface {
	FindLegacyObject(ctx context.Context, sku, key string) (int64, error)
	LegacyObjectKey(ctx context.Context, id int64) (string, error)
	LegacyObjectKeyForSKU(ctx context.Context, id int64, sku string) (string, error)
	ActiveObjectVersions(ctx context.Context, sku, key string) ([]int64, error)
	HasRegisteredObject(ctx context.Context, taskID int64, key string) (bool, error)
	ActiveAsset(ctx context.Context, id int64) (*domain.TaskAsset, error)
	Target(ctx context.Context, sku string) (ERPProductImageTarget, error)
	RequestReference(ctx context.Context, sku string, request ERPProductImageRequest, at time.Time) error
	PendingReference(ctx context.Context, sku string) (ERPProductImageRequest, error)
	ClearReference(ctx context.Context, sku string, request ERPProductImageRequest) error
	Schedule(ctx context.Context, sku string, at time.Time) error
	NextSKU(ctx context.Context, now time.Time) (string, error)
	ImageGapScanSKUs(ctx context.Context, after string, now time.Time, limit int) ([]string, error)
	Acquire(ctx context.Context, sku, token string, now, until time.Time) (bool, error)
	RecordReferences(ctx context.Context, sku, token string, refs []ERPProductImageReference, replace bool) error
	Finish(ctx context.Context, sku, token string, next time.Time, issue string) error
	FindReference(ctx context.Context, taskID int64, sku string, ref domain.ReferenceFileRef) (*domain.TaskAsset, error)
}
