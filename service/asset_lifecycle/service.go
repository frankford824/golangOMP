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

type lifecycleEventModuleResolver interface {
	ResolveOrCreateLifecycleEventModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (int64, error)
}

type Service struct {
	lifecycleRepo repo.TaskAssetLifecycleRepo
	txRunner      repo.TxRunner
	now           func() time.Time
}

func NewService(_ repo.TaskAssetSearchRepo, lifecycleRepo repo.TaskAssetLifecycleRepo, txRunner repo.TxRunner, _ ObjectDeleter) *Service {
	return &Service{
		lifecycleRepo: lifecycleRepo,
		txRunner:      txRunner,
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

func deleteAccessDenied(taskID int64) *domain.AppError {
	return domain.NewAppError(
		domain.ErrCodePermissionDenied,
		"asset deletion is outside the actor's explicit capability or task scope",
		map[string]interface{}{
			"deny_code":            "asset_manage_scope_denied",
			"task_id":              taskID,
			"required_permissions": []domain.PermissionCode{domain.PermissionAssetManage},
		},
	)
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

func (s *Service) resolveLifecycleEventModuleID(ctx context.Context, tx repo.Tx, row *repo.TaskAssetSearchRow) (int64, *domain.AppError) {
	if row == nil || row.Asset == nil || row.Task == nil {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset task context is required for lifecycle event", nil)
	}
	if moduleID, appErr := moduleIDFromAsset(row.Asset); appErr == nil {
		return moduleID, nil
	}
	resolver, ok := s.lifecycleRepo.(lifecycleEventModuleResolver)
	if !ok {
		return moduleIDFromAsset(row.Asset)
	}
	moduleKey := lifecycleSourceModuleKey(row.Asset, row.Task)
	moduleID, err := resolver.ResolveOrCreateLifecycleEventModule(ctx, tx, row.Task.ID, moduleKey)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if moduleID <= 0 {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "asset lifecycle event module could not be resolved", nil)
	}
	row.Asset.SourceModuleKey = moduleKey
	row.Asset.SourceTaskModuleID = &moduleID
	return moduleID, nil
}

func lifecycleSourceModuleKey(asset *domain.TaskAsset, task *domain.Task) string {
	if asset != nil {
		switch key := strings.TrimSpace(asset.SourceModuleKey); key {
		case domain.ModuleKeyBasicInfo, domain.ModuleKeyDesign, domain.ModuleKeyAudit,
			domain.ModuleKeyCustomization, domain.ModuleKeyRetouch:
			return key
		}
		assetType := domain.NormalizeTaskAssetType(asset.AssetType)
		if assetType.IsReference() {
			return domain.ModuleKeyBasicInfo
		}
		if task != nil && task.TaskType == domain.TaskTypeRetouchTask &&
			(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
			return domain.ModuleKeyRetouch
		}
		if task != nil && task.CustomizationRequired &&
			(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
			return domain.ModuleKeyCustomization
		}
	}
	return domain.ModuleKeyDesign
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
