package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type localERPBridgeClient struct {
	productRepo  repo.ProductRepo
	categoryRepo repo.CategoryRepo
	txRunner     repo.TxRunner
	callLogRepo  repo.IntegrationCallLogRepo
}

func NewLocalERPBridgeClient(productRepo repo.ProductRepo, categoryRepo repo.CategoryRepo, txRunner repo.TxRunner, callLogRepo repo.IntegrationCallLogRepo) ERPBridgeClient {
	return &localERPBridgeClient{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		txRunner:     txRunner,
		callLogRepo:  callLogRepo,
	}
}

func ShouldUseLocalERPBridgeClient(serverPort, bridgeBaseURL string) bool {
	serverPort = strings.TrimSpace(serverPort)
	if serverPort == "" {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(bridgeBaseURL))
	if err != nil {
		return false
	}
	bridgePort := strings.TrimSpace(parsed.Port())
	if bridgePort == "" {
		switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
		case "https":
			bridgePort = "443"
		default:
			bridgePort = "80"
		}
	}
	return serverPort == bridgePort
}

func (c *localERPBridgeClient) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	if c.productRepo == nil {
		return &domain.ERPProductListResponse{
			Items:      []*domain.ERPProduct{},
			Pagination: domain.PaginationMeta{Page: normalizePositiveInt(filter.Page, 1), PageSize: normalizePositiveInt(filter.PageSize, 20)},
		}, nil
	}

	keyword := strings.TrimSpace(filter.Q)
	if keyword == "" {
		keyword = strings.TrimSpace(filter.SKUCode)
	}
	category := strings.TrimSpace(filter.CategoryName)
	if category == "" {
		category = strings.TrimSpace(filter.CategoryID)
	}

	products, total, err := c.productRepo.Search(ctx, repo.ProductSearchFilter{
		Keyword:  keyword,
		Category: category,
		Page:     normalizePositiveInt(filter.Page, 1),
		PageSize: normalizePositiveInt(filter.PageSize, 20),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*domain.ERPProduct, 0, len(products))
	for _, product := range products {
		if item := localERPProductFromDomain(product); item != nil {
			items = append(items, item)
		}
	}
	if items == nil {
		items = []*domain.ERPProduct{}
	}

	return &domain.ERPProductListResponse{
		Items: items,
		Pagination: domain.PaginationMeta{
			Page:     normalizePositiveInt(filter.Page, 1),
			PageSize: normalizePositiveInt(filter.PageSize, 20),
			Total:    total,
		},
	}, nil
}

func (c *localERPBridgeClient) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error) {
	id = strings.TrimSpace(id)
	if id == "" || c.productRepo == nil {
		return nil, nil
	}

	product, err := c.productRepo.GetByERPProductID(ctx, id)
	if err != nil {
		return nil, err
	}
	if product != nil {
		return localERPProductFromDomain(product), nil
	}

	products, _, err := c.productRepo.Search(ctx, repo.ProductSearchFilter{
		Keyword:  id,
		Page:     1,
		PageSize: erpBridgeDetailFallbackPageSize,
	})
	if err != nil {
		return nil, err
	}
	for _, candidate := range products {
		item := localERPProductFromDomain(candidate)
		if erpProductMatchesReference(item, id) {
			return item, nil
		}
	}
	return nil, nil
}

func (c *localERPBridgeClient) ListCategories(ctx context.Context) ([]*domain.ERPCategory, error) {
	if c.categoryRepo == nil {
		return []*domain.ERPCategory{}, nil
	}

	active := true
	items, _, err := c.categoryRepo.List(ctx, repo.CategoryListFilter{
		IsActive: &active,
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		return nil, err
	}

	categories := make([]*domain.ERPCategory, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		category := &domain.ERPCategory{
			CategoryID:   firstNonEmptyString(strings.TrimSpace(item.CategoryCode), strings.TrimSpace(item.SearchEntryCode)),
			CategoryName: firstNonEmptyString(strings.TrimSpace(item.DisplayName), strings.TrimSpace(item.CategoryName)),
			Level:        item.Level,
		}
		if item.ParentID != nil {
			category.ParentID = strconv.FormatInt(*item.ParentID, 10)
		}
		if category.CategoryID == "" && category.CategoryName == "" {
			continue
		}
		categories = append(categories, category)
	}
	if categories == nil {
		return []*domain.ERPCategory{}, nil
	}
	return categories, nil
}

func (c *localERPBridgeClient) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	payload = normalizeERPProductUpsertPayload(payload)
	productID := strings.TrimSpace(payload.ProductID)
	snapshot := payload.Product
	if snapshot == nil {
		snapshot = normalizeERPProductSelectionSnapshot(&domain.ERPProductSelectionSnapshot{
			ProductID:        productID,
			SKUID:            payload.SKUID,
			IID:              payload.IID,
			SKUCode:          payload.SKUCode,
			Name:             payload.Name,
			ProductName:      payload.ProductName,
			ShortName:        payload.ShortName,
			CategoryID:       payload.CategoryID,
			CategoryCode:     payload.CategoryCode,
			CategoryName:     payload.CategoryName,
			ProductShortName: payload.ProductShortName,
			ImageURL:         payload.ImageURL,
			Price:            payload.Price,
			SPrice:           payload.SPrice,
			WMSCoID:          payload.WMSCoID,
			Currency:         payload.Currency,
		})
	}

	if c.productRepo != nil && c.txRunner != nil && productID != "" {
		specJSON, err := json.Marshal(snapshot)
		if err != nil {
			return nil, fmt.Errorf("marshal local erp bridge product snapshot: %w", err)
		}
		product := &domain.Product{
			ERPProductID: productID,
			SKUCode:      firstNonEmptyString(payload.SKUCode, payload.SKUID),
			ProductName:  firstNonEmptyString(payload.Name, payload.ProductName, payload.ShortName, payload.ProductShortName, payload.SKUCode, productID),
			Category:     firstNonEmptyString(payload.CategoryName, payload.CategoryCode, payload.CategoryID),
			SpecJSON:     string(specJSON),
			Status:       "active",
		}
		if err := c.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			_, err := c.productRepo.UpsertBatch(ctx, tx, []*domain.Product{product})
			return err
		}); err != nil {
			return nil, err
		}
	}

	result := &domain.ERPProductUpsertResult{
		ProductID:        productID,
		SKUID:            payload.SKUID,
		IID:              payload.IID,
		SKUCode:          payload.SKUCode,
		Name:             payload.Name,
		ProductName:      payload.ProductName,
		ShortName:        payload.ShortName,
		CategoryID:       payload.CategoryID,
		CategoryCode:     payload.CategoryCode,
		CategoryName:     payload.CategoryName,
		ProductShortName: payload.ProductShortName,
		SPrice:           payload.SPrice,
		WMSCoID:          payload.WMSCoID,
		Route:            "itemskubatchupload",
		Status:           "accepted",
		Message:          "stored locally",
	}
	syncLogID, err := c.recordLocalERPBridgeCall(ctx, domain.IntegrationConnectorKeyERPBridgeProductUpsert, "erp.bridge.products.upsert", payload, result, nil)
	if err != nil {
		return nil, err
	}
	result.SyncLogID = syncLogID
	return result, nil
}

func (c *localERPBridgeClient) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	normalized := normalizeERPItemStyleUpdatePayload(payload)
	result := &domain.ERPItemStyleUpdateResult{
		SKUID:     normalized.SKUID,
		IID:       normalized.IID,
		Name:      normalized.Name,
		ShortName: normalized.ShortName,
		Route:     "itemupload",
		Status:    "accepted",
		Message:   "stored locally",
	}
	syncLogID, err := c.recordLocalERPBridgeCall(ctx, domain.IntegrationConnectorKeyERPBridgeItemStyleUpdate, "erp.bridge.products.style.update", normalized, result, nil)
	if err != nil {
		return nil, err
	}
	result.SyncLogID = syncLogID
	return result, nil
}

func (c *localERPBridgeClient) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	if c.callLogRepo == nil {
		return &domain.ERPSyncLogListResponse{
			Items:      []*domain.ERPSyncLog{},
			Pagination: buildPaginationMeta(filter.Page, filter.PageSize, 0),
		}, nil
	}
	page := normalizePositiveInt(filter.Page, 1)
	pageSize := normalizePositiveInt(filter.PageSize, 20)
	repoFilter := repo.IntegrationCallLogListFilter{
		ConnectorKey: mapERPBridgeConnectorFilter(filter.Connector),
		Status:       mapERPBridgeSyncStatusFilter(filter.Status),
		ResourceType: strings.TrimSpace(filter.ResourceType),
		Page:         page,
		PageSize:     pageSize,
	}
	logs, total, err := c.callLogRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.ERPSyncLog, 0, len(logs))
	for _, item := range logs {
		syncLog := mapIntegrationCallLogToERPSyncLog(item)
		if syncLog == nil {
			continue
		}
		if strings.TrimSpace(filter.Operation) != "" && !strings.EqualFold(strings.TrimSpace(filter.Operation), syncLog.Operation) {
			continue
		}
		items = append(items, syncLog)
	}
	if items == nil {
		items = []*domain.ERPSyncLog{}
	}
	return &domain.ERPSyncLogListResponse{
		Items:      items,
		Pagination: buildPaginationMeta(page, pageSize, total),
	}, nil
}

func (c *localERPBridgeClient) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error) {
	if c.callLogRepo == nil {
		return nil, nil
	}
	parsedID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return nil, nil
	}
	log, err := c.callLogRepo.GetByID(ctx, parsedID)
	if err != nil {
		return nil, err
	}
	return mapIntegrationCallLogToERPSyncLog(log), nil
}

func (c *localERPBridgeClient) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	normalized := normalizeERPBatchMutationPayload(payload)
	result := &domain.ERPProductBatchMutationResult{
		Action:   "shelve",
		Total:    len(normalized.Items),
		Accepted: len(normalized.Items),
		Status:   "accepted",
		Message:  "stored locally",
	}
	syncLogID, err := c.recordLocalERPBridgeCall(ctx, domain.IntegrationConnectorKeyERPBridgeProductShelve, "erp.bridge.products.shelve.batch", normalized, result, nil)
	if err != nil {
		return nil, err
	}
	result.SyncLogID = syncLogID
	return result, nil
}

func (c *localERPBridgeClient) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	normalized := normalizeERPBatchMutationPayload(payload)
	result := &domain.ERPProductBatchMutationResult{
		Action:   "unshelve",
		Total:    len(normalized.Items),
		Accepted: len(normalized.Items),
		Status:   "accepted",
		Message:  "stored locally",
	}
	syncLogID, err := c.recordLocalERPBridgeCall(ctx, domain.IntegrationConnectorKeyERPBridgeProductUnshelve, "erp.bridge.products.unshelve.batch", normalized, result, nil)
	if err != nil {
		return nil, err
	}
	result.SyncLogID = syncLogID
	return result, nil
}

func (c *localERPBridgeClient) GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	return nil, fmt.Errorf("jst getcompanyusers is not available in local erp mode; use remote or hybrid mode")
}

func (c *localERPBridgeClient) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	normalized := normalizeERPVirtualInventoryPayload(payload)
	result := &domain.ERPVirtualInventoryUpdateResult{
		Total:    len(normalized.Items),
		Accepted: len(normalized.Items),
		Status:   "accepted",
		Message:  "stored locally",
	}
	syncLogID, err := c.recordLocalERPBridgeCall(ctx, domain.IntegrationConnectorKeyERPBridgeVirtualInventory, "erp.bridge.inventory.virtual_qty", normalized, result, nil)
	if err != nil {
		return nil, err
	}
	result.SyncLogID = syncLogID
	return result, nil
}

func localERPProductFromDomain(product *domain.Product) *domain.ERPProduct {
	if product == nil {
		return nil
	}
	snapshot := hydrateERPProductSelectionSnapshot(erpProductSnapshotFromSpecJSON(product.SpecJSON), product, nil)
	if snapshot == nil {
		snapshot = &domain.ERPProductSelectionSnapshot{}
	}
	return &domain.ERPProduct{
		ProductID:        firstNonEmptyString(snapshot.ProductID, product.ERPProductID),
		SKUID:            strings.TrimSpace(snapshot.SKUID),
		IID:              strings.TrimSpace(snapshot.IID),
		SKUCode:          firstNonEmptyString(snapshot.SKUCode, product.SKUCode),
		Name:             firstNonEmptyString(snapshot.Name, snapshot.ProductName, product.ProductName),
		ProductName:      firstNonEmptyString(snapshot.ProductName, product.ProductName),
		ShortName:        firstNonEmptyString(snapshot.ShortName, snapshot.ProductShortName),
		CategoryID:       firstNonEmptyString(snapshot.CategoryID, snapshot.CategoryCode),
		CategoryCode:     firstNonEmptyString(snapshot.CategoryCode, snapshot.CategoryID),
		CategoryName:     firstNonEmptyString(snapshot.CategoryName, product.Category),
		ProductShortName: strings.TrimSpace(snapshot.ProductShortName),
		ImageURL:         strings.TrimSpace(snapshot.ImageURL),
		Price:            cloneLocalERPPrice(snapshot.Price),
		SPrice:           cloneLocalERPPrice(snapshot.SPrice),
		WMSCoID:          strings.TrimSpace(snapshot.WMSCoID),
		Currency:         strings.TrimSpace(snapshot.Currency),
	}
}

func cloneLocalERPPrice(price *float64) *float64 {
	if price == nil {
		return nil
	}
	value := *price
	return &value
}

func normalizePositiveInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func mapERPBridgeConnectorFilter(raw string) *domain.IntegrationConnectorKey {
	raw = strings.TrimSpace(strings.ToLower(raw))
	var key domain.IntegrationConnectorKey
	switch raw {
	case "":
		return nil
	case "upsert", string(domain.IntegrationConnectorKeyERPBridgeProductUpsert):
		key = domain.IntegrationConnectorKeyERPBridgeProductUpsert
	case "item_style_update", "style_update", string(domain.IntegrationConnectorKeyERPBridgeItemStyleUpdate):
		key = domain.IntegrationConnectorKeyERPBridgeItemStyleUpdate
	case "shelve", "shelve_batch", string(domain.IntegrationConnectorKeyERPBridgeProductShelve):
		key = domain.IntegrationConnectorKeyERPBridgeProductShelve
	case "unshelve", "unshelve_batch", string(domain.IntegrationConnectorKeyERPBridgeProductUnshelve):
		key = domain.IntegrationConnectorKeyERPBridgeProductUnshelve
	case "virtual_qty", "inventory_virtual_qty", string(domain.IntegrationConnectorKeyERPBridgeVirtualInventory):
		key = domain.IntegrationConnectorKeyERPBridgeVirtualInventory
	default:
		return nil
	}
	return &key
}

func mapERPBridgeSyncStatusFilter(raw string) *domain.IntegrationCallStatus {
	raw = strings.TrimSpace(strings.ToLower(raw))
	var status domain.IntegrationCallStatus
	switch raw {
	case "":
		return nil
	case "queued":
		status = domain.IntegrationCallStatusQueued
	case "sent":
		status = domain.IntegrationCallStatusSent
	case "succeeded", "success", "accepted":
		status = domain.IntegrationCallStatusSucceeded
	case "failed", "error":
		status = domain.IntegrationCallStatusFailed
	case "cancelled", "canceled":
		status = domain.IntegrationCallStatusCancelled
	default:
		return nil
	}
	return &status
}

func mapIntegrationCallLogToERPSyncLog(log *domain.IntegrationCallLog) *domain.ERPSyncLog {
	if log == nil {
		return nil
	}
	result := &domain.ERPSyncLog{
		SyncLogID:       strconv.FormatInt(log.CallLogID, 10),
		Connector:       string(log.ConnectorKey),
		Operation:       log.OperationKey,
		Status:          string(log.Status),
		ResourceType:    log.ResourceType,
		ResourceID:      log.ResourceID,
		RequestPayload:  log.RequestPayload,
		ResponsePayload: log.ResponsePayload,
		ErrorMessage:    strings.TrimSpace(log.ErrorMessage),
	}
	if !log.CreatedAt.IsZero() {
		createdAt := log.CreatedAt
		result.CreatedAt = &createdAt
	}
	if !log.UpdatedAt.IsZero() {
		updatedAt := log.UpdatedAt
		result.UpdatedAt = &updatedAt
	}
	if len(log.ResponsePayload) > 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(log.ResponsePayload, &raw); err == nil {
			result.Message = firstString(raw, "message", "msg", "detail")
			result.ProductID = firstString(raw, "product_id", "productId")
			result.SKUCode = firstString(raw, "sku_code", "skuCode")
		}
	}
	return result
}

func normalizeERPBatchMutationPayload(payload domain.ERPProductBatchMutationPayload) domain.ERPProductBatchMutationPayload {
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
		if normalized.ProductID == "" && normalized.SKUID == "" && normalized.SKUCode == "" {
			continue
		}
		if normalized.SKUID == "" {
			normalized.SKUID = firstNonEmptyString(normalized.SKUCode, normalized.ProductID)
		}
		if normalized.ProductID == "" {
			normalized.ProductID = normalized.SKUID
		}
		items = append(items, normalized)
	}
	payload.Items = items
	return payload
}

func normalizeERPVirtualInventoryPayload(payload domain.ERPVirtualInventoryUpdatePayload) domain.ERPVirtualInventoryUpdatePayload {
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

func (c *localERPBridgeClient) recordLocalERPBridgeCall(
	ctx context.Context,
	connector domain.IntegrationConnectorKey,
	operation string,
	requestPayload interface{},
	responsePayload interface{},
	runErr error,
) (string, error) {
	if c.callLogRepo == nil || c.txRunner == nil {
		return "", nil
	}
	now := time.Now().UTC()
	actor, _ := resolveWorkbenchActorScope(ctx)
	status := domain.IntegrationCallStatusSucceeded
	errorMessage := ""
	if runErr != nil {
		status = domain.IntegrationCallStatusFailed
		errorMessage = runErr.Error()
	}
	requestRaw, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("marshal erp bridge local request payload: %w", err)
	}
	responseRaw, err := json.Marshal(responsePayload)
	if err != nil {
		return "", fmt.Errorf("marshal erp bridge local response payload: %w", err)
	}
	callLog := &domain.IntegrationCallLog{
		ConnectorKey:    connector,
		OperationKey:    strings.TrimSpace(operation),
		Direction:       domain.IntegrationCallDirectionOutbound,
		ResourceType:    "erp_bridge",
		Status:          status,
		RequestedBy:     actor,
		RequestPayload:  requestRaw,
		ResponsePayload: responseRaw,
		ErrorMessage:    errorMessage,
		LatestStatusAt:  now,
		StartedAt:       &now,
		FinishedAt:      &now,
		Remark:          "local_erp_bridge_write",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	var callLogID int64
	if err := c.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, createErr := c.callLogRepo.Create(ctx, tx, callLog)
		if createErr != nil {
			return createErr
		}
		callLogID = id
		return nil
	}); err != nil {
		return "", fmt.Errorf("create erp bridge local call log: %w", err)
	}
	if callLogID <= 0 {
		return "", nil
	}
	return strconv.FormatInt(callLogID, 10), nil
}
