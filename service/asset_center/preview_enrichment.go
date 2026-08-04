package asset_center

import (
	"path/filepath"
	"strings"

	"workflow/repo"
	baseservice "workflow/service"
)

const assetCenterPreviewProcess = "image/auto-orient,1/resize,w_1600,m_lfit/quality,Q_85/format,jpg"

type previewPresigner interface {
	PresignPreviewURL(objectKey string) *baseservice.OSSDirectDownloadInfo
	PresignPreviewURLWithProcess(objectKey, process string) *baseservice.OSSDirectDownloadInfo
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *Service) enrichSystemAssetPreview(detail *AssetDetail, row *repo.TaskAssetSearchRow) {
	if detail == nil || row == nil || row.Asset == nil {
		return
	}
	key := strings.TrimSpace(row.DerivedPreviewStorageKey)
	filename := strings.TrimSpace(row.DerivedPreviewFilename)
	process := ""
	if key == "" {
		key = stringPtrValue(row.Asset.StorageKey)
		filename = firstNonEmptyAssetFilename(row.Asset.FileName, stringPtrValue(row.Asset.OriginalName))
		var previewable bool
		process, previewable = assetCenterDirectPreviewProcess(filename, stringPtrValue(row.Asset.MimeType))
		if !previewable {
			return
		}
	}
	detail.PreviewAvailable = key != ""
	if key == "" || s == nil || s.presigner == nil || !s.presigner.Enabled() {
		return
	}
	presigner, ok := s.presigner.(previewPresigner)
	if !ok {
		return
	}
	signed := presigner.PresignPreviewURL(key)
	if process != "" {
		signed = presigner.PresignPreviewURLWithProcess(key, process)
	}
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		return
	}
	urlValue := strings.TrimSpace(signed.DownloadURL)
	detail.PreviewURL = &urlValue
}

func assetCenterDirectPreviewProcess(filename, mimeType string) (string, bool) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	if ext == "" {
		switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "image/gif":
			ext = ".gif"
		case "image/tiff":
			ext = ".tiff"
		case "image/heic", "image/heif":
			ext = ".heic"
		case "image/avif":
			ext = ".avif"
		}
	}
	switch ext {
	case ".tif", ".tiff", ".heic", ".heif", ".avif":
		return assetCenterPreviewProcess, true
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg":
		return "", true
	default:
		return "", false
	}
}

func firstNonEmptyAssetFilename(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
