package asset_center

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

const (
	MaxBatchDownloadAssets     = 100
	MaxBatchDownloadTotalBytes = int64(512 * 1024 * 1024)
)

type BatchDownloadManifest struct {
	Items        []BatchDownloadItem    `json:"items"`
	Failures     []BatchDownloadFailure `json:"failures,omitempty"`
	SuccessCount int                    `json:"success_count"`
	FailureCount int                    `json:"failure_count"`
	TotalSize    int64                  `json:"total_size"`
	ExpiresAt    *time.Time             `json:"expires_at,omitempty"`
}

type BatchDownloadItem struct {
	AssetID     int64      `json:"asset_id"`
	TaskID      int64      `json:"task_id"`
	Filename    string     `json:"filename"`
	FileSize    int64      `json:"file_size"`
	MimeType    string     `json:"mime_type,omitempty"`
	DownloadURL string     `json:"download_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type BatchDownloadFailure struct {
	AssetID  int64  `json:"asset_id"`
	TaskID   int64  `json:"task_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	Reason   string `json:"reason"`
}

func (s *Service) BuildBatchDownloadManifest(ctx context.Context, assetIDs []int64) (*BatchDownloadManifest, *domain.AppError) {
	if len(assetIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_ids must not be empty", nil)
	}
	if len(assetIDs) > MaxBatchDownloadAssets {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_ids exceed batch download limit", map[string]interface{}{
			"limit": MaxBatchDownloadAssets,
		})
	}
	if s.presigner == nil || !s.presigner.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "oss direct download presigner is not configured", nil)
	}

	rows, err := s.searchRepo.ListCurrentByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}

	rowMap := make(map[int64]*repo.TaskAssetSearchRow, len(rows))
	for _, row := range rows {
		if row == nil || row.Asset == nil {
			continue
		}
		id := valueInt64(row.Asset.AssetID, row.Asset.ID)
		if id > 0 {
			rowMap[id] = row
		}
	}

	manifest := &BatchDownloadManifest{
		Items:    make([]BatchDownloadItem, 0, len(assetIDs)),
		Failures: make([]BatchDownloadFailure, 0),
	}
	usedNames := map[string]int{}
	var totalSize int64

	for _, requestedAssetID := range assetIDs {
		item, failure := s.buildBatchDownloadItem(rowMap[requestedAssetID], requestedAssetID, totalSize, usedNames)
		if failure != nil {
			manifest.Failures = append(manifest.Failures, *failure)
			continue
		}
		if item.DownloadURL == "" {
			manifest.Failures = append(manifest.Failures, BatchDownloadFailure{
				AssetID:  item.AssetID,
				TaskID:   item.TaskID,
				Filename: item.Filename,
				Reason:   "download_url_unavailable",
			})
			continue
		}
		totalSize += item.FileSize
		if manifest.ExpiresAt == nil || (item.ExpiresAt != nil && item.ExpiresAt.Before(*manifest.ExpiresAt)) {
			manifest.ExpiresAt = item.ExpiresAt
		}
		manifest.Items = append(manifest.Items, item)
	}

	if len(manifest.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "all requested assets are unavailable for download", map[string]interface{}{
			"asset_ids":      assetIDs,
			"failure_count":  len(manifest.Failures),
			"total_size_max": MaxBatchDownloadTotalBytes,
		})
	}
	manifest.SuccessCount = len(manifest.Items)
	manifest.FailureCount = len(manifest.Failures)
	manifest.TotalSize = totalSize
	return manifest, nil
}

func (s *Service) buildBatchDownloadItem(row *repo.TaskAssetSearchRow, requestedAssetID int64, currentTotal int64, usedNames map[string]int) (BatchDownloadItem, *BatchDownloadFailure) {
	if row == nil || row.Asset == nil || row.Task == nil {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: requestedAssetID, Reason: "asset_not_found"}
	}
	asset := row.Asset
	taskID := asset.TaskID
	assetID := valueInt64(asset.AssetID, asset.ID)
	filename := resolveBatchFilename(asset, assetID)

	if asset.DeletedAt != nil {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "deleted"}
	}
	if asset.CleanedAt != nil {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "cleaned"}
	}
	storageKey := ""
	if asset.StorageKey != nil {
		storageKey = strings.TrimSpace(*asset.StorageKey)
	}
	if storageKey == "" {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "missing_storage_key"}
	}
	if asset.UploadStatus == nil || domain.DesignAssetUploadStatus(strings.TrimSpace(*asset.UploadStatus)) != domain.DesignAssetUploadStatusUploaded {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "upload_status_not_uploaded"}
	}

	fileSize := int64(0)
	if asset.FileSize != nil {
		fileSize = *asset.FileSize
	}
	if fileSize > 0 && currentTotal+fileSize > MaxBatchDownloadTotalBytes {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "total_size_limit_exceeded"}
	}

	filename = ensureUniqueBatchFilename(filename, usedNames)
	signed := s.presigner.PresignDownloadURL(storageKey)
	if filenamePresigner, ok := s.presigner.(DownloadFilenamePresigner); ok {
		signed = filenamePresigner.PresignDownloadURLWithFilename(storageKey, filename)
	}
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		return BatchDownloadItem{}, &BatchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "download_url_unavailable"}
	}

	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.TrimSpace(*asset.MimeType)
	}
	expiresAt := signed.ExpiresAt
	return BatchDownloadItem{
		AssetID:     assetID,
		TaskID:      taskID,
		Filename:    filename,
		FileSize:    fileSize,
		MimeType:    mimeType,
		DownloadURL: strings.TrimSpace(signed.DownloadURL),
		ExpiresAt:   &expiresAt,
	}, nil
}

func resolveBatchFilename(asset *domain.TaskAsset, assetID int64) string {
	originalName := ""
	if asset != nil && asset.OriginalName != nil {
		originalName = *asset.OriginalName
	}
	fileName := ""
	if asset != nil {
		fileName = asset.FileName
	}
	return sanitizeBatchFilename(baseservice.ResolveAssetDownloadFilename(originalName, fileName, assetID))
}

func sanitizeBatchFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "asset"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "\x00", "", "\r", "_", "\n", "_")
	name = replacer.Replace(name)
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "asset"
	}
	return name
}

func ensureUniqueBatchFilename(filename string, registry map[string]int) string {
	count := registry[filename] + 1
	registry[filename] = count
	if count == 1 {
		return filename
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if strings.TrimSpace(base) == "" {
		base = "asset"
	}
	return fmt.Sprintf("%s (%d)%s", base, count, ext)
}
