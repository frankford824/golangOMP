package asset_lifecycle

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) Archive(ctx context.Context, actor domain.RequestActor, assetID int64, reason string) *domain.AppError {
	if !isSuperAdmin(actor) {
		return roleDenied()
	}
	if appErr := requireReason(reason); appErr != nil {
		return appErr
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
		if !CanArchive(state) {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset cannot be archived from current lifecycle state", map[string]interface{}{"state": state})
		}
		moduleID, appErr := moduleIDFromAsset(row.Asset)
		if appErr != nil {
			return appErr
		}
		if err := s.lifecycleRepo.Archive(ctx, tx, repo.TaskAssetLifecycleUpdate{AssetID: assetID, ActorID: actor.ID, Reason: reason, Now: now}); err != nil {
			return err
		}
		actorID := actor.ID
		return s.lifecycleRepo.InsertLifecycleEvent(ctx, tx, moduleID, domain.ModuleEventType("asset_archived_by_admin"), &actorID, lifecyclePayload(row.Asset, actor, reason, storageKey(row.Asset)))
	})
	if err != nil {
		return toAppError(err)
	}
	return nil
}
