package asset_center

import (
	"context"
	pathpkg "path"
	"strings"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	externalassets "workflow/service/external_assets"
)

const materialSystemRoot = "/系统资源"

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
	if assetSearchHasSystemOnlyFilters(query) && query.Source == domain.AssetResourceSourceExternal {
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
		!assetSearchHasSystemOnlyFilters(query) &&
		s.externalSvc != nil && s.externalSvc.Enabled() {
		external, externalTotal, appErr := s.searchExternalRows(ctx, query)
		if appErr != nil {
			return nil, appErr
		}
		items = append(items, external...)
		if len(items) > query.Size {
			items = items[:query.Size]
		}
		total += externalTotal
	}
	return &SearchResult{Items: items, Total: total, Page: query.Page, Size: query.Size}, nil
}

func assetSearchHasSystemOnlyFilters(query domain.AssetSearchQuery) bool {
	return query.UsableState != domain.AssetUsableStateFilterAll ||
		query.BusinessLane.Valid() ||
		query.AssetType.Valid() ||
		strings.TrimSpace(query.ModuleKey) != "" ||
		strings.TrimSpace(query.OwnerTeamCode) != "" ||
		query.TimeBasis == domain.AssetSearchTimeBasisTaskCreatedAt
}

func (s *Service) BrowseMaterials(ctx context.Context, query MaterialBrowseQuery) (*MaterialBrowseResult, *domain.AppError) {
	query = normalizeMaterialBrowseQuery(query)
	result := &MaterialBrowseResult{
		Path:    query.Path,
		Folders: []MaterialFolder{},
		Files:   []*AssetDetail{},
		Total:   0,
		Page:    query.Page,
		Size:    query.Size,
	}
	includeSystem := query.Source == domain.AssetResourceSourceAll || query.Source == domain.AssetResourceSourceSystem
	includeExternal := query.Source == domain.AssetResourceSourceAll || query.Source == domain.AssetResourceSourceExternal

	if query.Path == "" {
		if includeSystem {
			total, appErr := s.countSystemMaterials(ctx)
			if appErr != nil {
				return nil, appErr
			}
			result.Folders = append(result.Folders, MaterialFolder{
				Path:       materialSystemRoot,
				Name:       strings.TrimPrefix(materialSystemRoot, "/"),
				SourceType: string(domain.AssetResourceSourceSystem),
				FileCount:  total,
			})
		}
		if includeExternal && s.externalSvc != nil && s.externalSvc.Enabled() {
			folders, err := s.externalSvc.ListDirectoryChildren(ctx, "", 2000)
			if err != nil {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
			}
			result.Folders = append(result.Folders, materialFoldersFromExternal(folders)...)
		}
		return result, nil
	}

	if query.Path == materialSystemRoot || strings.HasPrefix(query.Path, materialSystemRoot+"/") {
		if !includeSystem || query.Path != materialSystemRoot {
			return result, nil
		}
		search, appErr := s.Search(ctx, materialSystemSearchQuery(query.Page, query.Size))
		if appErr != nil {
			return nil, appErr
		}
		result.Files = search.Items
		result.Total = search.Total
		result.Page = search.Page
		result.Size = search.Size
		return result, nil
	}

	if includeExternal && s.externalSvc != nil && s.externalSvc.Enabled() {
		folders, err := s.externalSvc.ListDirectoryChildren(ctx, query.Path, 2000)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		files, total, err := s.externalSvc.ListDirectoryFiles(ctx, query.Path, query.Page, query.Size)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		result.Folders = materialFoldersFromExternal(folders)
		result.Files = make([]*AssetDetail, 0, len(files))
		for _, row := range files {
			result.Files = append(result.Files, s.buildExternalAssetDetail(row))
		}
		result.Total = total
	}
	return result, nil
}

func normalizeMaterialBrowseQuery(query MaterialBrowseQuery) MaterialBrowseQuery {
	query.Path = normalizeMaterialBrowsePath(query.Path)
	switch query.Source {
	case domain.AssetResourceSourceSystem, domain.AssetResourceSourceExternal:
	default:
		query.Source = domain.AssetResourceSourceAll
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 50
	}
	if query.Size > 100 {
		query.Size = 100
	}
	return query
}

func normalizeMaterialBrowsePath(raw string) string {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" || value == "/" {
		return ""
	}
	cleaned := pathpkg.Clean("/" + strings.TrimLeft(value, "/"))
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return cleaned
}

func materialSystemSearchQuery(page, size int) domain.AssetSearchQuery {
	return domain.AssetSearchQuery{
		Page:           page,
		Size:           size,
		Source:         domain.AssetResourceSourceSystem,
		UsableState:    domain.AssetUsableStateFilterAll,
		FormatCategory: domain.AssetFormatCategoryAll,
		IsArchived:     domain.AssetArchiveFilterFalse,
		TaskStatus:     domain.AssetTaskStatusFilterAll,
	}
}

func (s *Service) countSystemMaterials(ctx context.Context) (int64, *domain.AppError) {
	search, appErr := s.Search(ctx, materialSystemSearchQuery(1, 1))
	if appErr != nil {
		return 0, appErr
	}
	return search.Total, nil
}

func materialFoldersFromExternal(entries []domain.ExternalAssetDirectoryEntry) []MaterialFolder {
	folders := make([]MaterialFolder, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = pathpkg.Base(entry.Path)
		}
		folders = append(folders, MaterialFolder{
			Path:            entry.Path,
			Name:            name,
			SourceType:      string(domain.AssetResourceSourceExternal),
			FileCount:       entry.FileCount,
			DirectFileCount: entry.DirectFileCount,
		})
	}
	return folders
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
		Keyword:        query.Keyword,
		CreatedFrom:    query.CreatedFrom,
		CreatedTo:      query.CreatedTo,
		FormatCategory: query.FormatCategory,
		Page:           query.Page,
		Size:           query.Size,
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
