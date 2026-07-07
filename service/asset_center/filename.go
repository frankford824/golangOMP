package asset_center

import (
	"strings"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

type BatchDownloadNamingMode string

const (
	BatchDownloadNamingOriginal BatchDownloadNamingMode = "original"
	BatchDownloadNamingBusiness BatchDownloadNamingMode = "business"
)

func NormalizeBatchDownloadNamingMode(raw string) BatchDownloadNamingMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(BatchDownloadNamingBusiness):
		return BatchDownloadNamingBusiness
	default:
		return BatchDownloadNamingOriginal
	}
}

type BatchDownloadOptions struct {
	NamingMode BatchDownloadNamingMode
}

type BatchDownloadOption func(*BatchDownloadOptions)

func WithBatchDownloadNamingMode(mode BatchDownloadNamingMode) BatchDownloadOption {
	return func(options *BatchDownloadOptions) {
		options.NamingMode = NormalizeBatchDownloadNamingMode(string(mode))
	}
}

func normalizeBatchDownloadOptions(options []BatchDownloadOption) BatchDownloadOptions {
	out := BatchDownloadOptions{NamingMode: BatchDownloadNamingOriginal}
	for _, apply := range options {
		if apply != nil {
			apply(&out)
		}
	}
	out.NamingMode = NormalizeBatchDownloadNamingMode(string(out.NamingMode))
	return out
}

func resolveSingleDownloadFilename(row *repo.TaskAssetSearchRow) string {
	if row == nil || row.Asset == nil {
		return "asset"
	}
	assetID := valueInt64(row.Asset.AssetID, row.Asset.ID)
	return resolveUnifiedAssetDownloadFilename(row, assetID)
}

func resolveBatchFilenameForMode(row *repo.TaskAssetSearchRow, assetID int64, mode BatchDownloadNamingMode) string {
	if mode == BatchDownloadNamingBusiness {
		return sanitizeBatchFilename(resolveUnifiedAssetDownloadFilename(row, assetID))
	}
	if row == nil {
		return sanitizeBatchFilename(baseservice.ResolveAssetDownloadFilename("", "", assetID))
	}
	return resolveBatchFilename(row.Asset, assetID)
}

func resolveUnifiedAssetDownloadFilename(row *repo.TaskAssetSearchRow, assetID int64) string {
	if row == nil || row.Asset == nil {
		return baseservice.ResolveAssetDownloadFilename("", "", assetID)
	}
	if isBusinessNamedAsset(row.Asset) && rowSKUCode(row) != "" && rowProductName(row) != "" {
		return baseservice.ResolveAssetDownloadFilenameForBusiness(
			taskAssetOriginalName(row.Asset),
			row.Asset.FileName,
			assetID,
			rowSKUCode(row),
			rowProductName(row),
		)
	}
	return baseservice.ResolveAssetDownloadFilename(taskAssetOriginalName(row.Asset), row.Asset.FileName, assetID)
}

func isBusinessNamedAsset(asset *domain.TaskAsset) bool {
	if asset == nil {
		return false
	}
	assetType := asset.AssetType.Canonical()
	return assetType == domain.TaskAssetTypeSource || assetType == domain.TaskAssetTypeDelivery
}

func rowProductName(row *repo.TaskAssetSearchRow) string {
	if row == nil || row.Task == nil {
		return ""
	}
	return strings.TrimSpace(row.Task.ProductNameSnapshot)
}

func taskAssetOriginalName(asset *domain.TaskAsset) string {
	if asset == nil || asset.OriginalName == nil {
		return ""
	}
	return strings.TrimSpace(*asset.OriginalName)
}

func rowSKUCode(row *repo.TaskAssetSearchRow) string {
	if row == nil {
		return ""
	}
	if row.Asset != nil && row.Asset.ScopeSKUCode != nil {
		if value := strings.TrimSpace(*row.Asset.ScopeSKUCode); value != "" {
			return value
		}
	}
	if row.Task != nil {
		if value := strings.TrimSpace(row.Task.SKUCode); value != "" {
			return value
		}
		if value := strings.TrimSpace(row.Task.PrimarySKUCode); value != "" {
			return value
		}
	}
	return ""
}
