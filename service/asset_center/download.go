package asset_center

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

func (s *Service) DownloadLatest(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, err := s.searchRepo.GetCurrentByAssetID(ctx, assetID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return s.downloadRow(row)
}

func (s *Service) DownloadVersion(ctx context.Context, assetID, versionID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, err := s.searchRepo.GetVersion(ctx, assetID, versionID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return s.downloadRow(row)
}

func (s *Service) DownloadExternal(ctx context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.externalSvc == nil || !s.externalSvc.Enabled() {
		return nil, domain.ErrNotFound
	}
	return s.externalSvc.DownloadInfo(ctx, externalID)
}

func (s *Service) PreviewExternal(ctx context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.externalSvc == nil || !s.externalSvc.Enabled() {
		return nil, domain.ErrNotFound
	}
	return s.externalSvc.PreviewInfo(ctx, externalID)
}

func (s *Service) downloadRow(row *repo.TaskAssetSearchRow) (*domain.AssetDownloadInfo, *domain.AppError) {
	if row == nil || row.Asset == nil || row.Task == nil {
		return nil, domain.ErrNotFound
	}
	state := domain.DeriveLifecycleState(*row.Asset, *row.Task)
	switch state {
	case domain.AssetLifecycleStateDeleted:
		return nil, domain.ErrNotFound
	case domain.AssetLifecycleStateAutoCleaned:
		return nil, domain.NewAppError(ErrCodeAssetGone, "asset version has been auto-cleaned", map[string]interface{}{
			"asset_id":    valueInt64(row.Asset.AssetID, row.Asset.ID),
			"version_id":  row.Asset.ID,
			"cleaned_at":  row.Asset.CleanedAt,
			"storage_key": row.Asset.StorageKey,
		})
	}
	key := ""
	if row.Asset.StorageKey != nil {
		key = strings.TrimSpace(*row.Asset.StorageKey)
	}
	if key == "" {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "asset storage_key is missing", nil)
	}
	originalName := ""
	if row.Asset.OriginalName != nil {
		originalName = *row.Asset.OriginalName
	}
	filename := baseservice.ResolveAssetDownloadFilename(originalName, row.Asset.FileName, valueInt64(row.Asset.AssetID, row.Asset.ID))
	fileSize := int64(0)
	if row.Asset.FileSize != nil {
		fileSize = *row.Asset.FileSize
	}
	mimeType := ""
	if row.Asset.MimeType != nil {
		mimeType = *row.Asset.MimeType
	}
	if s.presigner != nil && s.presigner.Enabled() {
		signed := s.presigner.PresignDownloadURL(key)
		if filenamePresigner, ok := s.presigner.(DownloadFilenamePresigner); ok {
			signed = filenamePresigner.PresignDownloadURLWithFilename(key, filename)
		}
		if signed != nil && strings.TrimSpace(signed.DownloadURL) != "" {
			url := signed.DownloadURL
			return &domain.AssetDownloadInfo{
				DownloadMode:     domain.AssetDownloadModeDirect,
				DownloadURL:      &url,
				AccessHint:       "oss_presigned",
				PreviewAvailable: false,
				Filename:         filename,
				FileSize:         fileSize,
				MimeType:         mimeType,
				ExpiresAt:        &signed.ExpiresAt,
			}, nil
		}
	}
	var downloadURL *string
	if s.urlBuilder != nil {
		downloadURL = s.urlBuilder.BuildBrowserFileURL(key)
	}
	if downloadURL != nil {
		urlValue := baseservice.AppendProxyDownloadFilenameQuery(*downloadURL, filename)
		downloadURL = &urlValue
	}
	if downloadURL == nil {
		expires := time.Now().UTC().Add(15 * time.Minute)
		return &domain.AssetDownloadInfo{
			DownloadMode:     domain.AssetDownloadModeProxy,
			DownloadURL:      nil,
			AccessHint:       "storage_key_only",
			PreviewAvailable: false,
			Filename:         filename,
			FileSize:         fileSize,
			MimeType:         mimeType,
			ExpiresAt:        &expires,
		}, nil
	}
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeProxy,
		DownloadURL:      downloadURL,
		AccessHint:       "upload_service_browser_url",
		PreviewAvailable: false,
		Filename:         filename,
		FileSize:         fileSize,
		MimeType:         mimeType,
	}, nil
}
