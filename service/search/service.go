package search

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	CodeInvalidQuery = "invalid_query"

	globalExternalPreviewBudget = 250 * time.Millisecond
	externalSearchBudget        = 650 * time.Millisecond
	productSearchBudget         = 900 * time.Millisecond
	autoHybridBudget            = 700 * time.Millisecond
)

type Service struct {
	repo      repo.SearchRepo
	external  ExternalAssetSearchProvider
	retrieval HybridRetrievalProvider
	logger    *zap.Logger
}

func NewService(repo repo.SearchRepo) *Service {
	return &Service{repo: repo, logger: zap.NewNop()}
}

type ExternalAssetSearchProvider interface {
	SearchGlobal(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error)
}

type ResourceGroupSearchProvider interface {
	SearchResourceGroups(ctx context.Context, q string, limit int, publishedOnly bool, access domain.ResourceGroupAccessFilter) ([]domain.SearchAsset, error)
}

type ScopedTaskSearchProvider interface {
	SearchTasksScoped(ctx context.Context, q string, limit int, access domain.ResourceGroupAccessFilter) ([]domain.SearchTask, error)
}

type HybridRetrievalProvider interface {
	HybridReady() bool
	Search(ctx context.Context, actor domain.RequestActor, query string, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error)
}

func (s *Service) SetExternalAssetSearchProvider(provider ExternalAssetSearchProvider) {
	s.external = provider
}

func (s *Service) SetHybridRetrievalProvider(provider HybridRetrievalProvider) {
	s.retrieval = provider
}

func (s *Service) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *Service) Search(ctx context.Context, actor domain.RequestActor, q string, scope string, limit int) (*domain.SearchResultGroup, *domain.AppError) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, domain.NewAppError(CodeInvalidQuery, "q is required", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "all"
	}
	result := &domain.SearchResultGroup{
		Tasks:    []domain.SearchTask{},
		Assets:   []domain.SearchAsset{},
		Products: []domain.SearchProduct{},
		Users:    []domain.SearchUser{},
	}

	var err error
	switch scope {
	case "all":
		var externalAssets []domain.SearchAsset
		if err = runSearchJobs(
			s.logger,
			searchJob{name: "tasks", run: func() error {
				rows, err := s.searchTasks(ctx, actor, q, limit)
				result.Tasks = rows
				return err
			}},
			searchJob{name: "assets", run: func() error {
				rows, err := s.searchResourceGroups(ctx, actor, q, limit)
				result.Assets = rows
				return err
			}},
			searchJob{name: "external", run: func() error {
				rows, err := s.searchExternalAssetsWithBudget(ctx, actor, q, limit, globalExternalPreviewBudget)
				externalAssets = rows
				return err
			}},
			searchJob{name: "products", run: func() error {
				rows, err := s.searchProducts(ctx, actor, q, limit)
				result.Products = rows
				if err != nil {
					// Global search is a multi-source preview. Keep usable task,
					// asset and user matches when the catalog branch is under
					// pressure; an explicit products scope still surfaces the
					// backend error below.
					s.logger.Warn("global search product branch degraded", zap.Error(err))
					result.Products = []domain.SearchProduct{}
					return nil
				}
				return nil
			}},
			searchJob{name: "users", run: func() error {
				rows, err := s.searchUsers(ctx, actor, q, limit)
				result.Users = rows
				return err
			}},
		); err != nil {
			return nil, internalErr(err)
		}
		result.Assets = append(result.Assets, externalAssets...)
	case "tasks":
		if result.Tasks, err = s.searchTasks(ctx, actor, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "assets":
		var systemAssets []domain.SearchAsset
		var externalAssets []domain.SearchAsset
		if err = runSearchJobs(
			s.logger,
			searchJob{name: "assets", run: func() error {
				rows, err := s.searchResourceGroups(ctx, actor, q, limit)
				systemAssets = rows
				return err
			}},
			searchJob{name: "external", run: func() error {
				rows, err := s.searchExternalAssets(ctx, actor, q, limit)
				externalAssets = rows
				return err
			}},
		); err != nil {
			return nil, internalErr(err)
		}
		result.Assets = append(systemAssets, externalAssets...)
	case "products":
		if result.Products, err = s.searchProducts(ctx, actor, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "users":
		result.Users, err = s.searchUsers(ctx, actor, q, limit)
		if err != nil {
			return nil, internalErr(err)
		}
	default:
		return nil, domain.NewAppError(CodeInvalidQuery, "invalid scope", nil)
	}
	normalizeNilSlices(result)
	return result, nil
}

func (s *Service) SearchWithMode(ctx context.Context, actor domain.RequestActor, q string, scope string, limit int, requestedMode string) (*domain.SearchResultGroup, domain.SearchRetrievalMeta, *domain.AppError) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, domain.SearchRetrievalMeta{}, domain.NewAppError(CodeInvalidQuery, "q is required", nil)
	}
	requestedMode = strings.ToLower(strings.TrimSpace(requestedMode))
	if requestedMode == "" {
		requestedMode = "auto"
	}
	if requestedMode != "auto" && requestedMode != "exact" && requestedMode != "hybrid" {
		return nil, domain.SearchRetrievalMeta{}, domain.NewAppError(CodeInvalidQuery, "invalid search mode", nil)
	}
	selectedMode := requestedMode
	if selectedMode == "auto" {
		selectedMode = "hybrid"
		if deterministicSearchQuery(q) {
			selectedMode = "exact"
		}
	}
	meta := domain.SearchRetrievalMeta{RequestedMode: requestedMode, Mode: selectedMode}
	if selectedMode != "hybrid" {
		result, appErr := s.Search(ctx, actor, q, scope, limit)
		if appErr != nil {
			return nil, meta, appErr
		}
		return result, meta, nil
	}
	if s.retrieval == nil {
		result, appErr := s.Search(ctx, actor, q, scope, limit)
		if appErr != nil {
			return nil, meta, appErr
		}
		meta.Mode, meta.Degraded, meta.Reason = "exact", true, "hybrid_not_configured"
		return result, meta, nil
	}
	type exactOutcome struct {
		result *domain.SearchResultGroup
		err    *domain.AppError
	}
	type retrievalOutcome struct {
		hits []domain.AIRetrievalHit
		meta domain.AIRetrievalMeta
		err  error
	}
	exactCh := make(chan exactOutcome, 1)
	retrievalCh := make(chan retrievalOutcome, 1)
	retrievalCtx := ctx
	cancelRetrieval := func() {}
	if requestedMode == "auto" {
		retrievalCtx, cancelRetrieval = context.WithTimeout(ctx, autoHybridBudget)
	}
	defer cancelRetrieval()
	go func() {
		result, appErr := s.Search(ctx, actor, q, scope, limit)
		exactCh <- exactOutcome{result: result, err: appErr}
	}()
	go func() {
		hits, retrievalMeta, retrievalErr := s.retrieval.Search(retrievalCtx, actor, q, min(limit, 20))
		retrievalCh <- retrievalOutcome{hits: hits, meta: retrievalMeta, err: retrievalErr}
	}()
	exact := <-exactCh
	if exact.err != nil {
		return nil, meta, exact.err
	}
	var retrieval retrievalOutcome
	if requestedMode == "auto" {
		select {
		case retrieval = <-retrievalCh:
		case <-retrievalCtx.Done():
			meta.Mode, meta.Degraded, meta.Reason = "exact", true, "hybrid_timeout"
			return exact.result, meta, nil
		}
	} else {
		retrieval = <-retrievalCh
	}
	if retrieval.err != nil {
		meta.Mode, meta.Degraded, meta.Reason = "exact", true, "hybrid_unavailable"
		s.logger.Warn("hybrid global search degraded", zap.Error(retrieval.err))
		return exact.result, meta, nil
	}
	meta.Mode, meta.Degraded, meta.Candidates, meta.Reason = retrieval.meta.Mode, retrieval.meta.Degraded, retrieval.meta.Candidates, retrieval.meta.Reason
	mergeRetrievalHits(exact.result, retrieval.hits, strings.TrimSpace(scope), normalizeSearchLimit(limit))
	normalizeNilSlices(exact.result)
	return exact.result, meta, nil
}

var deterministicQueryPattern = regexp.MustCompile(`(?i)^(?:[a-z]{1,8}[-_/])?[a-z0-9][a-z0-9._/-]{2,63}$`)

func deterministicSearchQuery(query string) bool {
	query = strings.TrimSpace(query)
	if deterministicQueryPattern.MatchString(query) {
		return true
	}
	for _, marker := range []string{"任务号", "SKU", "sku", "文件名"} {
		if strings.Contains(query, marker) {
			return true
		}
	}
	return false
}

func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func mergeRetrievalHits(result *domain.SearchResultGroup, hits []domain.AIRetrievalHit, scope string, limit int) {
	if result == nil {
		return
	}
	if scope == "" {
		scope = "all"
	}
	seenTasks := map[int64]struct{}{}
	for _, item := range result.Tasks {
		seenTasks[item.ID] = struct{}{}
	}
	seenAssets := map[string]struct{}{}
	for _, item := range result.Assets {
		seenAssets[item.ResourceID] = struct{}{}
	}
	for _, hit := range hits {
		switch hit.EntityType {
		case "task":
			if scope != "all" && scope != "tasks" {
				continue
			}
			id, err := strconv.ParseInt(hit.EntityID, 10, 64)
			if err != nil {
				continue
			}
			if _, exists := seenTasks[id]; exists {
				continue
			}
			taskNo, highlight := metadataString(hit.Metadata, "task_no"), hit.Excerpt
			result.Tasks = append(result.Tasks, domain.SearchTask{ID: id, TaskNo: taskNo, Highlight: &highlight})
			seenTasks[id] = struct{}{}
		case "task_resource_group":
			if scope != "all" && scope != "assets" {
				continue
			}
			id, err := strconv.ParseInt(hit.EntityID, 10, 64)
			if err != nil {
				continue
			}
			resourceID := fmt.Sprintf("group:%d", id)
			if _, exists := seenAssets[resourceID]; exists {
				continue
			}
			result.Assets = append(result.Assets, domain.SearchAsset{
				AssetID: id, ResourceGroupID: id, ResourceID: resourceID, SourceType: "task_resource_group", SourceLabel: "任务资源组",
				TaskNo: metadataString(hit.Metadata, "task_no"), SKUCode: metadataString(hit.Metadata, "sku_code"), Mode: metadataString(hit.Metadata, "mode"),
				FinalizedRevisionID: metadataInt64(hit.Metadata, "finalized_revision_id"), FileName: hit.Title,
			})
			seenAssets[resourceID] = struct{}{}
		case "external_asset":
			if scope != "all" && scope != "assets" {
				continue
			}
			resourceID := "ext-" + hit.EntityID
			if _, exists := seenAssets[resourceID]; exists {
				continue
			}
			result.Assets = append(result.Assets, domain.SearchAsset{ResourceID: resourceID, FileName: hit.Title, SourceType: "external_asset", SourceLabel: "外部资源"})
			seenAssets[resourceID] = struct{}{}
		}
	}
	if len(result.Tasks) > limit {
		result.Tasks = result.Tasks[:limit]
	}
	if len(result.Assets) > limit {
		result.Assets = result.Assets[:limit]
	}
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func metadataInt64(metadata map[string]any, key string) int64 {
	if value, ok := metadata[key].(float64); ok {
		return int64(value)
	}
	value, _ := strconv.ParseInt(metadataString(metadata, key), 10, 64)
	return value
}

func (s *Service) searchResourceGroups(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchAsset, error) {
	if provider, ok := s.repo.(ResourceGroupSearchProvider); ok {
		if !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
			return []domain.SearchAsset{}, nil
		}
		publishedOnly := publishedAssetSearchOnly(actor)
		if publishedOnly {
			return provider.SearchResourceGroups(ctx, q, limit, true, domain.ResourceGroupAccessFilter{})
		}
		return provider.SearchResourceGroups(ctx, q, limit, false, domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionAssetView))
	}
	// Cutover deliberately fails closed when the repository has not implemented
	// resource-group search. Falling back to historical file-version search
	// would reintroduce both scope leaks and retired result semantics.
	return []domain.SearchAsset{}, nil
}

func (s *Service) searchTasks(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchTask, error) {
	if !domain.ActorHasPermission(actor, domain.PermissionTaskView) {
		return []domain.SearchTask{}, nil
	}
	provider, ok := s.repo.(ScopedTaskSearchProvider)
	if !ok {
		return []domain.SearchTask{}, nil
	}
	return provider.SearchTasksScoped(ctx, q, limit, domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView))
}

func (s *Service) searchProducts(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchProduct, error) {
	if !domain.ActorHasPermission(actor, domain.PermissionCatalogView) {
		return []domain.SearchProduct{}, nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, productSearchBudget)
	defer cancel()
	return s.repo.SearchProducts(searchCtx, q, limit)
}

func (s *Service) searchExternalAssets(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchAsset, error) {
	return s.searchExternalAssetsWithBudget(ctx, actor, q, limit, externalSearchBudget)
}

func (s *Service) searchExternalAssetsWithBudget(ctx context.Context, actor domain.RequestActor, q string, limit int, budget time.Duration) ([]domain.SearchAsset, error) {
	if s.external == nil || !domain.ActorHasPermission(actor, domain.PermissionAssetView) || publishedAssetSearchOnly(actor) {
		return []domain.SearchAsset{}, nil
	}
	if budget <= 0 {
		budget = externalSearchBudget
	}
	searchCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	items, err := s.external.SearchGlobal(searchCtx, q, limit)
	if err != nil {
		return []domain.SearchAsset{}, nil
	}
	return items, nil
}

func publishedAssetSearchOnly(actor domain.RequestActor) bool {
	if actor.EffectiveAccess == nil {
		return false
	}
	found := false
	for _, source := range actor.EffectiveAccess.Sources {
		if source.Permission != domain.PermissionAssetView {
			continue
		}
		found = true
		if source.RoleCode != "asset_submitter" {
			return false
		}
	}
	return found
}

type searchJob struct {
	name string
	run  func() error
}

func runSearchJobs(logger *zap.Logger, jobs ...searchJob) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			err := job.run()
			duration := time.Since(started)
			if err != nil || duration >= 200*time.Millisecond {
				logger.Warn("global search branch slow or failed",
					zap.String("branch", job.name),
					zap.Int64("duration_ms", duration.Milliseconds()),
					zap.Bool("error", err != nil),
					zap.Error(err),
				)
			}
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func (s *Service) searchUsers(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchUser, error) {
	if !domain.ActorHasPermission(actor, domain.PermissionAccessView) &&
		!domain.ActorHasPermission(actor, domain.PermissionAccessManage) {
		return []domain.SearchUser{}, nil
	}
	return s.repo.SearchUsers(ctx, q, limit)
}

func normalizeNilSlices(result *domain.SearchResultGroup) {
	if result.Tasks == nil {
		result.Tasks = []domain.SearchTask{}
	}
	if result.Assets == nil {
		result.Assets = []domain.SearchAsset{}
	}
	if result.Products == nil {
		result.Products = []domain.SearchProduct{}
	}
	if result.Users == nil {
		result.Users = []domain.SearchUser{}
	}
}

func internalErr(err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
}
