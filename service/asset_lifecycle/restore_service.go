package asset_lifecycle

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) Restore(ctx context.Context, actor domain.RequestActor, assetID int64) *domain.AppError {
	if !isSuperAdmin(actor) {
		return roleDenied()
	}
	now := s.now().UTC()
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		row, err := s.lifecycleRepo.GetCurrentForUpdate(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if row == nil || row.Asset == nil || row.Task == nil {
			return domain.ErrNotFound
		}
		state := domain.DeriveLifecycleState(*row.Asset, *row.Task)
		if !CanRestore(state) {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset cannot be restored from current lifecycle state", map[string]interface{}{"state": state})
		}
		moduleID, appErr := moduleIDFromAsset(row.Asset)
		if appErr != nil {
			return appErr
		}
		if err := s.lifecycleRepo.Restore(ctx, tx, repo.TaskAssetLifecycleUpdate{AssetID: assetID, ActorID: actor.ID, Now: now}); err != nil {
			return err
		}
		actorID := actor.ID
		return s.lifecycleRepo.InsertLifecycleEvent(ctx, tx, moduleID, domain.ModuleEventType("asset_unarchived_by_admin"), &actorID, lifecyclePayload(row.Asset, actor, "", storageKey(row.Asset)))
	})
	if err != nil {
		return toAppError(err)
	}
	return nil
}
