package asset_center

import (
	"context"
	"strconv"
	"time"

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

func (s *Service) GetExternalDetail(ctx context.Context, externalID int64) (*AssetDetail, *domain.AppError) {
	if externalID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id must be greater than zero", nil)
	}
	if s.externalSvc == nil || !s.externalSvc.Enabled() {
		return nil, domain.ErrNotFound
	}
	row, err := s.externalSvc.Get(ctx, externalID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if row == nil {
		return nil, domain.ErrNotFound
	}
	return s.buildExternalAssetDetail(row), nil
}

func buildAssetDetail(row *repo.TaskAssetSearchRow, versions []*repo.TaskAssetSearchRow) *AssetDetail {
	if row == nil || row.Asset == nil || row.Task == nil {
		return nil
	}
	a := row.Asset
	t := row.Task
	state := domain.DeriveLifecycleState(*a, *t)
	usableState := domain.DeriveTaskAssetUsableState(*a)
	currentVersionID := a.ID
	uploadedAt := a.UploadedAt
	if uploadedAt == nil && !a.CreatedAt.IsZero() {
		v := a.CreatedAt
		uploadedAt = &v
	}
	var taskCreatedAt *time.Time
	if !t.CreatedAt.IsZero() {
		v := t.CreatedAt
		taskCreatedAt = &v
	}
	detail := &AssetDetail{
		ID:                    valueInt64(a.AssetID, a.ID),
		ResourceID:            strconv.FormatInt(valueInt64(a.AssetID, a.ID), 10),
		SourceType:            string(domain.AssetResourceSourceSystem),
		SourceLabel:           "系统资源",
		TaskID:                a.TaskID,
		AssetNo:               row.AssetNo,
		AssetType:             a.AssetType,
		CurrentVersionID:      &currentVersionID,
		SourceModuleKey:       a.SourceModuleKey,
		LifecycleState:        state,
		ArchiveStatus:         archiveStatus(state),
		CurrentStorageKey:     a.StorageKey,
		FileName:              a.FileName,
		OriginalFilename:      valueString(a.OriginalName, a.FileName),
		FileSize:              a.FileSize,
		MimeType:              valueString(a.MimeType, ""),
		TaskNo:                t.TaskNo,
		SKUCode:               t.SKUCode,
		PrimarySKUCode:        t.PrimarySKUCode,
		ProductName:           t.ProductNameSnapshot,
		TaskStatus:            t.TaskStatus,
		BusinessLane:          t.BusinessLane,
		OwnerTeamCode:         row.OwnerTeamCode,
		CreatedBy:             row.DesignCreatedBy,
		CreatedByUsername:     row.AssetCreatorUsername,
		CreatedByName:         row.AssetCreatorName,
		TaskCreatorID:         t.CreatorID,
		TaskCreatorUsername:   row.TaskCreatorUsername,
		TaskCreatorName:       row.TaskCreatorName,
		UploadedAt:            uploadedAt,
		TaskCreatedAt:         taskCreatedAt,
		CreatedAt:             row.DesignCreatedAt,
		UpdatedAt:             row.DesignUpdatedAt,
		ArchivedAt:            a.ArchivedAt,
		CleanedAt:             a.CleanedAt,
		DeletedAt:             a.DeletedAt,
		FlowReviewStatus:      domain.NormalizeTaskAssetFlowReviewStatus(a.FlowReviewStatus, a.AssetType),
		UsableState:           usableState,
		UsableLabel:           assetUsableLabel(usableState),
		ApprovedAt:            a.ApprovedAt,
		ApprovedBy:            a.ApprovedBy,
		RejectedAt:            a.RejectedAt,
		RejectedBy:            a.RejectedBy,
		SupersededByVersionID: a.SupersededByVersionID,
		SupersededAt:          a.SupersededAt,
		CleanupAfterAt:        a.CleanupAfterAt,
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
				VersionID:             va.ID,
				VersionNo:             versionNo,
				StorageKey:            va.StorageKey,
				FileSize:              va.FileSize,
				MimeType:              va.MimeType,
				FlowReviewStatus:      domain.NormalizeTaskAssetFlowReviewStatus(va.FlowReviewStatus, va.AssetType),
				UsableState:           domain.DeriveTaskAssetUsableState(*va),
				ApprovedAt:            va.ApprovedAt,
				ApprovedBy:            va.ApprovedBy,
				RejectedAt:            va.RejectedAt,
				RejectedBy:            va.RejectedBy,
				SupersededByVersionID: va.SupersededByVersionID,
				SupersededAt:          va.SupersededAt,
				CleanupAfterAt:        va.CleanupAfterAt,
				CreatedAt:             va.CreatedAt,
				CreatedBy:             Actor{UserID: va.UploadedBy, Username: version.UploadedByUsername, Name: version.UploadedByName},
			})
		}
	}
	return detail
}

func assetUsableLabel(state domain.TaskAssetUsableState) string {
	switch state {
	case domain.TaskAssetUsableStateReadyForUse:
		return "可直接使用"
	case domain.TaskAssetUsableStatePendingReview:
		return "待审核"
	case domain.TaskAssetUsableStateRejected:
		return "审核未通过"
	case domain.TaskAssetUsableStateHistory:
		return "历史版本"
	case domain.TaskAssetUsableStateCleaned:
		return "文件已清理"
	default:
		return "不进入审核流"
	}
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
