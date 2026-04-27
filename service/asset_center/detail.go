package asset_center

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) GetDetail(ctx context.Context, assetID int64) (*AssetDetail, *domain.AppError) {
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id must be greater than zero", nil)
	}
	current, err := s.searchRepo.GetCurrentByAssetID(ctx, assetID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if current == nil || current.Asset == nil || current.Asset.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	versions, err := s.searchRepo.ListVersionsByAssetID(ctx, assetID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return buildAssetDetail(current, versions), nil
}

func buildAssetDetail(row *repo.TaskAssetSearchRow, versions []*repo.TaskAssetSearchRow) *AssetDetail {
	if row == nil || row.Asset == nil || row.Task == nil {
		return nil
	}
	a := row.Asset
	t := row.Task
	state := domain.DeriveLifecycleState(*a, *t)
	currentVersionID := a.ID
	detail := &AssetDetail{
		ID:                valueInt64(a.AssetID, a.ID),
		TaskID:            a.TaskID,
		AssetNo:           row.AssetNo,
		AssetType:         a.AssetType,
		CurrentVersionID:  &currentVersionID,
		SourceModuleKey:   a.SourceModuleKey,
		LifecycleState:    state,
		ArchiveStatus:     archiveStatus(state),
		CurrentStorageKey: a.StorageKey,
		FileName:          a.FileName,
		OriginalFilename:  valueString(a.OriginalName, a.FileName),
		FileSize:          a.FileSize,
		MimeType:          valueString(a.MimeType, ""),
		TaskNo:            t.TaskNo,
		TaskStatus:        t.TaskStatus,
		OwnerTeamCode:     row.OwnerTeamCode,
		CreatedBy:         row.DesignCreatedBy,
		CreatedAt:         row.DesignCreatedAt,
		UpdatedAt:         row.DesignUpdatedAt,
		ArchivedAt:        a.ArchivedAt,
		CleanedAt:         a.CleanedAt,
		DeletedAt:         a.DeletedAt,
	}
	if detail.CreatedAt.IsZero() {
		detail.CreatedAt = a.CreatedAt
	}
	if detail.UpdatedAt.IsZero() {
		detail.UpdatedAt = a.CreatedAt
	}
	if a.UploadStatus != nil && domain.DesignAssetUploadStatus(*a.UploadStatus).Valid() {
		detail.UploadStatus = domain.DesignAssetUploadStatus(*a.UploadStatus)
	}
	if a.ScopeSKUCode != nil {
		detail.ScopeSKUCode = *a.ScopeSKUCode
	}
	if a.ArchivedBy != nil {
		detail.ArchivedBy = &Actor{UserID: *a.ArchivedBy}
	}
	if len(versions) > 0 {
		detail.Versions = make([]AssetVersion, 0, len(versions))
		for _, version := range versions {
			if version == nil || version.Asset == nil {
				continue
			}
			va := version.Asset
			versionNo := va.VersionNo
			if va.AssetVersionNo != nil {
				versionNo = *va.AssetVersionNo
			}
			detail.Versions = append(detail.Versions, AssetVersion{
				VersionID:  va.ID,
				VersionNo:  versionNo,
				StorageKey: va.StorageKey,
				FileSize:   va.FileSize,
				MimeType:   va.MimeType,
				CreatedAt:  va.CreatedAt,
				CreatedBy:  Actor{UserID: va.UploadedBy},
			})
		}
	}
	return detail
}

func archiveStatus(state domain.AssetLifecycleState) domain.AssetArchiveStatus {
	switch state {
	case domain.AssetLifecycleStateArchived, domain.AssetLifecycleStateAutoCleaned:
		return domain.AssetArchiveStatusArchived
	default:
		return domain.AssetArchiveStatusActive
	}
}

func valueInt64(ptr *int64, fallback int64) int64 {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func valueString(ptr *string, fallback string) string {
	if ptr == nil {
		return fallback
	}
	return *ptr
}
