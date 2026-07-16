package asset_lifecycle

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) Delete(ctx context.Context, actor domain.RequestActor, assetID int64, reason string) *domain.AppError {
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
		if actor.EffectiveAccess == nil || !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetManage, row.Task.AccessSubject()) {
			return deleteAccessDenied(row.Task.ID)
		}
		if row.Task.TaskStatus == domain.TaskStatusCompleted || row.Task.TaskStatus == domain.TaskStatusArchived {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "finalized task resources can only be changed after the task is reopened", map[string]interface{}{
				"deny_code":   "finalized_resource_requires_reopen",
				"task_id":     row.Task.ID,
				"task_status": row.Task.TaskStatus,
			})
		}
		state := domain.DeriveLifecycleState(*row.Asset, *row.Task)
		if !CanDelete(state) {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "asset cannot be deleted from current lifecycle state", map[string]interface{}{"state": state})
		}
		guard, err := s.lifecycleRepo.LockGenericDeleteGuard(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if !guard.AllStagedUnbound {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "only staged and unbound resources may be deleted from the generic asset endpoint", map[string]interface{}{
				"deny_code":      "asset_delete_requires_staged_unbound",
				"task_id":        row.Task.ID,
				"task_asset_ids": guard.TaskAssetIDs,
			})
		}
		if guard.Referenced() {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource is referenced by workflow history or a publication pin", map[string]interface{}{
				"deny_code":              "asset_delete_resource_referenced",
				"task_id":                row.Task.ID,
				"revision_reference_ids": guard.RevisionReferenceIDs,
				"publication_pin_ids":    guard.PublicationPinIDs,
			})
		}
		moduleID, appErr := s.resolveLifecycleEventModuleID(ctx, tx, row)
		if appErr != nil {
			return appErr
		}
		if err := s.lifecycleRepo.EnqueueObjectDeletions(ctx, tx, guard.TaskAssetIDs); err != nil {
			return err
		}
		if err := s.lifecycleRepo.SoftDelete(ctx, tx, repo.TaskAssetLifecycleUpdate{AssetID: assetID, TaskAssetIDs: guard.TaskAssetIDs, ActorID: actor.ID, Reason: reason, Now: now}); err != nil {
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
