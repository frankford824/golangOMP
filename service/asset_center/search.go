package asset_center

import (
	"context"
	"sort"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	externalassets "workflow/service/external_assets"
)

type Service struct {
	searchRepo   repo.TaskAssetSearchRepo
	presigner    DownloadPresigner
	urlBuilder   BrowserURLBuilder
	streamOpener baseservice.StorageStreamOpener
	externalSvc  *externalassets.Service
}

type DownloadPresigner interface {
	Enabled() bool
	PresignDownloadURL(objectKey string) *baseservice.OSSDirectDownloadInfo
}

type DownloadFilenamePresigner interface {
	PresignDownloadURLWithFilename(objectKey, filename string) *baseservice.OSSDirectDownloadInfo
}

type BrowserURLBuilder interface {
	BuildBrowserFileURL(storageKey string) *string
}

func NewService(searchRepo repo.TaskAssetSearchRepo, presigner DownloadPresigner, urlBuilder BrowserURLBuilder) *Service {
	return &Service{searchRepo: searchRepo, presigner: presigner, urlBuilder: urlBuilder}
}

func (s *Service) SetStorageStreamOpener(opener baseservice.StorageStreamOpener) {
	s.streamOpener = opener
}

func (s *Service) SetExternalAssetService(externalSvc *externalassets.Service) {
	s.externalSvc = externalSvc
}

func (s *Service) Search(ctx context.Context, query domain.AssetSearchQuery) (*SearchResult, *domain.AppError) {
	query = query.Normalized()
	if query.UsableState != domain.AssetUsableStateFilterAll && query.Source == domain.AssetResourceSourceExternal {
		return &SearchResult{Items: []*AssetDetail{}, Total: 0, Page: query.Page, Size: query.Size}, nil
	}
	if query.Source == domain.AssetResourceSourceExternal {
		return s.searchExternal(ctx, query)
	}
	rows, total, err := s.searchRepo.Search(ctx, query)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	items := make([]*AssetDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildAssetDetail(row, nil))
	}
	if query.Source == domain.AssetResourceSourceAll &&
		query.UsableState == domain.AssetUsableStateFilterAll &&
		s.externalSvc != nil && s.externalSvc.Enabled() {
		external, externalTotal, appErr := s.searchExternalRows(ctx, query)
		if appErr != nil {
			return nil, appErr
		}
		items = append(items, external...)
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		})
		if len(items) > query.Size {
			items = items[:query.Size]
		}
		total += externalTotal
	}
	return &SearchResult{Items: items, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *Service) searchExternal(ctx context.Context, query domain.AssetSearchQuery) (*SearchResult, *domain.AppError) {
	items, total, appErr := s.searchExternalRows(ctx, query)
	if appErr != nil {
		return nil, appErr
	}
	return &SearchResult{Items: items, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *Service) searchExternalRows(ctx context.Context, query domain.AssetSearchQuery) ([]*AssetDetail, int64, *domain.AppError) {
	if s.externalSvc == nil || !s.externalSvc.Enabled() {
		return []*AssetDetail{}, 0, nil
	}
	rows, total, err := s.externalSvc.Search(ctx, domain.ExternalAssetSearchQuery{
		Keyword: query.Keyword,
		Page:    query.Page,
		Size:    query.Size,
	})
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	items := make([]*AssetDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.buildExternalAssetDetail(row))
	}
	return items, total, nil
}

func (s *Service) buildExternalAssetDetail(row *domain.ExternalAssetRecord) *AssetDetail {
	if row == nil {
		return nil
	}
	previewAvailable := canExternalAssetDirectPreview(row)
	if row.OSSPreviewKey != "" && row.PreviewStatus == domain.ExternalAssetPreviewStatusReady {
		previewAvailable = true
	}
	downloadURL := ""
	if s != nil && s.externalSvc != nil {
		if previewURL := s.externalSvc.BrowserPreviewURL(row); previewURL != "" {
			downloadURL = previewURL
			previewAvailable = true
		} else if url := s.externalSvc.BrowserDownloadURL(row); url != "" {
			downloadURL = url
		}
	}
	return &AssetDetail{
		ID:                    row.ID,
		ResourceID:            row.ResourceID,
		SourceType:            string(domain.AssetResourceSourceExternal),
		SourceLabel:           "外部资源",
		TaskID:                0,
		AssetType:             domain.TaskAssetTypeSource,
		SourceModuleKey:       "external_assets",
		LifecycleState:        domain.AssetLifecycleStateActive,
		ArchiveStatus:         domain.AssetArchiveStatusActive,
		FileName:              row.FileName,
		OriginalFilename:      row.FileName,
		FileSize:              &row.FileSize,
		MimeType:              row.MimeType,
		DownloadURL:           stringPtrIfNotEmpty(downloadURL),
		PreviewAvailable:      previewAvailable,
		UsableState:           domain.TaskAssetUsableStateNotApplicable,
		UsableLabel:           "外部资源",
		ProductName:           row.OriginPath,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		ExternalKind:          string(row.Kind),
		ExternalMountPath:     row.MountPath,
		ExternalDriver:        row.Driver,
		OriginPath:            row.OriginPath,
		OSSSyncStatus:         string(row.OSSSyncStatus),
		ExternalPreviewStatus: string(row.PreviewStatus),
		LastPrepareError:      row.LastPrepareError,
	}
}

func canExternalAssetDirectPreview(row *domain.ExternalAssetRecord) bool {
	if row == nil || row.IsDir {
		return false
	}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		return row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady
	}
	ext := row.FileExt
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg", ".pdf", ".mp4", ".mov":
		return true
	default:
		return false
	}
}

func stringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
