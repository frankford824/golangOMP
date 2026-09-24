package externalassets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) versionedMediaEnabled() bool {
	return s.mediaConfig.VersionedSources || s.mediaConfig.NASWorkerEnabled || s.mediaConfig.NASScanEnabled || s.mediaConfig.NASDeliveryEnabled
}

func (s *Service) attachMedia(ctx context.Context, row *domain.ExternalAssetRecord) error {
	if row == nil || !s.versionedMediaEnabled() {
		return nil
	}
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	if !ok {
		return fmt.Errorf("external media repository unavailable")
	}
	var err error
	row.Media, err = r.GetMediaFingerprint(ctx, row.ID)
	return err
}

func (s *Service) MediaRepository() (repo.ExternalAssetMediaRepo, bool) {
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	return r, ok
}

func (s *Service) GetMediaRecord(ctx context.Context, id int64) (*domain.ExternalAssetRecord, *domain.AppError) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "load external resource", nil)
	}
	if appErr := s.sourcePolicyError(row); appErr != nil {
		return nil, appErr
	}
	if row == nil || row.IsDir || !s.isOriginVisible(row.MountPath, row.OriginPath) {
		return nil, domain.ErrNotFound
	}
	if row.Status == domain.ExternalAssetStatusMissing {
		return nil, domain.NewAppError("source_missing", "外部原文件已删除或不可用", nil)
	}
	if err = s.attachMedia(ctx, row); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "load source version", nil)
	}
	return row, nil
}

func (s *Service) mediaObjects(row *domain.ExternalAssetRecord) domain.ExternalMediaResult {
	var result domain.ExternalMediaResult
	if row != nil && row.Media != nil {
		_ = json.Unmarshal(row.Media.Result, &result)
	}
	return result
}

func (s *Service) ExternalMediaInfo(ctx context.Context, id int64, rendition, delivery string) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, appErr := s.GetMediaRecord(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if rendition != "original" && rendition != "thumbnail" && rendition != "preview" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid rendition", nil)
	}
	info := &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, Filename: row.FileName, MimeType: row.MimeType, FileSize: row.FileSize, Rendition: rendition, State: "queued", RetryAfter: 5, AccessHint: "external_media_prepare_required"}
	if row.Media == nil || row.Media.SourceVersion == "" {
		info.State = "temporarily_unavailable"
		info.ErrorCode = "source_index_pending"
		return info, nil
	}
	info.SourceVersion = row.Media.SourceVersion
	if expected := domain.AssetMediaOptionsFromContext(ctx).ExpectedVersion; expected != "" && expected != info.SourceVersion {
		return nil, domain.NewAppError("source_version_changed", "文件已更新，请刷新后重试", nil)
	}
	info.ContentID = domain.AssetMediaIdentity(row.ResourceID, info.SourceVersion, rendition, domain.AssetMediaRecipe)
	if domain.AssetMediaOptionsFromContext(ctx).AuthorizeOnly {
		info.State = "ready"
		return info, nil
	}
	if rendition == "original" && delivery == "lan" {
		info.LocalSource = row
		info.State = "ready"
		info.AccessHint = "nas_original"
		info.RetryAfter = 0
		return info, nil
	}
	objects := s.mediaObjects(row)
	object := objects.Original
	if rendition == "thumbnail" {
		object = objects.Thumbnail
	}
	if rendition == "preview" {
		object = objects.Preview
	}
	if object != nil && object.SourceVersion == info.SourceVersion && object.Key != "" && s.ossDirect != nil && s.ossDirect.Enabled() {
		url := s.ossDirect.PresignDownloadURLWithFilename(object.Key, object.Filename)
		if rendition != "original" {
			url = s.ossDirect.PresignPreviewURL(object.Key)
		}
		if url != nil {
			info.State = "ready"
			info.AccessHint = "external_media_oss"
			info.RetryAfter = 0
			info.DownloadURL = &url.DownloadURL
			info.ExpiresAt = &url.ExpiresAt
			info.ObjectKey = object.Key
			info.FileSize = object.Size
			info.MimeType = object.MimeType
			info.PreviewAvailable = rendition != "original"
			info.CloudDelivery = &domain.AssetMediaDelivery{State: "ready", URL: url.DownloadURL, ExpiresAt: &url.ExpiresAt}
			return info, nil
		}
	}
	if rendition != "original" && !canRenderDerivedPreview(row.FileName, row.MimeType) {
		info.State = "unsupported"
		info.AccessHint = "preview_unsupported"
		info.RetryAfter = 0
		return info, nil
	}
	if s.mediaJobs == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "media queue unavailable", nil)
	}
	if !s.mediaConfig.NASWorkerEnabled {
		info.State = "temporarily_unavailable"
		info.ErrorCode = "media_worker_disabled"
		info.RetryAfter = 0
		return info, nil
	}
	kind := "external_original"
	if rendition != "original" {
		kind = "external_renditions"
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	options := domain.AssetMediaOptionsFromContext(ctx)
	if actor.ID > 0 && options.AccessReference == nil {
		purpose := "download"
		if rendition != "original" {
			purpose = "preview"
		}
		options.AccessReference = &domain.AssetMediaAccessReference{ResourceKind: "external_asset", ResourceID: row.ResourceID, Purpose: purpose, Rendition: rendition, ExpectedSourceVersion: row.Media.SourceVersion}
		ctx = domain.WithAssetMediaOptions(ctx, options)
	}
	input := domain.AssetMediaJobInput{Filename: row.FileName, ExternalID: row.ID, ActorID: actor.ID}
	payload, _ := json.Marshal(input)
	priority := 20
	if actor.ID > 0 {
		priority = 10
		if rendition == "preview" {
			priority = 0
		} else if rendition == "thumbnail" {
			priority = 15
		}
	}
	job, requestID, err := s.mediaJobs.Enqueue(ctx, &domain.AssetMediaJob{Kind: kind, Pool: "nas", ResourceID: row.ResourceID, SourceVersion: row.Media.SourceVersion,
		Recipe: domain.AssetMediaRecipe, Priority: priority, Payload: payload, TotalBytes: row.FileSize, RefreshMissing: true}, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "queue media preparation", nil)
	}
	info.JobID = job.ID
	info.RequestID = requestID
	info.State = job.PublicState()
	info.ErrorCode = job.ErrorCode
	info.Retryable = job.Retryable
	if info.State == "failed" {
		info.AccessHint = "external_media_failed"
		info.RetryAfter = 0
	}
	return info, nil
}

func (s *Service) MediaSourceAllowed(row *domain.ExternalAssetRecord) bool {
	return row != nil && s.sourcePolicyError(row) == nil && row.Status == domain.ExternalAssetStatusIndexed && strings.HasPrefix(row.OriginPath, "/p3/")
}
