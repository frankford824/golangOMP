package asset_center

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// Retouch inputs are attached to an active requirement, not promoted to a
// design delivery revision. Resolve that narrow case without enabling the
// legacy global "latest upload is current" fallback or mutating pointers.
func (s *Service) retouchInputDownloadRow(ctx context.Context, assetID int64) (*repo.TaskAssetSearchRow, *domain.AppError) {
	if s.retouchRequirements == nil {
		return nil, nil
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.EffectiveAccess == nil || !actor.EffectiveAccess.Has(domain.PermissionAssetDownload) {
		return nil, nil
	}
	versions, err := s.searchRepo.ListVersionsByAssetID(ctx, assetID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "cannot resolve retouch input version", nil)
	}
	var latest *repo.TaskAssetSearchRow
	for _, row := range versions {
		if row == nil || row.Asset == nil || row.Asset.AssetID == nil || *row.Asset.AssetID != assetID {
			continue
		}
		if latest == nil || row.Asset.ID > latest.Asset.ID {
			latest = row
		}
	}
	if latest == nil || latest.Task == nil || latest.Task.TaskType != domain.TaskTypeRetouchTask {
		return nil, nil
	}
	asset := latest.Asset
	if asset.TaskID != latest.Task.ID || !asset.AssetType.IsSource() || asset.RetouchRequirementID == nil || *asset.RetouchRequirementID <= 0 {
		return nil, nil
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetDownload, latest.Task.AccessSubject()) {
		return nil, nil
	}
	// Never walk back to an older upload when the latest one was removed or is
	// incomplete. A deleted requirement likewise cannot authorize file access.
	if asset.DeletedAt != nil || asset.CleanedAt != nil || asset.UploadStatus == nil || *asset.UploadStatus != string(domain.DesignAssetUploadStatusUploaded) || asset.StorageKey == nil || strings.TrimSpace(*asset.StorageKey) == "" {
		return nil, nil
	}
	if asset.StorageRef != nil && (asset.StorageRef.Status == domain.AssetStorageRefStatusArchived || asset.StorageRef.Status == domain.AssetStorageRefStatusHistoricalUnavailable) {
		return nil, nil
	}
	requirement, err := s.retouchRequirements.GetByID(ctx, *asset.RetouchRequirementID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "cannot verify retouch input scope", nil)
	}
	if requirement == nil || requirement.TaskID != latest.Task.ID {
		return nil, nil
	}
	return latest, nil
}
