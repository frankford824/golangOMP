package asset_center

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	pathpkg "path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	externalassets "workflow/service/external_assets"
)

const materialSystemRoot = "/系统资源"
const assetSearchTotalCacheTTL = 30 * time.Second

type Service struct {
	searchRepo         repo.TaskAssetSearchRepo
	productionRepo     repo.ProductionPackageRepo
	finalizedSyncRepo  repo.FinalizedAssetSyncRepo
	finalizedSyncStore FinalizedAssetSyncObjectStore
	packageJobRepo     repo.ProductionPackageJobRepo
	packageStore       ProductionPackageObjectStore
	presigner          DownloadPresigner
	urlBuilder         BrowserURLBuilder
	streamOpener       baseservice.StorageStreamOpener
	externalSvc        *externalassets.Service
	cache              AssetCenterCache
	flightMu           sync.Mutex
	searchFlights      map[string]*assetSearchFlight
}

type assetSearchFlight struct {
	done  chan struct{}
	rows  []*repo.TaskAssetSearchRow
	total int64
	err   error
}

type Option func(*Service)

type AssetCenterCache interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
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

func NewService(searchRepo repo.TaskAssetSearchRepo, presigner DownloadPresigner, urlBuilder BrowserURLBuilder, opts ...Option) *Service {
	s := &Service{searchRepo: searchRepo, presigner: presigner, urlBuilder: urlBuilder}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithAssetCenterRedis(cache AssetCenterCache) Option {
	return func(s *Service) {
		s.cache = cache
	}
}

func WithProductionPackageRepo(repository repo.ProductionPackageRepo) Option {
	return func(s *Service) {
		s.productionRepo = repository
	}
}

func WithProductionPackageJobs(repository repo.ProductionPackageJobRepo, store ProductionPackageObjectStore) Option {
	return func(s *Service) {
		s.packageJobRepo = repository
		s.packageStore = store
	}
}

func WithFinalizedAssetSync(repository repo.FinalizedAssetSyncRepo, store FinalizedAssetSyncObjectStore) Option {
	return func(s *Service) {
		s.finalizedSyncRepo = repository
		s.finalizedSyncStore = store
	}
}

func (s *Service) SetStorageStreamOpener(opener baseservice.StorageStreamOpener) {
	s.streamOpener = opener
}

func (s *Service) SetExternalAssetService(externalSvc *externalassets.Service) {
	s.externalSvc = externalSvc
}

func (s *Service) Search(ctx context.Context, query domain.AssetSearchQuery) (*SearchResult, *domain.AppError) {
	query = query.Normalized()
	if query.Page*query.Size > 10000 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset search pagination window exceeds 10000", nil)
	}
	if assetSearchHasSystemOnlyFilters(query) && query.Source == domain.AssetResourceSourceExternal {
		return &SearchResult{Items: []*AssetDetail{}, Total: 0, Page: query.Page, Size: query.Size}, nil
	}
	if query.Source == domain.AssetResourceSourceExternal {
		return s.searchExternal(ctx, query)
	}
	includeExternal := query.Source == domain.AssetResourceSourceAll &&
		!assetSearchHasSystemOnlyFilters(query) &&
		s.externalSvc != nil && s.externalSvc.Enabled()
	var rows []*repo.TaskAssetSearchRow
	var total int64
	var systemErr error
	var external []*AssetDetail
	var externalTotal int64
	var externalAppErr *domain.AppError
	if includeExternal {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			rows, total, systemErr = s.searchSystemRows(ctx, query)
		}()
		go func() {
			defer wg.Done()
			external, externalTotal, externalAppErr = s.searchExternalRows(ctx, query)
		}()
		wg.Wait()
	} else {
		rows, total, systemErr = s.searchSystemRows(ctx, query)
	}
	if systemErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, systemErr.Error(), nil)
	}
	if externalAppErr != nil {
		return nil, externalAppErr
	}
	items := make([]*AssetDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.buildAssetDetail(row, nil))
	}
	if includeExternal {
		items = append(items, external...)
		if len(items) > query.Size {
			items = items[:query.Size]
		}
		total += externalTotal
	}
	return &SearchResult{Items: items, Total: total, Page: query.Page, Size: query.Size}, nil
}

type assetSearchRowsRepo interface {
	SearchRows(ctx context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, error)
}

func (s *Service) searchSystemRows(ctx context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	if total, ok := s.getAssetSearchTotalCache(ctx, query); ok {
		if rowsRepo, ok := s.searchRepo.(assetSearchRowsRepo); ok {
			rows, err := rowsRepo.SearchRows(ctx, query)
			return rows, total, err
		}
	}
	rows, total, err := s.searchSystemRowsSingleflight(ctx, query)
	if err == nil {
		s.setAssetSearchTotalCache(ctx, query, total)
	}
	return rows, total, err
}

func (s *Service) searchSystemRowsSingleflight(ctx context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	key := assetSearchTotalCacheKey(query) + ":" + strconv.Itoa(query.Page) + ":" + strconv.Itoa(query.Size)
	s.flightMu.Lock()
	if s.searchFlights == nil {
		s.searchFlights = make(map[string]*assetSearchFlight)
	}
	if existing := s.searchFlights[key]; existing != nil {
		s.flightMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-existing.done:
			return existing.rows, existing.total, existing.err
		}
	}
	flight := &assetSearchFlight{done: make(chan struct{})}
	s.searchFlights[key] = flight
	s.flightMu.Unlock()

	flight.rows, flight.total, flight.err = s.searchRepo.Search(ctx, query)
	s.flightMu.Lock()
	delete(s.searchFlights, key)
	close(flight.done)
	s.flightMu.Unlock()
	return flight.rows, flight.total, flight.err
}

func (s *Service) getAssetSearchTotalCache(ctx context.Context, query domain.AssetSearchQuery) (int64, bool) {
	if s == nil || s.cache == nil {
		return 0, false
	}
	raw, err := s.cache.Get(ctx, assetSearchTotalCacheKey(query)).Result()
	if err != nil {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || total < 0 {
		return 0, false
	}
	return total, true
}

func (s *Service) setAssetSearchTotalCache(ctx context.Context, query domain.AssetSearchQuery, total int64) {
	if s == nil || s.cache == nil || total < 0 {
		return
	}
	_ = s.cache.Set(ctx, assetSearchTotalCacheKey(query), strconv.FormatInt(total, 10), assetSearchTotalCacheTTL).Err()
}

func assetSearchTotalCacheKey(query domain.AssetSearchQuery) string {
	query = query.Normalized()
	source := query.Source
	if source == domain.AssetResourceSourceAll {
		source = domain.AssetResourceSourceSystem
	}
	parts := []string{
		strings.TrimSpace(query.AccessScopeKey),
		strings.TrimSpace(query.Keyword),
		strings.TrimSpace(query.ModuleKey),
		strings.TrimSpace(query.OwnerTeamCode),
		assetSearchTimeKey(query.CreatedFrom),
		assetSearchTimeKey(query.CreatedTo),
		string(query.TimeBasis),
		string(query.IsArchived),
		string(query.TaskStatus),
		string(source),
		string(query.UsableState),
		string(query.FormatCategory),
		string(query.BusinessLane),
		string(query.AssetType.Canonical()),
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "\x00")))
	return "omp:perf:asset-center:search-total:v1:" + hex.EncodeToString(sum[:])
}

func assetSearchTimeKey(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
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
	if query.BusinessLane.Valid() {
		includeExternal = false
	}

	if query.Path == "" {
		if includeSystem {
			total, appErr := s.countSystemMaterials(ctx, query.FormatCategory, query.BusinessLane)
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
			folders, err := s.externalSvc.ListDirectoryChildren(ctx, "", 2000, query.FormatCategory)
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
		search, appErr := s.Search(ctx, materialSystemSearchQuery(query.Page, query.Size, query.FormatCategory, query.BusinessLane))
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
		folders, err := s.externalSvc.ListDirectoryChildren(ctx, query.Path, 2000, query.FormatCategory)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		files, total, err := s.externalSvc.ListDirectoryFiles(ctx, query.Path, query.Page, query.Size, query.FormatCategory)
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
	switch query.FormatCategory {
	case domain.AssetFormatCategoryImage,
		domain.AssetFormatCategoryDesign,
		domain.AssetFormatCategoryPDF,
		domain.AssetFormatCategoryVideo,
		domain.AssetFormatCategoryArchive:
	default:
		query.FormatCategory = domain.AssetFormatCategoryAll
	}
	if !query.BusinessLane.Valid() {
		query.BusinessLane = ""
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

func materialSystemSearchQuery(page, size int, formatCategory domain.AssetFormatCategoryFilter, businessLane domain.TaskBusinessLane) domain.AssetSearchQuery {
	return domain.AssetSearchQuery{
		Page:           page,
		Size:           size,
		Source:         domain.AssetResourceSourceSystem,
		UsableState:    domain.AssetUsableStateFilterAll,
		FormatCategory: formatCategory,
		BusinessLane:   businessLane,
		IsArchived:     domain.AssetArchiveFilterFalse,
		TaskStatus:     domain.AssetTaskStatusFilterAll,
	}
}

func (s *Service) countSystemMaterials(ctx context.Context, formatCategory domain.AssetFormatCategoryFilter, businessLane domain.TaskBusinessLane) (int64, *domain.AppError) {
	search, appErr := s.Search(ctx, materialSystemSearchQuery(1, 1, formatCategory, businessLane))
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
		Keyword:                query.Keyword,
		CreatedFrom:            query.CreatedFrom,
		CreatedTo:              query.CreatedTo,
		FormatCategory:         query.FormatCategory,
		OperationalVisibleOnly: query.OperationalVisibleOnly,
		IncludeOSSArchive:      query.IncludeExternalOSSArchive,
		Page:                   query.Page,
		Size:                   query.Size,
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
	previewURL := ""
	if s != nil && s.externalSvc != nil {
		if url := s.externalSvc.BrowserPreviewURL(row); url != "" {
			previewURL = url
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
		PreviewURL:            stringPtrIfNotEmpty(previewURL),
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
