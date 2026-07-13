package asset_lifecycle

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) Delete(ctx context.Context, actor domain.RequestActor, assetID int64, reason string) *domain.AppError {
	current, err := s.searchRepo.GetCurrentByAssetID(ctx, assetID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if current == nil || current.Asset == nil || current.Task == nil {
		return domain.ErrNotFound
	}
	if !canDeleteCompletedTaskAsset(actor, current) {
		return deleteRoleDenied()
	}
	if appErr := requireReason(reason); appErr != nil {
		return appErr
	}
	state := domain.DeriveLifecycleState(*current.Asset, *current.Task)
	if !CanDelete(state) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset cannot be deleted from current lifecycle state", map[string]interface{}{"state": state})
	}
	versions, err := s.searchRepo.ListVersionsByAssetID(ctx, assetID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if len(versions) == 0 {
		return domain.ErrNotFound
	}
	if s.deleter != nil && s.deleter.Enabled() {
		for _, version := range versions {
			key := storageKey(version.Asset)
			if key == "" {
				continue
			}
			if err := s.deleter.DeleteObject(ctx, key); err != nil {
				return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
			}
		}
	}
	now := s.now().UTC()
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		row, err := s.lifecycleRepo.GetCurrentForUpdate(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if row == nil || row.Asset == nil || row.Task == nil {
			return domain.ErrNotFound
		}
		state := domain.DeriveLifecycleState(*row.Asset, *row.Task)
		if !CanDelete(state) {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset cannot be deleted from current lifecycle state", map[string]interface{}{"state": state})
		}
		moduleID, appErr := s.resolveLifecycleEventModuleID(ctx, tx, row)
		if appErr != nil {
			return appErr
		}
		if err := s.lifecycleRepo.SoftDelete(ctx, tx, repo.TaskAssetLifecycleUpdate{AssetID: assetID, ActorID: actor.ID, Reason: reason, Now: now}); err != nil {
			return err
		}
		actorID := actor.ID
		return s.lifecycleRepo.InsertLifecycleEvent(ctx, tx, moduleID, domain.ModuleEventType("asset_deleted_by_admin"), &actorID, lifecyclePayload(row.Asset, actor, reason, storageKey(row.Asset)))
	})
	if err != nil {
		return toAppError(err)
	}
	return nil
}
