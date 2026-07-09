package asset_center

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"workflow/domain"
	"workflow/repo"
)

func TestSearchUsesCachedSystemTotalWithRowsOnlyRepo(t *testing.T) {
	searchRepo := &assetSearchCacheRepoStub{
		rows:  []*repo.TaskAssetSearchRow{assetSearchCacheRow()},
		total: 42,
	}
	cache := newFakeAssetCenterCache()
	svc := NewService(searchRepo, nil, nil, WithAssetCenterRedis(cache))
	query := domain.AssetSearchQuery{
		Source:     domain.AssetResourceSourceSystem,
		IsArchived: domain.AssetArchiveFilterFalse,
		Page:       1,
		Size:       20,
	}

	first, appErr := svc.Search(context.Background(), query)
	if appErr != nil {
		t.Fatalf("first Search() appErr = %+v", appErr)
	}
	if first.Total != 42 || len(first.Items) != 1 {
		t.Fatalf("first result total/items = %d/%d, want 42/1", first.Total, len(first.Items))
	}
	if searchRepo.searchCalls != 1 || searchRepo.rowsOnlyCalls != 0 {
		t.Fatalf("repo calls after first search = search:%d rows:%d, want 1/0", searchRepo.searchCalls, searchRepo.rowsOnlyCalls)
	}
	if len(cache.values) != 1 {
		t.Fatalf("cache values after first search = %d, want 1", len(cache.values))
	}

	second, appErr := svc.Search(context.Background(), query)
	if appErr != nil {
		t.Fatalf("second Search() appErr = %+v", appErr)
	}
	if second.Total != 42 || len(second.Items) != 1 {
		t.Fatalf("second result total/items = %d/%d, want 42/1", second.Total, len(second.Items))
	}
	if searchRepo.searchCalls != 1 || searchRepo.rowsOnlyCalls != 1 {
		t.Fatalf("repo calls after second search = search:%d rows:%d, want 1/1", searchRepo.searchCalls, searchRepo.rowsOnlyCalls)
	}
	for key, ttl := range cache.ttls {
		if ttl != assetSearchTotalCacheTTL {
			t.Fatalf("ttl for %s = %s, want %s", key, ttl, assetSearchTotalCacheTTL)
		}
	}
}

func TestBrowseMaterialsPassesBusinessLaneToSystemSearch(t *testing.T) {
	searchRepo := &assetSearchCacheRepoStub{
		rows:  []*repo.TaskAssetSearchRow{assetSearchCacheRow()},
		total: 1,
	}
	svc := NewService(searchRepo, nil, nil)

	result, appErr := svc.BrowseMaterials(context.Background(), MaterialBrowseQuery{
		Path:         "/系统资源",
		BusinessLane: domain.TaskBusinessLaneCustomization,
		Page:         1,
		Size:         20,
	})
	if appErr != nil {
		t.Fatalf("BrowseMaterials() appErr = %+v", appErr)
	}
	if result.Total != 1 {
		t.Fatalf("result total = %d, want 1", result.Total)
	}
	if searchRepo.lastSearchQuery.BusinessLane != domain.TaskBusinessLaneCustomization {
		t.Fatalf("business lane = %q, want %q", searchRepo.lastSearchQuery.BusinessLane, domain.TaskBusinessLaneCustomization)
	}
}

type assetSearchCacheRepoStub struct {
	rows            []*repo.TaskAssetSearchRow
	total           int64
	lastSearchQuery domain.AssetSearchQuery
	searchCalls     int
	rowsOnlyCalls   int
}

func (s *assetSearchCacheRepoStub) Search(_ context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	s.searchCalls++
	s.lastSearchQuery = query.Normalized()
	return s.rows, s.total, nil
}

func (s *assetSearchCacheRepoStub) SearchRows(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, error) {
	s.rowsOnlyCalls++
	return s.rows, nil
}

func (s *assetSearchCacheRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *assetSearchCacheRepoStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *assetSearchCacheRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s *assetSearchCacheRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func assetSearchCacheRow() *repo.TaskAssetSearchRow {
	now := time.Date(2026, 7, 9, 7, 30, 0, 0, time.UTC)
	assetID := int64(101)
	return &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:               201,
			TaskID:           301,
			AssetID:          &assetID,
			AssetType:        domain.TaskAssetTypeDelivery,
			FileName:         "delivery.png",
			FlowReviewStatus: domain.TaskAssetFlowReviewStatusApproved,
			CreatedAt:        now,
		},
		Task: &domain.Task{
			ID:                  301,
			TaskNo:              "RW-20260709-A-000001",
			TaskStatus:          domain.TaskStatusInProgress,
			ProductNameSnapshot: "缓存验证任务",
			CreatedAt:           now,
		},
		DesignCreatedAt: now,
		DesignUpdatedAt: now,
	}
}

type fakeAssetCenterCache struct {
	values map[string]string
	ttls   map[string]time.Duration
}

func newFakeAssetCenterCache() *fakeAssetCenterCache {
	return &fakeAssetCenterCache{
		values: map[string]string{},
		ttls:   map[string]time.Duration{},
	}
}

func (f *fakeAssetCenterCache) Get(_ context.Context, key string) *redis.StringCmd {
	if value, ok := f.values[key]; ok {
		return redis.NewStringResult(value, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (f *fakeAssetCenterCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	if f.values == nil {
		f.values = map[string]string{}
	}
	if f.ttls == nil {
		f.ttls = map[string]time.Duration{}
	}
	f.values[key] = value.(string)
	f.ttls[key] = ttl
	return redis.NewStatusResult("OK", nil)
}
