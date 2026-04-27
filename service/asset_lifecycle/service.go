package asset_lifecycle

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type ObjectDeleter interface {
	Enabled() bool
	DeleteObject(ctx context.Context, objectKey string) error
}

type Service struct {
	searchRepo    repo.TaskAssetSearchRepo
	lifecycleRepo repo.TaskAssetLifecycleRepo
	txRunner      repo.TxRunner
	deleter       ObjectDeleter
	now           func() time.Time
}

func NewService(searchRepo repo.TaskAssetSearchRepo, lifecycleRepo repo.TaskAssetLifecycleRepo, txRunner repo.TxRunner, deleter ObjectDeleter) *Service {
	return &Service{
		searchRepo:    searchRepo,
		lifecycleRepo: lifecycleRepo,
		txRunner:      txRunner,
		deleter:       deleter,
		now:           time.Now,
	}
}

func (s *Service) WithNow(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

func roleDenied() *domain.AppError {
	return domain.NewAppError(domain.DenyModuleActionRoleDenied, "SuperAdmin role is required", nil)
}

func requireReason(reason string) *domain.AppError {
	if strings.TrimSpace(reason) == "" {
		return domain.ErrReasonRequired
	}
	return nil
}

func moduleIDFromAsset(asset *domain.TaskAsset) (int64, *domain.AppError) {
	if asset == nil || asset.SourceTaskModuleID == nil || *asset.SourceTaskModuleID <= 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset source_task_module_id is required for lifecycle event", nil)
	}
	return *asset.SourceTaskModuleID, nil
}

func lifecyclePayload(asset *domain.TaskAsset, actor domain.RequestActor, reason string, originalStorageKey string) map[string]interface{} {
	payload := map[string]interface{}{
		"asset_id":    valueAssetID(asset),
		"version_id":  asset.ID,
		"reason":      strings.TrimSpace(reason),
		"actor_id":    actor.ID,
		"actor_roles": actor.Roles,
		"storage_key": originalStorageKey,
		"module_key":  asset.SourceModuleKey,
	}
	return payload
}

func valueAssetID(asset *domain.TaskAsset) int64 {
	if asset == nil {
		return 0
	}
	if asset.AssetID != nil {
		return *asset.AssetID
	}
	return asset.ID
}

func storageKey(asset *domain.TaskAsset) string {
	if asset == nil || asset.StorageKey == nil {
		return ""
	}
	return strings.TrimSpace(*asset.StorageKey)
}
