package service

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TaskReferenceBatchDownloadManifest struct {
	Items        []TaskReferenceBatchDownloadItem    `json:"items"`
	Failures     []TaskReferenceBatchDownloadFailure `json:"failures,omitempty"`
	SuccessCount int                                 `json:"success_count"`
	FailureCount int                                 `json:"failure_count"`
	TotalSize    int64                               `json:"total_size"`
	ExpiresAt    *time.Time                          `json:"expires_at,omitempty"`
}

type TaskReferenceBatchDownloadItem struct {
	Key         string     `json:"key"`
	Filename    string     `json:"filename"`
	FileSize    int64      `json:"file_size"`
	MimeType    string     `json:"mime_type,omitempty"`
	DownloadURL string     `json:"download_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	SourceKind  string     `json:"source_kind"`
	AssetID     *int64     `json:"asset_id,omitempty"`
	RefID       *string    `json:"ref_id,omitempty"`
}

type TaskReferenceBatchDownloadFailure struct {
	Key        string  `json:"key,omitempty"`
	SourceKind string  `json:"source_kind,omitempty"`
	AssetID    *int64  `json:"asset_id,omitempty"`
	RefID      *string `json:"ref_id,omitempty"`
	Filename   string  `json:"filename,omitempty"`
	Reason     string  `json:"reason"`
}

func (s *taskAssetCenterService) BuildTaskReferenceBatchDownloadManifest(ctx context.Context, taskID int64, actorID int64) (*TaskReferenceBatchDownloadManifest, *domain.AppError) {
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id must be greater than zero", nil)
	}
	if actorID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "actor_id must be greater than zero", nil)
	}
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	decision := s.taskActionAuthorizer().EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionReadDetail, decision)
	}

	manifest := &TaskReferenceBatchDownloadManifest{
		Items:    make([]TaskReferenceBatchDownloadItem, 0),
		Failures: make([]TaskReferenceBatchDownloadFailure, 0),
	}
	seenByRefID := map[string]struct{}{}
	seenByStorageKey := map[string]struct{}{}
	seenByURL := map[string]struct{}{}

	referenceType := domain.TaskAssetTypeReference
	assets, err := s.designAssetRepo.List(ctx, repo.DesignAssetListFilter{
		TaskID:    &taskID,
		AssetType: &referenceType,
	})
	if err != nil {
		return nil, infraError("list task reference assets", err)
	}
	for _, asset := range assets {
		if asset == nil || asset.CurrentVersionID == nil || *asset.CurrentVersionID <= 0 {
			continue
		}
		version, getErr := s.taskAssetRepo.GetByID(ctx, *asset.CurrentVersionID)
		if getErr != nil || version == nil {
			continue
		}
		if version.AssetID == nil || *version.AssetID <= 0 {
			continue
		}
		designVersion := domain.BuildDesignAssetVersion(version)
		if designVersion == nil {
			continue
		}
		if checkErr := validateAssetVersionObjectAvailable(designVersion); checkErr != nil {
			manifest.Failures = append(manifest.Failures, TaskReferenceBatchDownloadFailure{
				SourceKind: "formalized_asset",
				AssetID:    version.AssetID,
				Filename:   version.FileName,
				Reason:     checkErr.Code,
			})
			continue
		}
		downloadInfo := buildAssetDownloadInfoWithOSS(designVersion, s.uploadClient, s.ossDirectService)
		if downloadInfo == nil || strings.TrimSpace(valueOrEmpty(downloadInfo.DownloadURL)) == "" {
			manifest.Failures = append(manifest.Failures, TaskReferenceBatchDownloadFailure{
				SourceKind: "formalized_asset",
				AssetID:    version.AssetID,
				Filename:   version.FileName,
				Reason:     "download_url_unavailable",
			})
			continue
		}
		assetID := *version.AssetID
		key := "asset:" + taskRefInt64ToString(assetID)
		fileSize := int64(0)
		if version.FileSize != nil {
			fileSize = *version.FileSize
		}
		filename := resolveTaskReferenceFilename(downloadInfo.Filename, version.FileName, assetID)
		downloadURL := strings.TrimSpace(valueOrEmpty(downloadInfo.DownloadURL))
		item := TaskReferenceBatchDownloadItem{
			Key:         key,
			Filename:    filename,
			FileSize:    fileSize,
			MimeType:    firstNonEmpty(versionMimeType(version), downloadInfo.MimeType),
			DownloadURL: downloadURL,
			ExpiresAt:   downloadInfo.ExpiresAt,
			SourceKind:  "formalized_asset",
			AssetID:     &assetID,
		}
		manifest.Items = append(manifest.Items, item)
		manifest.TotalSize += fileSize
		markSeenReferenceAsset(version, downloadURL, seenByRefID, seenByStorageKey, seenByURL)
		adjustManifestExpiry(manifest, item.ExpiresAt)
	}

	detail, detailErr := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if detailErr != nil {
		return nil, infraError("get task detail for legacy references", detailErr)
	}
	var skuRefs []domain.ReferenceFileRef
	skuItems, skuErr := s.taskRepo.ListSKUItemsByTaskID(ctx, taskID)
	if skuErr == nil {
		for _, item := range skuItems {
			if item == nil {
				continue
			}
			skuRefs = append(skuRefs, item.ReferenceFileRefs...)
		}
	}
	legacyRefs := make([]domain.ReferenceFileRef, 0)
	if detail != nil {
		legacyRefs = append(legacyRefs, domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON)...)
	}
	legacyRefs = append(legacyRefs, skuRefs...)
	legacyRefs = domain.NormalizeReferenceFileRefs(legacyRefs)
	for _, ref := range legacyRefs {
		refID := strings.TrimSpace(ref.CanonicalID())
		storageKey := firstNonEmpty(strings.TrimSpace(ref.StorageKey), extractStorageKeyFromReferenceURL(valueOrEmpty(ref.DownloadURL)), extractStorageKeyFromReferenceURL(valueOrEmpty(ref.URL)))
		downloadURL := strings.TrimSpace(firstNonEmpty(valueOrEmpty(ref.DownloadURL), valueOrEmpty(ref.URL)))
		if refID != "" {
			if _, ok := seenByRefID[refID]; ok {
				continue
			}
		}
		if storageKey != "" {
			if _, ok := seenByStorageKey[storageKey]; ok {
				continue
			}
		}
		if downloadURL != "" {
			if _, ok := seenByURL[downloadURL]; ok {
				continue
			}
		}

		downloadURL, downloadExpiresAt := s.resolveLegacyReferenceDownload(downloadURL, storageKey, strings.TrimSpace(ref.Filename), ref.DownloadURLExpiresAt)
		if downloadURL == "" {
			manifest.Failures = append(manifest.Failures, TaskReferenceBatchDownloadFailure{
				SourceKind: "legacy_ref",
				RefID:      optionalStringPtr(refID),
				Filename:   strings.TrimSpace(ref.Filename),
				Reason:     "download_url_unavailable",
			})
			continue
		}
		fileSize := int64(0)
		if ref.FileSize != nil {
			fileSize = *ref.FileSize
		}
		filename := resolveLegacyReferenceFilename(ref, refID)
		item := TaskReferenceBatchDownloadItem{
			Key:         "ref:" + firstNonEmpty(refID, storageKey, filename),
			Filename:    filename,
			FileSize:    fileSize,
			MimeType:    strings.TrimSpace(ref.MimeType),
			DownloadURL: downloadURL,
			ExpiresAt:   downloadExpiresAt,
			SourceKind:  "legacy_ref",
			RefID:       optionalStringPtr(refID),
		}
		manifest.Items = append(manifest.Items, item)
		manifest.TotalSize += fileSize
		if refID != "" {
			seenByRefID[refID] = struct{}{}
		}
		if storageKey != "" {
			seenByStorageKey[storageKey] = struct{}{}
		}
		seenByURL[downloadURL] = struct{}{}
		adjustManifestExpiry(manifest, item.ExpiresAt)
	}

	manifest.SuccessCount = len(manifest.Items)
	manifest.FailureCount = len(manifest.Failures)
	return manifest, nil
}

func (s *taskAssetCenterService) resolveLegacyReferenceDownload(existingURL, storageKey, filename string, existingExpiresAt *time.Time) (string, *time.Time) {
	existingURL = strings.TrimSpace(existingURL)
	storageKey = strings.TrimSpace(storageKey)
	filename = strings.TrimSpace(filename)

	// A stored compatibility proxy URL is not a direct-download manifest item:
	// it still requires a session cookie and is therefore unsuitable for the
	// frontend's cross-origin ZIP fetch. Prefer a fresh OSS URL whenever the
	// canonical storage key is known, even when a legacy URL is already set.
	if storageKey != "" && s != nil && s.ossDirectService != nil && s.ossDirectService.Enabled() {
		if signed := s.ossDirectService.PresignDownloadURLWithFilename(storageKey, filename); signed != nil {
			urlValue := strings.TrimSpace(signed.DownloadURL)
			if urlValue != "" {
				if signed.ExpiresAt.IsZero() {
					return urlValue, existingExpiresAt
				}
				expiresAt := signed.ExpiresAt
				return urlValue, &expiresAt
			}
		}
	}
	if existingURL != "" {
		return existingURL, existingExpiresAt
	}
	if storageKey == "" {
		return "", existingExpiresAt
	}
	proxyURL := domain.BuildRelativeEscapedURLPath("/v1/assets/files", storageKey)
	return AppendProxyDownloadFilenameQuery(proxyURL, filename), existingExpiresAt
}

func markSeenReferenceAsset(version *domain.TaskAsset, downloadURL string, seenByRefID, seenByStorageKey, seenByURL map[string]struct{}) {
	if version == nil {
		return
	}
	if version.StorageRefID != nil {
		if refID := strings.TrimSpace(*version.StorageRefID); refID != "" {
			seenByRefID[refID] = struct{}{}
		}
	}
	if version.StorageKey != nil {
		if key := strings.TrimSpace(*version.StorageKey); key != "" {
			seenByStorageKey[key] = struct{}{}
		}
	}
	if downloadURL != "" {
		seenByURL[downloadURL] = struct{}{}
	}
}

func resolveTaskReferenceFilename(preferred, fallback string, assetID int64) string {
	base := strings.TrimSpace(preferred)
	if base == "" {
		base = strings.TrimSpace(fallback)
	}
	base = sanitizeReferenceBatchFilename(base)
	if base == "" {
		base = "reference-" + taskRefInt64ToString(assetID)
	}
	return base
}

func resolveLegacyReferenceFilename(ref domain.ReferenceFileRef, refID string) string {
	name := sanitizeReferenceBatchFilename(strings.TrimSpace(ref.Filename))
	if name != "" {
		return name
	}
	if refID != "" {
		return "reference-" + refID
	}
	if key := strings.TrimSpace(ref.StorageKey); key != "" {
		return "reference-" + sanitizeReferenceBatchFilename(filepath.Base(key))
	}
	return "reference"
}

func sanitizeReferenceBatchFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "\x00", "", "\r", "_", "\n", "_")
	name = replacer.Replace(name)
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return ""
	}
	return name
}

func adjustManifestExpiry(manifest *TaskReferenceBatchDownloadManifest, expiresAt *time.Time) {
	if manifest == nil || expiresAt == nil {
		return
	}
	if manifest.ExpiresAt == nil || expiresAt.Before(*manifest.ExpiresAt) {
		manifest.ExpiresAt = expiresAt
	}
}

func extractStorageKeyFromReferenceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	path := raw
	if parsed, err := url.Parse(raw); err == nil && parsed != nil && parsed.Path != "" {
		path = parsed.Path
	}
	const prefix = "/v1/assets/files/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if trimmed == "" {
		return ""
	}
	segments := strings.Split(trimmed, "/")
	for i, segment := range segments {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return ""
		}
		segments[i] = decoded
	}
	return strings.TrimSpace(strings.Join(segments, "/"))
}

func versionMimeType(version *domain.TaskAsset) string {
	if version == nil || version.MimeType == nil {
		return ""
	}
	return strings.TrimSpace(*version.MimeType)
}

func taskRefInt64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
