package asset_center

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type assetCenterExternalRepoStub struct {
	searchRows    []*domain.ExternalAssetRecord
	searchQueries []domain.ExternalAssetSearchQuery
	getRows       map[int64]*domain.ExternalAssetRecord
	previewIDs    []int64
	ossPendingIDs []int64
	getIDs        []int64
}

func (r *assetCenterExternalRepoStub) Search(_ context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	query = query.Normalized()
	r.searchQueries = append(r.searchQueries, query)
	rows := make([]*domain.ExternalAssetRecord, 0, len(r.searchRows))
	for _, row := range r.searchRows {
		if row == nil || row.IsDir || row.Status == domain.ExternalAssetStatusMissing {
			continue
		}
		if query.Kind != "" && row.Kind != query.Kind {
			continue
		}
		if query.MountPath != "" && row.MountPath != query.MountPath {
			continue
		}
		if query.Keyword != "" && !externalAssetTestMatchesKeyword(row, query.Keyword) {
			continue
		}
		if !externalAssetTestMatchesFormat(row, query.FormatCategory) {
			continue
		}
		rows = append(rows, cloneExternalAssetRecord(row))
	}
	return rows, int64(len(rows)), nil
}

func (r *assetCenterExternalRepoStub) Upsert(_ context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error) {
	record := &domain.ExternalAssetRecord{
		ID:            int64(len(r.searchRows) + 1),
		ResourceID:    domain.ExternalAssetResourceID(int64(len(r.searchRows) + 1)),
		Provider:      item.Provider,
		Kind:          item.Kind,
		Driver:        item.Driver,
		MountPath:     item.MountPath,
		OriginPath:    item.OriginPath,
		ParentPath:    item.ParentPath,
		FileName:      item.FileName,
		FileExt:       item.FileExt,
		MimeType:      item.MimeType,
		FileSize:      item.FileSize,
		IsDir:         item.IsDir,
		Status:        domain.ExternalAssetStatusIndexed,
		OSSSyncStatus: domain.ExternalAssetOSSStatusNone,
		PreviewStatus: domain.ExternalAssetPreviewStatusNone,
		CreatedAt:     item.ScannedAt,
		UpdatedAt:     item.ScannedAt,
	}
	r.searchRows = append(r.searchRows, record)
	if r.getRows == nil {
		r.getRows = map[int64]*domain.ExternalAssetRecord{}
	}
	r.getRows[record.ID] = record
	return cloneExternalAssetRecord(record), nil
}

func (r *assetCenterExternalRepoStub) GetByID(_ context.Context, id int64) (*domain.ExternalAssetRecord, error) {
	r.getIDs = append(r.getIDs, id)
	if r.getRows != nil {
		if row := r.getRows[id]; row != nil {
			return cloneExternalAssetRecord(row), nil
		}
	}
	for _, row := range r.searchRows {
		if row != nil && row.ID == id {
			return cloneExternalAssetRecord(row), nil
		}
	}
	return nil, nil
}

func (r *assetCenterExternalRepoStub) CreateSyncRun(_ context.Context, _ *domain.ExternalAssetSyncRun) (int64, error) {
	return 1, nil
}

func (r *assetCenterExternalRepoStub) FinishSyncRun(context.Context, int64, string, int, int, string) error {
	return nil
}

func (r *assetCenterExternalRepoStub) MarkMountMissingBefore(context.Context, string, time.Time) error {
	return nil
}

func (r *assetCenterExternalRepoStub) MarkOriginPrefixesMissingBefore(context.Context, []repo.ExternalAssetOriginPrefix, time.Time) error {
	return nil
}

func (r *assetCenterExternalRepoStub) MarkOriginPathMissing(context.Context, string, string, string) error {
	return nil
}

func (r *assetCenterExternalRepoStub) UpdateDirectURL(_ context.Context, id int64, rawURL string, expiresAt *time.Time, status string) error {
	row, err := r.GetByID(context.Background(), id)
	if err != nil || row == nil {
		return err
	}
	row.RawURL = rawURL
	row.RawURLExpiresAt = expiresAt
	row.DirectURLStatus = status
	if r.getRows == nil {
		r.getRows = map[int64]*domain.ExternalAssetRecord{}
	}
	r.getRows[id] = row
	return nil
}

func (r *assetCenterExternalRepoStub) MarkOSSPreparePending(_ context.Context, id int64) error {
	r.ossPendingIDs = append(r.ossPendingIDs, id)
	return nil
}

func (r *assetCenterExternalRepoStub) MarkOSSPendingByOriginPrefixes(context.Context, []repo.ExternalAssetOriginPrefix) (int64, error) {
	return 0, nil
}

func (r *assetCenterExternalRepoStub) MarkPreviewPreparePending(_ context.Context, id int64) error {
	r.previewIDs = append(r.previewIDs, id)
	return nil
}

func (r *assetCenterExternalRepoStub) MarkPreviewPendingByOriginPrefixes(_ context.Context, prefixes []repo.ExternalAssetOriginPrefix) (int64, error) {
	return int64(len(prefixes)), nil
}

func (r *assetCenterExternalRepoStub) ListDirectURLRefreshCandidates(context.Context, []string, int, time.Time) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *assetCenterExternalRepoStub) ListPendingOSS(context.Context, []string, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *assetCenterExternalRepoStub) ListPendingOSSPrioritized(context.Context, []repo.ExternalAssetOriginPrefix, []string, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *assetCenterExternalRepoStub) ListPendingPreview(context.Context, []string, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *assetCenterExternalRepoStub) MarkOSSReady(context.Context, int64, string) error {
	return nil
}

func (r *assetCenterExternalRepoStub) MarkPreviewReady(context.Context, int64, string) error {
	return nil
}

func (r *assetCenterExternalRepoStub) MarkPrepareFailed(context.Context, int64, string, string) error {
	return nil
}

func externalAssetTestMatchesKeyword(row *domain.ExternalAssetRecord, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		row.ResourceID,
		row.OriginPath,
		row.ParentPath,
		row.FileName,
		row.SearchableText,
	}, " "))
	return strings.Contains(haystack, keyword)
}

func externalAssetTestMatchesFormat(row *domain.ExternalAssetRecord, category domain.AssetFormatCategoryFilter) bool {
	category = domain.ExternalAssetSearchQuery{FormatCategory: category}.Normalized().FormatCategory
	ext := strings.TrimPrefix(strings.ToLower(row.FileExt), ".")
	if ext == "" {
		if idx := strings.LastIndex(row.FileName, "."); idx >= 0 && idx+1 < len(row.FileName) {
			ext = strings.ToLower(row.FileName[idx+1:])
		}
	}
	mimeType := strings.ToLower(strings.TrimSpace(row.MimeType))
	switch category {
	case domain.AssetFormatCategoryAll:
		return true
	case domain.AssetFormatCategoryImage:
		return strings.HasPrefix(mimeType, "image/") || containsString([]string{"jpg", "jpeg", "png", "webp", "gif", "bmp", "svg", "tif", "tiff"}, ext)
	case domain.AssetFormatCategoryDesign:
		return containsString([]string{"psd", "psb", "ai", "cdr", "eps", "svg", "fig", "xd", "sketch", "indd"}, ext)
	case domain.AssetFormatCategoryPDF:
		return ext == "pdf" || mimeType == "application/pdf"
	case domain.AssetFormatCategoryVideo:
		return strings.HasPrefix(mimeType, "video/") || containsString([]string{"mp4", "mov", "avi", "mkv", "webm"}, ext)
	case domain.AssetFormatCategoryArchive:
		return containsString([]string{"zip", "rar", "7z", "tar", "gz"}, ext)
	default:
		return true
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneExternalAssetRecord(row *domain.ExternalAssetRecord) *domain.ExternalAssetRecord {
	if row == nil {
		return nil
	}
	cloned := *row
	return &cloned
}
