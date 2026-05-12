package asset_center

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
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

type BatchDownloadResult struct {
	Filename     string
	ZipBytes     []byte
	SuccessCount int
	FailureCount int
}

type batchDownloadFailure struct {
	AssetID  int64
	TaskID   int64
	Filename string
	Reason   string
}

func (s *Service) BuildBatchDownloadZip(ctx context.Context, assetIDs []int64) (*BatchDownloadResult, *domain.AppError) {
	if len(assetIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_ids must not be empty", nil)
	}
	if len(assetIDs) > MaxBatchDownloadAssets {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_ids exceed batch download limit", map[string]interface{}{
			"limit": MaxBatchDownloadAssets,
		})
	}
	if s.streamOpener == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "storage stream opener is not configured", nil)
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

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	nameRegistry := map[string]map[string]int{}
	failures := make([]batchDownloadFailure, 0)
	successCount := 0
	var totalSize int64

	for _, requestedAssetID := range assetIDs {
		row := rowMap[requestedAssetID]
		if row == nil || row.Asset == nil || row.Task == nil {
			failures = append(failures, batchDownloadFailure{
				AssetID: requestedAssetID,
				Reason:  "asset_not_found",
			})
			continue
		}
		asset := row.Asset
		taskID := asset.TaskID
		assetID := valueInt64(asset.AssetID, asset.ID)
		filename := resolveBatchFilename(asset, assetID)

		if asset.DeletedAt != nil {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "deleted"})
			continue
		}
		if asset.CleanedAt != nil {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "cleaned"})
			continue
		}
		storageKey := ""
		if asset.StorageKey != nil {
			storageKey = strings.TrimSpace(*asset.StorageKey)
		}
		if storageKey == "" {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "missing_storage_key"})
			continue
		}
		if asset.UploadStatus == nil || domain.DesignAssetUploadStatus(strings.TrimSpace(*asset.UploadStatus)) != domain.DesignAssetUploadStatusUploaded {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "upload_status_not_uploaded"})
			continue
		}
		fileSize := int64(0)
		if asset.FileSize != nil {
			fileSize = *asset.FileSize
		}
		if fileSize > 0 && totalSize+fileSize > MaxBatchDownloadTotalBytes {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "total_size_limit_exceeded"})
			continue
		}

		stream, openErr := s.streamOpener.Open(ctx, storageKey)
		if openErr != nil {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "stream_open_failed"})
			continue
		}

		dir := "task-" + strconv.FormatInt(taskID, 10)
		entryName := ensureUniqueEntry(dir, filename, nameRegistry)
		entryWriter, createErr := zipWriter.Create(entryName)
		if createErr != nil {
			_ = stream.Close()
			return nil, domain.NewAppError(domain.ErrCodeInternalError, createErr.Error(), nil)
		}
		written, copyErr := io.Copy(entryWriter, stream)
		closeErr := stream.Close()
		if copyErr != nil || closeErr != nil {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "stream_read_failed"})
			continue
		}
		totalSize += written
		if totalSize > MaxBatchDownloadTotalBytes {
			failures = append(failures, batchDownloadFailure{AssetID: assetID, TaskID: taskID, Filename: filename, Reason: "total_size_limit_exceeded"})
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "all requested assets are unavailable for download", map[string]interface{}{
			"asset_ids":      assetIDs,
			"failure_count":  len(failures),
			"total_size_max": MaxBatchDownloadTotalBytes,
		})
	}

	if len(failures) > 0 {
		if err := writeDownloadErrors(zipWriter, failures); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}

	return &BatchDownloadResult{
		Filename:     "assets-" + time.Now().UTC().Format("20060102-150405") + ".zip",
		ZipBytes:     buf.Bytes(),
		SuccessCount: successCount,
		FailureCount: len(failures),
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
	return sanitizeZipFilename(baseservice.ResolveAssetDownloadFilename(originalName, fileName, assetID))
}

func sanitizeZipFilename(name string) string {
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

func ensureUniqueEntry(dir, filename string, registry map[string]map[string]int) string {
	if registry[dir] == nil {
		registry[dir] = map[string]int{}
	}
	count := registry[dir][filename] + 1
	registry[dir][filename] = count
	if count == 1 {
		return dir + "/" + filename
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return dir + "/" + fmt.Sprintf("%s (%d)%s", base, count, ext)
}

func writeDownloadErrors(zipWriter *zip.Writer, failures []batchDownloadFailure) error {
	entryWriter, err := zipWriter.Create("download_errors.txt")
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, item := range failures {
		_, _ = b.WriteString(fmt.Sprintf("asset_id=%d task_id=%d filename=%s reason=%s\n", item.AssetID, item.TaskID, item.Filename, item.Reason))
	}
	_, err = io.Copy(entryWriter, strings.NewReader(b.String()))
	return err
}
