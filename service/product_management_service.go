package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"workflow/domain"
	"workflow/repo"
)

type ProductManagementService interface {
	List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError)
	ListComboTree(ctx context.Context, filter repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError)
	CostDashboard(ctx context.Context) (*domain.ProductCostDashboardResponse, *domain.AppError)
	ReconcileCost(ctx context.Context, recordID int64) (*domain.ProductCostReconciliation, *domain.AppError)
	GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, *domain.AppError)
	ListImageCandidates(ctx context.Context, actor domain.RequestActor, recordID int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError)
	ReparseImage(ctx context.Context, actor domain.RequestActor, recordID int64) (*domain.ProductManagementRecord, *domain.AppError)
	SetManualImage(ctx context.Context, actor domain.RequestActor, recordID int64, assetID int64) (*domain.ProductManagementRecord, *domain.AppError)
	RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	RequestBaseSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	RequestImageSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError)
	AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError
	RefreshReadModelNow(ctx context.Context) *domain.AppError
	QueuePendingBaseSyncForTask(ctx context.Context, taskID int64) (int, *domain.AppError)
	ProcessQueuedERPSync(ctx context.Context, limit int) (int, *domain.AppError)
}

type ProductManagementServiceOption func(*productManagementService)

type productManagementFinalizedResourceRepo interface {
	ListFlatResourceItems(ctx context.Context, params domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64, error)
}

type productManagementService struct {
	records                        repo.ProductManagementRepo
	taskAssets                     repo.TaskAssetRepo
	assetSearch                    repo.TaskAssetSearchRepo
	finalizedResources             productManagementFinalizedResourceRepo
	taskEvents                     repo.TaskEventRepo
	skuCombos                      repo.SKUComboRepo
	costRuns                       repo.CostRecalculationRunRepo
	txRunner                       repo.TxRunner
	erpBridge                      ERPBridgeService
	ossDirect                      *OSSDirectService
	uploadClient                   UploadServiceClient
	imageProxy                     *ERPImageProxySigner
	notifications                  taskNotificationService
	cache                          productManagementCache
	now                            func() time.Time
	refreshEvery                   time.Duration
	costDashboardCacheTTL          time.Duration
	costLegacyAliasFallbackEnabled bool
	refreshMu                      sync.Mutex
	lastRefresh                    time.Time
}

func NewProductManagementService(records repo.ProductManagementRepo, taskAssets repo.TaskAssetRepo, assetSearch repo.TaskAssetSearchRepo, txRunner repo.TxRunner, opts ...ProductManagementServiceOption) ProductManagementService {
	s := &productManagementService{
		records:                        records,
		taskAssets:                     taskAssets,
		assetSearch:                    assetSearch,
		txRunner:                       txRunner,
		now:                            time.Now,
		refreshEvery:                   30 * time.Second,
		costDashboardCacheTTL:          5 * time.Minute,
		costLegacyAliasFallbackEnabled: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func WithProductManagementERPBridge(erpBridge ERPBridgeService) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.erpBridge = erpBridge
	}
}

func WithProductManagementAssetURLServices(ossDirect *OSSDirectService, uploadClient UploadServiceClient) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.ossDirect = ossDirect
		s.uploadClient = uploadClient
	}
}

func WithProductManagementERPImageProxy(imageProxy *ERPImageProxySigner) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.imageProxy = imageProxy
	}
}

func WithProductManagementFinalizedResources(resources productManagementFinalizedResourceRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.finalizedResources = resources
	}
}

func WithProductManagementTaskEventRepo(taskEvents repo.TaskEventRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.taskEvents = taskEvents
	}
}

func WithProductManagementSKUComboRepo(skuCombos repo.SKUComboRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.skuCombos = skuCombos
	}
}

func WithProductManagementNotificationService(notifications taskNotificationService) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.notifications = notifications
	}
}

type productManagementCache interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
}

func WithProductManagementRedis(cache productManagementCache) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.cache = cache
	}
}

func WithProductManagementCostRecalculationRunRepo(costRuns repo.CostRecalculationRunRepo) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.costRuns = costRuns
	}
}

func WithProductManagementCostLegacyAliasFallbackEnabled(enabled bool) ProductManagementServiceOption {
	return func(s *productManagementService) {
		s.costLegacyAliasFallbackEnabled = enabled
	}
}

func (s *productManagementService) List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError) {
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}
	items, total, err := s.records.List(ctx, filter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraAppError("list product management records", err)
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	s.decorateRecords(ctx, actor, items)
	page, pageSize := normalizeProductManagementPage(filter.Page, filter.PageSize)
	return items, domain.PaginationMeta{Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *productManagementService) ListComboTree(ctx context.Context, filter repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError) {
	displayScope := normalizeProductManagementDisplayScope(filter.DisplayScope)
	if displayScope == "combo" {
		return s.listComboTreeByComboGroups(ctx, filter)
	}
	items, meta, appErr := s.List(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	groups := s.productManagementComboGroups(ctx, items)
	if displayScope == "single" || hasExactProductManagementIdentifier(items, filter.Keyword) {
		groups = productManagementSingleGroups(items)
	}
	summary, appErr := s.productManagementComboSyncSummary(ctx)
	if appErr != nil {
		return nil, appErr
	}
	return &domain.ProductManagementComboTreeResponse{
		Groups:      groups,
		Data:        items,
		Pagination:  meta,
		SyncSummary: summary,
	}, nil
}

func (s *productManagementService) CostDashboard(ctx context.Context) (*domain.ProductCostDashboardResponse, *domain.AppError) {
	if cached, ok := s.getCostDashboardCache(ctx); ok {
		return cached, nil
	}
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, appErr
	}
	result, err := s.records.CostDashboard(ctx)
	if err != nil {
		return nil, infraAppError("get product cost dashboard", err)
	}
	s.decorateCostDashboardPolicy(result)
	s.setCostDashboardCache(ctx, result)
	return result, nil
}

func (s *productManagementService) ReconcileCost(ctx context.Context, recordID int64) (*domain.ProductCostReconciliation, *domain.AppError) {
	if recordID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record id must be positive", nil)
	}
	record, err := s.records.GetByID(ctx, recordID)
	if errors.Is(err, repo.ErrNotFound) || record == nil {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, infraAppError("get product cost reconciliation record", err)
	}
	checkedAt := s.now().UTC()
	result := &domain.ProductCostReconciliation{
		ProductManagementRecordID: record.ID,
		SKUCode:                   strings.TrimSpace(record.SKUCode),
		SystemCostPrice:           cloneFloat64Ptr(record.CostPrice),
		Status:                    domain.ProductCostReconciliationUnavailable,
		CheckedAt:                 checkedAt,
		Message:                   "ERP 实际成本暂不可核对",
		SystemTrace:               cloneProductManagementCostTrace(record.CostTrace),
	}
	if s.erpBridge == nil {
		result.Message = "ERP 成本核对服务未配置；系统计算成本未被修改"
		return result, nil
	}
	if result.SKUCode == "" {
		result.Message = "当前记录缺少 SKU 编码，无法读取 ERP 实际成本"
		return result, nil
	}
	product, appErr := s.erpBridge.GetProductByID(ctx, result.SKUCode)
	if appErr != nil || product == nil {
		if appErr != nil && strings.TrimSpace(appErr.Message) != "" {
			result.Message = "ERP 实际成本读取失败：" + strings.TrimSpace(appErr.Message)
		} else {
			result.Message = "ERP 未返回该 SKU；系统计算成本未被修改"
		}
		return result, nil
	}
	result.ERPCostPrice = cloneFloat64Ptr(product.CostPrice)
	result.ERPProductIID = strings.TrimSpace(product.IID)
	result.ERPProductName = strings.TrimSpace(firstNonEmptyString(product.ProductName, product.Name))
	switch {
	case result.SystemCostPrice == nil && result.ERPCostPrice == nil:
		result.Status = domain.ProductCostReconciliationSystemMissing
		result.Message = "系统与 ERP 均没有成本，需先补齐计价输入或人工成本"
	case result.SystemCostPrice == nil:
		result.Status = domain.ProductCostReconciliationSystemMissing
		result.Message = "ERP 已有实际成本，但系统没有可解释的成本快照"
	case result.ERPCostPrice == nil:
		result.Status = domain.ProductCostReconciliationERPMissing
		result.Message = "系统已有计算成本，但 ERP 未返回成本"
	default:
		delta := *result.ERPCostPrice - *result.SystemCostPrice
		result.CostDelta = &delta
		if math.Abs(delta) <= productManagementERPBaseCostTolerance {
			result.Status = domain.ProductCostReconciliationMatched
			result.Message = "系统计算成本与 ERP 实际成本一致"
		} else {
			result.Status = domain.ProductCostReconciliationMismatch
			result.Message = "ERP 实际成本与系统计算成本不一致；两边均保留，不会被本次核对静默覆盖"
		}
	}
	return result, nil
}

func cloneProductManagementCostTrace(trace *domain.ProductManagementCostTrace) *domain.ProductManagementCostTrace {
	if trace == nil {
		return nil
	}
	copied := *trace
	copied.InputSnapshot = append(json.RawMessage(nil), trace.InputSnapshot...)
	copied.CalculationSnapshot = append(json.RawMessage(nil), trace.CalculationSnapshot...)
	return &copied
}

func (s *productManagementService) decorateCostDashboardPolicy(result *domain.ProductCostDashboardResponse) {
	if result == nil {
		return
	}
	result.LegacyFallbackEnabled = s.costLegacyAliasFallbackEnabled
	if s.costLegacyAliasFallbackEnabled {
		result.LegacyFallbackMode = "warn"
		result.LegacyFallbackWarning = "未关联款式仍会按名称猜价，请优先把款式关联到定价规则，降低后续成本偏差。"
	} else {
		result.LegacyFallbackMode = "disabled"
		result.LegacyFallbackWarning = "未关联款式猜价已关闭，未关联记录会进入算不出来的成本问题。"
	}
}

func (s *productManagementService) getCostDashboardCache(ctx context.Context) (*domain.ProductCostDashboardResponse, bool) {
	if s.cache == nil {
		return nil, false
	}
	raw, err := s.cache.Get(ctx, s.costDashboardCacheKey()).Bytes()
	if err != nil {
		return nil, false
	}
	var cached domain.ProductCostDashboardResponse
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, false
	}
	return &cached, true
}

func (s *productManagementService) setCostDashboardCache(ctx context.Context, result *domain.ProductCostDashboardResponse) {
	if s.cache == nil || result == nil || s.costDashboardCacheTTL <= 0 {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = s.cache.Set(ctx, s.costDashboardCacheKey(), raw, s.costDashboardCacheTTL).Err()
}

func (s *productManagementService) invalidateCostDashboardCache(ctx context.Context) {
	invalidateProductManagementCostDashboardCache(ctx, s.cache)
}

func invalidateProductManagementCostDashboardCache(ctx context.Context, cache productManagementCache) {
	if cache == nil {
		return
	}
	_ = cache.Del(ctx,
		"omp:perf:product-management:cost-dashboard:v1:legacy=true",
		"omp:perf:product-management:cost-dashboard:v1:legacy=false",
	).Err()
}

func (s *productManagementService) costDashboardCacheKey() string {
	if s.costLegacyAliasFallbackEnabled {
		return "omp:perf:product-management:cost-dashboard:v1:legacy=true"
	}
	return "omp:perf:product-management:cost-dashboard:v1:legacy=false"
}

func (s *productManagementService) listComboTreeByComboGroups(ctx context.Context, filter repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError) {
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, appErr
	}
	items, appErr := s.collectProductManagementRecordsForGrouping(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	groups := s.productManagementComboGroups(ctx, items)
	comboGroups := make([]domain.ProductManagementComboGroup, 0, len(groups))
	for _, group := range groups {
		if group.GroupType == "combo" {
			comboGroups = append(comboGroups, group)
		}
	}
	page, pageSize := normalizeProductManagementPage(filter.Page, filter.PageSize)
	pagedGroups := paginateProductManagementComboGroups(comboGroups, page, pageSize)
	data := productManagementRecordsFromComboGroups(pagedGroups)
	actor, _ := domain.RequestActorFromContext(ctx)
	s.decorateRecords(ctx, actor, data)
	summary, appErr := s.productManagementComboSyncSummary(ctx)
	if appErr != nil {
		return nil, appErr
	}
	return &domain.ProductManagementComboTreeResponse{
		Groups:      pagedGroups,
		Data:        data,
		Pagination:  domain.PaginationMeta{Page: page, PageSize: pageSize, Total: int64(len(comboGroups))},
		SyncSummary: summary,
	}, nil
}

func (s *productManagementService) collectProductManagementRecordsForGrouping(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, *domain.AppError) {
	const scanPageSize = 100
	const maxScanRecords = 5000
	filter.Page = 1
	filter.PageSize = scanPageSize
	items := make([]*domain.ProductManagementRecord, 0, scanPageSize)
	for len(items) < maxScanRecords {
		pageItems, total, err := s.records.List(ctx, filter)
		if err != nil {
			return nil, infraAppError("list product management records for combo grouping", err)
		}
		items = append(items, pageItems...)
		if len(pageItems) < scanPageSize || int64(len(items)) >= total {
			break
		}
		filter.Page++
	}
	return items, nil
}

func (s *productManagementService) productManagementComboSyncSummary(ctx context.Context) (*domain.OMPSKUComboSyncState, *domain.AppError) {
	if s.skuCombos == nil {
		return nil, nil
	}
	state, err := s.skuCombos.GetLatestSyncState(ctx)
	if err != nil {
		if isMySQLTableMissing(err) {
			return nil, nil
		}
		return nil, infraAppError("get product management combo sync state", err)
	}
	return state, nil
}

func (s *productManagementService) GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, *domain.AppError) {
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil)
	}
	if appErr := s.refreshReadModel(ctx); appErr != nil {
		return nil, appErr
	}
	items, err := s.records.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraAppError("list task product management records", err)
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	s.decorateRecords(ctx, actor, items)
	return items, nil
}

// GetByTaskIDs is consumed by the task-resource read model. Production uses
// the repository batch path; the fallback keeps test doubles and alternative
// repositories functional without turning the resource list into a hard
// dependency on a concrete MySQL implementation.
func (s *productManagementService) GetByTaskIDs(ctx context.Context, taskIDs []int64) ([]*domain.ProductManagementRecord, *domain.AppError) {
	ids := uniqueProductManagementTaskIDs(taskIDs)
	if len(ids) == 0 {
		return []*domain.ProductManagementRecord{}, nil
	}
	var (
		items []*domain.ProductManagementRecord
		err   error
	)
	if batch, ok := s.records.(interface {
		GetByTaskIDs(context.Context, []int64) ([]*domain.ProductManagementRecord, error)
	}); ok {
		items, err = batch.GetByTaskIDs(ctx, ids)
	} else {
		for _, taskID := range ids {
			var taskItems []*domain.ProductManagementRecord
			taskItems, err = s.records.GetByTaskID(ctx, taskID)
			if err != nil {
				break
			}
			items = append(items, taskItems...)
		}
	}
	if err != nil {
		return nil, infraAppError("list task product profiles", err)
	}
	return items, nil
}
func uniqueProductManagementTaskIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	items := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func (s *productManagementService) ListImageCandidates(ctx context.Context, actor domain.RequestActor, recordID int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can maintain ERP product image", nil)
	}
	candidates, err := s.imageCandidatesForRecord(ctx, record)
	if err != nil {
		return nil, infraAppError("list product image candidates", err)
	}
	return candidates, nil
}

func (s *productManagementService) ReparseImage(ctx context.Context, actor domain.RequestActor, recordID int64) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can reparse ERP product image", nil)
	}
	patch := s.autoImagePatch(ctx, record)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	}); err != nil {
		return nil, infraAppError("reparse product management image", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) SetManualImage(ctx context.Context, actor domain.RequestActor, recordID int64, assetID int64) (*domain.ProductManagementRecord, *domain.AppError) {
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required", nil)
	}
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canMaintainProductManagementImage(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only task creator or ERP/Admin can maintain ERP product image", nil)
	}
	row, err := s.assetSearch.GetCurrentByAssetID(ctx, assetID)
	if err != nil {
		return nil, infraAppError("get selected product image asset", err)
	}
	if row == nil || row.Asset == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "asset not found", nil)
	}
	if !isProductManagementERPImageAsset(row.Asset) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "ERP product image must be jpg, png, webp, or gif", nil)
	}
	if row.Asset.TaskID != record.TaskID && !isProductManagementAdmin(actor) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "cross-task image selection requires ERP/Admin", nil)
	}
	taskNo := ""
	if row.Task != nil {
		taskNo = row.Task.TaskNo
	}
	source := domain.ProductManagementImageSourceManual
	if domain.NormalizeTaskAssetType(row.Asset.AssetType).IsERPProductImage() {
		source = domain.ProductManagementImageSourceERPProduct
	}
	patch := imagePatchFromCandidate(candidateFromAsset(row.Asset, taskNo, source))
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	}); err != nil {
		return nil, infraAppError("set product management image", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) ProcessQueuedERPSync(ctx context.Context, limit int) (int, *domain.AppError) {
	if s.erpBridge == nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "ERP bridge is not configured for product management sync", nil)
	}
	now := s.now()
	claimToken := fmt.Sprintf("product-management-%d", now.UnixNano())
	records, err := s.records.ClaimQueuedSyncRecords(ctx, limit, claimToken, now)
	if err != nil {
		return 0, infraAppError("claim product management erp sync records", err)
	}
	processed := 0
	for _, record := range records {
		if record == nil {
			continue
		}
		if ctx.Err() != nil {
			return processed, domain.NewAppError(domain.ErrCodeInternalError, "product management sync worker stopped", map[string]string{"cause": ctx.Err().Error()})
		}
		didWork := false
		if productManagementStatusInFlight(record.BaseSyncStatus) {
			didWork = true
			if appErr := s.syncBaseRecordToERP(ctx, record); appErr != nil {
				s.markProductManagementBaseSyncFailed(ctx, record, appErr.Message)
			} else {
				s.markProductManagementBaseSyncSucceeded(ctx, record)
			}
		}
		if productManagementStatusInFlight(record.ImageSyncStatus) {
			didWork = true
			if appErr := s.syncImageRecordToERP(ctx, record); appErr != nil {
				if isProductManagementERPImageSyncRetryable(appErr) {
					s.markProductManagementImageSyncRetrying(ctx, record, appErr.Message)
				} else {
					s.markProductManagementImageSyncFailed(ctx, record, appErr.Message)
				}
			} else {
				s.markProductManagementImageSyncSucceeded(ctx, record)
			}
		}
		if !didWork {
			if appErr := s.syncRecordToERP(ctx, record); appErr != nil {
				s.markProductManagementSyncFailed(ctx, record, appErr.Message)
			} else {
				s.markProductManagementSyncSucceeded(ctx, record)
			}
		}
		processed++
	}
	return processed, nil
}

func (s *productManagementService) RequestSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "all")
}

func (s *productManagementService) RequestBaseSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "base")
}

func (s *productManagementService) RequestImageSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return s.queueProductManagementSync(ctx, actor, recordID, force, "image")
}

func (s *productManagementService) AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError {
	if taskID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil)
	}
	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model for closed task", err)
	}
	records, err := s.records.GetByTaskID(ctx, taskID)
	if err != nil {
		return infraAppError("list closed task product management records", err)
	}
	if len(records) == 0 {
		return nil
	}
	assetsByTaskID := make(map[int64][]*domain.TaskAsset)
	for _, record := range records {
		if record == nil || strings.TrimSpace(record.SKUCode) == "" {
			continue
		}
		candidates, candidateErr := s.finalizedImageCandidatesForRecord(ctx, record, assetsByTaskID)
		if candidateErr != nil {
			return infraAppError("resolve finalized product image for closed task", candidateErr)
		}
		patch := imagePatchFromCandidate(firstProductManagementImageCandidate(candidates))
		if patch.ImageSource == domain.ProductManagementImageSourceDelivery {
			patch.ImageSource = domain.ProductManagementImageSourceAutoOnClose
			patch.ImageSyncSource = domain.ProductManagementImageSourceAutoOnClose
		}
		if patch.ImageSource == domain.ProductManagementImageSourceMissing {
			patch.ImageSyncStatus = domain.ProductManagementERPSyncStatusWaitingImage
			if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				if err := s.records.UpdateImage(ctx, tx, record.ID, patch); err != nil {
					return err
				}
				return s.appendProductManagementTaskEvent(ctx, tx, record.TaskID, domain.TaskEventERPImageAwaitingUpload, actorID,
					fmt.Sprintf("未找到最终成品图，SKU %s 待人工上传 ERP 商品图", strings.TrimSpace(record.SKUCode)),
					record, patch.ImageMissingReason)
			}); err != nil {
				return infraAppError("mark product management image awaiting upload", err)
			}
			continue
		}
		patch.ImageSyncStatus = domain.ProductManagementERPSyncStatusSyncing
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return s.records.UpdateImage(ctx, tx, record.ID, patch)
		}); err != nil {
			return infraAppError("prepare closed task erp image sync", err)
		}
		record.ImageSource = patch.ImageSource
		record.ImageSelectionMode = patch.ImageSelectionMode
		record.ImageAssetID = patch.ImageAssetID
		record.ImageAssetVersionID = patch.ImageAssetVersionID
		record.ImageFilename = patch.ImageFilename
		record.ImageMimeType = patch.ImageMimeType
		record.ImageMissingReason = patch.ImageMissingReason
		record.ImageSyncSource = patch.ImageSyncSource
		record.ImageSyncStatus = patch.ImageSyncStatus
		if s.erpBridge == nil {
			msg := "ERP 图片同步服务未配置，可稍后手动重试"
			s.markProductManagementImageSyncFailed(ctx, record, msg)
			_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSyncFailed, actorID, fmt.Sprintf("ERP 图片自动同步失败：SKU %s，%s", strings.TrimSpace(record.SKUCode), msg), msg)
			continue
		}
		if appErr := s.syncImageRecordToERP(ctx, record); appErr != nil {
			msg := truncateProductManagementSyncError(appErr.Message)
			if isProductManagementERPImageSyncRetryable(appErr) {
				s.markProductManagementImageSyncRetrying(ctx, record, msg)
				continue
			}
			s.markProductManagementImageSyncFailed(ctx, record, msg)
			_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSyncFailed, actorID, fmt.Sprintf("ERP 图片自动同步失败：SKU %s，%s", strings.TrimSpace(record.SKUCode), msg), msg)
			continue
		}
		s.markProductManagementImageSyncSucceeded(ctx, record)
		_ = s.appendProductManagementTaskEventInTx(ctx, record, domain.TaskEventERPImageAutoSynced, actorID, fmt.Sprintf("ERP 图片已自动同步：SKU %s", strings.TrimSpace(record.SKUCode)), domain.ProductManagementImageSourceLabel(record.ImageSource))
	}
	return nil
}

func (s *productManagementService) queueProductManagementSync(ctx context.Context, actor domain.RequestActor, recordID int64, force bool, scope string) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, recordID)
	if appErr != nil {
		return nil, appErr
	}
	if !canSyncProductManagementERP(actor, record) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "ERP sync requires ERP/Admin or task creator", nil)
	}
	now := s.now()
	status := domain.ProductManagementERPSyncStatusQueued
	errMsg := ""
	cooldown := now.Add(5 * time.Minute)
	if record.SyncCooldownUntil != nil && record.SyncCooldownUntil.After(now) && !force {
		status = domain.ProductManagementERPSyncStatusCoolingDown
		errMsg = fmt.Sprintf("单 SKU 刷新冷却中，%s 后可再次同步", record.SyncCooldownUntil.Format("2006-01-02 15:04:05"))
		cooldown = *record.SyncCooldownUntil
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		patch := repo.ProductManagementSyncPatch{
			Status:            status,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: &cooldown,
			LastSyncError:     errMsg,
		}
		switch scope {
		case "base":
			patch.BaseStatus = status
			patch.BaseSyncError = errMsg
			return s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, patch)
		case "image":
			patch.ImageStatus = status
			patch.ImageSyncError = errMsg
			return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, patch)
		default:
			patch.BaseStatus = status
			patch.ImageStatus = status
			patch.BaseSyncError = errMsg
			patch.ImageSyncError = errMsg
			return s.records.UpdateSyncStatus(ctx, tx, record.ID, patch)
		}
	}); err != nil {
		return nil, infraAppError("request product management erp sync", err)
	}
	return s.getAndDecorateRecord(ctx, actor, record.ID)
}

func (s *productManagementService) refreshReadModel(ctx context.Context) *domain.AppError {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	now := s.now()
	if !s.lastRefresh.IsZero() && now.Sub(s.lastRefresh) < s.refreshEvery {
		return nil
	}
	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model", err)
	}
	s.lastRefresh = now
	// Intentionally do NOT invalidate the cost dashboard cache here. This is the
	// throttled (30s) background refresh triggered by ordinary product-center
	// traffic; invalidating on every refresh would defeat the 5-minute weakly
	// consistent cache. Cache invalidation stays on cost recalculation apply,
	// where the underlying dashboard facts actually change.
	return nil
}

func (s *productManagementService) RefreshReadModelNow(ctx context.Context) *domain.AppError {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if err := s.records.RefreshReadModel(ctx); err != nil {
		return infraAppError("refresh product management read model", err)
	}
	s.lastRefresh = s.now()
	return nil
}

func (s *productManagementService) QueuePendingBaseSyncForTask(ctx context.Context, taskID int64) (int, *domain.AppError) {
	if taskID <= 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id is required", nil)
	}
	if appErr := s.RefreshReadModelNow(ctx); appErr != nil {
		return 0, appErr
	}
	now := s.now()
	cooldownUntil := now.Add(5 * time.Minute)
	var queued int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		count, err := s.records.QueuePendingBaseSyncByTaskID(ctx, tx, taskID, now, cooldownUntil)
		if err != nil {
			return err
		}
		queued = count
		return nil
	}); err != nil {
		return 0, infraAppError("queue product management base sync for task", err)
	}
	return int(queued), nil
}

func (s *productManagementService) syncRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if appErr := s.syncBaseRecordToERP(ctx, record); appErr != nil {
		return appErr
	}
	if record != nil && record.ImageAssetID != nil && *record.ImageAssetID > 0 {
		return s.syncImageRecordToERP(ctx, record)
	}
	return nil
}

func (s *productManagementService) syncBaseRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record is required", nil)
	}
	if strings.TrimSpace(record.SKUCode) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP product sync", nil)
	}
	productName := truncateERPShortName(
		firstNonEmptyString(strings.TrimSpace(record.ProductName), strings.TrimSpace(record.SKUCode)),
		ERPProductNameMaxLength,
	)
	productIID := firstNonEmptyString(strings.TrimSpace(record.ERPIID), strings.TrimSpace(record.ProductIID))
	shortName := productManagementERPShortName(productName, productIID, record.SKUCode)
	payload := domain.ERPProductUpsertPayload{
		ProductID:        strings.TrimSpace(record.SKUCode),
		SKUID:            strings.TrimSpace(record.SKUCode),
		SKUCode:          strings.TrimSpace(record.SKUCode),
		IID:              productIID,
		Name:             productName,
		ProductName:      productName,
		ShortName:        shortName,
		ProductShortName: shortName,
		Operation:        "product_management_base_sync",
		Source:           "product_management",
		TaskContext: &domain.ERPTaskFilingContext{
			TaskNo: strings.TrimSpace(record.TaskNo),
			Remark: "产品管理同步 ERP 基础资料",
		},
	}
	if record.CostPrice != nil && *record.CostPrice > 0 {
		cost := *record.CostPrice
		payload.CostPrice = &cost
	}
	_, upsertErr := s.erpBridge.UpsertProduct(ctx, payload)
	readbackErr := s.verifyERPBaseReadback(ctx, record, payload)
	if readbackErr == nil {
		return nil
	}
	if upsertErr != nil {
		return upsertErr
	}
	return readbackErr
}

func (s *productManagementService) syncImageRecordToERP(ctx context.Context, record *domain.ProductManagementRecord) *domain.AppError {
	if record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "product management record is required", nil)
	}
	skuCode := strings.TrimSpace(record.SKUCode)
	if skuCode == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP image sync", nil)
	}
	if productManagementImageSourceRequiresFinalized(record.ImageSource) {
		candidates, err := s.finalizedImageCandidatesForRecord(ctx, record, nil)
		if err != nil {
			return infraAppError("resolve finalized product image for ERP sync", err)
		}
		candidate := firstProductManagementImageCandidate(candidates)
		if candidate == nil {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "当前 SKU 尚无已定稿成品图，ERP 图片同步已阻止", nil)
		}
		patch := imagePatchFromCandidate(candidate)
		if productManagementImagePatchChanged(record, patch) {
			if s.txRunner == nil {
				return domain.NewAppError(domain.ErrCodeInternalError, "ERP 图片权威版本无法保存", nil)
			}
			if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				return s.records.UpdateImage(ctx, tx, record.ID, patch)
			}); err != nil {
				return infraAppError("persist finalized product image before ERP sync", err)
			}
			applyProductManagementImagePatch(record, patch)
		}
	}
	imageURL, appErr := s.resolveERPImageURL(ctx, record)
	if appErr != nil {
		return appErr
	}
	productIID, lookupErr := s.resolveProductManagementStyleIID(ctx, record)
	if lookupErr != nil {
		return lookupErr
	}
	productName := truncateERPShortName(
		firstNonEmptyString(strings.TrimSpace(record.ProductName), skuCode),
		ERPProductNameMaxLength,
	)
	shortName := productManagementERPShortName(productName, productIID, skuCode)
	payload := domain.ERPProductUpsertPayload{
		ProductID:        skuCode,
		SKUID:            skuCode,
		SKUCode:          skuCode,
		IID:              productIID,
		Name:             productName,
		ProductName:      productName,
		ShortName:        shortName,
		ProductShortName: shortName,
		Pic:              imageURL,
		PicBig:           imageURL,
		SKUPic:           imageURL,
		Operation:        "product_management_image_sync",
		Source:           "product_management",
		TaskContext: &domain.ERPTaskFilingContext{
			TaskNo: strings.TrimSpace(record.TaskNo),
			Remark: fmt.Sprintf("产品管理同步 ERP 图片，图片来源：%s", domain.ProductManagementImageSourceLabel(record.ImageSource)),
		},
	}
	_, appErr = s.erpBridge.UpsertProduct(ctx, payload)
	if appErr != nil {
		return appErr
	}
	return s.verifyERPImageReadback(ctx, record, imageURL)
}

func (s *productManagementService) resolveProductManagementStyleIID(ctx context.Context, record *domain.ProductManagementRecord) (string, *domain.AppError) {
	if s == nil || s.erpBridge == nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步服务未配置，无法确认 SKU 是否已建档", nil)
	}
	skuCode := strings.TrimSpace(record.SKUCode)
	product, appErr := s.erpBridge.GetProductByID(ctx, skuCode)
	if appErr != nil {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步前未找到该 SKU："+appErr.Message, nil)
	}
	productIID := firstNonEmptyString(strings.TrimSpace(record.ERPIID), strings.TrimSpace(record.ProductIID))
	if product != nil {
		productIID = firstNonEmptyString(productIID, strings.TrimSpace(product.IID))
	}
	if productIID == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片同步缺少款式编码，请先确认该 SKU 已在聚水潭建档", nil)
	}
	return productIID, nil
}

var productManagementERPImageReadbackRetryDelays = []time.Duration{
	300 * time.Millisecond,
	800 * time.Millisecond,
	1500 * time.Millisecond,
}

var productManagementERPImageReadbackSleep = time.Sleep

var productManagementERPBaseReadbackRetryDelays = []time.Duration{
	300 * time.Millisecond,
	800 * time.Millisecond,
	1500 * time.Millisecond,
}

var productManagementERPBaseReadbackSleep = time.Sleep

const productManagementERPBaseCostTolerance = 0.0005

func (s *productManagementService) verifyERPBaseReadback(ctx context.Context, record *domain.ProductManagementRecord, payload domain.ERPProductUpsertPayload) *domain.AppError {
	if s == nil || s.erpBridge == nil || record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 基础资料回读校验失败：同步服务未配置", nil)
	}
	sku := strings.TrimSpace(firstNonEmptyString(payload.SKUCode, payload.SKUID, record.SKUCode))
	if sku == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP base readback", nil)
	}
	maxAttempts := 1 + len(productManagementERPBaseReadbackRetryDelays)
	var lastMessage string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		product, appErr := s.erpBridge.GetProductByID(ctx, sku)
		if appErr != nil {
			lastMessage = strings.TrimSpace(appErr.Message)
		} else if product == nil {
			lastMessage = "ERP 未返回该 SKU 商品资料"
		} else if mismatches := productManagementERPBaseReadbackMismatches(product, payload); len(mismatches) > 0 {
			lastMessage = strings.Join(mismatches, "；")
		} else {
			return nil
		}
		if attempt < maxAttempts {
			productManagementERPBaseReadbackSleep(productManagementERPBaseReadbackRetryDelays[attempt-1])
		}
	}
	if lastMessage == "" {
		lastMessage = "ERP 基础资料状态未知"
	}
	return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 基础资料回读校验未通过："+lastMessage, nil)
}

func productManagementERPBaseReadbackMismatches(product *domain.ERPProduct, payload domain.ERPProductUpsertPayload) []string {
	if product == nil {
		return []string{"ERP 未返回该 SKU 商品资料"}
	}
	var mismatches []string
	expectedSKU := strings.TrimSpace(firstNonEmptyString(payload.SKUCode, payload.SKUID, payload.ProductID))
	actualSKU := strings.TrimSpace(firstNonEmptyString(product.SKUCode, product.SKUID, product.ProductID))
	if expectedSKU != "" && actualSKU != "" && actualSKU != expectedSKU {
		mismatches = append(mismatches, fmt.Sprintf("ERP 返回 SKU %s，与系统 SKU %s 不一致", actualSKU, expectedSKU))
	}
	expectedIID := strings.TrimSpace(payload.IID)
	actualIID := strings.TrimSpace(product.IID)
	if expectedIID != "" {
		if actualIID == "" {
			mismatches = append(mismatches, "ERP 未返回款式编码")
		} else if actualIID != expectedIID {
			mismatches = append(mismatches, fmt.Sprintf("ERP 款式编码 %s，与系统款式编码 %s 不一致", actualIID, expectedIID))
		}
	}
	expectedName := strings.TrimSpace(firstNonEmptyString(payload.ProductName, payload.Name))
	actualName := strings.TrimSpace(firstNonEmptyString(product.ProductName, product.Name))
	if expectedName != "" {
		if actualName == "" {
			mismatches = append(mismatches, "ERP 未返回商品名称")
		} else if actualName != expectedName {
			mismatches = append(mismatches, "ERP 商品名称与系统商品名称不一致")
		}
	}
	if payload.CostPrice != nil {
		if product.CostPrice == nil {
			mismatches = append(mismatches, "ERP 未返回成本价")
		} else if math.Abs(*product.CostPrice-*payload.CostPrice) > productManagementERPBaseCostTolerance {
			mismatches = append(mismatches, fmt.Sprintf("ERP 成本 %.4f，与系统成本 %.4f 不一致", *product.CostPrice, *payload.CostPrice))
		}
	}
	return mismatches
}

func (s *productManagementService) verifyERPImageReadback(ctx context.Context, record *domain.ProductManagementRecord, expectedImageURL ...string) *domain.AppError {
	if s == nil || s.erpBridge == nil || record == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验失败：同步服务未配置", nil)
	}
	sku := strings.TrimSpace(record.SKUCode)
	if sku == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "SKU is required for ERP image readback", nil)
	}
	maxAttempts := 1 + len(productManagementERPImageReadbackRetryDelays)
	var lastMessage string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		product, appErr := s.erpBridge.GetProductByID(ctx, sku)
		if appErr != nil {
			lastMessage = strings.TrimSpace(appErr.Message)
		} else if product == nil {
			lastMessage = "ERP 未返回该 SKU 商品资料"
		} else {
			imageURL := strings.TrimSpace(product.ImageURL)
			if isAbsoluteHTTPURL(imageURL) && productManagementERPImageURLsMatch(firstNonEmptyString(expectedImageURL...), imageURL) {
				return nil
			}
			if imageURL == "" {
				lastMessage = "ERP 尚未返回商品图"
			} else {
				if !isAbsoluteHTTPURL(imageURL) {
					lastMessage = "ERP 返回的图片地址不是公网地址"
				} else {
					lastMessage = "ERP 返回的图片与本次同步图片不一致"
				}
			}
		}
		if attempt < maxAttempts {
			productManagementERPImageReadbackSleep(productManagementERPImageReadbackRetryDelays[attempt-1])
		}
	}
	if lastMessage == "" {
		lastMessage = "ERP 图片状态未知"
	}
	return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片回读校验未通过："+lastMessage, nil)
}

func isProductManagementERPImageReadbackPending(appErr *domain.AppError) bool {
	if appErr == nil {
		return false
	}
	msg := strings.TrimSpace(appErr.Message)
	if !strings.HasPrefix(msg, "ERP 图片回读校验未通过：") {
		return false
	}
	return strings.Contains(msg, "ERP 尚未返回商品图") ||
		strings.Contains(msg, "ERP 返回的图片地址不是公网地址") ||
		strings.Contains(msg, "ERP 返回的图片与本次同步图片不一致")
}

func isProductManagementERPImageSyncRetryable(appErr *domain.AppError) bool {
	if appErr == nil {
		return false
	}
	if isProductManagementERPImageReadbackPending(appErr) {
		return true
	}
	return strings.Contains(strings.TrimSpace(appErr.Message), "upstream business rejected request")
}

func productManagementERPShortName(productName, productIID, skuCode string) string {
	return strings.TrimSpace(firstNonEmptyString(productName, skuCode, productIID))
}

func productManagementStatusInFlight(status domain.ProductManagementERPSyncStatus) bool {
	switch status {
	case domain.ProductManagementERPSyncStatusQueued,
		domain.ProductManagementERPSyncStatusSyncing,
		domain.ProductManagementERPSyncStatusCoolingDown:
		return true
	default:
		return false
	}
}

func (s *productManagementService) resolveERPImageURL(ctx context.Context, record *domain.ProductManagementRecord) (string, *domain.AppError) {
	if record == nil || record.ImageAssetID == nil || *record.ImageAssetID <= 0 {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片待补充，无法同步 ERP", nil)
	}
	var row *repo.TaskAssetSearchRow
	var err error
	if record.ImageAssetVersionID != nil && *record.ImageAssetVersionID > 0 {
		row, err = s.assetSearch.GetVersion(ctx, *record.ImageAssetID, *record.ImageAssetVersionID)
	} else {
		row, err = s.assetSearch.GetCurrentByAssetID(ctx, *record.ImageAssetID)
	}
	if err != nil {
		return "", infraAppError("get product management erp image asset", err)
	}
	if row == nil || row.Asset == nil {
		return "", domain.NewAppError(domain.ErrCodeNotFound, "ERP 图片资产不存在", nil)
	}
	asset := row.Asset
	if !isProductManagementERPImageAsset(asset) {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片只支持 jpg、png、webp、gif，请先选择可预览图片资产", nil)
	}
	storageKey := ""
	if asset.StorageKey != nil {
		storageKey = strings.TrimSpace(*asset.StorageKey)
	}
	if storageKey == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片资产缺少 OSS 存储地址", nil)
	}
	if isAbsoluteHTTPURL(storageKey) {
		return storageKey, nil
	}
	if s.imageProxy != nil {
		if proxyURL := s.imageProxy.BuildImageURL(asset); proxyURL != nil && isAbsoluteHTTPURL(*proxyURL) {
			return strings.TrimSpace(*proxyURL), nil
		}
	}
	filename := strings.TrimSpace(asset.FileName)
	if asset.OriginalName != nil && strings.TrimSpace(*asset.OriginalName) != "" {
		filename = strings.TrimSpace(*asset.OriginalName)
	}
	ossSignedURLTooLong := false
	if s.ossDirect != nil && s.ossDirect.Enabled() {
		if info := s.ossDirect.PresignDownloadURLWithFilename(storageKey, filename); info != nil && isAbsoluteHTTPURL(info.DownloadURL) {
			signedURL := strings.TrimSpace(info.DownloadURL)
			if len(signedURL) <= 300 {
				return signedURL, nil
			}
			ossSignedURLTooLong = true
		}
	}
	if s.uploadClient != nil {
		if directURL := s.uploadClient.BuildBrowserFileURL(storageKey); directURL != nil && isAbsoluteHTTPURL(*directURL) {
			return strings.TrimSpace(*directURL), nil
		}
	}
	if ossSignedURLTooLong {
		return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "ERP 图片地址超过聚水潭 300 字符限制，请启用 ERP 图片短链代理", nil)
	}
	return "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "缺少可供 ERP 拉取的公网图片地址配置", nil)
}

func (s *productManagementService) markProductManagementSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	imageStatus := domain.ProductManagementERPSyncStatusSynced
	imageErr := ""
	imageSyncedAt := &now
	if record.ImageAssetID == nil || *record.ImageAssetID <= 0 {
		imageStatus = domain.ProductManagementERPSyncStatusWaitingImage
		imageErr = "ERP 商品图待上传"
		imageSyncedAt = nil
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			BaseStatus:        domain.ProductManagementERPSyncStatusSynced,
			ImageStatus:       imageStatus,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastBaseSyncedAt:  &now,
			LastImageSyncedAt: imageSyncedAt,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			BaseSyncError:     "",
			ImageSyncError:    imageErr,
		}); err != nil {
			return err
		}
		if err := s.records.MarkBaseSyncProjectionSynced(ctx, tx, record.TaskID, record.TaskSKUItemID, now); err != nil {
			return err
		}
		if s.costRuns != nil {
			if err := s.costRuns.MarkERPResultForProductManagementRecord(ctx, tx, record.ID, domain.CostRunItemStatusERPSynced, ""); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *productManagementService) MarkTaskSKUItemBaseSyncSucceeded(ctx context.Context, taskID int64, taskSKUItemID int64) error {
	if taskID <= 0 || taskSKUItemID <= 0 {
		return fmt.Errorf("task_id and task_sku_item_id are required")
	}
	now := s.now()
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.MarkBaseSyncProjectionSynced(ctx, tx, taskID, &taskSKUItemID, now)
	})
}

func (s *productManagementService) markProductManagementSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusFailed,
			BaseStatus:        domain.ProductManagementERPSyncStatusFailed,
			ImageStatus:       domain.ProductManagementERPSyncStatusFailed,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     truncateProductManagementSyncError(message),
			BaseSyncError:     truncateProductManagementSyncError(message),
			ImageSyncError:    truncateProductManagementSyncError(message),
		})
	})
}

func (s *productManagementService) markProductManagementBaseSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			BaseStatus:        domain.ProductManagementERPSyncStatusSynced,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastBaseSyncedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			BaseSyncError:     "",
		}); err != nil {
			return err
		}
		if err := s.records.MarkBaseSyncProjectionSynced(ctx, tx, record.TaskID, record.TaskSKUItemID, now); err != nil {
			return err
		}
		if s.costRuns != nil {
			if err := s.costRuns.MarkERPResultForProductManagementRecord(ctx, tx, record.ID, domain.CostRunItemStatusERPSynced, ""); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return
	}
	s.clearProductManagementBaseSyncDedupe(record)
}

func (s *productManagementService) markProductManagementBaseSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	msg := truncateProductManagementSyncError(message)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.records.UpdateBaseSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusFailed,
			BaseStatus:        domain.ProductManagementERPSyncStatusFailed,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     msg,
			BaseSyncError:     msg,
		}); err != nil {
			return err
		}
		if s.costRuns != nil {
			if err := s.costRuns.MarkERPResultForProductManagementRecord(ctx, tx, record.ID, domain.CostRunItemStatusERPFailed, msg); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return
	}
	s.notifyProductManagementBaseSyncFailed(record, msg)
}

func (s *productManagementService) notifyProductManagementBaseSyncFailed(record *domain.ProductManagementRecord, message string) {
	if s == nil || record == nil || s.notifications == nil {
		return
	}
	notifier, ok := s.notifications.(taskSKUSyncFailureNotificationService)
	if !ok {
		return
	}
	req := domain.SKUSyncFailureNotificationRequest{
		Source:   domain.SKUSyncFailureSourceProductBaseSync,
		TaskID:   record.TaskID,
		TaskNo:   record.TaskNo,
		RecordID: record.ID,
		Summary:  message,
		FailureItems: []domain.SKUSyncFailureItem{{
			SKUItemID:   derefInt64Ptr(record.TaskSKUItemID),
			SKUCode:     record.SKUCode,
			ProductName: record.ProductName,
			Error:       message,
		}},
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = notifier.NotifyTaskSKUSyncFailure(ctx, req)
	}()
}

func (s *productManagementService) clearProductManagementBaseSyncDedupe(record *domain.ProductManagementRecord) {
	if s == nil || record == nil || s.notifications == nil {
		return
	}
	notifier, ok := s.notifications.(taskSKUSyncFailureNotificationService)
	if !ok {
		return
	}
	scope := fmt.Sprintf("task_sku_sync_failed:v1:pm_base:%d", record.ID)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = notifier.ClearNotificationDedupeScope(ctx, scope)
	}()
}

func derefInt64Ptr(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (s *productManagementService) markProductManagementImageSyncSucceeded(ctx context.Context, record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusSynced,
			ImageStatus:       domain.ProductManagementERPSyncStatusSynced,
			LastERPCheckedAt:  &now,
			LastERPSyncedAt:   &now,
			LastImageSyncedAt: &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     "",
			ImageSyncError:    "",
		})
	})
}

func (s *productManagementService) markProductManagementImageSyncFailed(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	cooldown := record.SyncCooldownUntil
	if cooldown == nil || cooldown.Before(now) {
		next := now.Add(5 * time.Minute)
		cooldown = &next
	}
	status := domain.ProductManagementERPSyncStatusFailed
	msg := truncateProductManagementSyncError(message)
	if strings.Contains(msg, "待补充") || strings.Contains(msg, "待人工上传") {
		status = domain.ProductManagementERPSyncStatusWaitingImage
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            status,
			ImageStatus:       status,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: cooldown,
			LastSyncError:     msg,
			ImageSyncError:    msg,
		})
	})
}

func (s *productManagementService) markProductManagementImageSyncRetrying(ctx context.Context, record *domain.ProductManagementRecord, message string) {
	if record == nil {
		return
	}
	now := s.now()
	retryAt := now.Add(5 * time.Minute)
	msg := truncateProductManagementSyncError(message)
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImageSyncStatus(ctx, tx, record.ID, repo.ProductManagementSyncPatch{
			Status:            domain.ProductManagementERPSyncStatusCoolingDown,
			ImageStatus:       domain.ProductManagementERPSyncStatusCoolingDown,
			LastERPCheckedAt:  &now,
			SyncCooldownUntil: &retryAt,
			LastSyncError:     msg,
			ImageSyncError:    msg,
		})
	})
}

func (s *productManagementService) appendProductManagementTaskEventInTx(ctx context.Context, record *domain.ProductManagementRecord, eventType string, actorID int64, summary string, reason string) error {
	if record == nil {
		return nil
	}
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.appendProductManagementTaskEvent(ctx, tx, record.TaskID, eventType, actorID, summary, record, reason)
	})
}

func (s *productManagementService) appendProductManagementTaskEvent(ctx context.Context, tx repo.Tx, taskID int64, eventType string, actorID int64, summary string, record *domain.ProductManagementRecord, reason string) error {
	if s.taskEvents == nil || taskID <= 0 {
		return nil
	}
	var actorPtr *int64
	if actorID > 0 {
		actorPtr = &actorID
	}
	payload := map[string]interface{}{
		"summary": strings.TrimSpace(summary),
	}
	if record != nil {
		payload["sku_code"] = strings.TrimSpace(record.SKUCode)
		payload["task_no"] = strings.TrimSpace(record.TaskNo)
		payload["image_source"] = string(record.ImageSource)
		payload["image_source_label"] = domain.ProductManagementImageSourceLabel(record.ImageSource)
	}
	if strings.TrimSpace(reason) != "" {
		payload["reason"] = strings.TrimSpace(reason)
	}
	_, err := s.taskEvents.Append(ctx, tx, taskID, eventType, actorPtr, payload)
	return err
}

func (s *productManagementService) getRecord(ctx context.Context, id int64) (*domain.ProductManagementRecord, *domain.AppError) {
	if id <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid product management record id", nil)
	}
	record, err := s.records.GetByID(ctx, id)
	if err != nil {
		return nil, infraAppError("get product management record", err)
	}
	if record == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "product management record not found", nil)
	}
	return record, nil
}

func (s *productManagementService) getAndDecorateRecord(ctx context.Context, actor domain.RequestActor, id int64) (*domain.ProductManagementRecord, *domain.AppError) {
	record, appErr := s.getRecord(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	s.decorateRecords(ctx, actor, []*domain.ProductManagementRecord{record})
	return record, nil
}

func (s *productManagementService) decorateRecords(ctx context.Context, actor domain.RequestActor, records []*domain.ProductManagementRecord) {
	assetsByTaskID := make(map[int64][]*domain.TaskAsset)
	for _, record := range records {
		if record == nil {
			continue
		}
		record.ImageSourceLabel = domain.ProductManagementImageSourceLabel(record.ImageSource)
		record.CanMaintainImage = canMaintainProductManagementImage(actor, record)
		record.CanCrossTaskSelect = isProductManagementAdmin(actor)
		record.CanForceOverride = isProductManagementAdmin(actor)
		record.CanSyncERP = canSyncProductManagementERP(actor, record)
		decorateProductManagementArea(record)
		if record.ImageAssetID != nil {
			record.ImagePreviewURL = fmt.Sprintf("/v1/assets/%d/preview", *record.ImageAssetID)
		}
		if record.ImageSelectionMode == domain.ProductManagementImageSelectionManual {
			continue
		}
		patch := s.autoImagePatchWithCache(ctx, record, assetsByTaskID)
		patchChanged := productManagementImagePatchChanged(record, patch)
		if patchChanged {
			s.persistAutoImagePatch(ctx, record, patch)
		}
		record.ImageSource = patch.ImageSource
		record.ImageSelectionMode = patch.ImageSelectionMode
		record.ImageAssetID = patch.ImageAssetID
		record.ImageAssetVersionID = patch.ImageAssetVersionID
		record.ImageFilename = patch.ImageFilename
		record.ImageMimeType = patch.ImageMimeType
		record.ImageMissingReason = patch.ImageMissingReason
		record.ImageSyncSource = patch.ImageSyncSource
		if record.ImageSyncSource == "" {
			record.ImageSyncSource = patch.ImageSource
		}
		record.ImageSyncStatus = productManagementDecoratedImageSyncStatus(record, patch, patchChanged)
		record.ImageSourceLabel = domain.ProductManagementImageSourceLabel(record.ImageSource)
		if record.ImageAssetID != nil {
			record.ImagePreviewURL = fmt.Sprintf("/v1/assets/%d/preview", *record.ImageAssetID)
		} else {
			record.ImagePreviewURL = ""
		}
	}
}

func decorateProductManagementArea(record *domain.ProductManagementRecord) {
	if record == nil {
		return
	}
	variant := taskSKUItemVariantObject(record.DimensionVariantJSON)
	variantSpec := productManagementVariantStringFromObject(variant, "spec_text")
	variantSize := productManagementVariantStringFromObject(variant, "size_text")
	record.SpecText = firstNonEmptyString(record.SpecText, variantSpec, record.DimensionTaskSpecText)
	record.SizeText = firstNonEmptyString(record.SizeText, variantSize, record.DimensionTaskSizeText)

	width := costDimensionCentimetersToMeters(variantFloatFromObject(variant, "width", "width_m"))
	height := costDimensionCentimetersToMeters(variantFloatFromObject(variant, "height", "height_m"))
	area := cloneFloat64Ptr(variantFloatFromObject(variant, "area", "area_m2"))
	quantity := cloneFloat64Ptr(variantFloatFromObject(variant, "quantity", "qty"))
	source := ""
	sourceLabel := ""
	confidence := ""
	if variantSpec != "" || variantSize != "" || width != nil || height != nil || area != nil || quantity != nil {
		source = "sku_item_variant"
		sourceLabel = "SKU 子项规格"
		confidence = "high"
	}

	if width == nil {
		width = costDimensionCentimetersToMeters(record.DimensionTaskWidthM)
	}
	if height == nil {
		height = costDimensionCentimetersToMeters(record.DimensionTaskHeightM)
	}
	if area == nil {
		area = cloneFloat64Ptr(record.DimensionTaskAreaM2)
	}
	if quantity == nil {
		quantity = cloneFloat64Ptr(record.DimensionSKUQuantity)
	}
	if quantity == nil {
		quantity = cloneFloat64Ptr(record.DimensionTaskQuantity)
	}
	if source == "" && (record.DimensionTaskSpecText != "" || record.DimensionTaskSizeText != "" || width != nil || height != nil || area != nil || quantity != nil) {
		source = "task_detail"
		sourceLabel = "任务规格"
		confidence = "high"
	}

	if area == nil && width != nil && height != nil {
		qty := productManagementAreaQuantityOrDefault(quantity)
		computedArea := *width * *height * qty
		area = &computedArea
	}
	if area == nil {
		text := strings.Join(uniqueNonEmptyStrings(
			record.SizeText,
			record.SpecText,
			record.ProductName,
			record.ProductFamily,
			record.CategoryName,
		), " ")
		extracted := extractCostDimensionsFromText(text)
		if width == nil {
			width = cloneFloat64Ptr(extracted.WidthM)
		}
		if height == nil {
			height = cloneFloat64Ptr(extracted.HeightM)
		}
		if area == nil {
			area = cloneFloat64Ptr(extracted.AreaM2)
		}
		if area != nil && source == "" {
			source = "text_extractor"
			sourceLabel = "规格文本解析"
			if record.SizeText != "" || record.SpecText != "" {
				confidence = "medium"
			} else {
				confidence = "low"
			}
		}
	}

	trace := &domain.ProductManagementAreaTrace{
		WidthM:      cloneFloat64Ptr(width),
		HeightM:     cloneFloat64Ptr(height),
		Quantity:    cloneFloat64Ptr(quantity),
		AreaM2:      cloneFloat64Ptr(area),
		Source:      source,
		SourceLabel: sourceLabel,
		Confidence:  confidence,
	}
	if trace.AreaM2 != nil {
		trace.Formula = productManagementAreaFormula(trace.WidthM, trace.HeightM, trace.Quantity, trace.AreaM2)
	} else {
		trace.Warning = "缺少可识别尺寸，成本面积待人工核对。"
		if trace.Source == "" {
			trace.Source = "missing"
			trace.SourceLabel = "未识别到尺寸"
			trace.Confidence = "low"
		}
	}
	record.AreaTrace = trace
}

func productManagementVariantStringFromObject(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			if text, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func productManagementAreaQuantityOrDefault(quantity *float64) float64 {
	if quantity != nil && *quantity > 0 {
		return *quantity
	}
	return 1
}

func productManagementAreaFormula(width, height, quantity, area *float64) string {
	if area == nil {
		return ""
	}
	if width != nil && height != nil {
		qty := productManagementAreaQuantityOrDefault(quantity)
		if quantity != nil && qty > 1 {
			return fmt.Sprintf("面积 = 宽 %.4g m × 高 %.4g m × 数量 %.4g = %.4g ㎡", *width, *height, qty, *area)
		}
		return fmt.Sprintf("面积 = 宽 %.4g m × 高 %.4g m = %.4g ㎡", *width, *height, *area)
	}
	return fmt.Sprintf("面积 = %.4g ㎡", *area)
}

func (s *productManagementService) productManagementComboGroups(ctx context.Context, records []*domain.ProductManagementRecord) []domain.ProductManagementComboGroup {
	if len(records) == 0 {
		return []domain.ProductManagementComboGroup{}
	}
	if s.skuCombos == nil {
		return productManagementSingleGroups(records)
	}
	skus := make([]string, 0, len(records))
	seenSKU := make(map[string]struct{})
	for _, record := range records {
		if record == nil {
			continue
		}
		sku := strings.TrimSpace(record.SKUCode)
		if sku == "" {
			continue
		}
		key := strings.ToUpper(sku)
		if _, ok := seenSKU[key]; !ok {
			seenSKU[key] = struct{}{}
			skus = append(skus, sku)
		}
	}
	if len(skus) == 0 {
		return productManagementSingleGroups(records)
	}
	relations, err := s.skuCombos.ListRelationsByChildSKUs(ctx, skus)
	if err != nil {
		return productManagementSingleGroups(records)
	}
	relationsByChild := make(map[string][]*domain.OMPSKUComboRelationWithRecord)
	for _, rel := range relations {
		if rel == nil {
			continue
		}
		childKey := strings.ToUpper(strings.TrimSpace(rel.Relation.ChildSKUCode))
		if childKey == "" {
			continue
		}
		relationsByChild[childKey] = append(relationsByChild[childKey], rel)
	}
	groupMap := make(map[string]*domain.ProductManagementComboGroup)
	groups := make([]domain.ProductManagementComboGroup, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		childKey := strings.ToUpper(strings.TrimSpace(record.SKUCode))
		rels := relationsByChild[childKey]
		if len(rels) == 0 {
			groups = append(groups, productManagementSingleGroup(record))
			continue
		}
		addedToCombo := false
		for _, rel := range rels {
			if rel == nil {
				continue
			}
			comboCode := strings.TrimSpace(rel.Relation.ComboSKUCode)
			if comboCode == "" {
				continue
			}
			groupKey := "combo:" + strings.ToUpper(comboCode)
			group := groupMap[groupKey]
			if group == nil {
				group = &domain.ProductManagementComboGroup{
					GroupKey:     groupKey,
					GroupType:    "combo",
					ComboSKUCode: comboCode,
					Children:     []domain.ProductManagementComboChild{},
				}
				if rel.Record != nil {
					domain.HydrateOMPSKUComboRecordDerived(rel.Record)
					group.ComboName = strings.TrimSpace(rel.Record.Name)
					group.ComboShortName = strings.TrimSpace(rel.Record.ShortName)
					group.ERPIID = strings.TrimSpace(rel.Record.ERPIID)
					group.EntitySKUID = strings.TrimSpace(rel.Record.EntitySKUID)
					group.PicURL = strings.TrimSpace(rel.Record.PicURL)
					group.Brand = strings.TrimSpace(rel.Record.Brand)
					group.VCName = strings.TrimSpace(rel.Record.VCName)
					group.Properties = strings.TrimSpace(rel.Record.Properties)
					group.Enabled = rel.Record.Enabled
					group.CostPrice = rel.Record.CostPrice
					group.SalePrice = rel.Record.SalePrice
					group.Weight = rel.Record.Weight
					group.SKUQty = rel.Record.SKUQty
					group.ERPCreatedAt = rel.Record.ERPCreatedAt
					group.ModifiedAt = rel.Record.ModifiedAt
					if !rel.Record.LastSyncedAt.IsZero() {
						group.LastSyncedAt = &rel.Record.LastSyncedAt
					}
				}
				groupMap[groupKey] = group
				groups = append(groups, *group)
			}
			qty := rel.Relation.Quantity
			if qty <= 0 {
				qty = 1
			}
			group.Children = append(group.Children, domain.ProductManagementComboChild{Record: record, Quantity: qty})
			addedToCombo = true
		}
		if !addedToCombo {
			groups = append(groups, productManagementSingleGroup(record))
		}
	}
	for idx := range groups {
		if groups[idx].GroupType != "combo" {
			continue
		}
		if group := groupMap[groups[idx].GroupKey]; group != nil {
			groups[idx] = *group
		}
	}
	return groups
}

func productManagementSingleGroups(records []*domain.ProductManagementRecord) []domain.ProductManagementComboGroup {
	groups := make([]domain.ProductManagementComboGroup, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		groups = append(groups, productManagementSingleGroup(record))
	}
	return groups
}

func hasExactProductManagementIdentifier(records []*domain.ProductManagementRecord, keyword string) bool {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return false
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		for _, value := range []string{record.SKUCode, record.TaskNo, record.ProductIID, record.ERPIID} {
			if strings.EqualFold(strings.TrimSpace(value), keyword) {
				return true
			}
		}
	}
	return false
}

func productManagementSingleGroup(record *domain.ProductManagementRecord) domain.ProductManagementComboGroup {
	return domain.ProductManagementComboGroup{
		GroupKey:  fmt.Sprintf("single:%d", record.ID),
		GroupType: "single",
		Children:  []domain.ProductManagementComboChild{{Record: record, Quantity: 1}},
	}
}

func paginateProductManagementComboGroups(groups []domain.ProductManagementComboGroup, page, pageSize int) []domain.ProductManagementComboGroup {
	page, pageSize = normalizeProductManagementPage(page, pageSize)
	start := (page - 1) * pageSize
	if start >= len(groups) {
		return []domain.ProductManagementComboGroup{}
	}
	end := start + pageSize
	if end > len(groups) {
		end = len(groups)
	}
	return groups[start:end]
}

func productManagementRecordsFromComboGroups(groups []domain.ProductManagementComboGroup) []*domain.ProductManagementRecord {
	out := make([]*domain.ProductManagementRecord, 0, len(groups))
	seen := make(map[int64]struct{})
	for _, group := range groups {
		for _, child := range group.Children {
			if child.Record == nil {
				continue
			}
			if _, ok := seen[child.Record.ID]; ok {
				continue
			}
			seen[child.Record.ID] = struct{}{}
			out = append(out, child.Record)
		}
	}
	return out
}

func isMySQLTableMissing(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

func productManagementDecoratedImageSyncStatus(record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch, patchChanged bool) domain.ProductManagementERPSyncStatus {
	patchStatus := patch.ImageSyncStatus
	if patchStatus == "" {
		patchStatus = domain.ProductManagementERPSyncStatusWaitingImage
		if patch.ImageAssetID != nil && *patch.ImageAssetID > 0 {
			patchStatus = domain.ProductManagementERPSyncStatusPendingSync
		}
	}
	if record == nil {
		return patchStatus
	}
	if productManagementStatusInFlight(record.ERPSyncStatus) {
		return record.ERPSyncStatus
	}
	if productManagementStatusInFlight(record.ImageSyncStatus) {
		return record.ImageSyncStatus
	}
	if !patchChanged && record.ImageSyncStatus != "" {
		if record.ImageSyncStatus == domain.ProductManagementERPSyncStatusSynced && record.LastImageSyncedAt == nil {
			return domain.ProductManagementERPSyncStatusPendingSync
		}
		return record.ImageSyncStatus
	}
	return patchStatus
}

func (s *productManagementService) persistAutoImagePatch(ctx context.Context, record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch) {
	if record == nil || !productManagementImagePatchChanged(record, patch) {
		return
	}
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.records.UpdateImage(ctx, tx, record.ID, patch)
	})
}

func productManagementImagePatchChanged(record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch) bool {
	if record.ImageSource != patch.ImageSource {
		return true
	}
	if record.ImageSelectionMode != patch.ImageSelectionMode {
		return true
	}
	if !sameProductManagementInt64Ptr(record.ImageAssetID, patch.ImageAssetID) {
		return true
	}
	if !sameProductManagementInt64Ptr(record.ImageAssetVersionID, patch.ImageAssetVersionID) {
		return true
	}
	if strings.TrimSpace(record.ImageFilename) != strings.TrimSpace(patch.ImageFilename) {
		return true
	}
	if strings.TrimSpace(record.ImageMimeType) != strings.TrimSpace(patch.ImageMimeType) {
		return true
	}
	return strings.TrimSpace(record.ImageMissingReason) != strings.TrimSpace(patch.ImageMissingReason)
}

func sameProductManagementInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *productManagementService) autoImagePatch(ctx context.Context, record *domain.ProductManagementRecord) repo.ProductManagementImagePatch {
	return s.autoImagePatchWithCache(ctx, record, nil)
}

func (s *productManagementService) autoImagePatchWithCache(ctx context.Context, record *domain.ProductManagementRecord, assetsByTaskID map[int64][]*domain.TaskAsset) repo.ProductManagementImagePatch {
	if record != nil && record.ImageSource == domain.ProductManagementImageSourceAutoOnClose && record.ImageAssetID != nil && *record.ImageAssetID > 0 {
		return repo.ProductManagementImagePatch{
			ImageSource:         record.ImageSource,
			ImageSelectionMode:  record.ImageSelectionMode,
			ImageAssetID:        record.ImageAssetID,
			ImageAssetVersionID: record.ImageAssetVersionID,
			ImageFilename:       record.ImageFilename,
			ImageMimeType:       record.ImageMimeType,
			ImageMissingReason:  record.ImageMissingReason,
			ImageSyncSource:     record.ImageSyncSource,
			ImageSyncStatus:     record.ImageSyncStatus,
		}
	}
	candidates, err := s.imageCandidatesForRecordWithCache(ctx, record, assetsByTaskID)
	if err != nil || len(candidates) == 0 {
		reason := "ERP 图片待补充"
		if record != nil && strings.EqualFold(strings.TrimSpace(record.TaskType), string(domain.TaskTypePurchaseTask)) {
			reason = "采购任务不会自动产生设计成品图，请上传 ERP 商品图"
		}
		return repo.ProductManagementImagePatch{
			ImageSource:        domain.ProductManagementImageSourceMissing,
			ImageSelectionMode: domain.ProductManagementImageSelectionAuto,
			ImageMissingReason: reason,
			ImageSyncSource:    domain.ProductManagementImageSourceMissing,
			ImageSyncStatus:    domain.ProductManagementERPSyncStatusWaitingImage,
		}
	}
	return imagePatchFromCandidate(candidates[0])
}

func (s *productManagementService) imageCandidatesForRecord(ctx context.Context, record *domain.ProductManagementRecord) ([]*domain.ProductManagementImageCandidate, error) {
	return s.imageCandidatesForRecordWithCache(ctx, record, nil)
}

func (s *productManagementService) imageCandidatesForRecordWithCache(ctx context.Context, record *domain.ProductManagementRecord, assetsByTaskID map[int64][]*domain.TaskAsset) ([]*domain.ProductManagementImageCandidate, error) {
	if record == nil || record.TaskID <= 0 {
		return []*domain.ProductManagementImageCandidate{}, nil
	}
	assets, ok := assetsByTaskID[record.TaskID]
	if assetsByTaskID == nil {
		ok = false
	}
	if !ok {
		var err error
		assets, err = s.taskAssets.ListByTaskID(ctx, record.TaskID)
		if err != nil {
			return nil, err
		}
		if assetsByTaskID != nil {
			assetsByTaskID[record.TaskID] = assets
		}
	}
	sku := strings.TrimSpace(record.SKUCode)
	candidates := make([]*domain.ProductManagementImageCandidate, 0, len(assets))
	sources := []domain.ProductManagementImageSource{
		domain.ProductManagementImageSourceERPProduct,
	}
	if !strings.EqualFold(strings.TrimSpace(record.TaskType), string(domain.TaskTypePurchaseTask)) {
		sources = append(sources,
			domain.ProductManagementImageSourceDelivery,
			domain.ProductManagementImageSourceDerivedPreview,
			domain.ProductManagementImageSourceTaskReference,
		)
	}
	for _, source := range sources {
		for _, asset := range bestAssetsBySource(assets, sku, source) {
			candidates = append(candidates, candidateFromAsset(asset, record.TaskNo, source))
		}
	}
	return candidates, nil
}

func bestAssetsBySource(assets []*domain.TaskAsset, sku string, source domain.ProductManagementImageSource) []*domain.TaskAsset {
	out := make([]*domain.TaskAsset, 0, len(assets))
	for _, asset := range assets {
		if asset == nil || asset.AssetID == nil || asset.DeletedAt != nil || asset.IsArchived {
			continue
		}
		if !isProductManagementERPImageAsset(asset) {
			continue
		}
		if strings.TrimSpace(sku) != "" {
			if asset.ScopeSKUCode == nil || strings.TrimSpace(*asset.ScopeSKUCode) == "" || !strings.EqualFold(strings.TrimSpace(*asset.ScopeSKUCode), sku) {
				continue
			}
		}
		assetType := domain.NormalizeTaskAssetType(asset.AssetType)
		switch source {
		case domain.ProductManagementImageSourceERPProduct:
			if !assetType.IsERPProductImage() {
				continue
			}
		case domain.ProductManagementImageSourceDelivery:
			if !assetType.IsDelivery() {
				continue
			}
		case domain.ProductManagementImageSourceDerivedPreview:
			if !assetType.IsPreview() && !assetType.IsDesignThumb() {
				continue
			}
		case domain.ProductManagementImageSourceTaskReference:
			if !assetType.IsReference() {
				continue
			}
		default:
			continue
		}
		out = appendNewestTaskAsset(out, asset)
	}
	return out
}

func (s *productManagementService) finalizedImageCandidatesForRecord(ctx context.Context, record *domain.ProductManagementRecord, assetsByTaskID map[int64][]*domain.TaskAsset) ([]*domain.ProductManagementImageCandidate, error) {
	if record == nil || record.TaskID <= 0 || strings.TrimSpace(record.SKUCode) == "" || s.finalizedResources == nil {
		return []*domain.ProductManagementImageCandidate{}, nil
	}
	flat, _, err := s.finalizedResources.ListFlatResourceItems(ctx, domain.ResourceGroupListParams{
		TaskID:       record.TaskID,
		SKUCode:      strings.TrimSpace(record.SKUCode),
		ResourceRole: domain.ResourceRoleFilterFinal,
		Page:         1,
		PageSize:     100,
		Access:       domain.ResourceGroupAccessFilter{Global: true},
	})
	if err != nil {
		return nil, err
	}
	if len(flat) == 0 {
		return []*domain.ProductManagementImageCandidate{}, nil
	}
	if s.taskAssets == nil {
		return nil, errors.New("task asset repository is not configured")
	}
	assets, ok := assetsByTaskID[record.TaskID]
	if assetsByTaskID == nil {
		ok = false
	}
	if !ok {
		assets, err = s.taskAssets.ListByTaskID(ctx, record.TaskID)
		if err != nil {
			return nil, err
		}
		if assetsByTaskID != nil {
			assetsByTaskID[record.TaskID] = assets
		}
	}
	byID := make(map[int64]*domain.TaskAsset, len(assets))
	previewsBySource := make(map[int64][]*domain.TaskAsset)
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		byID[asset.ID] = asset
		if asset.SourceAssetVersionID != nil && *asset.SourceAssetVersionID > 0 && isProductManagementERPImageAsset(asset) {
			assetType := domain.NormalizeTaskAssetType(asset.AssetType)
			if assetType.IsPreview() || assetType.IsDesignThumb() {
				previewsBySource[*asset.SourceAssetVersionID] = appendNewestTaskAsset(previewsBySource[*asset.SourceAssetVersionID], asset)
			}
		}
	}
	candidates := make([]*domain.ProductManagementImageCandidate, 0, len(flat))
	for _, item := range flat {
		if item.TaskAssetID <= 0 || !strings.EqualFold(strings.TrimSpace(item.SKUCode), strings.TrimSpace(record.SKUCode)) {
			continue
		}
		if asset := byID[item.TaskAssetID]; isProductManagementERPImageAsset(asset) {
			candidate := candidateFromAsset(asset, record.TaskNo, domain.ProductManagementImageSourceDelivery)
			if candidate == nil {
				continue
			}
			candidate.SKUCode = strings.TrimSpace(record.SKUCode)
			candidates = append(candidates, candidate)
			continue
		}
		previews := previewsBySource[item.TaskAssetID]
		sortTaskAssetsNewestFirst(previews)
		if len(previews) > 0 {
			candidate := candidateFromAsset(previews[0], record.TaskNo, domain.ProductManagementImageSourceDerivedPreview)
			if candidate == nil {
				continue
			}
			candidate.SKUCode = strings.TrimSpace(record.SKUCode)
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

func sortTaskAssetsNewestFirst(assets []*domain.TaskAsset) {
	sort.SliceStable(assets, func(i, j int) bool { return taskAssetNewer(assets[i], assets[j]) })
}

func firstProductManagementImageCandidate(candidates []*domain.ProductManagementImageCandidate) *domain.ProductManagementImageCandidate {
	for _, candidate := range candidates {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func productManagementImageSourceRequiresFinalized(source domain.ProductManagementImageSource) bool {
	switch source {
	case domain.ProductManagementImageSourceDelivery,
		domain.ProductManagementImageSourceDerivedPreview,
		domain.ProductManagementImageSourceTaskReference,
		domain.ProductManagementImageSourceAutoOnClose:
		return true
	default:
		return false
	}
}

func applyProductManagementImagePatch(record *domain.ProductManagementRecord, patch repo.ProductManagementImagePatch) {
	if record == nil {
		return
	}
	record.ImageSource = patch.ImageSource
	record.ImageSelectionMode = patch.ImageSelectionMode
	record.ImageAssetID = patch.ImageAssetID
	record.ImageAssetVersionID = patch.ImageAssetVersionID
	record.ImageFilename = patch.ImageFilename
	record.ImageMimeType = patch.ImageMimeType
	record.ImageMissingReason = patch.ImageMissingReason
	record.ImageSyncSource = patch.ImageSyncSource
	record.ImageSyncStatus = patch.ImageSyncStatus
}

func productManagementERPImageURLsMatch(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	if expected == "" {
		return true
	}
	expectedURL, expectedErr := url.Parse(expected)
	actualURL, actualErr := url.Parse(actual)
	if expectedErr != nil || actualErr != nil || expectedURL.Host == "" || actualURL.Host == "" {
		return expected == actual
	}
	return strings.EqualFold(expectedURL.Scheme, actualURL.Scheme) &&
		strings.EqualFold(expectedURL.Host, actualURL.Host) &&
		expectedURL.EscapedPath() == actualURL.EscapedPath()
}

func appendNewestTaskAsset(items []*domain.TaskAsset, asset *domain.TaskAsset) []*domain.TaskAsset {
	for idx, existing := range items {
		if existing != nil && existing.AssetID != nil && asset.AssetID != nil && *existing.AssetID == *asset.AssetID {
			if taskAssetNewer(asset, existing) {
				items[idx] = asset
			}
			return items
		}
	}
	return append(items, asset)
}

func taskAssetNewer(a, b *domain.TaskAsset) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	if a.AssetVersionNo != nil && b.AssetVersionNo != nil && *a.AssetVersionNo != *b.AssetVersionNo {
		return *a.AssetVersionNo > *b.AssetVersionNo
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return a.ID > b.ID
}

func candidateFromAsset(asset *domain.TaskAsset, taskNo string, source domain.ProductManagementImageSource) *domain.ProductManagementImageCandidate {
	if asset == nil || asset.AssetID == nil {
		return nil
	}
	sku := ""
	if asset.ScopeSKUCode != nil {
		sku = strings.TrimSpace(*asset.ScopeSKUCode)
	}
	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.TrimSpace(*asset.MimeType)
	}
	return &domain.ProductManagementImageCandidate{
		AssetID:        *asset.AssetID,
		AssetVersionID: asset.ID,
		TaskID:         asset.TaskID,
		TaskNo:         strings.TrimSpace(taskNo),
		SKUCode:        sku,
		Source:         source,
		SourceLabel:    domain.ProductManagementImageSourceLabel(source),
		PreviewURL:     fmt.Sprintf("/v1/assets/%d/preview", *asset.AssetID),
		FileName:       asset.FileName,
		MimeType:       mimeType,
		CreatedAt:      asset.CreatedAt,
	}
}

func imagePatchFromCandidate(candidate *domain.ProductManagementImageCandidate) repo.ProductManagementImagePatch {
	if candidate == nil || candidate.AssetID <= 0 {
		return repo.ProductManagementImagePatch{
			ImageSource:        domain.ProductManagementImageSourceMissing,
			ImageSelectionMode: domain.ProductManagementImageSelectionAuto,
			ImageMissingReason: "ERP 图片待补充",
			ImageSyncSource:    domain.ProductManagementImageSourceMissing,
			ImageSyncStatus:    domain.ProductManagementERPSyncStatusWaitingImage,
		}
	}
	assetID := candidate.AssetID
	versionID := candidate.AssetVersionID
	mode := domain.ProductManagementImageSelectionAuto
	if candidate.Source == domain.ProductManagementImageSourceManual {
		mode = domain.ProductManagementImageSelectionManual
	}
	return repo.ProductManagementImagePatch{
		ImageSource:         candidate.Source,
		ImageSelectionMode:  mode,
		ImageAssetID:        &assetID,
		ImageAssetVersionID: &versionID,
		ImageFilename:       candidate.FileName,
		ImageMimeType:       candidate.MimeType,
		ImageSyncSource:     candidate.Source,
		ImageSyncStatus:     domain.ProductManagementERPSyncStatusPendingSync,
	}
}

func isProductManagementAdmin(actor domain.RequestActor) bool {
	return actorHasAnyRole(actor, domain.RoleERP, domain.RoleAdmin, domain.RoleSuperAdmin)
}

func canMaintainProductManagementImage(actor domain.RequestActor, record *domain.ProductManagementRecord) bool {
	if record == nil {
		return false
	}
	if isProductManagementAdmin(actor) {
		return true
	}
	return actor.ID > 0 && actor.ID == record.CreatorID
}

func canSyncProductManagementERP(actor domain.RequestActor, record *domain.ProductManagementRecord) bool {
	if record == nil {
		return false
	}
	if isProductManagementAdmin(actor) {
		return true
	}
	return actor.ID > 0 && actor.ID == record.CreatorID
}

func actorHasAnyRole(actor domain.RequestActor, roles ...domain.Role) bool {
	for _, role := range actor.Roles {
		for _, candidate := range roles {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func normalizeProductManagementPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func normalizeProductManagementDisplayScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "combo":
		return "combo"
	case "single":
		return "single"
	default:
		return "all"
	}
}

func isProductManagementERPImageAsset(asset *domain.TaskAsset) bool {
	if asset == nil {
		return false
	}
	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.ToLower(strings.TrimSpace(*asset.MimeType))
	}
	switch mimeType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif":
		return true
	case "image/vnd.adobe.photoshop", "image/tiff", "image/heic", "image/heif", "image/avif":
		return false
	}
	filename := strings.ToLower(strings.TrimSpace(asset.FileName))
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(filename, suffix) {
			return true
		}
	}
	return false
}

func isAbsoluteHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func truncateProductManagementSyncError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "ERP 图片同步失败"
	}
	const max = 500
	runes := []rune(message)
	if len(runes) <= max {
		return message
	}
	return string(runes[:max])
}

func infraAppError(message string, err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInternalError, message, map[string]string{"cause": err.Error()})
}
