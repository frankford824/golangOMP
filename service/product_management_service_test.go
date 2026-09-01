package service

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"workflow/domain"
	"workflow/repo"
)

func TestProductManagementReconcileCostKeepsSystemAndERPValuesSeparate(t *testing.T) {
	systemCost := 8.6
	erpCost := 9.1
	record := &domain.ProductManagementRecord{
		ID: 31, SKUCode: "SKU-009", CostPrice: &systemCost,
		CostTrace: &domain.ProductManagementCostTrace{RuleName: "杯具面积规则"},
	}
	bridge := &productManagementERPBridgeCapture{
		readbackProduct: &domain.ERPProduct{SKUCode: "SKU-009", IID: "STYLE-009", ProductName: "北欧客厅组合", CostPrice: &erpCost},
	}
	svc := &productManagementService{
		records:   &productManagementRecordRepoFake{items: []*domain.ProductManagementRecord{record}},
		erpBridge: bridge,
		now:       func() time.Time { return time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC) },
	}

	result, appErr := svc.ReconcileCost(context.Background(), 31)
	if appErr != nil {
		t.Fatalf("ReconcileCost() appErr = %+v", appErr)
	}
	if result.Status != domain.ProductCostReconciliationMismatch {
		t.Fatalf("status = %s, want mismatched", result.Status)
	}
	if result.SystemCostPrice == nil || *result.SystemCostPrice != systemCost {
		t.Fatalf("system cost = %+v, want %.2f", result.SystemCostPrice, systemCost)
	}
	if result.ERPCostPrice == nil || *result.ERPCostPrice != erpCost {
		t.Fatalf("erp cost = %+v, want %.2f", result.ERPCostPrice, erpCost)
	}
	if result.CostDelta == nil || math.Abs(*result.CostDelta-0.5) > 0.0001 {
		t.Fatalf("delta = %+v, want 0.5", result.CostDelta)
	}
	if record.CostPrice == nil || *record.CostPrice != systemCost {
		t.Fatalf("reconciliation mutated retained system cost: %+v", record.CostPrice)
	}
}

func TestProductManagementSyncRecordToERPUsesProductNameAsShortName(t *testing.T) {
	assetID := int64(7301)
	versionID := int64(4395)
	asset := erpImageProxyTestAsset(versionID, "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/image.jpg")
	asset.AssetID = &assetID
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	expectedImageURL := signer.BuildImageURL(asset)
	bridge := &productManagementERPBridgeCapture{
		readbackProduct: &domain.ERPProduct{
			SKUCode:  "CGK000181",
			ImageURL: *expectedImageURL,
		},
	}
	svc := &productManagementService{
		assetSearch: productManagementAssetSearchStub{
			versionRow: &repo.TaskAssetSearchRow{Asset: asset},
		},
		imageProxy: signer,
		erpBridge:  bridge,
		now:        time.Now,
	}

	productName := strings.Repeat("产", ERPProductNameMaxLength)
	appErr := svc.syncRecordToERP(context.Background(), &domain.ProductManagementRecord{
		ID:                  1,
		TaskNo:              "RW-20260604-A-001114",
		SKUCode:             "CGK000181",
		ProductIID:          "KT板",
		ProductName:         productName,
		ImageAssetID:        &assetID,
		ImageAssetVersionID: &versionID,
	})
	if appErr != nil {
		t.Fatalf("syncRecordToERP() appErr = %+v", appErr)
	}
	if bridge.payload.Name != productName || bridge.payload.ProductName != productName {
		t.Fatalf("full product name was not preserved in payload: %+v", bridge.payload)
	}
	if bridge.payload.ShortName == "" {
		t.Fatal("ShortName is empty")
	}
	if bridge.payload.ShortName != productName {
		t.Fatalf("ShortName = %q, want full product name %q", bridge.payload.ShortName, productName)
	}
	if bridge.payload.ProductShortName != bridge.payload.ShortName {
		t.Fatalf("ProductShortName = %q, want ShortName %q", bridge.payload.ProductShortName, bridge.payload.ShortName)
	}
}

func TestProductManagementComboScopePaginatesComboGroups(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	records := []*domain.ProductManagementRecord{
		productManagementTestRecord(1, "SKU-A", now),
		productManagementTestRecord(2, "SKU-B", now),
		productManagementTestRecord(3, "SKU-C", now),
	}
	svc := &productManagementService{
		records: &productManagementRecordRepoFake{items: records},
		skuCombos: &skuComboRepoFake{
			relations: []*domain.OMPSKUComboRelationWithRecord{
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-1", ChildSKUCode: "SKU-A", Quantity: 2},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-1", Name: "组合 1", LastSyncedAt: now},
				},
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-1", ChildSKUCode: "SKU-B", Quantity: 1},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-1", Name: "组合 1", LastSyncedAt: now},
				},
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-2", ChildSKUCode: "SKU-C", Quantity: 1},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-2", Name: "组合 2", LastSyncedAt: now},
				},
			},
		},
		now: func() time.Time { return now },
	}

	result, appErr := svc.ListComboTree(context.Background(), repo.ProductManagementListFilter{
		DisplayScope: "combo",
		Page:         1,
		PageSize:     1,
	})
	if appErr != nil {
		t.Fatalf("ListComboTree() appErr = %+v", appErr)
	}
	if result.Pagination.Total != 2 {
		t.Fatalf("combo total = %d, want 2", result.Pagination.Total)
	}
	if len(result.Groups) != 1 || result.Groups[0].GroupKey != "combo:COMBO-1" {
		t.Fatalf("groups = %#v, want first combo group", result.Groups)
	}
	if len(result.Groups[0].Children) != 2 {
		t.Fatalf("first combo child count = %d, want 2", len(result.Groups[0].Children))
	}
	if len(result.Data) != 2 {
		t.Fatalf("page data count = %d, want combo child count 2", len(result.Data))
	}
}

func TestProductManagementAllScopeShowsExactIdentifierAsSingleRecord(t *testing.T) {
	now := time.Date(2026, 7, 2, 13, 0, 0, 0, time.UTC)
	record := productManagementTestRecord(1, "CGP000111", now)
	record.TaskNo = "RW-20260630-A-001931"
	svc := &productManagementService{
		records: &productManagementRecordRepoFake{items: []*domain.ProductManagementRecord{record}},
		skuCombos: &skuComboRepoFake{
			relations: []*domain.OMPSKUComboRelationWithRecord{
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-1", ChildSKUCode: "CGP000111", Quantity: 1},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-1", Name: "无关组合父级", LastSyncedAt: now},
				},
			},
		},
		now: func() time.Time { return now },
	}

	for _, keyword := range []string{"CGP000111", "RW-20260630-A-001931"} {
		result, appErr := svc.ListComboTree(context.Background(), repo.ProductManagementListFilter{
			Keyword:      keyword,
			DisplayScope: "all",
			Page:         1,
			PageSize:     20,
		})
		if appErr != nil {
			t.Fatalf("ListComboTree(%q) appErr = %+v", keyword, appErr)
		}
		if len(result.Groups) != 1 || result.Groups[0].GroupType != "single" {
			t.Fatalf("ListComboTree(%q) groups = %#v, want exact single record", keyword, result.Groups)
		}
		if got := result.Groups[0].Children[0].Record; got == nil || got.SKUCode != "CGP000111" {
			t.Fatalf("ListComboTree(%q) child = %#v, want CGP000111", keyword, got)
		}
	}
}

func TestProductManagementCostDashboardAddsLegacyFallbackPolicyState(t *testing.T) {
	svc := NewProductManagementService(
		&productManagementRecordRepoFake{},
		nil,
		nil,
		nil,
		WithProductManagementCostLegacyAliasFallbackEnabled(false),
	)
	result, appErr := svc.CostDashboard(context.Background())
	if appErr != nil {
		t.Fatalf("CostDashboard() appErr = %+v", appErr)
	}
	if result.LegacyFallbackEnabled {
		t.Fatal("LegacyFallbackEnabled = true, want false")
	}
	if result.LegacyFallbackMode != "disabled" {
		t.Fatalf("LegacyFallbackMode = %q, want disabled", result.LegacyFallbackMode)
	}
	if !strings.Contains(result.LegacyFallbackWarning, "已关闭") {
		t.Fatalf("LegacyFallbackWarning = %q, want disabled warning", result.LegacyFallbackWarning)
	}

	defaultSvc := NewProductManagementService(&productManagementRecordRepoFake{}, nil, nil, nil)
	defaultResult, appErr := defaultSvc.CostDashboard(context.Background())
	if appErr != nil {
		t.Fatalf("default CostDashboard() appErr = %+v", appErr)
	}
	if !defaultResult.LegacyFallbackEnabled || defaultResult.LegacyFallbackMode != "warn" {
		t.Fatalf("default fallback policy = enabled %t mode %q, want enabled warn", defaultResult.LegacyFallbackEnabled, defaultResult.LegacyFallbackMode)
	}
}

func TestProductManagementCostDashboardUsesRedisCacheBeforeRefresh(t *testing.T) {
	records := &productManagementRecordRepoFake{}
	cache := newFakeProductManagementCache()
	svc := NewProductManagementService(
		records,
		nil,
		nil,
		nil,
		WithProductManagementRedis(cache),
	)
	for i := 0; i < 2; i++ {
		result, appErr := svc.CostDashboard(context.Background())
		if appErr != nil {
			t.Fatalf("CostDashboard() appErr = %+v", appErr)
		}
		if result == nil || !result.LegacyFallbackEnabled {
			t.Fatalf("dashboard policy not decorated: %+v", result)
		}
	}
	if records.refreshCalls != 1 || records.dashboardCalls != 1 {
		t.Fatalf("repo calls refresh=%d dashboard=%d", records.refreshCalls, records.dashboardCalls)
	}
	if cache.ttls["omp:perf:product-management:cost-dashboard:v1:legacy=true"] != 5*time.Minute {
		t.Fatalf("dashboard ttl=%s", cache.ttls["omp:perf:product-management:cost-dashboard:v1:legacy=true"])
	}
}

func TestProductManagementThrottledRefreshKeepsCostDashboardCache(t *testing.T) {
	records := &productManagementRecordRepoFake{}
	cache := newFakeProductManagementCache()
	svc := NewProductManagementService(records, nil, nil, nil, WithProductManagementRedis(cache)).(*productManagementService)

	if _, appErr := svc.CostDashboard(context.Background()); appErr != nil {
		t.Fatalf("CostDashboard() appErr = %+v", appErr)
	}
	if _, ok := cache.values["omp:perf:product-management:cost-dashboard:v1:legacy=true"]; !ok {
		t.Fatal("cost dashboard cache not populated after first read")
	}

	// Simulate ordinary product-center traffic expiring the throttle window; the
	// background read-model refresh must not wipe the 5-minute dashboard cache.
	svc.lastRefresh = svc.now().Add(-svc.refreshEvery - time.Second)
	if appErr := svc.refreshReadModel(context.Background()); appErr != nil {
		t.Fatalf("refreshReadModel() appErr = %+v", appErr)
	}
	if _, ok := cache.values["omp:perf:product-management:cost-dashboard:v1:legacy=true"]; !ok {
		t.Fatal("throttled refresh must not invalidate cost dashboard cache")
	}

	if _, appErr := svc.CostDashboard(context.Background()); appErr != nil {
		t.Fatalf("CostDashboard() appErr = %+v", appErr)
	}
	if records.dashboardCalls != 1 {
		t.Fatalf("dashboard repo calls = %d, want 1 (second read served from cache)", records.dashboardCalls)
	}
}

func TestProductManagementExplicitRefreshKeepsCostDashboardCache(t *testing.T) {
	records := &productManagementRecordRepoFake{}
	cache := newFakeProductManagementCache()
	svc := NewProductManagementService(records, nil, nil, nil, WithProductManagementRedis(cache)).(*productManagementService)

	if _, appErr := svc.CostDashboard(context.Background()); appErr != nil {
		t.Fatalf("CostDashboard() appErr = %+v", appErr)
	}
	if len(cache.values) == 0 {
		t.Fatal("cost dashboard cache not populated after first read")
	}
	if appErr := svc.RefreshReadModelNow(context.Background()); appErr != nil {
		t.Fatalf("RefreshReadModelNow() appErr = %+v", appErr)
	}
	if _, ok := cache.values["omp:perf:product-management:cost-dashboard:v1:legacy=true"]; !ok {
		t.Fatal("explicit refresh must not invalidate cost dashboard cache")
	}
	if _, appErr := svc.CostDashboard(context.Background()); appErr != nil {
		t.Fatalf("CostDashboard() appErr = %+v", appErr)
	}
	if records.dashboardCalls != 1 {
		t.Fatalf("dashboard repo calls = %d, want 1 (second read served from cache)", records.dashboardCalls)
	}
}

func TestInvalidateProductManagementCostDashboardCacheDeletesPolicyKeys(t *testing.T) {
	cache := newFakeProductManagementCache()
	cache.values["omp:perf:product-management:cost-dashboard:v1:legacy=true"] = `{}`
	cache.values["omp:perf:product-management:cost-dashboard:v1:legacy=false"] = `{}`
	invalidateProductManagementCostDashboardCache(context.Background(), cache)
	if len(cache.values) != 0 {
		t.Fatalf("cache values after invalidation = %+v", cache.values)
	}
}

func TestProductManagementAreaTraceUsesSKUItemVariant(t *testing.T) {
	record := productManagementTestRecord(11, "CGK000011", time.Now())
	record.DimensionVariantJSON = json.RawMessage(`{"spec_text":"单个 160*125cm","size_text":"160*125cm","width":160,"height":125,"quantity":3}`)

	decorateProductManagementArea(record)

	if record.SpecText != "单个 160*125cm" || record.SizeText != "160*125cm" {
		t.Fatalf("spec/size = %q/%q, want variant spec and size", record.SpecText, record.SizeText)
	}
	if record.AreaTrace == nil || record.AreaTrace.AreaM2 == nil {
		t.Fatal("area trace was not generated")
	}
	if math.Abs(*record.AreaTrace.AreaM2-6) > 0.000001 {
		t.Fatalf("area = %.6f, want 6", *record.AreaTrace.AreaM2)
	}
	if record.AreaTrace.WidthM == nil || record.AreaTrace.HeightM == nil || math.Abs(*record.AreaTrace.WidthM-1.6) > 0.000001 || math.Abs(*record.AreaTrace.HeightM-1.25) > 0.000001 {
		t.Fatalf("width/height = %+v/%+v, want 1.6m/1.25m", record.AreaTrace.WidthM, record.AreaTrace.HeightM)
	}
	if record.AreaTrace.Source != "sku_item_variant" {
		t.Fatalf("source = %q, want sku_item_variant", record.AreaTrace.Source)
	}
	if !strings.Contains(record.AreaTrace.Formula, "数量") {
		t.Fatalf("formula = %q, want quantity formula", record.AreaTrace.Formula)
	}
}

func TestProductManagementAreaTraceExtractsFromProductText(t *testing.T) {
	record := productManagementTestRecord(12, "CGK000012", time.Now())
	record.ProductName = "常规kt板/毕业手举牌/160*125cm"

	decorateProductManagementArea(record)

	if record.AreaTrace == nil || record.AreaTrace.AreaM2 == nil {
		t.Fatal("area trace was not generated")
	}
	if math.Abs(*record.AreaTrace.AreaM2-2) > 0.000001 {
		t.Fatalf("area = %.6f, want 2", *record.AreaTrace.AreaM2)
	}
	if record.AreaTrace.Source != "text_extractor" {
		t.Fatalf("source = %q, want text_extractor", record.AreaTrace.Source)
	}
	if record.AreaTrace.Confidence != "low" {
		t.Fatalf("confidence = %q, want low for product-name extraction", record.AreaTrace.Confidence)
	}
}

func TestProductManagementAreaTraceMarksMissingDimensions(t *testing.T) {
	record := productManagementTestRecord(13, "CGK000013", time.Now())
	record.ProductName = "常规kt板/无尺寸任务"

	decorateProductManagementArea(record)

	if record.AreaTrace == nil {
		t.Fatal("area trace was not generated")
	}
	if record.AreaTrace.AreaM2 != nil {
		t.Fatalf("area = %.6f, want nil", *record.AreaTrace.AreaM2)
	}
	if record.AreaTrace.Source != "missing" || record.AreaTrace.Warning == "" {
		t.Fatalf("trace = %+v, want missing warning", record.AreaTrace)
	}
}

func TestProductManagementBaseSyncSuccessMarksTaskProjection(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 40, 0, 0, time.UTC)
	skuItemID := int64(1492)
	records := &productManagementRecordRepoFake{}
	svc := &productManagementService{
		records:  records,
		txRunner: productManagementUnitTxRunner{},
		now:      func() time.Time { return now },
	}

	svc.markProductManagementBaseSyncSucceeded(context.Background(), &domain.ProductManagementRecord{
		ID:            6416,
		TaskID:        1497,
		TaskSKUItemID: &skuItemID,
		SKUCode:       "CGO000165",
	})

	if records.updatedBaseID != 6416 {
		t.Fatalf("updated base id = %d, want 6416", records.updatedBaseID)
	}
	if records.updatedBasePatch.BaseStatus != domain.ProductManagementERPSyncStatusSynced {
		t.Fatalf("base status = %s, want synced", records.updatedBasePatch.BaseStatus)
	}
	if records.projectionTaskID != 1497 {
		t.Fatalf("projection task id = %d, want 1497", records.projectionTaskID)
	}
	if records.projectionSKUItemID == nil || *records.projectionSKUItemID != skuItemID {
		t.Fatalf("projection sku item id = %+v, want %d", records.projectionSKUItemID, skuItemID)
	}
	if !records.projectionNow.Equal(now) {
		t.Fatalf("projection now = %s, want %s", records.projectionNow, now)
	}
}

func TestProductManagementCombinedSyncSuccessMarksTaskProjection(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 45, 0, 0, time.UTC)
	skuItemID := int64(1493)
	records := &productManagementRecordRepoFake{}
	svc := &productManagementService{
		records:  records,
		txRunner: productManagementUnitTxRunner{},
		now:      func() time.Time { return now },
	}

	svc.markProductManagementSyncSucceeded(context.Background(), &domain.ProductManagementRecord{
		ID:            6417,
		TaskID:        1498,
		TaskSKUItemID: &skuItemID,
		SKUCode:       "CGO000166",
	})

	if records.projectionTaskID != 1498 {
		t.Fatalf("projection task id = %d, want 1498", records.projectionTaskID)
	}
	if records.projectionSKUItemID == nil || *records.projectionSKUItemID != skuItemID {
		t.Fatalf("projection sku item id = %+v, want %d", records.projectionSKUItemID, skuItemID)
	}
}

func TestProductManagementBaseSyncTreatsTimeoutAsSuccessWhenReadbackMatches(t *testing.T) {
	previousSleeper := productManagementERPBaseReadbackSleep
	productManagementERPBaseReadbackSleep = func(time.Duration) {}
	defer func() { productManagementERPBaseReadbackSleep = previousSleeper }()

	cost := 20.328
	productName := "张三常规KT板/端午节/180*80cm"
	bridge := &productManagementERPBridgeCapture{
		upsertErr: domain.NewAppError(domain.ErrCodeInternalError, "erp bridge request timed out", nil),
		readbackProduct: &domain.ERPProduct{
			SKUCode:     "CGK000329",
			IID:         "常规kt板",
			ProductName: productName,
			CostPrice:   &cost,
		},
	}
	svc := &productManagementService{
		erpBridge: bridge,
		now:       time.Now,
	}

	appErr := svc.syncBaseRecordToERP(context.Background(), &domain.ProductManagementRecord{
		ID:          6420,
		TaskNo:      "RW-20260620-A-001600",
		SKUCode:     "CGK000329",
		ProductIID:  "常规kt板",
		ProductName: productName,
		CostPrice:   &cost,
	})
	if appErr != nil {
		t.Fatalf("syncBaseRecordToERP() appErr = %+v, want nil after matching readback", appErr)
	}
	if bridge.upsertCalls != 1 {
		t.Fatalf("UpsertProduct calls = %d, want 1", bridge.upsertCalls)
	}
}

func productManagementTestRecord(id int64, sku string, now time.Time) *domain.ProductManagementRecord {
	return &domain.ProductManagementRecord{
		ID:                 id,
		RecordKey:          sku,
		TaskID:             100 + id,
		TaskNo:             "RW-TEST",
		SKUCode:            sku,
		ProductName:        sku + " product",
		CreatorID:          1,
		CreatorName:        "tester",
		TaskCreatedAt:      now,
		ImageSource:        domain.ProductManagementImageSourceManual,
		ImageSelectionMode: domain.ProductManagementImageSelectionManual,
		ERPSyncStatus:      domain.ProductManagementERPSyncStatusPendingSync,
		BaseSyncStatus:     domain.ProductManagementERPSyncStatusPendingSync,
		ImageSyncStatus:    domain.ProductManagementERPSyncStatusWaitingImage,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

type fakeProductManagementCache struct {
	values map[string]string
	ttls   map[string]time.Duration
}

func newFakeProductManagementCache() *fakeProductManagementCache {
	return &fakeProductManagementCache{
		values: map[string]string{},
		ttls:   map[string]time.Duration{},
	}
}

func (f *fakeProductManagementCache) Get(_ context.Context, key string) *redis.StringCmd {
	if value, ok := f.values[key]; ok {
		return redis.NewStringResult(value, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (f *fakeProductManagementCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	switch raw := value.(type) {
	case []byte:
		f.values[key] = string(raw)
	case string:
		f.values[key] = raw
	default:
		f.values[key] = ""
	}
	f.ttls[key] = ttl
	return redis.NewStatusResult("OK", nil)
}

func (f *fakeProductManagementCache) Del(_ context.Context, keys ...string) *redis.IntCmd {
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(f.values, key)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

type productManagementRecordRepoFake struct {
	items               []*domain.ProductManagementRecord
	refreshCalls        int
	dashboardCalls      int
	updatedBaseID       int64
	updatedBasePatch    repo.ProductManagementSyncPatch
	projectionTaskID    int64
	projectionSKUItemID *int64
	projectionNow       time.Time
}

func (f *productManagementRecordRepoFake) RefreshReadModel(context.Context) error {
	f.refreshCalls++
	return nil
}

func (f *productManagementRecordRepoFake) List(_ context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, int64, error) {
	page, pageSize := normalizeProductManagementPage(filter.Page, filter.PageSize)
	start := (page - 1) * pageSize
	if start >= len(f.items) {
		return []*domain.ProductManagementRecord{}, int64(len(f.items)), nil
	}
	end := start + pageSize
	if end > len(f.items) {
		end = len(f.items)
	}
	return f.items[start:end], int64(len(f.items)), nil
}

func (f *productManagementRecordRepoFake) CostDashboard(context.Context) (*domain.ProductCostDashboardResponse, error) {
	f.dashboardCalls++
	return &domain.ProductCostDashboardResponse{}, nil
}

func (f *productManagementRecordRepoFake) GetByID(_ context.Context, id int64) (*domain.ProductManagementRecord, error) {
	for _, item := range f.items {
		if item != nil && item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}

func (f *productManagementRecordRepoFake) GetByTaskID(_ context.Context, taskID int64) ([]*domain.ProductManagementRecord, error) {
	var out []*domain.ProductManagementRecord
	for _, item := range f.items {
		if item != nil && item.TaskID == taskID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *productManagementRecordRepoFake) ClaimQueuedSyncRecords(context.Context, int, string, time.Time) ([]*domain.ProductManagementRecord, error) {
	return nil, nil
}

func (f *productManagementRecordRepoFake) QueuePendingBaseSyncByTaskID(context.Context, repo.Tx, int64, time.Time, time.Time) (int64, error) {
	return 0, nil
}

func (f *productManagementRecordRepoFake) UpdateImage(context.Context, repo.Tx, int64, repo.ProductManagementImagePatch) error {
	return nil
}

func (f *productManagementRecordRepoFake) UpdateSyncStatus(context.Context, repo.Tx, int64, repo.ProductManagementSyncPatch) error {
	return nil
}

func (f *productManagementRecordRepoFake) UpdateBaseSyncStatus(_ context.Context, _ repo.Tx, id int64, patch repo.ProductManagementSyncPatch) error {
	f.updatedBaseID = id
	f.updatedBasePatch = patch
	return nil
}

func (f *productManagementRecordRepoFake) UpdateImageSyncStatus(context.Context, repo.Tx, int64, repo.ProductManagementSyncPatch) error {
	return nil
}

func (f *productManagementRecordRepoFake) MarkBaseSyncProjectionSynced(_ context.Context, _ repo.Tx, taskID int64, taskSKUItemID *int64, now time.Time) error {
	f.projectionTaskID = taskID
	if taskSKUItemID != nil {
		id := *taskSKUItemID
		f.projectionSKUItemID = &id
	}
	f.projectionNow = now
	return nil
}

type productManagementUnitTx struct{}

func (productManagementUnitTx) IsTx() {}

type productManagementUnitTxRunner struct{}

func (productManagementUnitTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(productManagementUnitTx{})
}

type productManagementERPBridgeCapture struct {
	payload          domain.ERPProductUpsertPayload
	upsertErr        *domain.AppError
	upsertCalls      int
	itemStylePayload domain.ERPItemStyleUpdatePayload
	itemStyleCalls   int
	readbackProduct  *domain.ERPProduct
	readbackErr      *domain.AppError
	combineResponse  *domain.JSTCombineSKUListResponse
	combineErr       *domain.AppError
	combineCalls     int
}

func (s *productManagementERPBridgeCapture) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) ListIIDs(context.Context, domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	return s.readbackProduct, s.readbackErr
}

func (s *productManagementERPBridgeCapture) QueryCombineSKUs(context.Context, domain.JSTCombineSKUFilter) (*domain.JSTCombineSKUListResponse, *domain.AppError) {
	s.combineCalls++
	if s.combineErr != nil {
		return nil, s.combineErr
	}
	if s.combineResponse != nil {
		return s.combineResponse, nil
	}
	return &domain.JSTCombineSKUListResponse{
		Items:      []domain.JSTCombineSKUItem{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 50, Total: 0},
	}, nil
}

func (s *productManagementERPBridgeCapture) ListCategories(context.Context) ([]*domain.ERPCategory, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) ListWarehouses(context.Context) ([]domain.ERPWarehouse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) EnsureLocalProduct(context.Context, repo.Tx, *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) UpsertProduct(_ context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	s.payload = payload
	s.upsertCalls++
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.ERPProductUpsertResult{Status: "accepted"}, nil
}

func (s *productManagementERPBridgeCapture) UpdateItemStyle(_ context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	s.itemStylePayload = payload
	s.itemStyleCalls++
	return &domain.ERPItemStyleUpdateResult{Status: "accepted"}, nil
}

func (s *productManagementERPBridgeCapture) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) ListJSTUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError) {
	return nil, nil
}

func TestProductManagementVerifyERPImageReadbackRejectsNonPublicImage(t *testing.T) {
	previousSleeper := productManagementERPImageReadbackSleep
	productManagementERPImageReadbackSleep = func(time.Duration) {}
	defer func() { productManagementERPImageReadbackSleep = previousSleeper }()

	svc := &productManagementService{
		erpBridge: &productManagementERPBridgeCapture{
			readbackProduct: &domain.ERPProduct{
				SKUCode:  "CGG000038",
				ImageURL: "/v1/assets/files/tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/image.png",
			},
		},
	}
	appErr := svc.verifyERPImageReadback(context.Background(), &domain.ProductManagementRecord{SKUCode: "CGG000038"})
	if appErr == nil {
		t.Fatal("verifyERPImageReadback() appErr = nil, want non-public image failure")
	}
	if !strings.Contains(appErr.Message, "不是公网地址") {
		t.Fatalf("verifyERPImageReadback() message = %q", appErr.Message)
	}
}

func TestProductManagementVerifyERPImageReadbackRejectsDifferentAssetVersion(t *testing.T) {
	previousSleeper := productManagementERPImageReadbackSleep
	productManagementERPImageReadbackSleep = func(time.Duration) {}
	defer func() { productManagementERPImageReadbackSleep = previousSleeper }()

	svc := &productManagementService{
		erpBridge: &productManagementERPBridgeCapture{
			readbackProduct: &domain.ERPProduct{
				SKUCode:  "CGD000061",
				ImageURL: "https://yongbo.cloud/v1/public/erp-product-images/32799?exp=2&sig=old",
			},
		},
	}
	appErr := svc.verifyERPImageReadback(
		context.Background(),
		&domain.ProductManagementRecord{SKUCode: "CGD000061"},
		"https://yongbo.cloud/v1/public/erp-product-images/33204?exp=1&sig=new",
	)
	if appErr == nil {
		t.Fatal("verifyERPImageReadback() appErr = nil, want mismatched image failure")
	}
	if !strings.Contains(appErr.Message, "与本次同步图片不一致") {
		t.Fatalf("verifyERPImageReadback() message = %q", appErr.Message)
	}
}

func TestProductManagementBestAssetsBySourceRequiresExactSKU(t *testing.T) {
	assetID := int64(10)
	exactSKU := "CGD000061"
	wrongSKU := "CGK001293"
	mimeType := "image/webp"
	assets := []*domain.TaskAsset{
		{ID: 1, AssetID: &assetID, TaskID: 99, AssetType: domain.TaskAssetTypePreview, MimeType: &mimeType},
		{ID: 2, AssetID: &assetID, TaskID: 99, ScopeSKUCode: &wrongSKU, AssetType: domain.TaskAssetTypePreview, MimeType: &mimeType},
		{ID: 3, AssetID: &assetID, TaskID: 99, ScopeSKUCode: &exactSKU, AssetType: domain.TaskAssetTypePreview, MimeType: &mimeType},
	}

	got := bestAssetsBySource(assets, exactSKU, domain.ProductManagementImageSourceDerivedPreview)
	if len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("bestAssetsBySource() = %#v, want only exact SKU asset 3", got)
	}
}

func TestProductManagementFinalizedImageCandidateUsesDerivedPreviewOfFinal(t *testing.T) {
	finalAssetID := int64(51)
	previewAssetID := int64(52)
	finalVersionID := int64(410)
	previewVersionID := int64(411)
	sku := "CGD000061"
	tiff := "image/tiff"
	webp := "image/webp"
	finalStorage := "tasks/99/final.tif"
	previewStorage := "tasks/99/preview.webp"
	assets := []*domain.TaskAsset{
		{ID: finalVersionID, TaskID: 99, AssetID: &finalAssetID, ScopeSKUCode: &sku, AssetType: domain.TaskAssetTypeDelivery, FileName: "final.tif", MimeType: &tiff, StorageKey: &finalStorage},
		{ID: previewVersionID, TaskID: 99, AssetID: &previewAssetID, ScopeSKUCode: &sku, AssetType: domain.TaskAssetTypePreview, FileName: "preview.webp", MimeType: &webp, StorageKey: &previewStorage, SourceAssetVersionID: &finalVersionID},
	}
	svc := &productManagementService{
		taskAssets: &productManagementTaskAssetRepoFake{items: assets},
		finalizedResources: productManagementFinalizedResourceRepoStub{items: []domain.FlatResourceItem{
			{TaskID: 99, TaskAssetID: finalVersionID, SKUCode: sku, ResourceRole: domain.ResourceRoleFilterFinal},
		}},
	}

	got, err := svc.finalizedImageCandidatesForRecord(context.Background(), &domain.ProductManagementRecord{
		TaskID: 99, TaskNo: "RW-99", SKUCode: sku,
	}, nil)
	if err != nil {
		t.Fatalf("finalizedImageCandidatesForRecord() error = %v", err)
	}
	if len(got) != 1 || got[0].AssetVersionID != previewVersionID || got[0].Source != domain.ProductManagementImageSourceDerivedPreview {
		t.Fatalf("finalized candidates = %#v, want derived preview version %d", got, previewVersionID)
	}
}

func TestProductManagementSyncImageUsesProductUpsertWithImageFields(t *testing.T) {
	assetID := int64(7302)
	versionID := int64(4396)
	asset := erpImageProxyTestAsset(versionID, "tasks/RW-20260603-A-001081/assets/AST-0006/v1/delivery/image.jpg")
	asset.AssetID = &assetID
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	expectedImageURL := signer.BuildImageURL(asset)
	bridge := &productManagementERPBridgeCapture{
		readbackProduct: &domain.ERPProduct{
			SKUCode:  "NSAC000001",
			IID:      "定制亚克力",
			ImageURL: *expectedImageURL,
		},
	}
	svc := &productManagementService{
		assetSearch: productManagementAssetSearchStub{
			versionRow: &repo.TaskAssetSearchRow{Asset: asset},
		},
		imageProxy: signer,
		erpBridge:  bridge,
		now:        time.Now,
	}

	longHistoricalName := strings.Repeat("旧", ERPProductNameMaxLength+5)
	appErr := svc.syncImageRecordToERP(context.Background(), &domain.ProductManagementRecord{
		ID:                  2,
		TaskNo:              "RW-20260604-A-001115",
		SKUCode:             "NSAC000001",
		ProductName:         longHistoricalName,
		ImageAssetID:        &assetID,
		ImageAssetVersionID: &versionID,
	})
	if appErr != nil {
		t.Fatalf("syncImageRecordToERP() appErr = %+v", appErr)
	}
	if bridge.upsertCalls != 1 {
		t.Fatalf("UpsertProduct calls = %d, want 1", bridge.upsertCalls)
	}
	if bridge.itemStyleCalls != 0 {
		t.Fatalf("image sync used UpdateItemStyle %d times, want 0", bridge.itemStyleCalls)
	}
	expectedERPName := truncateERPShortName(longHistoricalName, ERPProductNameMaxLength)
	if bridge.payload.Name != expectedERPName || bridge.payload.ProductName != expectedERPName {
		t.Fatalf("image upsert payload should use an ERP-safe historical name: %+v", bridge.payload)
	}
	if bridge.payload.SKUID != "NSAC000001" || bridge.payload.SKUCode != "NSAC000001" || bridge.payload.IID != "定制亚克力" {
		t.Fatalf("image upsert payload identifiers = sku:%q sku_code:%q iid:%q", bridge.payload.SKUID, bridge.payload.SKUCode, bridge.payload.IID)
	}
	if bridge.payload.Pic == "" || bridge.payload.PicBig == "" || bridge.payload.SKUPic == "" {
		t.Fatalf("image upsert payload missing image fields: %+v", bridge.payload)
	}
}

func TestProductManagementERPImageReadbackPendingClassification(t *testing.T) {
	pending := domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验未通过：ERP 尚未返回商品图", nil)
	if !isProductManagementERPImageReadbackPending(pending) {
		t.Fatal("expected missing image readback to be classified as pending")
	}
	notFound := domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验未通过：Resource not found.", nil)
	if isProductManagementERPImageReadbackPending(notFound) {
		t.Fatal("resource not found must stay a hard failure")
	}
	transient := domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步前未找到该 SKU：erp bridge upstream business rejected request", nil)
	if !isProductManagementERPImageSyncRetryable(transient) {
		t.Fatal("upstream business rejection should be retried")
	}
	mismatch := domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验未通过：ERP 返回的图片与本次同步图片不一致", nil)
	if !isProductManagementERPImageSyncRetryable(mismatch) {
		t.Fatal("stale ERP image readback should be retried")
	}
}

func TestProductManagementDecoratedImageSyncStatusRequiresVerifiedTimestamp(t *testing.T) {
	got := productManagementDecoratedImageSyncStatus(&domain.ProductManagementRecord{
		ImageSyncStatus: domain.ProductManagementERPSyncStatusSynced,
	}, repo.ProductManagementImagePatch{
		ImageSyncStatus: domain.ProductManagementERPSyncStatusSynced,
	}, false)
	if got != domain.ProductManagementERPSyncStatusPendingSync {
		t.Fatalf("decorated status = %q, want %q", got, domain.ProductManagementERPSyncStatusPendingSync)
	}

	now := time.Now()
	got = productManagementDecoratedImageSyncStatus(&domain.ProductManagementRecord{
		ImageSyncStatus:   domain.ProductManagementERPSyncStatusSynced,
		LastImageSyncedAt: &now,
	}, repo.ProductManagementImagePatch{
		ImageSyncStatus: domain.ProductManagementERPSyncStatusSynced,
	}, false)
	if got != domain.ProductManagementERPSyncStatusSynced {
		t.Fatalf("decorated verified status = %q, want %q", got, domain.ProductManagementERPSyncStatusSynced)
	}
}

type productManagementFinalizedResourceRepoStub struct {
	items []domain.FlatResourceItem
	err   error
}

func (s productManagementFinalizedResourceRepoStub) ListFlatResourceItems(context.Context, domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64, error) {
	return s.items, int64(len(s.items)), s.err
}

type productManagementTaskAssetRepoFake struct {
	items []*domain.TaskAsset
}

func (f *productManagementTaskAssetRepoFake) Create(context.Context, repo.Tx, *domain.TaskAsset) (int64, error) {
	return 0, nil
}

func (f *productManagementTaskAssetRepoFake) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	for _, item := range f.items {
		if item != nil && item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}

func (f *productManagementTaskAssetRepoFake) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	items := make([]*domain.TaskAsset, 0, len(f.items))
	for _, item := range f.items {
		if item != nil && item.TaskID == taskID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *productManagementTaskAssetRepoFake) ListByAssetID(_ context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	items := make([]*domain.TaskAsset, 0, len(f.items))
	for _, item := range f.items {
		if item != nil && item.AssetID != nil && *item.AssetID == assetID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *productManagementTaskAssetRepoFake) NextVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 1, nil
}

func (f *productManagementTaskAssetRepoFake) NextAssetVersionNo(context.Context, repo.Tx, int64) (int, error) {
	return 1, nil
}
