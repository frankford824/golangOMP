package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	erpBridgeCategoryCacheTTL       = time.Minute
	erpBridgeRefinementMaxPages     = 20
	erpBridgeDetailFallbackPageSize = 20
)

type ERPBridgeService interface {
	// Query behavior stays Bridge-owned even when MAIN exposes a facade route.
	SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError)
	ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError)
	GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, *domain.AppError)
	ListCategories(ctx context.Context) ([]*domain.ERPCategory, *domain.AppError)
	ListWarehouses(ctx context.Context) ([]domain.ERPWarehouse, *domain.AppError)
	ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError)
	GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, *domain.AppError)
	EnsureLocalProduct(ctx context.Context, tx repo.Tx, snapshot *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError)
	// Mutation execution remains Bridge-owned; MAIN may invoke this only from explicit business boundaries.
	UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError)
	UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError)
	ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError)
	UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError)
	UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError)
	ListJSTUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError)
}

type erpBridgeService struct {
	client      ERPBridgeClient
	productRepo repo.ProductRepo
	txRunner    repo.TxRunner
	categoryMu  sync.RWMutex
	categorySet *erpBridgeCategoryCatalog
	warehouses  []domain.ERPWarehouse
}

type erpBridgeCategoryCatalog struct {
	items     []*domain.ERPCategory
	byID      map[string]*domain.ERPCategory
	byName    map[string]*domain.ERPCategory
	expiresAt time.Time
}

type erpBridgeCategoryConstraint struct {
	id   string
	name string
}

func NewERPBridgeService(client ERPBridgeClient, productRepo repo.ProductRepo, txRunner repo.TxRunner) ERPBridgeService {
	return &erpBridgeService{
		client:      client,
		productRepo: productRepo,
		txRunner:    txRunner,
		warehouses:  defaultERPWarehouses(),
	}
}

func (s *erpBridgeService) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	normalized := normalizeERPProductSearchFilter(filter)
	catalog, appErr := s.loadCategoryCatalog(ctx)
	if appErr != nil && (normalized.CategoryID != "" || normalized.CategoryName != "") {
		return nil, appErr
	}
	constraint, categoryInvalid := resolveERPBridgeCategoryConstraint(normalized, catalog)
	if categoryInvalid {
		if constraint != nil {
			normalized.CategoryID = constraint.id
			normalized.CategoryName = constraint.name
		}
		return emptyERPProductListResponse(normalized), nil
	}
	if constraint != nil {
		normalized.CategoryID = constraint.id
		normalized.CategoryName = constraint.name
	}

	items, appErr := s.searchERPBridgeProducts(ctx, normalized, catalog, constraint)
	if appErr != nil {
		return nil, appErr
	}
	items.NormalizedFilters = &normalized
	if items.Pagination.Page <= 0 {
		items.Pagination.Page = normalized.Page
	}
	if items.Pagination.PageSize <= 0 {
		items.Pagination.PageSize = normalized.PageSize
	}
	stabilizeERPProductPagination(&items.Pagination, len(items.Items))
	return items, nil
}

func (s *erpBridgeService) ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	normalized := normalizeERPIIDListFilter(filter)
	if s.productRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "product repo is unavailable", nil)
	}
	items, total, err := s.productRepo.ListIIDs(ctx, repo.ProductIIDListFilter{
		Q:        normalized.Q,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	})
	if err != nil {
		return nil, infraError("list erp i_id options", err)
	}
	return &domain.ERPIIDListResponse{
		Items:             items,
		Pagination:        buildPaginationMeta(normalized.Page, normalized.PageSize, total),
		NormalizedFilters: normalized,
	}, nil
}

func normalizeERPIIDListFilter(filter domain.ERPIIDListFilter) domain.ERPIIDListFilter {
	filter.Q = strings.TrimSpace(filter.Q)
	filter.Page = normalizePositiveInt(filter.Page, 1)
	filter.PageSize = normalizePositiveInt(filter.PageSize, 50)
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	return filter
}

func (s *erpBridgeService) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid erp product id", nil)
	}
	catalog, appErr := s.loadCategoryCatalog(ctx)
	if appErr != nil {
		catalog = nil
	}
	product, err := s.client.GetProductByID(ctx, id)
	if err != nil {
		var rnf *erpBridgeRemoteProductNotFoundError
		if errors.As(err, &rnf) {
			// OpenWeb truth: SKU not in ERP catalog; do not stitch from local products.
			return nil, domain.ErrNotFound
		}
		if !isERPBridgeHTTPStatus(err, http.StatusNotFound) {
			return nil, mapERPBridgeError("get erp bridge product", err)
		}
		product, appErr = s.findERPProductByReference(ctx, id, catalog)
		if appErr != nil {
			return nil, appErr
		}
		if product == nil {
			return nil, domain.ErrNotFound
		}
		if strings.TrimSpace(product.ProductID) != "" && !strings.EqualFold(strings.TrimSpace(product.ProductID), id) {
			detail, detailErr := s.client.GetProductByID(ctx, product.ProductID)
			if detailErr == nil && detail != nil {
				product = mergeERPProducts(prepareERPProduct(product, catalog), prepareERPProduct(detail, catalog))
			}
		}
		return prepareERPProduct(product, catalog), nil
	}
	if product == nil {
		product, appErr = s.findERPProductByReference(ctx, id, catalog)
		if appErr != nil {
			return nil, appErr
		}
		if product == nil {
			return nil, domain.ErrNotFound
		}
	}
	return prepareERPProduct(product, catalog), nil
}

func (s *erpBridgeService) ListCategories(ctx context.Context) ([]*domain.ERPCategory, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	catalog, appErr := s.loadCategoryCatalog(ctx)
	if appErr != nil {
		return nil, appErr
	}
	if catalog == nil || catalog.items == nil {
		return []*domain.ERPCategory{}, nil
	}
	items := make([]*domain.ERPCategory, 0, len(catalog.items))
	for _, item := range catalog.items {
		copied := *item
		items = append(items, &copied)
	}
	return items, nil
}

func (s *erpBridgeService) ListWarehouses(_ context.Context) ([]domain.ERPWarehouse, *domain.AppError) {
	items := make([]domain.ERPWarehouse, 0, len(s.warehouses))
	items = append(items, s.warehouses...)
	return items, nil
}

func (s *erpBridgeService) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Connector = strings.TrimSpace(filter.Connector)
	filter.Operation = strings.TrimSpace(filter.Operation)
	filter.ResourceType = strings.TrimSpace(filter.ResourceType)
	items, err := s.client.ListSyncLogs(ctx, filter)
	if err != nil {
		return nil, mapERPBridgeError("list erp bridge sync logs", err)
	}
	if items == nil {
		items = &domain.ERPSyncLogListResponse{
			Items:      []*domain.ERPSyncLog{},
			Pagination: buildPaginationMeta(filter.Page, filter.PageSize, 0),
		}
	}
	if items.Items == nil {
		items.Items = []*domain.ERPSyncLog{}
	}
	if items.Pagination.Page <= 0 {
		items.Pagination.Page = filter.Page
	}
	if items.Pagination.PageSize <= 0 {
		items.Pagination.PageSize = filter.PageSize
	}
	return items, nil
}

func (s *erpBridgeService) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid erp sync log id", nil)
	}
	item, err := s.client.GetSyncLogByID(ctx, id)
	if err != nil {
		if isERPBridgeHTTPStatus(err, http.StatusNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, mapERPBridgeError("get erp bridge sync log", err)
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *erpBridgeService) EnsureLocalProduct(ctx context.Context, tx repo.Tx, snapshot *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError) {
	if s.productRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge local product binding is unavailable", nil)
	}
	snapshot = normalizeERPProductSelectionSnapshot(snapshot)
	if snapshot == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.erp_product is required for erp bridge selection binding", nil)
	}

	erpProductID := strings.TrimSpace(snapshot.ProductID)
	if erpProductID == "" {
		erpProductID = strings.TrimSpace(snapshot.SKUID)
	}
	if erpProductID == "" {
		erpProductID = strings.TrimSpace(snapshot.SKUCode)
	}
	if erpProductID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.erp_product.product_id or sku_id or sku_code is required", nil)
	}

	localProduct, err := s.productRepo.GetByERPProductID(ctx, erpProductID)
	if err != nil {
		return nil, infraError("get local erp bridge product binding", err)
	}

	storedSnapshot := erpProductSnapshotFromSpecJSON("")
	if localProduct != nil {
		storedSnapshot = erpProductSnapshotFromSpecJSON(localProduct.SpecJSON)
	}
	snapshot = mergeERPProductSelectionSnapshots(storedSnapshot, snapshot)
	snapshot = hydrateERPProductSelectionSnapshot(snapshot, localProduct, nil)

	specJSON, marshalErr := json.Marshal(snapshot)
	if marshalErr != nil {
		return nil, infraError("marshal erp bridge product snapshot", marshalErr)
	}

	productName := firstNonEmptyString(snapshot.ProductName)
	if productName == "" && localProduct != nil {
		productName = strings.TrimSpace(localProduct.ProductName)
	}
	if productName == "" {
		productName = firstNonEmptyString(snapshot.SKUCode, snapshot.SKUID, erpProductID)
	}

	skuCode := firstNonEmptyString(snapshot.SKUCode)
	if skuCode == "" && localProduct != nil {
		skuCode = strings.TrimSpace(localProduct.SKUCode)
	}
	if skuCode == "" {
		skuCode = firstNonEmptyString(snapshot.SKUID, erpProductID)
	}

	status := "active"
	if localProduct != nil && strings.TrimSpace(localProduct.Status) != "" {
		status = strings.TrimSpace(localProduct.Status)
	}
	product := &domain.Product{
		ERPProductID: erpProductID,
		SKUCode:      skuCode,
		ProductName:  productName,
		Category:     firstNonEmptyString(snapshot.CategoryName),
		SpecJSON:     string(specJSON),
		Status:       status,
	}
	if localProduct != nil && product.Category == "" {
		product.Category = strings.TrimSpace(localProduct.Category)
	}

	if tx != nil {
		if _, err := s.productRepo.UpsertBatch(ctx, tx, []*domain.Product{product}); err != nil {
			return nil, infraError("upsert erp bridge product binding", err)
		}
	} else {
		if s.txRunner == nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge product binding transaction runner is unavailable", nil)
		}
		if err := s.txRunner.RunInTx(ctx, func(innerTx repo.Tx) error {
			_, err := s.productRepo.UpsertBatch(ctx, innerTx, []*domain.Product{product})
			return err
		}); err != nil {
			return nil, infraError("upsert erp bridge product binding", err)
		}
	}

	if localProduct, err = s.productRepo.GetByERPProductID(ctx, erpProductID); err != nil {
		return nil, infraError("reload erp bridge product binding", err)
	}
	if localProduct == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge product binding was not persisted", nil)
	}
	return localProduct, nil
}

func (s *erpBridgeService) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	payload = normalizeERPProductUpsertPayload(payload)
	if strings.TrimSpace(payload.SKUID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_id is required", nil)
	}
	result, err := s.client.UpsertProduct(ctx, payload)
	if err != nil {
		return nil, mapERPBridgeError("upsert erp bridge product", err)
	}
	if result == nil {
		result = &domain.ERPProductUpsertResult{}
	}
	if strings.TrimSpace(result.ProductID) == "" {
		result.ProductID = payload.ProductID
	}
	if strings.TrimSpace(result.SKUID) == "" {
		result.SKUID = payload.SKUID
	}
	if strings.TrimSpace(result.IID) == "" {
		result.IID = payload.IID
	}
	if strings.TrimSpace(result.SKUCode) == "" {
		result.SKUCode = payload.SKUCode
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = payload.Name
	}
	if strings.TrimSpace(result.ProductName) == "" {
		result.ProductName = payload.ProductName
	}
	if strings.TrimSpace(result.ShortName) == "" {
		result.ShortName = payload.ShortName
	}
	if strings.TrimSpace(result.CategoryID) == "" {
		result.CategoryID = payload.CategoryID
	}
	if strings.TrimSpace(result.CategoryCode) == "" {
		result.CategoryCode = payload.CategoryCode
	}
	if strings.TrimSpace(result.CategoryName) == "" {
		result.CategoryName = payload.CategoryName
	}
	if strings.TrimSpace(result.ProductShortName) == "" {
		result.ProductShortName = payload.ProductShortName
	}
	if result.SPrice == nil {
		result.SPrice = payload.SPrice
	}
	if strings.TrimSpace(result.WMSCoID) == "" {
		result.WMSCoID = payload.WMSCoID
	}
	if strings.TrimSpace(result.Route) == "" {
		result.Route = "itemskubatchupload"
	}
	return result, nil
}

func (s *erpBridgeService) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	payload = normalizeERPItemStyleUpdatePayload(payload)
	if strings.TrimSpace(payload.IID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "i_id is required", nil)
	}
	result, err := s.client.UpdateItemStyle(ctx, payload)
	if err != nil {
		return nil, mapERPBridgeError("update erp bridge item style", err)
	}
	if result == nil {
		result = &domain.ERPItemStyleUpdateResult{}
	}
	if strings.TrimSpace(result.SKUID) == "" {
		result.SKUID = payload.SKUID
	}
	if strings.TrimSpace(result.IID) == "" {
		result.IID = payload.IID
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = payload.Name
	}
	if strings.TrimSpace(result.ShortName) == "" {
		result.ShortName = payload.ShortName
	}
	if strings.TrimSpace(result.Route) == "" {
		result.Route = "itemupload"
	}
	return result, nil
}

func (s *erpBridgeService) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	payload = normalizeERPBridgeBatchMutationPayload(payload)
	if len(payload.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "shelve batch requires non-empty items", nil)
	}
	result, err := s.client.ShelveProductsBatch(ctx, payload)
	if err != nil {
		return nil, mapERPBridgeError("shelve erp bridge products batch", err)
	}
	if result == nil {
		result = &domain.ERPProductBatchMutationResult{
			Action:   "shelve",
			Total:    len(payload.Items),
			Accepted: len(payload.Items),
			Status:   "accepted",
		}
	}
	if result.Action == "" {
		result.Action = "shelve"
	}
	return result, nil
}

func (s *erpBridgeService) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	payload = normalizeERPBridgeBatchMutationPayload(payload)
	if len(payload.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unshelve batch requires non-empty items", nil)
	}
	result, err := s.client.UnshelveProductsBatch(ctx, payload)
	if err != nil {
		return nil, mapERPBridgeError("unshelve erp bridge products batch", err)
	}
	if result == nil {
		result = &domain.ERPProductBatchMutationResult{
			Action:   "unshelve",
			Total:    len(payload.Items),
			Accepted: len(payload.Items),
			Status:   "accepted",
		}
	}
	if result.Action == "" {
		result.Action = "unshelve"
	}
	return result, nil
}

func (s *erpBridgeService) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	payload = normalizeERPBridgeVirtualInventoryPayload(payload)
	if len(payload.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "virtual inventory update requires non-empty items", nil)
	}
	result, err := s.client.UpdateVirtualInventory(ctx, payload)
	if err != nil {
		return nil, mapERPBridgeError("update erp bridge virtual inventory", err)
	}
	if result == nil {
		result = &domain.ERPVirtualInventoryUpdateResult{
			Total:    len(payload.Items),
			Accepted: len(payload.Items),
			Status:   "accepted",
		}
	}
	return result, nil
}

func (s *erpBridgeService) ListJSTUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError) {
	if s.client == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge client is unavailable", nil)
	}
	if filter.CurrentPage <= 0 {
		filter.CurrentPage = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	result, err := s.client.GetCompanyUsers(ctx, filter)
	if err != nil {
		return nil, mapERPBridgeError("list jst company users", err)
	}
	if result == nil {
		result = &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}
	}
	return result, nil
}

func (s *erpBridgeService) loadCategoryCatalog(ctx context.Context) (*erpBridgeCategoryCatalog, *domain.AppError) {
	now := time.Now()

	s.categoryMu.RLock()
	cached := s.categorySet
	if cached != nil && now.Before(cached.expiresAt) {
		s.categoryMu.RUnlock()
		return cached, nil
	}
	s.categoryMu.RUnlock()

	items, err := s.client.ListCategories(ctx)
	if err != nil {
		return nil, mapERPBridgeError("list erp bridge categories", err)
	}
	catalog := buildERPBridgeCategoryCatalog(items)

	s.categoryMu.Lock()
	s.categorySet = catalog
	s.categoryMu.Unlock()
	return catalog, nil
}

func buildERPBridgeCategoryCatalog(items []*domain.ERPCategory) *erpBridgeCategoryCatalog {
	catalog := &erpBridgeCategoryCatalog{
		items:     make([]*domain.ERPCategory, 0, len(items)),
		byID:      map[string]*domain.ERPCategory{},
		byName:    map[string]*domain.ERPCategory{},
		expiresAt: time.Now().Add(erpBridgeCategoryCacheTTL),
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copied := *item
		copied.CategoryID = strings.TrimSpace(copied.CategoryID)
		copied.CategoryName = strings.TrimSpace(copied.CategoryName)
		copied.ParentID = strings.TrimSpace(copied.ParentID)
		if copied.CategoryID == "" && copied.CategoryName == "" {
			continue
		}
		catalog.items = append(catalog.items, &copied)
		if key := normalizeERPBridgeText(copied.CategoryID); key != "" {
			catalog.byID[key] = &copied
		}
		if key := normalizeERPBridgeText(copied.CategoryName); key != "" {
			catalog.byName[key] = &copied
		}
	}
	sort.SliceStable(catalog.items, func(i, j int) bool {
		left := firstNonEmptyString(catalog.items[i].CategoryName, catalog.items[i].CategoryID)
		right := firstNonEmptyString(catalog.items[j].CategoryName, catalog.items[j].CategoryID)
		return strings.ToLower(left) < strings.ToLower(right)
	})
	return catalog
}

func resolveERPBridgeCategoryConstraint(filter domain.ERPProductSearchFilter, catalog *erpBridgeCategoryCatalog) (*erpBridgeCategoryConstraint, bool) {
	rawID := strings.TrimSpace(filter.CategoryID)
	rawName := strings.TrimSpace(filter.CategoryName)
	if rawID == "" && rawName == "" {
		return nil, false
	}

	constraint := &erpBridgeCategoryConstraint{id: rawID, name: rawName}
	if catalog == nil {
		return constraint, false
	}

	var byID, byName *domain.ERPCategory
	if rawID != "" {
		byID = catalog.byID[normalizeERPBridgeText(rawID)]
	}
	if rawName != "" {
		byName = catalog.byName[normalizeERPBridgeText(rawName)]
	}

	switch {
	case rawID != "" && rawName != "" && (byID == nil || byName == nil):
		return constraint, true
	case rawID != "" && rawName != "" && normalizeERPBridgeText(byID.CategoryID) != normalizeERPBridgeText(byName.CategoryID):
		return constraint, true
	case rawID != "" && byID == nil:
		return constraint, true
	case rawName != "" && byName == nil:
		return constraint, true
	}

	if byID != nil {
		constraint.id = byID.CategoryID
		constraint.name = firstNonEmptyString(byID.CategoryName, constraint.name)
	}
	if byName != nil {
		constraint.id = firstNonEmptyString(constraint.id, byName.CategoryID)
		constraint.name = firstNonEmptyString(byName.CategoryName, constraint.name)
	}
	return constraint, false
}

func (s *erpBridgeService) searchERPBridgeProducts(ctx context.Context, normalized domain.ERPProductSearchFilter, catalog *erpBridgeCategoryCatalog, constraint *erpBridgeCategoryConstraint) (*domain.ERPProductListResponse, *domain.AppError) {
	if needsERPBridgeLocalRefinement(normalized, constraint) {
		return s.searchERPBridgeProductsWithLocalRefinement(ctx, normalized, catalog, constraint)
	}

	items, err := s.client.SearchProducts(ctx, normalized)
	if err != nil {
		if shouldFallbackERPBridgeKeywordTimeout(err, normalized) {
			return s.searchERPBridgeProductsWithBrowseFallback(ctx, normalized, catalog, constraint)
		}
		return nil, mapERPBridgeError("search erp bridge products", err)
	}
	if items == nil {
		return emptyERPProductListResponse(normalized), nil
	}
	if items.Items == nil {
		items.Items = []*domain.ERPProduct{}
	}
	items.Items = prepareERPProducts(items.Items, catalog)
	if items.Pagination.Page <= 0 {
		items.Pagination.Page = normalized.Page
	}
	if items.Pagination.PageSize <= 0 {
		items.Pagination.PageSize = normalized.PageSize
	}
	stabilizeERPProductPagination(&items.Pagination, len(items.Items))
	if len(items.Items) == 0 {
		localItems, appErr := s.searchERPBridgeProductsFromLocalReplica(ctx, normalized, catalog, constraint)
		if appErr != nil {
			return nil, appErr
		}
		if localItems != nil {
			return localItems, nil
		}
	}
	return items, nil
}

func (s *erpBridgeService) searchERPBridgeProductsFromLocalReplica(ctx context.Context, normalized domain.ERPProductSearchFilter, catalog *erpBridgeCategoryCatalog, constraint *erpBridgeCategoryConstraint) (*domain.ERPProductListResponse, *domain.AppError) {
	if s.productRepo == nil || normalized.QueryMode != "keyword" || strings.TrimSpace(normalized.Q) == "" {
		return nil, nil
	}
	category := strings.TrimSpace(normalized.CategoryName)
	if category == "" && constraint != nil {
		category = strings.TrimSpace(constraint.name)
	}
	if category == "" {
		category = strings.TrimSpace(normalized.CategoryID)
	}
	products, total, err := s.productRepo.Search(ctx, repo.ProductSearchFilter{
		Keyword:  strings.TrimSpace(normalized.Q),
		Category: category,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	})
	if err != nil {
		return nil, infraError("search local erp products replica", err)
	}
	items := make([]*domain.ERPProduct, 0, len(products))
	for _, product := range products {
		if item := localERPProductFromDomain(product); item != nil {
			items = append(items, item)
		}
	}
	items = filterERPProducts(prepareERPProducts(items, catalog), normalized, constraint)
	if len(items) == 0 && total == 0 {
		return nil, nil
	}
	return &domain.ERPProductListResponse{
		Items: items,
		Pagination: domain.PaginationMeta{
			Page:     normalized.Page,
			PageSize: normalized.PageSize,
			Total:    total,
		},
	}, nil
}

func (s *erpBridgeService) searchERPBridgeProductsWithBrowseFallback(ctx context.Context, normalized domain.ERPProductSearchFilter, catalog *erpBridgeCategoryCatalog, constraint *erpBridgeCategoryConstraint) (*domain.ERPProductListResponse, *domain.AppError) {
	accumulated := make([]*domain.ERPProduct, 0, normalized.Page*normalized.PageSize)
	seen := map[string]struct{}{}

	for page := 1; page <= erpBridgeRefinementMaxPages; page++ {
		pageFilter := normalized
		pageFilter.Page = page
		pageFilter.Q = ""

		items, err := s.client.SearchProducts(ctx, pageFilter)
		if err != nil {
			return nil, mapERPBridgeError("search erp bridge products", err)
		}
		if items == nil || len(items.Items) == 0 {
			break
		}

		prepared := prepareERPProducts(items.Items, catalog)
		filtered := filterERPProducts(prepared, normalized, constraint)
		newCount := 0
		for _, item := range filtered {
			key := normalizeERPProductIdentityKey(item)
			if key == "" {
				key = normalizeERPBridgeText(item.ProductName)
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			accumulated = append(accumulated, item)
			newCount++
		}

		if len(items.Items) < pageFilter.PageSize || (newCount == 0 && page >= normalized.Page) {
			break
		}
	}

	pageItems, total := sliceERPProductsPage(accumulated, normalized.Page, normalized.PageSize)
	return &domain.ERPProductListResponse{
		Items: pageItems,
		Pagination: domain.PaginationMeta{
			Page:     normalized.Page,
			PageSize: normalized.PageSize,
			Total:    total,
		},
	}, nil
}

func (s *erpBridgeService) searchERPBridgeProductsWithLocalRefinement(ctx context.Context, normalized domain.ERPProductSearchFilter, catalog *erpBridgeCategoryCatalog, constraint *erpBridgeCategoryConstraint) (*domain.ERPProductListResponse, *domain.AppError) {
	accumulated := make([]*domain.ERPProduct, 0, normalized.Page*normalized.PageSize)
	seen := map[string]struct{}{}

	for page := 1; page <= erpBridgeRefinementMaxPages; page++ {
		pageFilter := normalized
		pageFilter.Page = page
		items, err := s.client.SearchProducts(ctx, pageFilter)
		if err != nil {
			return nil, mapERPBridgeError("search erp bridge products", err)
		}
		if items == nil || len(items.Items) == 0 {
			break
		}

		prepared := prepareERPProducts(items.Items, catalog)
		filtered := filterERPProducts(prepared, normalized, constraint)
		newCount := 0
		for _, item := range filtered {
			key := normalizeERPProductIdentityKey(item)
			if key == "" {
				key = normalizeERPBridgeText(item.ProductName)
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			accumulated = append(accumulated, item)
			newCount++
		}

		if len(items.Items) < pageFilter.PageSize || (newCount == 0 && page >= normalized.Page) {
			break
		}
	}

	pageItems, total := sliceERPProductsPage(accumulated, normalized.Page, normalized.PageSize)
	return &domain.ERPProductListResponse{
		Items: pageItems,
		Pagination: domain.PaginationMeta{
			Page:     normalized.Page,
			PageSize: normalized.PageSize,
			Total:    total,
		},
	}, nil
}

func (s *erpBridgeService) findERPProductByReference(ctx context.Context, id string, catalog *erpBridgeCategoryCatalog) (*domain.ERPProduct, *domain.AppError) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}

	skuResp, appErr := s.SearchProducts(ctx, domain.ERPProductSearchFilter{
		SKUCode:  id,
		Page:     1,
		PageSize: erpBridgeDetailFallbackPageSize,
	})
	if appErr != nil {
		return nil, appErr
	}
	if product := findExactERPProductReference(skuResp.Items, id); product != nil {
		return product, nil
	}

	keywordResp, appErr := s.SearchProducts(ctx, domain.ERPProductSearchFilter{
		Q:        id,
		Page:     1,
		PageSize: erpBridgeDetailFallbackPageSize,
	})
	if appErr != nil {
		return nil, appErr
	}
	if product := findExactERPProductReference(keywordResp.Items, id); product != nil {
		return product, nil
	}
	if len(keywordResp.Items) == 1 {
		return prepareERPProduct(keywordResp.Items[0], catalog), nil
	}
	return nil, nil
}

func emptyERPProductListResponse(normalized domain.ERPProductSearchFilter) *domain.ERPProductListResponse {
	return &domain.ERPProductListResponse{
		Items: []*domain.ERPProduct{},
		Pagination: domain.PaginationMeta{
			Page:     normalized.Page,
			PageSize: normalized.PageSize,
			Total:    0,
		},
		NormalizedFilters: &normalized,
	}
}

func stabilizeERPProductPagination(meta *domain.PaginationMeta, itemCount int) {
	if meta == nil {
		return
	}
	if meta.Page <= 0 {
		meta.Page = 1
	}
	if meta.PageSize <= 0 {
		meta.PageSize = itemCount
		if meta.PageSize <= 0 {
			meta.PageSize = 20
		}
	}
	if meta.Total < 0 {
		meta.Total = 0
	}

	minTotal := int64((meta.Page-1)*meta.PageSize + itemCount)
	if meta.Total < minTotal {
		meta.Total = minTotal
	}
	if itemCount < meta.PageSize && meta.Total > minTotal {
		meta.Total = minTotal
	}
}

func needsERPBridgeLocalRefinement(normalized domain.ERPProductSearchFilter, constraint *erpBridgeCategoryConstraint) bool {
	return strings.TrimSpace(normalized.SKUCode) != "" || constraint != nil
}

func shouldFallbackERPBridgeKeywordTimeout(err error, normalized domain.ERPProductSearchFilter) bool {
	if strings.TrimSpace(normalized.Q) == "" {
		return false
	}
	var requestErr *erpBridgeRequestError
	return errors.As(err, &requestErr) && requestErr.Timeout
}

func prepareERPProducts(items []*domain.ERPProduct, catalog *erpBridgeCategoryCatalog) []*domain.ERPProduct {
	prepared := make([]*domain.ERPProduct, 0, len(items))
	for _, item := range items {
		if product := prepareERPProduct(item, catalog); product != nil {
			prepared = append(prepared, product)
		}
	}
	if prepared == nil {
		return []*domain.ERPProduct{}
	}
	return prepared
}

func prepareERPProduct(item *domain.ERPProduct, catalog *erpBridgeCategoryCatalog) *domain.ERPProduct {
	if item == nil {
		return nil
	}
	product := *item
	product.ProductID = strings.TrimSpace(product.ProductID)
	product.SKUID = strings.TrimSpace(product.SKUID)
	product.IID = strings.TrimSpace(product.IID)
	product.SKUCode = strings.TrimSpace(product.SKUCode)
	product.Name = strings.TrimSpace(product.Name)
	product.ProductName = strings.TrimSpace(product.ProductName)
	product.ShortName = strings.TrimSpace(product.ShortName)
	product.CategoryID = strings.TrimSpace(product.CategoryID)
	product.CategoryCode = strings.TrimSpace(product.CategoryCode)
	product.CategoryName = strings.TrimSpace(product.CategoryName)
	product.ProductShortName = strings.TrimSpace(product.ProductShortName)
	product.ImageURL = strings.TrimSpace(product.ImageURL)
	product.WMSCoID = strings.TrimSpace(product.WMSCoID)
	product.Currency = strings.TrimSpace(product.Currency)

	if product.ProductID == "" {
		product.ProductID = firstNonEmptyString(product.SKUID, product.SKUCode, product.ProductName)
	}
	if product.ProductName == "" {
		product.ProductName = firstNonEmptyString(product.ProductShortName, product.SKUCode, product.ProductID)
	}
	if product.Name == "" {
		product.Name = product.ProductName
	}
	if product.ProductName == "" {
		product.ProductName = product.Name
	}
	if product.SKUCode == "" && shouldUseERPProductIDAsSKUCode(product.ProductID, product.ProductName) {
		product.SKUCode = product.ProductID
	}
	if product.ProductShortName == "" {
		product.ProductShortName = ""
	}
	if product.ShortName == "" {
		product.ShortName = product.ProductShortName
	}
	if product.ProductShortName == "" {
		product.ProductShortName = product.ShortName
	}
	if product.SPrice == nil {
		product.SPrice = product.Price
	}
	if product.Price == nil {
		product.Price = product.SPrice
	}

	if catalog != nil {
		if category := findERPBridgeCategory(product.CategoryID, product.CategoryName, catalog); category != nil {
			if product.CategoryID == "" {
				product.CategoryID = category.CategoryID
			}
			if product.CategoryName == "" {
				product.CategoryName = category.CategoryName
			}
			if product.CategoryCode == "" {
				product.CategoryCode = category.CategoryID
			}
		}
	}
	if product.CategoryCode == "" {
		product.CategoryCode = product.CategoryID
	}
	if product.CategoryID == "" {
		product.CategoryID = product.CategoryCode
	}
	return &product
}

func filterERPProducts(items []*domain.ERPProduct, normalized domain.ERPProductSearchFilter, constraint *erpBridgeCategoryConstraint) []*domain.ERPProduct {
	filtered := make([]*domain.ERPProduct, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(normalized.Q) != "" && !erpProductMatchesKeyword(item, normalized.Q) {
			continue
		}
		if strings.TrimSpace(normalized.SKUCode) != "" && !erpProductMatchesSKUCode(item, normalized.SKUCode) {
			continue
		}
		if constraint != nil && !erpProductMatchesCategory(item, constraint) {
			continue
		}
		filtered = append(filtered, item)
	}
	if filtered == nil {
		return []*domain.ERPProduct{}
	}
	return filtered
}

func findExactERPProductReference(items []*domain.ERPProduct, id string) *domain.ERPProduct {
	id = strings.TrimSpace(id)
	for _, item := range items {
		if erpProductMatchesReference(item, id) {
			return item
		}
	}
	return nil
}

func erpProductMatchesReference(item *domain.ERPProduct, id string) bool {
	id = normalizeERPBridgeText(id)
	if item == nil || id == "" {
		return false
	}
	for _, candidate := range []string{item.ProductID, item.SKUID, item.IID, item.SKUCode, item.Name, item.ProductName, item.ShortName, item.ProductShortName} {
		if normalizeERPBridgeText(candidate) == id {
			return true
		}
	}
	return false
}

func erpProductMatchesSKUCode(item *domain.ERPProduct, skuCode string) bool {
	skuCode = normalizeERPBridgeText(skuCode)
	if item == nil || skuCode == "" {
		return false
	}
	if normalizeERPBridgeText(item.SKUCode) == skuCode {
		return true
	}
	return normalizeERPBridgeText(item.SKUID) == skuCode
}

func erpProductMatchesKeyword(item *domain.ERPProduct, keyword string) bool {
	keyword = normalizeERPBridgeText(keyword)
	if item == nil || keyword == "" {
		return false
	}
	for _, candidate := range []string{
		item.ProductID,
		item.SKUID,
		item.IID,
		item.SKUCode,
		item.Name,
		item.ProductName,
		item.ShortName,
		item.ProductShortName,
		item.CategoryID,
		item.CategoryCode,
		item.CategoryName,
	} {
		if strings.Contains(normalizeERPBridgeText(candidate), keyword) {
			return true
		}
	}
	return false
}

func erpProductMatchesCategory(item *domain.ERPProduct, constraint *erpBridgeCategoryConstraint) bool {
	if item == nil || constraint == nil {
		return true
	}
	if constraint.id != "" {
		if normalizeERPBridgeText(item.CategoryID) == normalizeERPBridgeText(constraint.id) || normalizeERPBridgeText(item.CategoryCode) == normalizeERPBridgeText(constraint.id) {
			return true
		}
	}
	if constraint.name != "" && normalizeERPBridgeText(item.CategoryName) == normalizeERPBridgeText(constraint.name) {
		return true
	}
	return false
}

func findERPBridgeCategory(categoryID, categoryName string, catalog *erpBridgeCategoryCatalog) *domain.ERPCategory {
	if catalog == nil {
		return nil
	}
	if categoryID != "" {
		if item, ok := catalog.byID[normalizeERPBridgeText(categoryID)]; ok {
			return item
		}
	}
	if categoryName != "" {
		if item, ok := catalog.byName[normalizeERPBridgeText(categoryName)]; ok {
			return item
		}
	}
	return nil
}

func sliceERPProductsPage(items []*domain.ERPProduct, page, pageSize int) ([]*domain.ERPProduct, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []*domain.ERPProduct{}, total
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total
}

func shouldUseERPProductIDAsSKUCode(productID, productName string) bool {
	productID = strings.TrimSpace(productID)
	productName = strings.TrimSpace(productName)
	if productID == "" || strings.EqualFold(productID, productName) {
		return false
	}
	if strings.ContainsAny(productID, "/\\ ") {
		return false
	}
	hasDigit := false
	for _, r := range productID {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return hasDigit
}

func normalizeERPBridgeText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeERPProductSearchFilter(filter domain.ERPProductSearchFilter) domain.ERPProductSearchFilter {
	normalized := domain.ERPProductSearchFilter{
		Q:            strings.TrimSpace(filter.Q),
		Keyword:      strings.TrimSpace(filter.Keyword),
		SKUCode:      strings.TrimSpace(filter.SKUCode),
		CategoryID:   strings.TrimSpace(filter.CategoryID),
		CategoryName: strings.TrimSpace(filter.CategoryName),
		Page:         filter.Page,
		PageSize:     filter.PageSize,
	}
	if normalized.Q == "" {
		normalized.Q = normalized.Keyword
	}
	switch {
	case normalized.Q != "":
		normalized.QueryMode = "keyword"
	case normalized.SKUCode != "":
		normalized.Q = normalized.SKUCode
		normalized.QueryMode = "sku_code"
	case normalized.CategoryID != "" || normalized.CategoryName != "":
		normalized.QueryMode = "category_auxiliary"
	default:
		normalized.QueryMode = "browse"
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = 20
	}
	if normalized.PageSize > 100 {
		normalized.PageSize = 100
	}
	return normalized
}

func normalizeERPProductUpsertPayload(payload domain.ERPProductUpsertPayload) domain.ERPProductUpsertPayload {
	payload.ProductID = strings.TrimSpace(payload.ProductID)
	payload.SKUID = strings.TrimSpace(payload.SKUID)
	payload.IID = strings.TrimSpace(payload.IID)
	payload.SKUCode = strings.TrimSpace(payload.SKUCode)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ProductName = strings.TrimSpace(payload.ProductName)
	payload.ShortName = strings.TrimSpace(payload.ShortName)
	payload.CategoryID = strings.TrimSpace(payload.CategoryID)
	payload.CategoryCode = strings.TrimSpace(payload.CategoryCode)
	payload.CategoryName = strings.TrimSpace(payload.CategoryName)
	payload.ProductShortName = strings.TrimSpace(payload.ProductShortName)
	payload.ImageURL = strings.TrimSpace(payload.ImageURL)
	payload.Remark = strings.TrimSpace(payload.Remark)
	payload.SupplierName = strings.TrimSpace(payload.SupplierName)
	payload.WMSCoID = strings.TrimSpace(payload.WMSCoID)
	payload.Brand = strings.TrimSpace(payload.Brand)
	payload.VCName = strings.TrimSpace(payload.VCName)
	payload.ItemType = strings.TrimSpace(payload.ItemType)
	payload.Pic = strings.TrimSpace(payload.Pic)
	payload.PicBig = strings.TrimSpace(payload.PicBig)
	payload.SKUPic = strings.TrimSpace(payload.SKUPic)
	payload.PropertiesValue = strings.TrimSpace(payload.PropertiesValue)
	payload.SupplierSKUID = strings.TrimSpace(payload.SupplierSKUID)
	payload.SupplierIID = strings.TrimSpace(payload.SupplierIID)
	payload.Other1 = strings.TrimSpace(payload.Other1)
	payload.Other2 = strings.TrimSpace(payload.Other2)
	payload.Other3 = strings.TrimSpace(payload.Other3)
	payload.Other4 = strings.TrimSpace(payload.Other4)
	payload.Other5 = strings.TrimSpace(payload.Other5)
	payload.Operation = strings.TrimSpace(payload.Operation)
	payload.ShortNameTemplateKey = strings.TrimSpace(payload.ShortNameTemplateKey)
	payload.Currency = strings.TrimSpace(payload.Currency)
	payload.Source = strings.TrimSpace(payload.Source)
	if payload.Product != nil {
		payload.Product = normalizeERPProductSelectionSnapshot(payload.Product)
		if payload.Product != nil {
			if payload.ProductID == "" {
				payload.ProductID = payload.Product.ProductID
			}
			if payload.SKUID == "" {
				payload.SKUID = payload.Product.SKUID
			}
			if payload.IID == "" {
				payload.IID = payload.Product.IID
			}
			if payload.SKUCode == "" {
				payload.SKUCode = payload.Product.SKUCode
			}
			if payload.Name == "" {
				payload.Name = payload.Product.Name
			}
			if payload.ProductName == "" {
				payload.ProductName = payload.Product.ProductName
			}
			if payload.ShortName == "" {
				payload.ShortName = payload.Product.ShortName
			}
			if payload.CategoryID == "" {
				payload.CategoryID = payload.Product.CategoryID
			}
			if payload.CategoryCode == "" {
				payload.CategoryCode = payload.Product.CategoryCode
			}
			if payload.CategoryName == "" {
				payload.CategoryName = payload.Product.CategoryName
			}
			if payload.ProductShortName == "" {
				payload.ProductShortName = payload.Product.ProductShortName
			}
			if payload.ImageURL == "" {
				payload.ImageURL = payload.Product.ImageURL
			}
			if payload.SPrice == nil {
				payload.SPrice = payload.Product.SPrice
			}
			if payload.Price == nil {
				payload.Price = payload.Product.Price
			}
			if payload.WMSCoID == "" {
				payload.WMSCoID = payload.Product.WMSCoID
			}
			if payload.Currency == "" {
				payload.Currency = payload.Product.Currency
			}
		}
	}
	if payload.Name == "" {
		payload.Name = payload.ProductName
	}
	if payload.ProductName == "" {
		payload.ProductName = payload.Name
	}
	if payload.ShortName == "" {
		payload.ShortName = payload.ProductShortName
	}
	if payload.ProductShortName == "" {
		payload.ProductShortName = payload.ShortName
	}
	if payload.SPrice == nil {
		payload.SPrice = payload.Price
	}
	if payload.Price == nil {
		payload.Price = payload.SPrice
	}
	if payload.TaskContext != nil {
		payload.TaskContext.TaskNo = strings.TrimSpace(payload.TaskContext.TaskNo)
		payload.TaskContext.TaskType = strings.TrimSpace(payload.TaskContext.TaskType)
		payload.TaskContext.SourceMode = strings.TrimSpace(payload.TaskContext.SourceMode)
		payload.TaskContext.FiledAt = strings.TrimSpace(payload.TaskContext.FiledAt)
		payload.TaskContext.Remark = strings.TrimSpace(payload.TaskContext.Remark)
	}
	if payload.BusinessInfo != nil {
		payload.BusinessInfo.Category = strings.TrimSpace(payload.BusinessInfo.Category)
		payload.BusinessInfo.CategoryCode = strings.TrimSpace(payload.BusinessInfo.CategoryCode)
		payload.BusinessInfo.CategoryName = strings.TrimSpace(payload.BusinessInfo.CategoryName)
		payload.BusinessInfo.SpecText = strings.TrimSpace(payload.BusinessInfo.SpecText)
		payload.BusinessInfo.Material = strings.TrimSpace(payload.BusinessInfo.Material)
		payload.BusinessInfo.SizeText = strings.TrimSpace(payload.BusinessInfo.SizeText)
		payload.BusinessInfo.CraftText = strings.TrimSpace(payload.BusinessInfo.CraftText)
		payload.BusinessInfo.Process = strings.TrimSpace(payload.BusinessInfo.Process)
		if payload.CostPrice == nil {
			payload.CostPrice = payload.BusinessInfo.CostPrice
		}
	}
	taskType := ""
	if payload.TaskContext != nil {
		taskType = strings.TrimSpace(payload.TaskContext.TaskType)
	}
	if payload.Operation == "" {
		switch taskType {
		case string(domain.TaskTypeOriginalProductDevelopment):
			payload.Operation = "original_product_update"
		default:
			payload.Operation = "product_profile_upsert"
		}
	}
	if payload.SKUImmutable == nil && payload.Operation == "original_product_update" {
		skuImmutable := true
		payload.SKUImmutable = &skuImmutable
	}
	scene := taskType
	if scene == "" {
		scene = payload.Operation
	}
	autoEnabled := payload.AutoGenerateShortName == nil || *payload.AutoGenerateShortName
	if autoEnabled && payload.ShortName == "" {
		payload.ShortName = generateERPShortName(scene, payload.ShortNameTemplateKey, payload.Name, payload.IID)
		payload.ProductShortName = payload.ShortName
	}
	return payload
}

func normalizeERPItemStyleUpdatePayload(payload domain.ERPItemStyleUpdatePayload) domain.ERPItemStyleUpdatePayload {
	payload.SKUID = strings.TrimSpace(payload.SKUID)
	payload.IID = strings.TrimSpace(payload.IID)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ShortName = strings.TrimSpace(payload.ShortName)
	payload.CategoryName = strings.TrimSpace(payload.CategoryName)
	payload.Pic = strings.TrimSpace(payload.Pic)
	payload.PicBig = strings.TrimSpace(payload.PicBig)
	payload.SKUPic = strings.TrimSpace(payload.SKUPic)
	payload.PropertiesValue = strings.TrimSpace(payload.PropertiesValue)
	payload.Brand = strings.TrimSpace(payload.Brand)
	payload.VCName = strings.TrimSpace(payload.VCName)
	payload.SupplierIID = strings.TrimSpace(payload.SupplierIID)
	payload.Operation = strings.TrimSpace(payload.Operation)
	payload.Source = strings.TrimSpace(payload.Source)
	payload.ShortNameTemplateKey = strings.TrimSpace(payload.ShortNameTemplateKey)
	if payload.TaskContext != nil {
		payload.TaskContext.TaskNo = strings.TrimSpace(payload.TaskContext.TaskNo)
		payload.TaskContext.TaskType = strings.TrimSpace(payload.TaskContext.TaskType)
		payload.TaskContext.SourceMode = strings.TrimSpace(payload.TaskContext.SourceMode)
		payload.TaskContext.FiledAt = strings.TrimSpace(payload.TaskContext.FiledAt)
		payload.TaskContext.Remark = strings.TrimSpace(payload.TaskContext.Remark)
	}
	if payload.Operation == "" {
		payload.Operation = "item_style_update"
	}
	scene := ""
	if payload.TaskContext != nil {
		scene = strings.TrimSpace(payload.TaskContext.TaskType)
	}
	if scene == "" {
		scene = payload.Operation
	}
	autoEnabled := payload.AutoGenerateShortName == nil || *payload.AutoGenerateShortName
	if autoEnabled && payload.ShortName == "" {
		payload.ShortName = generateERPShortName(scene, payload.ShortNameTemplateKey, payload.Name, payload.IID)
	}
	return payload
}

func normalizeERPBridgeBatchMutationPayload(payload domain.ERPProductBatchMutationPayload) domain.ERPProductBatchMutationPayload {
	payload.Reason = strings.TrimSpace(payload.Reason)
	payload.Source = strings.TrimSpace(payload.Source)
	items := make([]domain.ERPProductBatchMutationItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		normalized := domain.ERPProductBatchMutationItem{
			ProductID: strings.TrimSpace(item.ProductID),
			SKUID:     strings.TrimSpace(item.SKUID),
			IID:       strings.TrimSpace(item.IID),
			SKUCode:   strings.TrimSpace(item.SKUCode),
			Name:      strings.TrimSpace(item.Name),
			WMSCoID:   strings.TrimSpace(item.WMSCoID),
			BinID:     strings.TrimSpace(item.BinID),
			CarryID:   strings.TrimSpace(item.CarryID),
			BoxNo:     strings.TrimSpace(item.BoxNo),
			Qty:       cloneInt64Ptr(item.Qty),
		}
		if normalized.SKUID == "" {
			normalized.SKUID = firstNonEmptyString(normalized.SKUCode, normalized.ProductID)
		}
		if normalized.ProductID == "" {
			normalized.ProductID = normalized.SKUID
		}
		if normalized.SKUID == "" {
			continue
		}
		items = append(items, normalized)
	}
	payload.Items = items
	return payload
}

func normalizeERPBridgeVirtualInventoryPayload(payload domain.ERPVirtualInventoryUpdatePayload) domain.ERPVirtualInventoryUpdatePayload {
	payload.Reason = strings.TrimSpace(payload.Reason)
	payload.Source = strings.TrimSpace(payload.Source)
	items := make([]domain.ERPVirtualInventoryUpdateItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		normalized := domain.ERPVirtualInventoryUpdateItem{
			ProductID:     strings.TrimSpace(item.ProductID),
			SKUID:         strings.TrimSpace(item.SKUID),
			IID:           strings.TrimSpace(item.IID),
			SKUCode:       strings.TrimSpace(item.SKUCode),
			WarehouseCode: strings.TrimSpace(item.WarehouseCode),
			WMSCoID:       strings.TrimSpace(item.WMSCoID),
			VirtualQty:    item.VirtualQty,
		}
		if normalized.WMSCoID == "" {
			normalized.WMSCoID = normalized.WarehouseCode
		}
		if normalized.SKUID == "" {
			normalized.SKUID = firstNonEmptyString(normalized.SKUCode, normalized.ProductID)
		}
		if normalized.ProductID == "" {
			normalized.ProductID = normalized.SKUID
		}
		if normalized.SKUID == "" {
			continue
		}
		items = append(items, normalized)
	}
	payload.Items = items
	return payload
}

func defaultERPWarehouses() []domain.ERPWarehouse {
	return []domain.ERPWarehouse{
		{Name: "南京永箔金属材料有限公司", WMSCoID: "10562500", WarehouseType: "主仓"},
		{Name: "烘焙仓", WMSCoID: "11691353", WarehouseType: "分仓"},
		{Name: "箔类仓", WMSCoID: "12259763", WarehouseType: "分仓"},
		{Name: "货主-南京小豚", WMSCoID: "12571621", WarehouseType: "分仓"},
		{Name: "南京永箔西门子", WMSCoID: "12596978", WarehouseType: "分仓"},
		{Name: "外贸希音仓", WMSCoID: "12884398", WarehouseType: "分仓"},
		{Name: "内贸婚庆仓", WMSCoID: "12975002", WarehouseType: "分仓"},
		{Name: "义乌-外贸仓", WMSCoID: "12995929", WarehouseType: "分仓"},
		{Name: "定制加工仓", WMSCoID: "13614877", WarehouseType: "分仓"},
		{Name: "泗县朗歆仓", WMSCoID: "13785593", WarehouseType: "分仓"},
		{Name: "山东诚志文教", WMSCoID: "14368339", WarehouseType: "分仓"},
	}
}

func mergeERPProductSelectionSnapshots(base, incoming *domain.ERPProductSelectionSnapshot) *domain.ERPProductSelectionSnapshot {
	switch {
	case base == nil && incoming == nil:
		return nil
	case base == nil:
		return normalizeERPProductSelectionSnapshot(incoming)
	case incoming == nil:
		return normalizeERPProductSelectionSnapshot(base)
	}

	merged := *base
	if strings.TrimSpace(incoming.ProductID) != "" {
		merged.ProductID = strings.TrimSpace(incoming.ProductID)
	}
	if strings.TrimSpace(incoming.SKUID) != "" {
		merged.SKUID = strings.TrimSpace(incoming.SKUID)
	}
	if strings.TrimSpace(incoming.IID) != "" {
		merged.IID = strings.TrimSpace(incoming.IID)
	}
	if strings.TrimSpace(incoming.SKUCode) != "" {
		merged.SKUCode = strings.TrimSpace(incoming.SKUCode)
	}
	if strings.TrimSpace(incoming.Name) != "" {
		merged.Name = strings.TrimSpace(incoming.Name)
	}
	if strings.TrimSpace(incoming.ProductName) != "" {
		merged.ProductName = strings.TrimSpace(incoming.ProductName)
	}
	if strings.TrimSpace(incoming.ShortName) != "" {
		merged.ShortName = strings.TrimSpace(incoming.ShortName)
	}
	if strings.TrimSpace(incoming.CategoryID) != "" {
		merged.CategoryID = strings.TrimSpace(incoming.CategoryID)
	}
	if strings.TrimSpace(incoming.CategoryCode) != "" {
		merged.CategoryCode = strings.TrimSpace(incoming.CategoryCode)
	}
	if strings.TrimSpace(incoming.CategoryName) != "" {
		merged.CategoryName = strings.TrimSpace(incoming.CategoryName)
	}
	if strings.TrimSpace(incoming.ProductShortName) != "" {
		merged.ProductShortName = strings.TrimSpace(incoming.ProductShortName)
	}
	if strings.TrimSpace(incoming.ImageURL) != "" {
		merged.ImageURL = strings.TrimSpace(incoming.ImageURL)
	}
	if incoming.Price != nil {
		price := *incoming.Price
		merged.Price = &price
	}
	if incoming.SPrice != nil {
		sPrice := *incoming.SPrice
		merged.SPrice = &sPrice
	}
	if strings.TrimSpace(incoming.WMSCoID) != "" {
		merged.WMSCoID = strings.TrimSpace(incoming.WMSCoID)
	}
	if strings.TrimSpace(incoming.Currency) != "" {
		merged.Currency = strings.TrimSpace(incoming.Currency)
	}
	return normalizeERPProductSelectionSnapshot(&merged)
}

func erpProductSnapshotFromSpecJSON(raw string) *domain.ERPProductSelectionSnapshot {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var snapshot domain.ERPProductSelectionSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil
	}
	return normalizeERPProductSelectionSnapshot(&snapshot)
}

func hydrateERPProductSelectionSnapshot(snapshot *domain.ERPProductSelectionSnapshot, product *domain.Product, task *domain.Task) *domain.ERPProductSelectionSnapshot {
	snapshot = normalizeERPProductSelectionSnapshot(snapshot)
	if snapshot == nil && product == nil {
		return nil
	}
	if snapshot == nil {
		snapshot = &domain.ERPProductSelectionSnapshot{}
	}
	hydrated := *snapshot
	if strings.TrimSpace(hydrated.ProductID) == "" {
		if product != nil && strings.TrimSpace(product.ERPProductID) != "" {
			hydrated.ProductID = strings.TrimSpace(product.ERPProductID)
		} else {
			hydrated.ProductID = strings.TrimSpace(hydrated.SKUID)
		}
	}
	if strings.TrimSpace(hydrated.SKUCode) == "" {
		switch {
		case product != nil && strings.TrimSpace(product.SKUCode) != "":
			hydrated.SKUCode = strings.TrimSpace(product.SKUCode)
		case task != nil && strings.TrimSpace(task.SKUCode) != "":
			hydrated.SKUCode = strings.TrimSpace(task.SKUCode)
		}
	}
	if strings.TrimSpace(hydrated.ProductName) == "" {
		switch {
		case product != nil && strings.TrimSpace(product.ProductName) != "":
			hydrated.ProductName = strings.TrimSpace(product.ProductName)
		case task != nil && strings.TrimSpace(task.ProductNameSnapshot) != "":
			hydrated.ProductName = strings.TrimSpace(task.ProductNameSnapshot)
		}
	}
	if strings.TrimSpace(hydrated.Name) == "" {
		hydrated.Name = strings.TrimSpace(hydrated.ProductName)
	}
	if strings.TrimSpace(hydrated.ProductName) == "" {
		hydrated.ProductName = strings.TrimSpace(hydrated.Name)
	}
	if strings.TrimSpace(hydrated.ShortName) == "" {
		hydrated.ShortName = strings.TrimSpace(hydrated.ProductShortName)
	}
	if strings.TrimSpace(hydrated.ProductShortName) == "" {
		hydrated.ProductShortName = strings.TrimSpace(hydrated.ShortName)
	}
	if strings.TrimSpace(hydrated.CategoryName) == "" && product != nil {
		hydrated.CategoryName = strings.TrimSpace(product.Category)
	}
	if strings.TrimSpace(hydrated.CategoryCode) == "" {
		hydrated.CategoryCode = strings.TrimSpace(hydrated.CategoryID)
	}
	return normalizeERPProductSelectionSnapshot(&hydrated)
}

func mapERPBridgeError(op string, err error) *domain.AppError {
	var rnf *erpBridgeRemoteProductNotFoundError
	if errors.As(err, &rnf) {
		return domain.ErrNotFound
	}
	var httpErr *erpBridgeHTTPError
	var requestErr *erpBridgeRequestError
	var decodeErr *erpBridgeDecodeError
	var openWebErr *erpBridgeOpenWebError

	switch {
	case errors.Is(err, ErrERPRemoteOpenWebAuthRequired):
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "ERP live query requires ERP_REMOTE_AUTH_MODE=openweb on Bridge (8081)", map[string]interface{}{
			"operation": op,
			"hint":      "configure Bridge with JST OpenWeb credentials and ERP_REMOTE_SKU_QUERY_PATH",
		})
	case errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound:
		return domain.ErrNotFound
	case errors.As(err, &httpErr):
		return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge request failed", map[string]interface{}{
			"operation":   op,
			"status_code": httpErr.StatusCode,
			"response":    httpErr.Body,
			"url":         httpErr.URL,
			"duration_ms": httpErr.Duration.Milliseconds(),
			"retryable":   httpErr.Retryable,
			"retry_hint":  erpBridgeRetryHint(httpErr.Retryable, false),
		})
	case errors.As(err, &requestErr):
		message := "erp bridge is unavailable"
		if requestErr.Timeout {
			message = "erp bridge request timed out"
		}
		return domain.NewAppError(domain.ErrCodeInternalError, message, map[string]interface{}{
			"operation":   op,
			"error":       requestErr.Error(),
			"url":         requestErr.URL,
			"duration_ms": requestErr.Duration.Milliseconds(),
			"timeout":     requestErr.Timeout,
			"retryable":   requestErr.Retryable,
			"retry_hint":  erpBridgeRetryHint(requestErr.Retryable, requestErr.Timeout),
		})
	case errors.As(err, &decodeErr):
		return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge returned an invalid response", map[string]interface{}{
			"operation":     op,
			"error":         decodeErr.Error(),
			"url":           decodeErr.URL,
			"response":      decodeErr.BodySnippet,
			"retryable":     decodeErr.Retryable,
			"retry_hint":    erpBridgeRetryHint(decodeErr.Retryable, false),
			"normalization": "tolerant_envelope_decode",
		})
	case errors.As(err, &openWebErr):
		return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge upstream business rejected request", map[string]interface{}{
			"operation":       op,
			"openweb_code":    openWebErr.Code,
			"openweb_message": openWebErr.Message,
			"url":             openWebErr.URL,
			"duration_ms":     openWebErr.Duration.Milliseconds(),
			"response":        openWebErr.Body,
			"retryable":       openWebErr.Retryable,
			"retry_hint":      "manual_check_upstream_payload_or_config",
		})
	default:
		return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge is unavailable", map[string]interface{}{
			"operation":  op,
			"error":      err.Error(),
			"retryable":  false,
			"retry_hint": erpBridgeRetryHint(false, false),
		})
	}
}

func erpBridgeRetryHint(retryable, timeout bool) string {
	switch {
	case timeout:
		return "retry_after_timeout"
	case retryable:
		return "retryable_upstream_error"
	default:
		return "manual_check_upstream_payload_or_config"
	}
}

func isERPBridgeHTTPStatus(err error, status int) bool {
	var httpErr *erpBridgeHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == status
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
