package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

type ERPBridgeClient interface {
	SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error)
	GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error)
	ListCategories(ctx context.Context) ([]*domain.ERPCategory, error)
	ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error)
	GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error)
	UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error)
	UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error)
	ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error)
	UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error)
	UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error)
	GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error)
}

type ERPBridgeClientConfig struct {
	BaseURL string
	Timeout time.Duration
	Logger  *zap.Logger
}

type erpBridgeClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	logger     *zap.Logger
}

type erpBridgeHTTPError struct {
	StatusCode int
	Body       string
	URL        string
	Duration   time.Duration
	Retryable  bool
}

func (e *erpBridgeHTTPError) Error() string {
	return fmt.Sprintf("erp bridge http status %d", e.StatusCode)
}

type erpBridgeRequestError struct {
	URL       string
	Duration  time.Duration
	Timeout   bool
	Retryable bool
	Cause     error
}

func (e *erpBridgeRequestError) Error() string {
	if e == nil || e.Cause == nil {
		return "erp bridge request failed"
	}
	return e.Cause.Error()
}

func (e *erpBridgeRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type erpBridgeDecodeError struct {
	URL         string
	BodySnippet string
	Retryable   bool
	Cause       error
}

func (e *erpBridgeDecodeError) Error() string {
	if e == nil || e.Cause == nil {
		return "erp bridge decode failed"
	}
	return e.Cause.Error()
}

func (e *erpBridgeDecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewERPBridgeClient(cfg ERPBridgeClientConfig) (ERPBridgeClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("erp bridge base url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse erp bridge base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &erpBridgeClient{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}, nil
}

func (c *erpBridgeClient) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	query := url.Values{}
	if strings.TrimSpace(filter.Q) != "" {
		query.Set("q", strings.TrimSpace(filter.Q))
	}
	if strings.TrimSpace(filter.SKUCode) != "" {
		query.Set("sku_code", strings.TrimSpace(filter.SKUCode))
	}
	if strings.TrimSpace(filter.CategoryID) != "" {
		query.Set("category_id", strings.TrimSpace(filter.CategoryID))
	}
	if strings.TrimSpace(filter.CategoryName) != "" {
		query.Set("category_name", strings.TrimSpace(filter.CategoryName))
	}
	if filter.Page > 0 {
		query.Set("page", strconv.Itoa(filter.Page))
	}
	if filter.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(filter.PageSize))
	}

	// Bridge (erp_bridge) uses same binary as MAIN; router registers under /v1.
	payload, err := c.doGET(ctx, "/v1/erp/products", query)
	if err != nil {
		return nil, err
	}
	return decodeERPProductList(payload, filter.Page, filter.PageSize)
}

func (c *erpBridgeClient) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("erp bridge product id is required")
	}
	payload, err := c.doGET(ctx, "/v1/erp/products/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	return decodeERPProduct(payload)
}

func (c *erpBridgeClient) ListCategories(ctx context.Context) ([]*domain.ERPCategory, error) {
	payload, err := c.doGET(ctx, "/v1/erp/categories", nil)
	if err != nil {
		return nil, err
	}
	return decodeERPCategories(payload)
}

func (c *erpBridgeClient) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	query := url.Values{}
	if filter.Page > 0 {
		query.Set("page", strconv.Itoa(filter.Page))
	}
	if filter.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(filter.PageSize))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query.Set("status", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Connector) != "" {
		query.Set("connector", strings.TrimSpace(filter.Connector))
	}
	if strings.TrimSpace(filter.Operation) != "" {
		query.Set("operation", strings.TrimSpace(filter.Operation))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		query.Set("resource_type", strings.TrimSpace(filter.ResourceType))
	}
	payload, err := c.doGET(ctx, "/v1/erp/sync-logs", query)
	if err != nil {
		return nil, err
	}
	return decodeERPSyncLogList(payload, filter.Page, filter.PageSize)
}

func (c *erpBridgeClient) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("erp bridge sync log id is required")
	}
	payload, err := c.doGET(ctx, "/v1/erp/sync-logs/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	return decodeERPSyncLog(payload)
}

func (c *erpBridgeClient) GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	query := url.Values{}
	if filter.CurrentPage > 0 {
		query.Set("current_page", strconv.Itoa(filter.CurrentPage))
	}
	if filter.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(filter.PageSize))
	}
	if filter.PageAction >= 0 && filter.PageAction <= 2 {
		query.Set("page_action", strconv.Itoa(filter.PageAction))
	}
	if filter.Enabled != nil {
		query.Set("enabled", strconv.FormatBool(*filter.Enabled))
	}
	if filter.Version > 0 {
		query.Set("version", strconv.Itoa(filter.Version))
	}
	if strings.TrimSpace(filter.LoginID) != "" {
		query.Set("loginId", filter.LoginID)
	}
	if strings.TrimSpace(filter.CreatedBegin) != "" {
		query.Set("creatd_begin", filter.CreatedBegin)
	}
	if strings.TrimSpace(filter.CreatedEnd) != "" {
		query.Set("creatd_end", filter.CreatedEnd)
	}
	payload, err := c.doGET(ctx, "/v1/erp/users", query)
	if err != nil {
		return nil, err
	}
	return decodeJSTUserListResponse(payload)
}

func (c *erpBridgeClient) doGET(ctx context.Context, rawPath string, query url.Values) ([]byte, error) {
	return c.doJSON(ctx, http.MethodGet, rawPath, query, nil, []int{http.StatusOK}, false)
}

func (c *erpBridgeClient) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal erp bridge upsert payload: %w", err)
	}
	respPayload, err := c.doJSON(ctx, http.MethodPost, "/v1/erp/products/upsert", nil, raw, []int{http.StatusOK, http.StatusCreated}, true)
	if err != nil {
		return nil, err
	}
	return decodeERPProductUpsertResult(respPayload, payload.Product)
}

func (c *erpBridgeClient) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal erp bridge item style update payload: %w", err)
	}
	respPayload, err := c.doJSON(ctx, http.MethodPost, "/v1/erp/products/style/update", nil, raw, []int{http.StatusOK, http.StatusCreated}, true)
	if err != nil {
		return nil, err
	}
	return decodeERPItemStyleUpdateResult(respPayload, payload)
}

func (c *erpBridgeClient) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal erp bridge shelve payload: %w", err)
	}
	respPayload, err := c.doJSON(ctx, http.MethodPost, "/v1/erp/products/shelve/batch", nil, raw, []int{http.StatusOK, http.StatusCreated}, true)
	if err != nil {
		return nil, err
	}
	return decodeERPBatchMutationResult(respPayload, "shelve", len(payload.Items))
}

func (c *erpBridgeClient) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal erp bridge unshelve payload: %w", err)
	}
	respPayload, err := c.doJSON(ctx, http.MethodPost, "/v1/erp/products/unshelve/batch", nil, raw, []int{http.StatusOK, http.StatusCreated}, true)
	if err != nil {
		return nil, err
	}
	return decodeERPBatchMutationResult(respPayload, "unshelve", len(payload.Items))
}

func (c *erpBridgeClient) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal erp bridge virtual inventory payload: %w", err)
	}
	respPayload, err := c.doJSON(ctx, http.MethodPost, "/v1/erp/inventory/virtual-qty", nil, raw, []int{http.StatusOK, http.StatusCreated}, true)
	if err != nil {
		return nil, err
	}
	return decodeERPVirtualInventoryUpdateResult(respPayload, len(payload.Items))
}

func (c *erpBridgeClient) doJSON(ctx context.Context, method, rawPath string, query url.Values, body []byte, acceptedStatus []int, allowEmptyBody bool) ([]byte, error) {
	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), rawPath)
	target.RawQuery = query.Encode()

	var requestBody io.Reader
	if len(body) > 0 {
		requestBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), requestBody)
	if err != nil {
		return nil, fmt.Errorf("build erp bridge request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken, ok := domain.RequestBearerTokenFromContext(ctx); ok {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		requestErr := &erpBridgeRequestError{
			URL:       target.String(),
			Duration:  duration,
			Timeout:   isERPBridgeTimeout(err),
			Retryable: isERPBridgeRetryableRequest(err),
			Cause:     fmt.Errorf("do erp bridge request: %w", err),
		}
		c.logger.Warn("erp_bridge_request_failed",
			zap.String("url", target.String()),
			zap.Duration("duration", duration),
			zap.Bool("timeout", requestErr.Timeout),
			zap.Bool("retryable", requestErr.Retryable),
			zap.Error(requestErr.Cause),
		)
		return nil, requestErr
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		requestErr := &erpBridgeRequestError{
			URL:       target.String(),
			Duration:  duration,
			Timeout:   false,
			Retryable: true,
			Cause:     fmt.Errorf("read erp bridge response: %w", err),
		}
		c.logger.Warn("erp_bridge_response_read_failed",
			zap.String("url", target.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("duration", duration),
			zap.Bool("retryable", requestErr.Retryable),
			zap.Error(requestErr.Cause),
		)
		return nil, requestErr
	}
	if !containsERPBridgeStatus(acceptedStatus, resp.StatusCode) {
		httpErr := &erpBridgeHTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
			URL:        target.String(),
			Duration:   duration,
			Retryable:  isERPBridgeRetryableStatus(resp.StatusCode),
		}
		c.logger.Warn("erp_bridge_upstream_status",
			zap.String("url", target.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("duration", duration),
			zap.Bool("retryable", httpErr.Retryable),
			zap.String("body", truncateLogString(httpErr.Body, 240)),
		)
		return nil, httpErr
	}
	if strings.TrimSpace(string(responseBody)) == "" {
		if allowEmptyBody {
			c.logger.Info("erp_bridge_request_completed",
				zap.String("url", target.String()),
				zap.String("method", method),
				zap.Int("status_code", resp.StatusCode),
				zap.Duration("duration", duration),
			)
			return nil, nil
		}
		decodeErr := &erpBridgeDecodeError{
			URL:         target.String(),
			BodySnippet: "",
			Retryable:   true,
			Cause:       fmt.Errorf("decode erp bridge json: empty response body"),
		}
		c.logger.Warn("erp_bridge_empty_response",
			zap.String("url", target.String()),
			zap.Duration("duration", duration),
			zap.Bool("retryable", decodeErr.Retryable),
		)
		return nil, decodeErr
	}
	c.logger.Info("erp_bridge_request_completed",
		zap.String("url", target.String()),
		zap.String("method", method),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
	)
	return responseBody, nil
}

func decodeERPProductList(payload []byte, fallbackPage, fallbackPageSize int) (*domain.ERPProductListResponse, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	itemsRaw := extractCollection(container)
	if len(itemsRaw) == 0 {
		itemsRaw = extractCollection(root)
	}
	items := normalizeERPProductCollection(itemsRaw)
	pagination := adaptERPPagination(root, container, fallbackPage, fallbackPageSize, int64(len(items)))
	return &domain.ERPProductListResponse{
		Items:      items,
		Pagination: pagination,
	}, nil
}

func decodeERPProduct(payload []byte) (*domain.ERPProduct, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	product := adaptERPProduct(container)
	if product == nil {
		product = adaptERPProduct(extractSingular(container))
	}
	if product == nil {
		product = adaptERPProduct(extractSingular(root))
	}
	if product == nil {
		return nil, nil
	}
	return product, nil
}

func decodeERPCategories(payload []byte) ([]*domain.ERPCategory, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	itemsRaw := extractCollection(container)
	if len(itemsRaw) == 0 {
		itemsRaw = extractCollection(root)
	}
	items := make([]*domain.ERPCategory, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		category := adaptERPCategory(item)
		if category != nil {
			items = append(items, category)
		}
	}
	if items == nil {
		items = []*domain.ERPCategory{}
	}
	return items, nil
}

func decodeERPProductUpsertResult(payload []byte, fallback *domain.ERPProductSelectionSnapshot) (*domain.ERPProductUpsertResult, error) {
	result := &domain.ERPProductUpsertResult{}
	if fallback != nil {
		result.ProductID = fallback.ProductID
		result.SKUID = fallback.SKUID
		result.IID = fallback.IID
		result.SKUCode = fallback.SKUCode
		result.Name = fallback.Name
		result.ProductName = fallback.ProductName
		result.ShortName = fallback.ShortName
		result.CategoryID = fallback.CategoryID
		result.CategoryCode = fallback.CategoryCode
		result.CategoryName = fallback.CategoryName
		result.ProductShortName = fallback.ProductShortName
		result.SPrice = fallback.SPrice
		result.WMSCoID = fallback.WMSCoID
	}
	if len(payload) == 0 {
		return result, nil
	}

	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	product := adaptERPProduct(container)
	if product == nil {
		product = adaptERPProduct(extractSingular(container))
	}
	if product == nil {
		product = adaptERPProduct(extractSingular(root))
	}
	if product != nil {
		result.ProductID = firstNonEmptyString(result.ProductID, product.ProductID)
		result.SKUID = firstNonEmptyString(result.SKUID, product.SKUID)
		result.IID = firstNonEmptyString(result.IID, product.IID)
		result.SKUCode = firstNonEmptyString(result.SKUCode, product.SKUCode)
		result.Name = firstNonEmptyString(result.Name, product.Name)
		result.ProductName = firstNonEmptyString(result.ProductName, product.ProductName)
		result.ShortName = firstNonEmptyString(result.ShortName, product.ShortName)
		result.CategoryID = firstNonEmptyString(result.CategoryID, product.CategoryID)
		result.CategoryCode = firstNonEmptyString(result.CategoryCode, product.CategoryCode)
		result.CategoryName = firstNonEmptyString(result.CategoryName, product.CategoryName)
		result.ProductShortName = firstNonEmptyString(result.ProductShortName, product.ProductShortName)
		if result.SPrice == nil {
			result.SPrice = product.SPrice
		}
		result.WMSCoID = firstNonEmptyString(result.WMSCoID, product.WMSCoID)
	}
	result.SyncLogID = firstERPBridgeString(root, container, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId")
	if result.SyncLogID == "" {
		for _, key := range []string{"sync_log", "syncLog", "log", "sync"} {
			if mapped, ok := lookupMap(container, key); ok {
				result.SyncLogID = firstString(mapped, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId", "id")
				result.Status = firstNonEmptyString(result.Status, firstString(mapped, "status", "state"))
				result.Message = firstNonEmptyString(result.Message, firstString(mapped, "message", "msg", "detail"))
				break
			}
		}
	}
	result.Status = firstNonEmptyString(result.Status, firstERPBridgeString(root, container, "status", "state", "result"))
	result.Message = firstNonEmptyString(result.Message, firstERPBridgeString(root, container, "message", "msg", "detail"))
	result.Route = firstNonEmptyString(firstERPBridgeString(root, container, "route"), result.Route)
	return result, nil
}

func decodeERPItemStyleUpdateResult(payload []byte, fallback domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	result := &domain.ERPItemStyleUpdateResult{
		SKUID:     fallback.SKUID,
		IID:       fallback.IID,
		Name:      fallback.Name,
		ShortName: fallback.ShortName,
		Route:     "itemupload",
	}
	if len(payload) == 0 {
		return result, nil
	}
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	result.SyncLogID = firstERPBridgeString(root, container, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId")
	result.Status = firstNonEmptyString(firstERPBridgeString(root, container, "status", "state", "result"), result.Status)
	result.Message = firstNonEmptyString(firstERPBridgeString(root, container, "message", "msg", "detail"), result.Message)
	if item := adaptERPProduct(container); item != nil {
		if result.SKUID == "" {
			result.SKUID = item.SKUID
		}
		if result.IID == "" {
			result.IID = item.IID
		}
		if result.Name == "" {
			result.Name = firstNonEmptyString(item.Name, item.ProductName)
		}
		if result.ShortName == "" {
			result.ShortName = firstNonEmptyString(item.ShortName, item.ProductShortName)
		}
	}
	return result, nil
}

func decodeERPSyncLogList(payload []byte, fallbackPage, fallbackPageSize int) (*domain.ERPSyncLogListResponse, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	itemsRaw := extractCollection(container)
	if len(itemsRaw) == 0 {
		itemsRaw = extractCollection(root)
	}
	items := make([]*domain.ERPSyncLog, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		if syncLog := adaptERPSyncLog(item); syncLog != nil {
			items = append(items, syncLog)
		}
	}
	if items == nil {
		items = []*domain.ERPSyncLog{}
	}
	return &domain.ERPSyncLogListResponse{
		Items:      items,
		Pagination: adaptERPPagination(root, container, fallbackPage, fallbackPageSize, int64(len(items))),
	}, nil
}

func decodeERPSyncLog(payload []byte) (*domain.ERPSyncLog, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	syncLog := adaptERPSyncLog(container)
	if syncLog == nil {
		syncLog = adaptERPSyncLog(extractSingular(container))
	}
	if syncLog == nil {
		syncLog = adaptERPSyncLog(extractSingular(root))
	}
	return syncLog, nil
}

func decodeERPBatchMutationResult(payload []byte, action string, fallbackTotal int) (*domain.ERPProductBatchMutationResult, error) {
	result := &domain.ERPProductBatchMutationResult{
		Action:   strings.TrimSpace(action),
		Total:    fallbackTotal,
		Accepted: fallbackTotal,
		Rejected: 0,
		Status:   "accepted",
	}
	if len(payload) == 0 {
		result.Message = "accepted"
		return result, nil
	}
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	result.SyncLogID = firstERPBridgeString(root, container, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId")
	result.Status = firstNonEmptyString(firstERPBridgeString(root, container, "status", "state", "result"), result.Status)
	result.Message = firstNonEmptyString(firstERPBridgeString(root, container, "message", "msg", "detail"), result.Message)
	total := firstInt(container, "total", "count", "requested", "request_count")
	if total == 0 {
		total = firstInt(root, "total", "count", "requested", "request_count")
	}
	accepted := firstInt(container, "accepted", "success_count", "success", "updated")
	if accepted == 0 {
		accepted = firstInt(root, "accepted", "success_count", "success", "updated")
	}
	if total > 0 {
		result.Total = total
	}
	if accepted > 0 {
		result.Accepted = accepted
	}
	if result.Accepted > result.Total {
		result.Accepted = result.Total
	}
	result.Rejected = result.Total - result.Accepted
	if result.Rejected < 0 {
		result.Rejected = 0
	}
	if result.Total == 0 && fallbackTotal > 0 {
		result.Total = fallbackTotal
		if result.Accepted == 0 && !strings.EqualFold(result.Status, "failed") {
			result.Accepted = fallbackTotal
		}
	}
	return result, nil
}

func decodeERPVirtualInventoryUpdateResult(payload []byte, fallbackTotal int) (*domain.ERPVirtualInventoryUpdateResult, error) {
	result := &domain.ERPVirtualInventoryUpdateResult{
		Total:    fallbackTotal,
		Accepted: fallbackTotal,
		Status:   "accepted",
	}
	if len(payload) == 0 {
		result.Message = "accepted"
		return result, nil
	}
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	result.SyncLogID = firstERPBridgeString(root, container, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId")
	result.Status = firstNonEmptyString(firstERPBridgeString(root, container, "status", "state", "result"), result.Status)
	result.Message = firstNonEmptyString(firstERPBridgeString(root, container, "message", "msg", "detail"), result.Message)
	total := firstInt(container, "total", "count", "requested", "request_count")
	if total == 0 {
		total = firstInt(root, "total", "count", "requested", "request_count")
	}
	accepted := firstInt(container, "accepted", "success_count", "success", "updated", "updated_count")
	if accepted == 0 {
		accepted = firstInt(root, "accepted", "success_count", "success", "updated", "updated_count")
	}
	if total > 0 {
		result.Total = total
	}
	if accepted > 0 {
		result.Accepted = accepted
	}
	if result.Accepted > result.Total {
		result.Accepted = result.Total
	}
	result.Rejected = result.Total - result.Accepted
	if result.Rejected < 0 {
		result.Rejected = 0
	}
	if result.Total == 0 && fallbackTotal > 0 {
		result.Total = fallbackTotal
		if result.Accepted == 0 && !strings.EqualFold(result.Status, "failed") {
			result.Accepted = fallbackTotal
		}
	}
	return result, nil
}

func decodeERPBridgePayload(payload []byte) (interface{}, error) {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var root interface{}
	if err := decoder.Decode(&root); err != nil {
		return nil, &erpBridgeDecodeError{
			BodySnippet: truncateLogString(string(payload), 240),
			Retryable:   false,
			Cause:       fmt.Errorf("decode erp bridge json: %w", err),
		}
	}
	return root, nil
}

func unwrapERPBridgePayload(root interface{}) interface{} {
	for {
		mapped, ok := root.(map[string]interface{})
		if !ok {
			return root
		}
		switch {
		case mapped["data"] != nil:
			root = mapped["data"]
		case mapped["result"] != nil:
			root = mapped["result"]
		case mapped["payload"] != nil:
			root = mapped["payload"]
		case mapped["content"] != nil:
			root = mapped["content"]
		case mapped["response"] != nil:
			root = mapped["response"]
		default:
			return root
		}
	}
}

func decodeJSTUserListResponse(payload []byte) (*domain.JSTUserListResponse, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	mapped, ok := container.(map[string]interface{})
	if !ok {
		mapped, _ = root.(map[string]interface{})
	}
	if mapped == nil {
		return &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}, nil
	}
	resp := &domain.JSTUserListResponse{
		CurrentPage: anyToString(mapped["current_page"]),
		PageSize:    anyToString(mapped["page_size"]),
		Count:       anyToString(mapped["count"]),
		Pages:       anyToString(mapped["pages"]),
	}
	var itemsRaw []interface{}
	if arr, ok := mapped["datas"].([]interface{}); ok {
		itemsRaw = arr
	} else if arr := extractCollection(container); len(arr) > 0 {
		itemsRaw = arr
	}
	for _, item := range itemsRaw {
		if u := adaptJSTUser(item); u != nil {
			resp.Datas = append(resp.Datas, u)
		}
	}
	if resp.Datas == nil {
		resp.Datas = []*domain.JSTUser{}
	}
	return resp, nil
}

func adaptJSTUser(item interface{}) *domain.JSTUser {
	m, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}
	u := &domain.JSTUser{}
	if n, ok := anyToInt64(m["u_id"]); ok {
		u.UID = n
	}
	u.Name = anyToString(m["name"])
	u.LoginID = anyToString(m["loginId"])
	if b, ok := anyToBool(m["enabled"]); ok {
		u.Enabled = b
	}
	u.Created = anyToString(m["created"])
	u.Modified = anyToString(m["modified"])
	u.LastLoginTime = anyToString(m["last_login_time"])
	u.PwdModified = anyToString(m["pwd_modified"])
	u.Remark = anyToString(m["remark"])
	u.RoleIDs = anyToString(m["role_ids"])
	if u.RoleIDs == "" {
		u.RoleIDs = anyToString(m["roleIds"])
	}
	u.Roles = anyToString(m["roles"])
	u.UGIDs = anyToString(m["ug_ids"])
	if u.UGIDs == "" {
		u.UGIDs = anyToString(m["ugIds"])
	}
	if arr, ok := m["ug_names"].([]interface{}); ok {
		for _, v := range arr {
			if s := anyToString(v); s != "" {
				u.UGNames = append(u.UGNames, s)
			}
		}
	}
	u.Creator = anyToString(m["creator"])
	u.Modifier = anyToString(m["modifier"])
	u.EmpID = anyToString(m["empId"])
	return u
}

func anyToBool(value interface{}) (bool, bool) {
	if value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		return s == "true" || s == "1" || s == "yes", s != ""
	case json.Number:
		n, err := v.Int64()
		return n != 0, err == nil
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	}
	return false, false
}

func extractCollection(root interface{}) []interface{} {
	switch value := root.(type) {
	case []interface{}:
		return value
	case map[string]interface{}:
		for _, key := range []string{"items", "list", "results", "products", "categories", "rows", "records", "datas"} {
			if items, ok := value[key].([]interface{}); ok {
				return items
			}
			if nested := extractCollection(value[key]); len(nested) > 0 {
				return nested
			}
		}
	}
	return nil
}

func extractSingular(root interface{}) interface{} {
	switch value := root.(type) {
	case []interface{}:
		if len(value) > 0 {
			return value[0]
		}
	case map[string]interface{}:
		for _, key := range []string{"item", "product", "detail", "goods", "row"} {
			if nested, ok := value[key]; ok {
				return nested
			}
		}
	}
	return root
}

func normalizeERPProductCollection(itemsRaw []interface{}) []*domain.ERPProduct {
	items := make([]*domain.ERPProduct, 0, len(itemsRaw))
	seen := map[string]int{}
	for _, item := range itemsRaw {
		product := adaptERPProduct(item)
		if product == nil {
			continue
		}
		key := normalizeERPProductIdentityKey(product)
		if existingIdx, ok := seen[key]; ok {
			items[existingIdx] = mergeERPProducts(items[existingIdx], product)
			continue
		}
		seen[key] = len(items)
		items = append(items, product)
	}
	if items == nil {
		return []*domain.ERPProduct{}
	}
	return items
}

func normalizeERPProductIdentityKey(product *domain.ERPProduct) string {
	if product == nil {
		return ""
	}
	switch {
	case strings.TrimSpace(product.ProductID) != "":
		return "PRODUCT:" + strings.ToUpper(strings.TrimSpace(product.ProductID))
	case strings.TrimSpace(product.SKUID) != "":
		return "SKU_ID:" + strings.ToUpper(strings.TrimSpace(product.SKUID))
	case strings.TrimSpace(product.SKUCode) != "":
		return "SKU_CODE:" + strings.ToUpper(strings.TrimSpace(product.SKUCode))
	default:
		return strings.ToUpper(strings.TrimSpace(product.ProductName))
	}
}

func mergeERPProducts(current, incoming *domain.ERPProduct) *domain.ERPProduct {
	if current == nil {
		return incoming
	}
	if incoming == nil {
		return current
	}
	merged := *current
	if merged.ProductID == "" {
		merged.ProductID = incoming.ProductID
	}
	if merged.SKUID == "" {
		merged.SKUID = incoming.SKUID
	}
	if merged.IID == "" {
		merged.IID = incoming.IID
	}
	if merged.SKUCode == "" {
		merged.SKUCode = incoming.SKUCode
	}
	if merged.Name == "" {
		merged.Name = incoming.Name
	}
	if merged.ProductName == "" {
		merged.ProductName = incoming.ProductName
	}
	if merged.ShortName == "" {
		merged.ShortName = incoming.ShortName
	}
	if merged.CategoryID == "" {
		merged.CategoryID = incoming.CategoryID
	}
	if merged.CategoryCode == "" {
		merged.CategoryCode = incoming.CategoryCode
	}
	if merged.CategoryName == "" {
		merged.CategoryName = incoming.CategoryName
	}
	if merged.ProductShortName == "" {
		merged.ProductShortName = incoming.ProductShortName
	}
	if merged.ImageURL == "" {
		merged.ImageURL = incoming.ImageURL
	}
	if merged.Price == nil {
		merged.Price = incoming.Price
	}
	if merged.SPrice == nil {
		merged.SPrice = incoming.SPrice
	}
	if merged.WMSCoID == "" {
		merged.WMSCoID = incoming.WMSCoID
	}
	if merged.Currency == "" {
		merged.Currency = incoming.Currency
	}
	return &merged
}

func adaptERPProduct(root interface{}) *domain.ERPProduct {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	explicitProductID := firstString(mapped, "product_id", "erp_product_id", "productId", "goods_id", "spu_id")
	fallbackID := firstString(mapped, "id")
	product := &domain.ERPProduct{
		ProductID:        firstNonEmptyString(explicitProductID, firstString(mapped, "item_id", "itemId"), fallbackID),
		SKUID:            firstString(mapped, "sku_id", "skuId", "variant_id", "item_id", "spec_id", "goods_sku_id"),
		IID:              firstString(mapped, "i_id", "iId", "style_id", "styleId"),
		SKUCode:          firstString(mapped, "sku_code", "sku", "item_code", "goods_no", "product_code", "outer_code"),
		Name:             firstString(mapped, "name"),
		ProductName:      firstString(mapped, "product_name", "name", "title", "goods_name", "product_title", "sku_name"),
		ShortName:        firstString(mapped, "short_name", "shortName"),
		CategoryID:       firstString(mapped, "category_id", "categoryId", "cat_id", "class_id"),
		CategoryCode:     firstString(mapped, "category_code", "categoryCode", "cat_code", "class_code"),
		CategoryName:     firstString(mapped, "category_name", "category", "cat_name", "category_full_name", "class_name"),
		ProductShortName: firstString(mapped, "product_short_name", "productShortName", "short_name", "shortName", "short_title", "shortTitle", "simple_name"),
		ImageURL:         firstString(mapped, "image_url", "image", "main_image", "cover", "cover_url", "pic_url", "pic", "thumbnail", "thumb", "imageUrl"),
		Price:            firstFloatPtr(mapped, "price", "sale_price", "sales_price", "market_price", "unit_price", "amount", "min_price", "retail_price"),
		SPrice:           firstFloatPtr(mapped, "s_price", "sale_price", "sales_price", "price"),
		WMSCoID:          firstString(mapped, "wms_co_id", "warehouse_code"),
		Currency:         firstString(mapped, "currency", "currency_code", "currencyCode"),
	}
	if product.ProductID == "" {
		product.ProductID = firstNonEmptyString(product.SKUID, product.SKUCode, product.ProductName)
	}
	if product.SKUCode == "" && shouldUseERPProductIDAsSKUCode(product.ProductID, product.ProductName) {
		product.SKUCode = product.ProductID
	}
	if product.Name == "" {
		product.Name = product.ProductName
	}
	if product.ProductName == "" {
		product.ProductName = product.Name
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
	if product.CategoryCode == "" {
		product.CategoryCode = product.CategoryID
	}
	if product.CategoryID == "" {
		product.CategoryID = product.CategoryCode
	}
	if product.ProductName == "" && product.SKUCode == "" && product.ProductID == "" {
		return nil
	}
	return product
}

func adaptERPCategory(root interface{}) *domain.ERPCategory {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	category := &domain.ERPCategory{
		CategoryID:   firstString(mapped, "category_id", "id", "cat_id", "categoryId"),
		CategoryName: firstString(mapped, "category_name", "name", "title", "category", "cat_name"),
		ParentID:     firstString(mapped, "parent_id", "parentId", "p_id"),
		Level:        firstInt(mapped, "level", "level_no", "depth", "tier"),
	}
	if category.CategoryName == "" && category.CategoryID == "" {
		return nil
	}
	return category
}

func adaptERPSyncLog(root interface{}) *domain.ERPSyncLog {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	syncLogID := firstString(mapped, "sync_log_id", "syncLogId", "log_id", "logId", "run_id", "runId", "id")
	if syncLogID == "" {
		return nil
	}
	status := firstString(mapped, "status", "state", "result")
	message := firstString(mapped, "message", "msg", "detail")
	requestPayload := firstRawJSON(mapped, "request_payload", "request", "requestPayload")
	responsePayload := firstRawJSON(mapped, "response_payload", "response", "responsePayload")
	log := &domain.ERPSyncLog{
		SyncLogID:       syncLogID,
		Connector:       firstString(mapped, "connector", "connector_key", "connectorKey"),
		Operation:       firstString(mapped, "operation", "operation_key", "operationKey"),
		Status:          status,
		Message:         message,
		ResourceType:    firstString(mapped, "resource_type", "resourceType"),
		ProductID:       firstString(mapped, "product_id", "productId"),
		SKUCode:         firstString(mapped, "sku_code", "skuCode"),
		RequestPayload:  requestPayload,
		ResponsePayload: responsePayload,
		ErrorMessage:    firstString(mapped, "error", "error_message", "errorMessage"),
	}
	if resourceID := firstInt64(mapped, "resource_id", "resourceId"); resourceID > 0 {
		log.ResourceID = &resourceID
	}
	if createdAt, ok := firstTime(mapped, "created_at", "createdAt", "started_at", "startedAt"); ok {
		log.CreatedAt = &createdAt
	}
	if updatedAt, ok := firstTime(mapped, "updated_at", "updatedAt", "status_updated_at", "statusUpdatedAt", "finished_at", "finishedAt"); ok {
		log.UpdatedAt = &updatedAt
	}
	return log
}

func adaptERPPagination(root, container interface{}, fallbackPage, fallbackPageSize int, fallbackTotal int64) domain.PaginationMeta {
	page := firstInt(root, "page", "current_page", "pageNo")
	pageSize := firstInt(root, "page_size", "per_page", "size", "pageSize")
	total := firstInt64(root, "total", "total_count", "count", "totalCount")

	if page == 0 {
		page = firstInt(container, "page", "current_page", "pageNo")
	}
	if pageSize == 0 {
		pageSize = firstInt(container, "page_size", "per_page", "size", "pageSize")
	}
	if total == 0 {
		total = firstInt64(container, "total", "total_count", "count", "totalCount")
	}
	for _, key := range []string{"pagination", "pager", "meta"} {
		if pagination, ok := lookupMap(root, key); ok {
			if page == 0 {
				page = firstInt(pagination, "page", "current_page", "pageNo")
			}
			if pageSize == 0 {
				pageSize = firstInt(pagination, "page_size", "per_page", "size", "pageSize")
			}
			if total == 0 {
				total = firstInt64(pagination, "total", "total_count", "count", "totalCount")
			}
		}
	}
	if page <= 0 {
		page = fallbackPage
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = fallbackPageSize
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if total <= 0 {
		total = fallbackTotal
	}
	return domain.PaginationMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}

func lookupMap(root interface{}, key string) (map[string]interface{}, bool) {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return nil, false
	}
	value, ok := mapped[key].(map[string]interface{})
	return value, ok
}

func firstString(root interface{}, keys ...string) string {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return ""
	}
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			if s := anyToString(value); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstFloatPtr(root interface{}, keys ...string) *float64 {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			if f, ok := anyToFloat64(value); ok {
				return &f
			}
		}
	}
	return nil
}

func firstInt(root interface{}, keys ...string) int {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return 0
	}
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			if n, ok := anyToInt(value); ok {
				return n
			}
		}
	}
	return 0
}

func firstInt64(root interface{}, keys ...string) int64 {
	mapped, ok := root.(map[string]interface{})
	if !ok {
		return 0
	}
	for _, key := range keys {
		if value, ok := mapped[key]; ok {
			if n, ok := anyToInt64(value); ok {
				return n
			}
		}
	}
	return 0
}

func anyToString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	case map[string]interface{}:
		for _, key := range []string{"url", "href", "src", "value", "name", "label", "text"} {
			if nested, ok := typed[key]; ok {
				if s := anyToString(nested); s != "" {
					return s
				}
			}
		}
	case []interface{}:
		for _, item := range typed {
			if s := anyToString(item); s != "" {
				return s
			}
		}
	}
	return ""
}

func anyToFloat64(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		n, err := typed.Float64()
		if err == nil {
			return n, true
		}
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func anyToInt(value interface{}) (int, bool) {
	n, ok := anyToInt64(value)
	if !ok {
		return 0, false
	}
	return int(n), true
}

func anyToInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		n, err := typed.Int64()
		if err == nil {
			return n, true
		}
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func firstRawJSON(root map[string]interface{}, keys ...string) json.RawMessage {
	for _, key := range keys {
		value, ok := root[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			raw := json.RawMessage(text)
			if json.Valid(raw) {
				return raw
			}
		default:
			raw, err := json.Marshal(typed)
			if err == nil && len(raw) > 0 {
				return raw
			}
		}
	}
	return nil
}

func firstTime(root map[string]interface{}, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		value, ok := root[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
				if parsed, err := time.Parse(layout, text); err == nil {
					return parsed, true
				}
			}
		case float64:
			if typed > 0 {
				return time.Unix(int64(typed), 0).UTC(), true
			}
		case int64:
			if typed > 0 {
				return time.Unix(typed, 0).UTC(), true
			}
		case int:
			if typed > 0 {
				return time.Unix(int64(typed), 0).UTC(), true
			}
		case json.Number:
			if v, err := typed.Int64(); err == nil && v > 0 {
				return time.Unix(v, 0).UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func isERPBridgeRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isERPBridgeTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isERPBridgeRetryableRequest(err error) bool {
	if isERPBridgeTimeout(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

func truncateLogString(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func containsERPBridgeStatus(accepted []int, status int) bool {
	if len(accepted) == 0 {
		return status == http.StatusOK
	}
	for _, item := range accepted {
		if item == status {
			return true
		}
	}
	return false
}

func firstERPBridgeString(root, container interface{}, keys ...string) string {
	if value := firstString(container, keys...); value != "" {
		return value
	}
	return firstString(root, keys...)
}
