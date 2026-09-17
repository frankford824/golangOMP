package asset_center

import (
	"strings"

	"workflow/repo"
	baseservice "workflow/service"
)

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
		fileSize := int64(0)
		if row.Asset.FileSize != nil {
			fileSize = *row.Asset.FileSize
		}
		var previewable bool
		process, previewable = baseservice.OSSIMGPreviewProcessForSize(filename, stringPtrValue(row.Asset.MimeType), fileSize)
		if !previewable && assetCenterDirectSVGPreview(filename, stringPtrValue(row.Asset.MimeType)) {
			previewable = true
		}
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

func assetCenterDirectSVGPreview(filename, mimeType string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".svg") ||
		strings.EqualFold(strings.TrimSpace(strings.Split(mimeType, ";")[0]), "image/svg+xml")
}

func firstNonEmptyAssetFilename(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
