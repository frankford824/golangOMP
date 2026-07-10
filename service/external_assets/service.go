package externalassets

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"workflow/config"
	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

const ErrCodeExternalAssetPreparing = "EXTERNAL_ASSET_PREPARING"

type MountConfig struct {
	Path   string
	Kind   domain.ExternalAssetKind
	Driver string
}

type Config struct {
	Enabled             bool
	BFFBaseURL          string
	BFFBrowserBaseURL   string
	AListBaseURL        string
	AListToken          string
	Mounts              []MountConfig
	AListTimeout        time.Duration
	SyncInterval        time.Duration
	LinkRefreshInterval time.Duration
	FullSyncEnabled     bool
	FullSyncInterval    time.Duration
	FullSyncMounts      []string
	FullSyncRoots       []string
	FullSyncPageSize    int
	FullSyncMaxDepth    int
	FullSyncMaxFiles    int
	FullSyncMaxDirs     int
	OSSOriginalPrefix   string
	OSSPreviewPrefix    string
	OSSRequiredPrefixes []string
	LocalPathMappings   map[string]string
	PrepareInterval     time.Duration
	PrepareLimit        int
	PrepareConcurrency  int
	PrepareMounts       []string
}

type FullSyncResult struct {
	Mounts        []MountSyncResult `json:"mounts"`
	ScannedCount  int               `json:"scanned_count"`
	UpsertedCount int               `json:"upserted_count"`
}

type MountSyncResult struct {
	MountPath     string   `json:"mount_path"`
	RootPaths     []string `json:"root_paths,omitempty"`
	Status        string   `json:"status"`
	ScannedCount  int      `json:"scanned_count"`
	UpsertedCount int      `json:"upserted_count"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type fullSyncMountPlan struct {
	mount      MountConfig
	roots      []string
	rootScoped bool
}

type directoryBrowserRepo interface {
	ListDirectoryChildren(ctx context.Context, parentPath string, mountPaths []string, limit int, formatCategory domain.AssetFormatCategoryFilter) ([]domain.ExternalAssetDirectoryEntry, error)
	ListDirectoryFiles(ctx context.Context, parentPath string, mountPaths []string, page int, size int, formatCategory domain.AssetFormatCategoryFilter) ([]*domain.ExternalAssetRecord, int64, error)
}

type Service struct {
	cfg       Config
	repo      repo.ExternalAssetRepo
	bff       *BFFClient
	alist     *AListClient
	ossDirect *baseservice.OSSDirectService
	renderer  baseservice.AssetPreviewRenderer
	nowFn     func() time.Time
	recentMu  sync.Mutex
	recent    map[string]time.Time

	keywordRefreshMu       sync.Mutex
	keywordRefreshLast     map[string]time.Time
	keywordRefreshActive   map[string]struct{}
	keywordRefreshCooldown time.Duration
	keywordRefreshTimeout  time.Duration
	keywordRefreshAsyncFn  func(func())
	previewPrepareAsyncFn  func(func())
}

func ConfigFromApp(cfg config.ExternalAssetsConfig) Config {
	return Config{
		Enabled:             cfg.Enabled,
		BFFBaseURL:          strings.TrimSpace(cfg.BFFBaseURL),
		BFFBrowserBaseURL:   strings.TrimSpace(cfg.BFFBrowserBaseURL),
		AListBaseURL:        strings.TrimSpace(cfg.AListBaseURL),
		AListToken:          strings.TrimSpace(cfg.AListToken),
		Mounts:              ParseMounts(cfg.AListMounts),
		AListTimeout:        cfg.AListTimeout,
		SyncInterval:        cfg.SyncInterval,
		LinkRefreshInterval: cfg.LinkRefreshInterval,
		FullSyncEnabled:     cfg.FullSyncEnabled,
		FullSyncInterval:    cfg.FullSyncInterval,
		FullSyncMounts:      ParseMountPaths(cfg.FullSyncMounts),
		FullSyncRoots:       ParseMountPaths(cfg.FullSyncRoots),
		FullSyncPageSize:    cfg.FullSyncPageSize,
		FullSyncMaxDepth:    cfg.FullSyncMaxDepth,
		FullSyncMaxFiles:    cfg.FullSyncMaxFiles,
		FullSyncMaxDirs:     cfg.FullSyncMaxDirs,
		OSSOriginalPrefix:   strings.Trim(strings.TrimSpace(cfg.OSSOriginalPrefix), "/"),
		OSSPreviewPrefix:    strings.Trim(strings.TrimSpace(cfg.OSSPreviewPrefix), "/"),
		OSSRequiredPrefixes: ParseOSSPrefixes(cfg.OSSRequiredPrefixes),
		LocalPathMappings:   ParseLocalPathMappings(cfg.LocalPathMappings),
		PrepareInterval:     cfg.PrepareInterval,
		PrepareLimit:        cfg.PrepareLimit,
		PrepareConcurrency:  cfg.PrepareConcurrency,
		PrepareMounts:       ParseMountPaths(cfg.PrepareMounts),
	}
}

func NewService(repo repo.ExternalAssetRepo, cfg Config, ossDirect *baseservice.OSSDirectService) *Service {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = time.Hour
	}
	if cfg.LinkRefreshInterval <= 0 {
		cfg.LinkRefreshInterval = time.Hour
	}
	if cfg.OSSOriginalPrefix == "" {
		cfg.OSSOriginalPrefix = "external-assets/alist/original"
	}
	if cfg.OSSPreviewPrefix == "" {
		cfg.OSSPreviewPrefix = "external-assets/alist/preview"
	}
	if len(cfg.Mounts) == 0 {
		cfg.Mounts = ParseMounts("/quark:netdisk,/p3:nas_local")
	}
	if cfg.FullSyncPageSize <= 0 || cfg.FullSyncPageSize > 200 {
		cfg.FullSyncPageSize = 100
	}
	if cfg.AListTimeout <= 0 {
		cfg.AListTimeout = 30 * time.Second
	}
	if cfg.FullSyncMaxDepth <= 0 {
		cfg.FullSyncMaxDepth = 16
	}
	if cfg.FullSyncMaxFiles < 0 {
		cfg.FullSyncMaxFiles = 0
	}
	if cfg.FullSyncMaxDirs < 0 {
		cfg.FullSyncMaxDirs = 0
	}
	if cfg.PrepareInterval <= 0 {
		cfg.PrepareInterval = 30 * time.Second
	}
	if cfg.PrepareLimit <= 0 || cfg.PrepareLimit > 200 {
		cfg.PrepareLimit = 50
	}
	if cfg.PrepareConcurrency <= 0 || cfg.PrepareConcurrency > 16 {
		cfg.PrepareConcurrency = 4
	}
	return &Service{
		cfg:       cfg,
		repo:      repo,
		bff:       NewBFFClient(cfg.BFFBaseURL, cfg.BFFBrowserBaseURL, 30*time.Second),
		alist:     NewAListClient(cfg.AListBaseURL, cfg.AListToken, cfg.AListTimeout),
		ossDirect: ossDirect,
		renderer:  baseservice.NewExternalAssetPreviewRenderer(),
		nowFn:     time.Now,
		recent:    map[string]time.Time{},

		keywordRefreshLast:     map[string]time.Time{},
		keywordRefreshActive:   map[string]struct{}{},
		keywordRefreshCooldown: 10 * time.Minute,
		keywordRefreshTimeout:  2 * time.Minute,
		keywordRefreshAsyncFn: func(fn func()) {
			go fn()
		},
		previewPrepareAsyncFn: func(fn func()) {
			go fn()
		},
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil
}

func (s *Service) SyncInterval() time.Duration {
	if s == nil || s.cfg.SyncInterval <= 0 {
		return time.Hour
	}
	return s.cfg.SyncInterval
}

func (s *Service) FullSyncInterval() time.Duration {
	if s == nil {
		return time.Hour
	}
	if s.cfg.FullSyncInterval > 0 {
		return s.cfg.FullSyncInterval
	}
	return s.SyncInterval()
}

func (s *Service) PrepareInterval() time.Duration {
	if s == nil || s.cfg.PrepareInterval <= 0 {
		return 30 * time.Second
	}
	return s.cfg.PrepareInterval
}

func (s *Service) PrepareLimit() int {
	if s == nil || s.cfg.PrepareLimit <= 0 {
		return 50
	}
	return s.cfg.PrepareLimit
}

func (s *Service) ConfiguredMountPaths() []string {
	if s == nil {
		return nil
	}
	mounts := make([]string, 0, len(s.cfg.Mounts))
	for _, mount := range s.cfg.Mounts {
		if cleaned := cleanAListPath(mount.Path); cleaned != "" {
			mounts = append(mounts, cleaned)
		}
	}
	return ParseMountPaths(strings.Join(mounts, ","))
}

func (s *Service) PrepareMountPaths() []string {
	configured := s.ConfiguredMountPaths()
	if s == nil || len(s.cfg.PrepareMounts) == 0 {
		return configured
	}
	allowed := make(map[string]struct{}, len(configured))
	for _, mount := range configured {
		allowed[mount] = struct{}{}
	}
	selected := make([]string, 0, len(s.cfg.PrepareMounts))
	for _, raw := range s.cfg.PrepareMounts {
		mount := cleanAListPath(raw)
		if _, ok := allowed[mount]; ok {
			selected = append(selected, mount)
		}
	}
	return ParseMountPaths(strings.Join(selected, ","))
}

func (s *Service) FullSyncReady() bool {
	return s != nil && s.Enabled() && s.cfg.FullSyncEnabled && s.alist != nil && s.alist.Enabled()
}

func (s *Service) LegacyIndexRefreshReady() bool {
	return s != nil && s.Enabled() && !s.bffSourceReady() && s.searchBackendReady()
}

func ParseMounts(raw string) []MountConfig {
	var out []MountConfig
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		mount := cleanAListPath(fields[0])
		kind := domain.ExternalAssetKindNetdisk
		if len(fields) > 1 && strings.EqualFold(strings.TrimSpace(fields[1]), string(domain.ExternalAssetKindNASLocal)) {
			kind = domain.ExternalAssetKindNASLocal
		}
		driver := ""
		if len(fields) > 2 {
			driver = strings.TrimSpace(fields[2])
		}
		out = append(out, MountConfig{Path: mount, Kind: kind, Driver: driver})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func ParseMountPaths(raw string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		mount := cleanAListPath(part)
		if mount == "" {
			continue
		}
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}
		out = append(out, mount)
	}
	return out
}

func ParseOSSPrefixes(raw string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		prefix := cleanAListPath(part)
		if prefix == "/" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	return out
}

func ParseLocalPathMappings(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = cleanAListPath(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func normalizeRecentKeyword(keyword string) string {
	keyword = strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
	if keyword == "" {
		return ""
	}
	runes := []rune(keyword)
	if len(runes) > 80 {
		keyword = string(runes[:80])
	}
	return keyword
}

func (s *Service) Search(ctx context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	if !s.Enabled() {
		return []*domain.ExternalAssetRecord{}, 0, nil
	}
	query = query.Normalized()
	if query.Keyword != "" && s.searchBackendReady() {
		s.recordRecentKeyword(query.Keyword)
		if shouldScheduleKeywordRefresh(query.Keyword) {
			s.scheduleKeywordRefresh(query.Keyword, 50)
		}
	}
	rows, total, err := s.repo.Search(ctx, query)
	if err == nil {
		s.schedulePreviewPrepare(rows)
	}
	return rows, total, err
}

func (s *Service) ListDirectoryChildren(ctx context.Context, parentPath string, limit int, formatCategory domain.AssetFormatCategoryFilter) ([]domain.ExternalAssetDirectoryEntry, error) {
	if !s.Enabled() {
		return []domain.ExternalAssetDirectoryEntry{}, nil
	}
	browser, ok := s.repo.(directoryBrowserRepo)
	if !ok {
		return []domain.ExternalAssetDirectoryEntry{}, nil
	}
	parentPath = cleanExternalMaterialBrowsePath(parentPath)
	mounts := s.mountPathsForBrowse(parentPath)
	if parentPath != "" && len(mounts) == 0 {
		return []domain.ExternalAssetDirectoryEntry{}, nil
	}
	return browser.ListDirectoryChildren(ctx, parentPath, mounts, limit, formatCategory)
}

func (s *Service) ListDirectoryFiles(ctx context.Context, parentPath string, page int, size int, formatCategory domain.AssetFormatCategoryFilter) ([]*domain.ExternalAssetRecord, int64, error) {
	if !s.Enabled() {
		return []*domain.ExternalAssetRecord{}, 0, nil
	}
	browser, ok := s.repo.(directoryBrowserRepo)
	if !ok {
		return []*domain.ExternalAssetRecord{}, 0, nil
	}
	parentPath = cleanExternalMaterialBrowsePath(parentPath)
	mounts := s.mountPathsForBrowse(parentPath)
	if parentPath == "" || len(mounts) == 0 {
		return []*domain.ExternalAssetRecord{}, 0, nil
	}
	rows, total, err := browser.ListDirectoryFiles(ctx, parentPath, mounts, page, size, formatCategory)
	if err == nil {
		s.schedulePreviewPrepare(rows)
	}
	return rows, total, err
}

func (s *Service) mountPathsForBrowse(parentPath string) []string {
	if s == nil {
		return nil
	}
	parentPath = cleanExternalMaterialBrowsePath(parentPath)
	out := []string{}
	for _, mount := range s.cfg.Mounts {
		mountPath := cleanExternalMaterialBrowsePath(mount.Path)
		if mountPath == "" {
			continue
		}
		if parentPath == "" || parentPath == mountPath || strings.HasPrefix(parentPath, mountPath+"/") {
			out = append(out, mountPath)
		}
	}
	return out
}

func cleanExternalMaterialBrowsePath(value string) string {
	cleaned := cleanAListPath(value)
	if cleaned == "/" {
		return ""
	}
	return cleaned
}

func (s *Service) refreshSearchCache(ctx context.Context, query domain.ExternalAssetSearchQuery) error {
	if !s.Enabled() || !s.searchBackendReady() {
		return nil
	}
	query = query.Normalized()
	if query.Keyword == "" {
		return nil
	}
	limit := 200
	var firstErr error
	for _, mount := range s.mountsForQuery(query) {
		resp, err := s.searchMount(ctx, mount.Path, query.Keyword, limit)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, item := range resp.Content {
			if isSkippableExternalSearchItem(item) {
				continue
			}
			upsert := s.upsertFromSearchItem(mount, item)
			if _, err := s.repo.Upsert(ctx, upsert); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Service) mountsForQuery(query domain.ExternalAssetSearchQuery) []MountConfig {
	if s == nil {
		return nil
	}
	query = query.Normalized()
	mounts := make([]MountConfig, 0, len(s.cfg.Mounts))
	for _, mount := range s.cfg.Mounts {
		if query.MountPath != "" && mount.Path != query.MountPath {
			continue
		}
		if query.Kind != "" && mount.Kind != query.Kind {
			continue
		}
		mounts = append(mounts, mount)
	}
	return mounts
}

func shouldScheduleKeywordRefresh(keyword string) bool {
	keyword = normalizeRecentKeyword(keyword)
	if keyword == "" {
		return false
	}
	runes := []rune(keyword)
	if len(runes) >= 3 {
		return true
	}
	if len(runes) < 2 {
		return false
	}
	for _, r := range runes {
		if r > 127 {
			return true
		}
	}
	return false
}

func (s *Service) scheduleKeywordRefresh(keyword string, perMountLimit int) {
	if !s.Enabled() || !s.searchBackendReady() {
		return
	}
	keyword = normalizeRecentKeyword(keyword)
	if keyword == "" {
		return
	}
	if perMountLimit <= 0 {
		perMountLimit = 50
	}
	if perMountLimit > 200 {
		perMountLimit = 200
	}
	now := s.nowFn().UTC()
	cooldown := s.keywordRefreshCooldown
	if cooldown <= 0 {
		cooldown = 10 * time.Minute
	}
	timeout := s.keywordRefreshTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	s.keywordRefreshMu.Lock()
	if s.keywordRefreshLast == nil {
		s.keywordRefreshLast = map[string]time.Time{}
	}
	if s.keywordRefreshActive == nil {
		s.keywordRefreshActive = map[string]struct{}{}
	}
	if _, active := s.keywordRefreshActive[keyword]; active {
		s.keywordRefreshMu.Unlock()
		return
	}
	if last, ok := s.keywordRefreshLast[keyword]; ok && now.Sub(last) < cooldown {
		s.keywordRefreshMu.Unlock()
		return
	}
	s.keywordRefreshActive[keyword] = struct{}{}
	s.keywordRefreshLast[keyword] = now
	runAsync := s.keywordRefreshAsyncFn
	if runAsync == nil {
		runAsync = func(fn func()) { go fn() }
	}
	s.keywordRefreshMu.Unlock()

	runAsync(func() {
		defer func() {
			s.keywordRefreshMu.Lock()
			delete(s.keywordRefreshActive, keyword)
			s.keywordRefreshMu.Unlock()
		}()
		refreshCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_ = s.SyncKeyword(refreshCtx, keyword, perMountLimit)
		if s.directLinkBackendReady() {
			_, _, _ = s.RefreshDirectURLs(refreshCtx, perMountLimit)
		}
	})
}

func (s *Service) schedulePreviewPrepare(rows []*domain.ExternalAssetRecord) {
	if s == nil || s.repo == nil || len(rows) == 0 {
		return
	}
	ids := make([]int64, 0, len(rows))
	seen := map[int64]struct{}{}
	for _, row := range rows {
		if !shouldPrepareExternalDerivedPreview(row) {
			continue
		}
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		ids = append(ids, row.ID)
		if len(ids) >= 20 {
			break
		}
	}
	if len(ids) == 0 {
		return
	}
	runAsync := s.previewPrepareAsyncFn
	if runAsync == nil {
		runAsync = func(fn func()) { go fn() }
	}
	runAsync(func() {
		prepareCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range ids {
			_ = s.repo.MarkPreviewPreparePending(prepareCtx, id)
		}
	})
}

func shouldPrepareExternalDerivedPreview(row *domain.ExternalAssetRecord) bool {
	if row == nil || row.ID <= 0 || row.IsDir || row.Status == domain.ExternalAssetStatusMissing {
		return false
	}
	if row.OSSPreviewKey != "" && row.PreviewStatus == domain.ExternalAssetPreviewStatusReady {
		return false
	}
	if row.PreviewStatus == domain.ExternalAssetPreviewStatusPending {
		return false
	}
	return canRenderDerivedPreview(row.FileName, row.MimeType)
}

func (s *Service) recordRecentKeyword(keyword string) {
	if s == nil {
		return
	}
	keyword = normalizeRecentKeyword(keyword)
	if keyword == "" {
		return
	}
	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	if s.recent == nil {
		s.recent = map[string]time.Time{}
	}
	s.recent[keyword] = s.nowFn().UTC()
	if len(s.recent) <= 200 {
		return
	}
	type pair struct {
		keyword string
		seenAt  time.Time
	}
	items := make([]pair, 0, len(s.recent))
	for k, v := range s.recent {
		items = append(items, pair{keyword: k, seenAt: v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].seenAt.After(items[j].seenAt) })
	for _, item := range items[160:] {
		delete(s.recent, item.keyword)
	}
}

func (s *Service) RecentKeywords(limit int) []string {
	if s == nil {
		return nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	type pair struct {
		keyword string
		seenAt  time.Time
	}
	s.recentMu.Lock()
	items := make([]pair, 0, len(s.recent))
	for k, v := range s.recent {
		items = append(items, pair{keyword: k, seenAt: v})
	}
	s.recentMu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].seenAt.After(items[j].seenAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.keyword)
	}
	return out
}

func (s *Service) SearchGlobal(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, _, err := s.Search(ctx, domain.ExternalAssetSearchQuery{Keyword: q, Page: 1, Size: limit})
	if err != nil {
		return nil, err
	}
	out := make([]domain.SearchAsset, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SearchAsset{
			AssetID:           0,
			ResourceID:        row.ResourceID,
			FileName:          row.FileName,
			TaskID:            nil,
			SourceType:        string(domain.AssetResourceSourceExternal),
			SourceLabel:       "外部资源",
			ExternalKind:      string(row.Kind),
			ExternalMountPath: row.MountPath,
			ExternalDriver:    row.Driver,
			UsableState:       string(domain.TaskAssetUsableStateNotApplicable),
			UsableLabel:       "外部资源",
		})
	}
	return out, nil
}

func (s *Service) SyncKeyword(ctx context.Context, keyword string, perMountLimit int) error {
	if !s.Enabled() || !s.searchBackendReady() {
		return nil
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil
	}
	if perMountLimit <= 0 {
		perMountLimit = 50
	}
	if perMountLimit > 200 {
		perMountLimit = 200
	}
	var firstErr error
	for _, mount := range s.cfg.Mounts {
		runID := s.startSyncRun(ctx, domain.ExternalAssetSyncRunTypeKeyword, mount.Path, keyword)
		scanned := 0
		upserted := 0
		var mountErr error
		resp, err := s.searchMount(ctx, mount.Path, keyword, perMountLimit)
		if err != nil {
			s.finishSyncRun(ctx, runID, domain.ExternalAssetSyncRunStatusFailed, scanned, upserted, err.Error())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, item := range resp.Content {
			if isSkippableExternalSearchItem(item) {
				continue
			}
			scanned++
			upsert := s.upsertFromSearchItem(mount, item)
			available, err := s.keywordSearchItemAvailable(ctx, mount, upsert)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				if mountErr == nil {
					mountErr = err
				}
				continue
			}
			if !available {
				if err := s.repo.MarkOriginPathMissing(ctx, upsert.Provider, upsert.MountPath, upsert.OriginPath); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					if mountErr == nil {
						mountErr = err
					}
				}
				continue
			}
			if _, err := s.repo.Upsert(ctx, upsert); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				if mountErr == nil {
					mountErr = err
				}
				continue
			}
			upserted++
		}
		status := domain.ExternalAssetSyncRunStatusCompleted
		errMessage := ""
		if mountErr != nil {
			status = domain.ExternalAssetSyncRunStatusPartial
			errMessage = mountErr.Error()
		}
		s.finishSyncRun(ctx, runID, status, scanned, upserted, errMessage)
	}
	return firstErr
}

func (s *Service) keywordSearchItemAvailable(ctx context.Context, mount MountConfig, item domain.ExternalAssetUpsert) (bool, error) {
	if item.IsDir || mount.Kind != domain.ExternalAssetKindNASLocal {
		return true, nil
	}
	if base := strings.TrimSpace(s.cfg.LocalPathMappings[item.MountPath]); base != "" {
		if _, err := os.Stat(base); err == nil {
			rel := strings.TrimLeft(strings.TrimPrefix(item.OriginPath, item.MountPath), "/")
			localPath := filepath.Join(base, filepath.FromSlash(rel))
			if _, err := os.Stat(localPath); err == nil {
				return true, nil
			} else if os.IsNotExist(err) {
				return false, nil
			} else {
				return false, fmt.Errorf("verify external asset local path %s: %w", item.OriginPath, err)
			}
		} else if err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("verify external asset local mount %s: %w", item.MountPath, err)
		}
	}
	if s.bff != nil && s.bff.Enabled() {
		ok, err := s.bff.FetchAvailable(ctx, item.OriginPath)
		if err != nil {
			return false, fmt.Errorf("verify external asset bff path %s: %w", item.OriginPath, err)
		}
		return ok, nil
	}
	return false, fmt.Errorf("nas local keyword result cannot be verified: %s", item.OriginPath)
}

func (s *Service) SyncFullIndex(ctx context.Context) (*FullSyncResult, error) {
	result := &FullSyncResult{}
	if !s.Enabled() {
		return result, nil
	}
	if !s.cfg.FullSyncEnabled {
		return result, nil
	}
	if s.alist == nil || !s.alist.Enabled() {
		return result, fmt.Errorf("alist client is required for full external asset sync")
	}
	plans := s.fullSyncMountPlans()
	if len(plans) == 0 {
		return result, fmt.Errorf("no configured external asset full sync mounts matched filter")
	}
	var firstErr error
	for _, plan := range plans {
		mountResult, err := s.syncFullMount(ctx, plan)
		result.Mounts = append(result.Mounts, mountResult)
		result.ScannedCount += mountResult.ScannedCount
		result.UpsertedCount += mountResult.UpsertedCount
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return result, firstErr
}

func (s *Service) fullSyncMountPlans() []fullSyncMountPlan {
	if s == nil {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, mount := range s.cfg.FullSyncMounts {
		mount = cleanAListPath(mount)
		if mount != "" {
			allowed[mount] = struct{}{}
		}
	}
	rootsByMount := map[string][]string{}
	for _, rawRoot := range s.cfg.FullSyncRoots {
		root := cleanAListPath(rawRoot)
		mount, ok := matchFullSyncRootMount(root, s.cfg.Mounts)
		if !ok {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[mount.Path]; !ok {
				continue
			}
		}
		rootsByMount[mount.Path] = append(rootsByMount[mount.Path], root)
	}
	out := make([]fullSyncMountPlan, 0, len(s.cfg.Mounts))
	for _, mount := range s.cfg.Mounts {
		if len(allowed) > 0 {
			if _, ok := allowed[mount.Path]; !ok {
				continue
			}
		}
		roots := dedupeFullSyncRoots(rootsByMount[mount.Path])
		if len(s.cfg.FullSyncRoots) > 0 && len(allowed) == 0 && len(roots) == 0 {
			continue
		}
		if len(roots) == 0 {
			roots = []string{mount.Path}
		}
		out = append(out, fullSyncMountPlan{
			mount:      mount,
			roots:      roots,
			rootScoped: !(len(roots) == 1 && roots[0] == mount.Path),
		})
	}
	return out
}

func matchFullSyncRootMount(root string, mounts []MountConfig) (MountConfig, bool) {
	root = cleanAListPath(root)
	var best MountConfig
	bestLen := -1
	for _, mount := range mounts {
		if root == mount.Path || strings.HasPrefix(root, mount.Path+"/") {
			if len(mount.Path) > bestLen {
				best = mount
				bestLen = len(mount.Path)
			}
		}
	}
	return best, bestLen >= 0
}

func dedupeFullSyncRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		root = cleanAListPath(root)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	sort.Strings(out)
	return out
}

type fullSyncQueueItem struct {
	path  string
	depth int
}

func (s *Service) syncFullMount(ctx context.Context, plan fullSyncMountPlan) (MountSyncResult, error) {
	mount := plan.mount
	startedAt := s.nowFn().UTC().Truncate(time.Second)
	runID := s.startSyncRun(ctx, domain.ExternalAssetSyncRunTypeFull, mount.Path, "")
	result := MountSyncResult{
		MountPath: mount.Path,
		RootPaths: append([]string(nil), plan.roots...),
		Status:    domain.ExternalAssetSyncRunStatusCompleted,
	}
	queue := make([]fullSyncQueueItem, 0, len(plan.roots))
	seenDirs := map[string]struct{}{}
	for _, root := range plan.roots {
		root = cleanAListPath(root)
		if root == "" || (root != mount.Path && !strings.HasPrefix(root, mount.Path+"/")) {
			continue
		}
		if _, ok := seenDirs[root]; ok {
			continue
		}
		seenDirs[root] = struct{}{}
		queue = append(queue, fullSyncQueueItem{path: root, depth: 0})
	}
	if len(queue) == 0 {
		result.Status = domain.ExternalAssetSyncRunStatusFailed
		result.ErrorMessage = fmt.Sprintf("no valid full sync roots for %s", mount.Path)
		s.finishSyncRun(ctx, runID, result.Status, result.ScannedCount, result.UpsertedCount, result.ErrorMessage)
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}
	dirsRead := 0
	limited := false
	var firstErr error
	var partialErr error
	skippedDirs := 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			firstErr = err
			break
		}
		next := queue[0]
		queue = queue[1:]
		if s.cfg.FullSyncMaxDirs > 0 && dirsRead >= s.cfg.FullSyncMaxDirs {
			limited = true
			firstErr = fmt.Errorf("external asset full sync dir limit reached for %s", mount.Path)
			break
		}
		dirsRead++
		page := 1
		for {
			resp, err := s.listFullSyncPage(ctx, next.path, page)
			if err != nil {
				if next.path != mount.Path {
					skippedDirs++
					if partialErr == nil {
						partialErr = fmt.Errorf("external asset full sync skipped dir %s: %w", next.path, err)
					}
					break
				}
				firstErr = err
				break
			}
			if len(resp.Content) == 0 {
				break
			}
			for _, item := range resp.Content {
				if isSkippableExternalSearchItem(item) {
					continue
				}
				childPath := joinAListPath(item.Parent, item.Name)
				if item.IsDir {
					if next.depth+1 <= s.cfg.FullSyncMaxDepth {
						if _, ok := seenDirs[childPath]; !ok {
							seenDirs[childPath] = struct{}{}
							queue = append(queue, fullSyncQueueItem{path: childPath, depth: next.depth + 1})
						}
					}
					continue
				}
				result.ScannedCount++
				if s.cfg.FullSyncMaxFiles > 0 && result.ScannedCount > s.cfg.FullSyncMaxFiles {
					limited = true
					firstErr = fmt.Errorf("external asset full sync file limit reached for %s", mount.Path)
					break
				}
				upsert := s.upsertFromSearchItem(mount, item)
				upsert.ScannedAt = startedAt
				if _, err := s.repo.Upsert(ctx, upsert); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
				result.UpsertedCount++
			}
			if firstErr != nil {
				break
			}
			if resp.Total > 0 && int64(page*s.cfg.FullSyncPageSize) >= resp.Total {
				break
			}
			if len(resp.Content) < s.cfg.FullSyncPageSize {
				break
			}
			page++
		}
		if firstErr != nil {
			break
		}
	}
	if firstErr != nil {
		result.Status = domain.ExternalAssetSyncRunStatusFailed
		if limited {
			result.Status = domain.ExternalAssetSyncRunStatusPartial
		}
		result.ErrorMessage = firstErr.Error()
	} else if partialErr != nil {
		result.Status = domain.ExternalAssetSyncRunStatusPartial
		result.ErrorMessage = partialErr.Error()
		if skippedDirs > 1 {
			result.ErrorMessage = fmt.Sprintf("%s; skipped_dirs=%d", result.ErrorMessage, skippedDirs)
		}
	} else if plan.rootScoped {
		prefixes := make([]repo.ExternalAssetOriginPrefix, 0, len(plan.roots))
		for _, root := range plan.roots {
			prefixes = append(prefixes, repo.ExternalAssetOriginPrefix{MountPath: mount.Path, OriginPath: root})
		}
		if err := s.repo.MarkOriginPrefixesMissingBefore(ctx, prefixes, startedAt); err != nil {
			result.Status = domain.ExternalAssetSyncRunStatusPartial
			result.ErrorMessage = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
	} else if err := s.repo.MarkMountMissingBefore(ctx, mount.Path, startedAt); err != nil {
		result.Status = domain.ExternalAssetSyncRunStatusPartial
		result.ErrorMessage = err.Error()
		if firstErr == nil {
			firstErr = err
		}
	}
	s.finishSyncRun(ctx, runID, result.Status, result.ScannedCount, result.UpsertedCount, result.ErrorMessage)
	return result, firstErr
}

func (s *Service) listFullSyncPage(ctx context.Context, listPath string, page int) (*AListListResponse, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := s.alist.List(ctx, listPath, page, s.cfg.FullSyncPageSize)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt == maxAttempts || ctx.Err() != nil || !isRetryableAListListError(err) {
			break
		}
		delay := time.Duration(attempt) * 500 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isRetryableAListListError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "server closed idle connection")
}

func (s *Service) startSyncRun(ctx context.Context, runType, mountPath, keyword string) int64 {
	if s == nil || s.repo == nil {
		return 0
	}
	id, err := s.repo.CreateSyncRun(ctx, &domain.ExternalAssetSyncRun{
		RunType:   runType,
		MountPath: mountPath,
		Keyword:   keyword,
		Status:    domain.ExternalAssetSyncRunStatusRunning,
		StartedAt: s.nowFn().UTC(),
	})
	if err != nil {
		return 0
	}
	return id
}

func (s *Service) finishSyncRun(ctx context.Context, id int64, status string, scannedCount, upsertedCount int, errorMessage string) {
	if s == nil || s.repo == nil || id <= 0 {
		return
	}
	_ = s.repo.FinishSyncRun(ctx, id, status, scannedCount, upsertedCount, errorMessage)
}

func (s *Service) searchBackendReady() bool {
	return s != nil && ((s.bff != nil && s.bff.Enabled()) || (s.alist != nil && s.alist.Enabled()))
}

func (s *Service) bffSourceReady() bool {
	return s != nil && s.bff != nil && s.bff.Enabled()
}

func (s *Service) directLinkBackendReady() bool {
	return s != nil && ((s.bff != nil && s.bff.Enabled()) || (s.alist != nil && s.alist.Enabled()))
}

func (s *Service) searchMount(ctx context.Context, mountPath, keyword string, limit int) (*AListSearchResponse, error) {
	if s.bff != nil && s.bff.Enabled() {
		resp, err := s.bff.Search(ctx, mountPath, keyword, 1, limit)
		if err == nil && hasUsableExternalSearchItems(resp) {
			return resp, nil
		}
		if s.alist != nil && s.alist.Enabled() {
			fallback, fallbackErr := s.alist.Search(ctx, mountPath, keyword, 1, limit)
			if fallbackErr == nil {
				return fallback, nil
			}
			if err != nil {
				return nil, fmt.Errorf("external asset bff search failed: %v; alist fallback failed: %w", err, fallbackErr)
			}
		}
		return resp, err
	}
	return s.alist.Search(ctx, mountPath, keyword, 1, limit)
}

func hasUsableExternalSearchItems(resp *AListSearchResponse) bool {
	if resp == nil {
		return false
	}
	for _, item := range resp.Content {
		if !isSkippableExternalSearchItem(item) {
			return true
		}
	}
	return false
}

func (s *Service) resolveNetdiskDirectURL(ctx context.Context, row *domain.ExternalAssetRecord, preview bool) (string, error) {
	if s.bff != nil && s.bff.Enabled() {
		return s.bff.DirectURL(ctx, row.OriginPath, preview)
	}
	info, err := s.alist.Get(ctx, row.OriginPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(info.RawURL), nil
}

func (s *Service) RefreshDirectURLs(ctx context.Context, limit int) (int, int, error) {
	if !s.Enabled() || !s.directLinkBackendReady() {
		return 0, 0, nil
	}
	if s.bffSourceReady() {
		return 0, 0, nil
	}
	staleBefore := s.nowFn().UTC().Add(-s.cfg.LinkRefreshInterval)
	rows, err := s.repo.ListDirectURLRefreshCandidates(ctx, s.ConfiguredMountPaths(), limit, staleBefore)
	if err != nil {
		return 0, 0, err
	}
	success := 0
	failed := 0
	for _, row := range rows {
		if row == nil || row.Kind != domain.ExternalAssetKindNetdisk {
			continue
		}
		rawURL, err := s.resolveNetdiskDirectURL(ctx, row, false)
		if err != nil {
			failed++
			_ = s.repo.UpdateDirectURL(ctx, row.ID, "", nil, "failed")
			continue
		}
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || s.isInternalProviderURL(rawURL) {
			failed++
			_ = s.repo.UpdateDirectURL(ctx, row.ID, "", nil, "missing")
			continue
		}
		success++
		_ = s.repo.UpdateDirectURL(ctx, row.ID, rawURL, nil, "ready")
	}
	return success, failed, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*domain.ExternalAssetRecord, error) {
	if !s.Enabled() {
		return nil, nil
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) DownloadInfo(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if row == nil || row.Status == domain.ExternalAssetStatusMissing {
		return nil, domain.ErrNotFound
	}
	if row.IsDir {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "文件夹暂不支持下载", nil)
	}
	if urlValue := s.BrowserDownloadURL(row); urlValue != "" {
		accessHint := "external_netdisk_direct"
		if row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady {
			accessHint = "external_original_oss"
		}
		return &domain.AssetDownloadInfo{
			DownloadMode:     domain.AssetDownloadModeDirect,
			DownloadURL:      &urlValue,
			AccessHint:       accessHint,
			PreviewAvailable: canDirectBrowserPreview(row.FileName, row.MimeType),
			Filename:         row.FileName,
			FileSize:         row.FileSize,
			MimeType:         row.MimeType,
		}, nil
	}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		return s.localDownloadInfo(ctx, row)
	}
	return s.netdiskDownloadInfo(ctx, row, false)
}

func (s *Service) BatchDownloadInfo(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if row == nil || row.Status == domain.ExternalAssetStatusMissing {
		return nil, domain.ErrNotFound
	}
	if row.IsDir {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "文件夹暂不支持下载", nil)
	}
	if row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady {
		return s.ossDownloadInfo(row, row.OSSOriginalKey, false, "external_batch_oss")
	}
	if err := s.repo.MarkOSSPreparePending(ctx, row.ID); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return prepareInfo(row, "external_batch_prepare_required"), nil
}

func (s *Service) PreviewInfo(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if row == nil || row.Status == domain.ExternalAssetStatusMissing {
		return nil, domain.ErrNotFound
	}
	if row.IsDir {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "文件夹暂不支持预览", nil)
	}
	if urlValue := s.BrowserPreviewURL(row); urlValue != "" {
		return &domain.AssetDownloadInfo{
			DownloadMode:     domain.AssetDownloadModeDirect,
			DownloadURL:      &urlValue,
			AccessHint:       "external_bff_browser_preview",
			PreviewAvailable: true,
			Filename:         row.FileName,
			FileSize:         row.FileSize,
			MimeType:         row.MimeType,
		}, nil
	}
	if canRenderDerivedPreview(row.FileName, row.MimeType) {
		_ = s.repo.MarkPreviewPreparePending(ctx, row.ID)
		return prepareInfo(row, "external_preview_prepare_required"), nil
	}
	return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "该文件暂不支持在线预览，可直接下载原文件", nil)
}

func (s *Service) BrowserPreviewURL(row *domain.ExternalAssetRecord) string {
	if row == nil || row.IsDir {
		return ""
	}
	if row.OSSPreviewKey != "" && row.PreviewStatus == domain.ExternalAssetPreviewStatusReady {
		if urlValue := s.presignedPreviewURL(row); urlValue != "" {
			return urlValue
		}
	}
	if !canDirectBrowserPreview(row.FileName, row.MimeType) {
		return ""
	}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		if row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady {
			if urlValue := s.presignedOriginalPreviewURL(row); urlValue != "" {
				return urlValue
			}
		}
		return ""
	}
	if row.Kind == domain.ExternalAssetKindNetdisk {
		rawURL := strings.TrimSpace(row.RawURL)
		if rawURL != "" && !s.isInternalProviderURL(rawURL) {
			return rawURL
		}
	}
	return ""
}

func (s *Service) BrowserDownloadURL(row *domain.ExternalAssetRecord) string {
	if row == nil || row.IsDir {
		return ""
	}
	if row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady {
		return s.presignedOriginalURL(row)
	}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		return ""
	}
	if row.Kind == domain.ExternalAssetKindNetdisk {
		rawURL := strings.TrimSpace(row.RawURL)
		if rawURL != "" && !s.isInternalProviderURL(rawURL) {
			return rawURL
		}
	}
	return ""
}

func (s *Service) presignedPreviewURL(row *domain.ExternalAssetRecord) string {
	if s == nil || s.ossDirect == nil || !s.ossDirect.Enabled() || row == nil || strings.TrimSpace(row.OSSPreviewKey) == "" {
		return ""
	}
	signed := s.ossDirect.PresignPreviewURL(row.OSSPreviewKey)
	if signed == nil {
		return ""
	}
	return strings.TrimSpace(signed.DownloadURL)
}

func (s *Service) presignedOriginalPreviewURL(row *domain.ExternalAssetRecord) string {
	if s == nil || s.ossDirect == nil || !s.ossDirect.Enabled() || row == nil || strings.TrimSpace(row.OSSOriginalKey) == "" {
		return ""
	}
	signed := s.ossDirect.PresignPreviewURL(row.OSSOriginalKey)
	if signed == nil {
		return ""
	}
	return strings.TrimSpace(signed.DownloadURL)
}

func (s *Service) presignedOriginalURL(row *domain.ExternalAssetRecord) string {
	if s == nil || s.ossDirect == nil || !s.ossDirect.Enabled() || row == nil || strings.TrimSpace(row.OSSOriginalKey) == "" {
		return ""
	}
	signed := s.ossDirect.PresignDownloadURLWithFilename(row.OSSOriginalKey, row.FileName)
	if signed == nil {
		return ""
	}
	return strings.TrimSpace(signed.DownloadURL)
}

func (s *Service) localDownloadInfo(ctx context.Context, row *domain.ExternalAssetRecord) (*domain.AssetDownloadInfo, *domain.AppError) {
	if row.OSSOriginalKey != "" && row.OSSSyncStatus == domain.ExternalAssetOSSStatusReady {
		return s.ossDownloadInfo(row, row.OSSOriginalKey, false, "external_local_oss")
	}
	if err := s.repo.MarkOSSPreparePending(ctx, row.ID); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return prepareInfo(row, "external_local_prepare_required"), nil
}

func (s *Service) netdiskDownloadInfo(ctx context.Context, row *domain.ExternalAssetRecord, preview bool) (*domain.AssetDownloadInfo, *domain.AppError) {
	rawURL := strings.TrimSpace(row.RawURL)
	if s.shouldRefreshDirectURL(row) {
		nextURL, err := s.resolveNetdiskDirectURL(ctx, row, preview)
		if err != nil {
			_ = s.repo.UpdateDirectURL(ctx, row.ID, "", nil, "failed")
			rawURL = ""
		} else {
			rawURL = strings.TrimSpace(nextURL)
			_ = s.repo.UpdateDirectURL(ctx, row.ID, rawURL, nil, directURLStatus(rawURL))
		}
	}
	if rawURL == "" || s.isInternalProviderURL(rawURL) {
		if err := s.repo.MarkOSSPreparePending(ctx, row.ID); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		return prepareInfo(row, "external_netdisk_prepare_required"), nil
	}
	filename := row.FileName
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &rawURL,
		AccessHint:       "external_netdisk_direct",
		PreviewAvailable: preview && canDirectBrowserPreview(row.FileName, row.MimeType),
		Filename:         filename,
		FileSize:         row.FileSize,
		MimeType:         row.MimeType,
	}, nil
}

func (s *Service) shouldRefreshDirectURL(row *domain.ExternalAssetRecord) bool {
	if row == nil || row.Kind != domain.ExternalAssetKindNetdisk {
		return false
	}
	if strings.TrimSpace(row.RawURL) == "" {
		return true
	}
	if row.LastLinkCheckedAt == nil {
		return true
	}
	return s.nowFn().UTC().Sub(*row.LastLinkCheckedAt) >= s.cfg.LinkRefreshInterval
}

func (s *Service) isInternalProviderURL(rawURL string) bool {
	if strings.TrimSpace(rawURL) == "" {
		return false
	}
	if isPrivateNetworkURL(rawURL) {
		return true
	}
	if s.alist != nil && s.alist.IsAListURL(rawURL) {
		return true
	}
	if s.bff != nil && s.bff.IsBFFURL(rawURL) {
		return true
	}
	return false
}

func isPrivateNetworkURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

func (s *Service) ossDownloadInfo(row *domain.ExternalAssetRecord, objectKey string, preview bool, hint string) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.ossDirect == nil || !s.ossDirect.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "OSS 下载服务未配置", nil)
	}
	var signed *baseservice.OSSDirectDownloadInfo
	if preview {
		signed = s.ossDirect.PresignPreviewURL(objectKey)
	} else {
		signed = s.ossDirect.PresignDownloadURLWithFilename(objectKey, row.FileName)
	}
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "OSS 下载地址生成失败", nil)
	}
	urlValue := signed.DownloadURL
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &urlValue,
		AccessHint:       hint,
		PreviewAvailable: preview,
		Filename:         row.FileName,
		FileSize:         row.FileSize,
		MimeType:         row.MimeType,
		ExpiresAt:        &signed.ExpiresAt,
	}, nil
}

func prepareInfo(row *domain.ExternalAssetRecord, hint string) *domain.AssetDownloadInfo {
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      nil,
		AccessHint:       hint,
		PreviewAvailable: false,
		Filename:         row.FileName,
		FileSize:         row.FileSize,
		MimeType:         row.MimeType,
	}
}

func (s *Service) BuildOSSOriginalKey(row *domain.ExternalAssetRecord) string {
	return path.Join(s.cfg.OSSOriginalPrefix, row.MountPath, rowOriginShortHash(row), safeObjectFilename(row.FileName))
}

func (s *Service) BuildOSSPreviewKey(row *domain.ExternalAssetRecord) string {
	return path.Join(s.cfg.OSSPreviewPrefix, row.MountPath, rowOriginShortHash(row), safeObjectFilename(stripExt(row.FileName)+".webp"))
}

func (s *Service) LocalFilesystemPath(row *domain.ExternalAssetRecord) (string, bool) {
	if row == nil {
		return "", false
	}
	base, ok := s.cfg.LocalPathMappings[row.MountPath]
	if !ok || strings.TrimSpace(base) == "" {
		return "", false
	}
	rel := strings.TrimPrefix(row.OriginPath, row.MountPath)
	rel = strings.TrimLeft(rel, "/")
	return filepath.Join(base, filepath.FromSlash(rel)), true
}

func (s *Service) EnsureOSSRequiredPrefixesPending(ctx context.Context) (int64, error) {
	if !s.Enabled() {
		return 0, nil
	}
	prefixes := s.ossRequiredOriginPrefixes()
	if len(prefixes) == 0 {
		return 0, nil
	}
	return s.repo.MarkOSSPendingByOriginPrefixes(ctx, prefixes)
}

func (s *Service) ossRequiredOriginPrefixes() []repo.ExternalAssetOriginPrefix {
	if s == nil {
		return nil
	}
	out := make([]repo.ExternalAssetOriginPrefix, 0, len(s.cfg.OSSRequiredPrefixes))
	seen := map[string]struct{}{}
	for _, raw := range s.cfg.OSSRequiredPrefixes {
		origin := cleanAListPath(raw)
		if origin == "/" {
			continue
		}
		mount := s.mountPathForOrigin(origin)
		if mount == "" {
			continue
		}
		key := mount + "\x00" + origin
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, repo.ExternalAssetOriginPrefix{MountPath: mount, OriginPath: origin})
	}
	return out
}

func (s *Service) mountPathForOrigin(origin string) string {
	origin = cleanAListPath(origin)
	if origin == "/" {
		return ""
	}
	best := ""
	for _, mount := range s.cfg.Mounts {
		mountPath := cleanAListPath(mount.Path)
		if mountPath == "/" {
			continue
		}
		if origin == mountPath || strings.HasPrefix(origin, mountPath+"/") {
			if len(mountPath) > len(best) {
				best = mountPath
			}
		}
	}
	if best != "" {
		return best
	}
	trimmed := strings.Trim(origin, "/")
	if trimmed == "" {
		return ""
	}
	first, _, _ := strings.Cut(trimmed, "/")
	return cleanAListPath(first)
}

func (s *Service) ProcessPendingOSS(ctx context.Context, limit int) (int, error) {
	prepareMounts := s.PrepareMountPaths()
	if len(prepareMounts) == 0 {
		return 0, nil
	}
	priorityPrefixes := externalAssetPrefixesForMounts(s.ossRequiredOriginPrefixes(), prepareMounts)
	if len(priorityPrefixes) > 0 {
		if _, err := s.repo.MarkOSSPendingByOriginPrefixes(ctx, priorityPrefixes); err != nil {
			return 0, err
		}
	}
	rows, err := s.repo.ListPendingOSSPrioritized(ctx, priorityPrefixes, prepareMounts, limit)
	if err != nil {
		return 0, err
	}
	var done int
	var mu sync.Mutex
	s.runPrepareWorkers(ctx, rows, func(row *domain.ExternalAssetRecord) {
		if err := s.uploadLocalOriginal(ctx, row); err != nil {
			_ = s.repo.MarkPrepareFailed(ctx, row.ID, "oss", err.Error())
			return
		}
		mu.Lock()
		done++
		mu.Unlock()
	})
	return done, nil
}

func (s *Service) ProcessPendingPreview(ctx context.Context, limit int) (int, error) {
	prepareMounts := s.PrepareMountPaths()
	if len(prepareMounts) == 0 {
		return 0, nil
	}
	rows, err := s.repo.ListPendingPreview(ctx, prepareMounts, limit)
	if err != nil {
		return 0, err
	}
	var done int
	var mu sync.Mutex
	s.runPrepareWorkers(ctx, rows, func(row *domain.ExternalAssetRecord) {
		if err := s.renderAndUploadPreview(ctx, row); err != nil {
			_ = s.repo.MarkPrepareFailed(ctx, row.ID, "preview", err.Error())
			return
		}
		mu.Lock()
		done++
		mu.Unlock()
	})
	return done, nil
}

func externalAssetPrefixesForMounts(prefixes []repo.ExternalAssetOriginPrefix, mountPaths []string) []repo.ExternalAssetOriginPrefix {
	allowed := make(map[string]struct{}, len(mountPaths))
	for _, mount := range mountPaths {
		allowed[cleanAListPath(mount)] = struct{}{}
	}
	filtered := make([]repo.ExternalAssetOriginPrefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		if _, ok := allowed[cleanAListPath(prefix.MountPath)]; ok {
			filtered = append(filtered, prefix)
		}
	}
	return filtered
}

func (s *Service) runPrepareWorkers(ctx context.Context, rows []*domain.ExternalAssetRecord, handle func(*domain.ExternalAssetRecord)) {
	if len(rows) == 0 || handle == nil {
		return
	}
	netdisk := make([]*domain.ExternalAssetRecord, 0, len(rows))
	local := make([]*domain.ExternalAssetRecord, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Kind == domain.ExternalAssetKindNetdisk {
			netdisk = append(netdisk, row)
		} else {
			local = append(local, row)
		}
	}
	// Quark rejects parallel source reads with ExceedMaxConcurrency. Keep those
	// jobs serial while allowing NAS-backed preparation to use configured concurrency.
	s.runPrepareWorkersWithLimit(ctx, netdisk, 1, handle)
	s.runPrepareWorkersWithLimit(ctx, local, s.cfg.PrepareConcurrency, handle)
}

func (s *Service) runPrepareWorkersWithLimit(ctx context.Context, rows []*domain.ExternalAssetRecord, workers int, handle func(*domain.ExternalAssetRecord)) {
	if len(rows) == 0 || handle == nil {
		return
	}
	if workers <= 0 {
		workers = 1
	}
	if workers > len(rows) {
		workers = len(rows)
	}
	ch := make(chan *domain.ExternalAssetRecord)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range ch {
				if row == nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
				handle(row)
			}
		}()
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			close(ch)
			wg.Wait()
			return
		case ch <- row:
		}
	}
	close(ch)
	wg.Wait()
}

func (s *Service) uploadLocalOriginal(ctx context.Context, row *domain.ExternalAssetRecord) error {
	if s.ossDirect == nil || !s.ossDirect.Enabled() {
		return fmt.Errorf("oss direct is not enabled")
	}
	rc, err := s.openSourceForUpload(ctx, row)
	if err != nil {
		return err
	}
	defer rc.Close()
	key := s.BuildOSSOriginalKey(row)
	if err := s.ossDirect.UploadObjectFromReader(ctx, key, normalizeMimeType(row.FileName, row.MimeType), rc); err != nil {
		return err
	}
	return s.repo.MarkOSSReady(ctx, row.ID, key)
}

func (s *Service) renderAndUploadPreview(ctx context.Context, row *domain.ExternalAssetRecord) error {
	if s.ossDirect == nil || !s.ossDirect.Enabled() {
		return fmt.Errorf("oss direct is not enabled")
	}
	sourcePath := ""
	cleanup := func() {}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		if localPath, ok := s.LocalFilesystemPath(row); ok {
			if _, err := os.Stat(localPath); err == nil {
				sourcePath = localPath
			}
		}
		if sourcePath == "" {
			rc, err := s.openSourceForUpload(ctx, row)
			if err != nil {
				return err
			}
			defer rc.Close()
			tmp, err := os.CreateTemp("", "external-asset-*"+filepath.Ext(row.FileName))
			if err != nil {
				return err
			}
			sourcePath = tmp.Name()
			if _, err := io.Copy(tmp, rc); err != nil {
				_ = tmp.Close()
				_ = os.Remove(sourcePath)
				return err
			}
			_ = tmp.Close()
			cleanup = func() { _ = os.Remove(sourcePath) }
		}
	} else {
		rc, err := s.openSourceForUpload(ctx, row)
		if err != nil {
			return err
		}
		defer rc.Close()
		tmp, err := os.CreateTemp("", "external-asset-*"+filepath.Ext(row.FileName))
		if err != nil {
			return err
		}
		sourcePath = tmp.Name()
		if _, err := io.Copy(tmp, rc); err != nil {
			_ = tmp.Close()
			_ = os.Remove(sourcePath)
			return err
		}
		_ = tmp.Close()
		cleanup = func() { _ = os.Remove(sourcePath) }
	}
	defer cleanup()
	webp, err := s.renderer.Render(ctx, sourcePath, baseservice.AssetPreviewSourceMeta{
		Filename: row.FileName,
		MimeType: row.MimeType,
	}, baseservice.AssetPreviewRenderSpec{MaxWidth: 1600, MaxHeight: 1600, Quality: 82})
	if err != nil {
		return err
	}
	key := s.BuildOSSPreviewKey(row)
	if err := s.ossDirect.UploadObject(ctx, key, "image/webp", webp); err != nil {
		return err
	}
	return s.repo.MarkPreviewReady(ctx, row.ID, key)
}

func (s *Service) openSourceForUpload(ctx context.Context, row *domain.ExternalAssetRecord) (io.ReadCloser, error) {
	if row == nil {
		return nil, fmt.Errorf("external asset row is required")
	}
	if row.Kind == domain.ExternalAssetKindNASLocal {
		if localPath, ok := s.LocalFilesystemPath(row); ok {
			if f, err := os.Open(localPath); err == nil {
				return f, nil
			}
		}
		if s.bff != nil && s.bff.Enabled() {
			return s.bff.OpenFetch(ctx, row.OriginPath, false)
		}
		return nil, fmt.Errorf("local file is not available")
	}
	rawURL := strings.TrimSpace(row.RawURL)
	if rawURL == "" || s.isInternalProviderURL(rawURL) {
		nextURL, err := s.resolveNetdiskDirectURL(ctx, row, false)
		if err != nil {
			return nil, err
		}
		rawURL = strings.TrimSpace(nextURL)
	}
	if rawURL != "" && !s.isInternalProviderURL(rawURL) {
		return openHTTPBody(ctx, rawURL)
	}
	if s.bff != nil && s.bff.Enabled() {
		return s.bff.OpenFetch(ctx, row.OriginPath, false)
	}
	return nil, fmt.Errorf("external asset source is not available")
}

func (s *Service) upsertFromSearchItem(mount MountConfig, item AListSearchItem) domain.ExternalAssetUpsert {
	originPath := joinAListPath(item.Parent, item.Name)
	ext := strings.ToLower(filepath.Ext(item.Name))
	mimeType := normalizeMimeType(item.Name, "")
	driver := mount.Driver
	if driver == "" {
		driver = driverForMountKind(mount.Kind, mount.Path)
	}
	return domain.ExternalAssetUpsert{
		Provider:         "alist",
		Kind:             mount.Kind,
		Driver:           driver,
		MountPath:        mount.Path,
		OriginPath:       originPath,
		ParentPath:       item.Parent,
		FileName:         item.Name,
		FileExt:          ext,
		MimeType:         mimeType,
		FileSize:         item.Size,
		SourceModifiedAt: sourceModifiedTimePtr(item.Modified),
		IsDir:            item.IsDir,
		SearchableText:   strings.Join([]string{originPath, item.Parent, item.Name, driver, string(mount.Kind)}, " "),
		ScannedAt:        s.nowFn().UTC(),
	}
}

func sourceModifiedTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC().Truncate(time.Second)
	return &normalized
}

func isSkippableExternalSearchItem(item AListSearchItem) bool {
	originPath := strings.ToLower(joinAListPath(item.Parent, item.Name))
	fileName := strings.ToLower(strings.TrimSpace(item.Name))
	return strings.Contains(originPath, "/@eadir/") ||
		strings.HasSuffix(originPath, "/@eadir") ||
		strings.Contains(originPath, "/#recycle/") ||
		strings.HasSuffix(originPath, "/#recycle") ||
		strings.Contains(fileName, "@syno") ||
		isExternalSystemNoiseFile(fileName)
}

func isExternalSystemNoiseFile(fileName string) bool {
	switch strings.ToLower(strings.TrimSpace(fileName)) {
	case "thumbs.db", "desktop.ini", ".ds_store":
		return true
	default:
		return false
	}
}

func driverForMountKind(kind domain.ExternalAssetKind, mountPath string) string {
	if kind == domain.ExternalAssetKindNASLocal {
		return "Local"
	}
	switch mountPath {
	case "/quark":
		return "Quark"
	case "/p1", "/p2":
		return "AliyundriveOpen"
	default:
		return "Netdisk"
	}
}

func directURLStatus(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return "empty"
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "invalid"
	}
	return "ready"
}

func normalizeMimeType(filename, fallback string) string {
	if fallback = strings.TrimSpace(fallback); fallback != "" {
		return fallback
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	return "application/octet-stream"
}

func canDirectBrowserPreview(filename, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(filepath.Ext(filename))
	if (mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/webp" || mimeType == "image/gif" || mimeType == "image/bmp" || mimeType == "image/svg+xml") &&
		!strings.Contains(mimeType, "photoshop") {
		return true
	}
	if mimeType == "application/pdf" {
		return true
	}
	if strings.HasPrefix(mimeType, "video/") {
		switch ext {
		case ".mp4", ".webm", ".mov", ".m4v":
			return true
		}
		return false
	}
	if strings.HasPrefix(mimeType, "image/") {
		return false
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg", ".pdf", ".mp4", ".webm", ".mov", ".m4v":
		return true
	default:
		return false
	}
}

func canRenderDerivedPreview(filename, mimeType string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "image/") || strings.Contains(mimeType, "photoshop") || strings.Contains(mimeType, "illustrator") {
		return true
	}
	if mimeType == "application/pdf" {
		return true
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg", ".tif", ".tiff", ".heic", ".heif", ".avif", ".pdf", ".psd", ".psb", ".ai", ".eps", ".ps":
		return true
	default:
		return false
	}
}

func rowOriginShortHash(row *domain.ExternalAssetRecord) string {
	h := sha1.Sum([]byte(row.OriginPathHash + row.OriginPath))
	return hex.EncodeToString(h[:])[:16]
}

func safeObjectFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "file"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "\x00", "")
	filename = replacer.Replace(filename)
	if len([]rune(filename)) > 160 {
		runes := []rune(filename)
		ext := filepath.Ext(filename)
		prefixLen := 140
		if prefixLen > len(runes) {
			prefixLen = len(runes)
		}
		filename = string(runes[:prefixLen]) + ext
	}
	return filename
}

func stripExt(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}
