package service

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

type ERPRemoteClientConfig struct {
	BaseURL                  string
	UpsertPath               string
	ItemStyleUpdatePath      string
	ShelveBatchPath          string
	UnshelveBatchPath        string
	VirtualQtyPath           string
	SyncLogsPath             string
	GetCompanyUsersPath      string
	SkuQueryPath             string
	OpenWebCharset           string
	OpenWebVersion           string
	Timeout                  time.Duration
	RetryMax                 int
	RetryBackoff             time.Duration
	AuthMode                 string
	AuthHeaderToken          string
	AppKey                   string
	AppSecret                string
	AccessToken              string
	HeaderAppKey             string
	HeaderAccessToken        string
	HeaderTimestamp          string
	HeaderNonce              string
	HeaderSignature          string
	SignatureIncludeBodyHash bool
	Logger                   *zap.Logger
}

type remoteERPBridgeClient struct {
	baseURL                  *url.URL
	upsertPath               string
	itemStyleUpdatePath      string
	shelveBatchPath          string
	unshelveBatchPath        string
	virtualQtyPath           string
	syncLogsPath             string
	getCompanyUsersPath      string
	skuQueryPath             string
	openWebCharset           string
	openWebVersion           string
	httpClient               *http.Client
	retryMax                 int
	retryBackoff             time.Duration
	authMode                 string
	authHeaderToken          string
	appKey                   string
	appSecret                string
	accessToken              string
	headerAppKey             string
	headerAccessToken        string
	headerTimestamp          string
	headerNonce              string
	headerSignature          string
	signatureIncludeBodyHash bool
	logger                   *zap.Logger
}

type hybridERPBridgeClient struct {
	localFallback  ERPBridgeClient
	remote         ERPBridgeClient
	enableFallback bool
	logger         *zap.Logger
}

type erpBridgeOpenWebError struct {
	Code      int
	Message   string
	Body      string
	URL       string
	Duration  time.Duration
	Retryable bool
}

func (e *erpBridgeOpenWebError) Error() string {
	return fmt.Sprintf("remote erp openweb business code %d: %s", e.Code, strings.TrimSpace(e.Message))
}

// ErrERPRemoteOpenWebAuthRequired means remote SKU query cannot run; hybrid must not silently use local products.
var ErrERPRemoteOpenWebAuthRequired = errors.New("erp_remote_openweb_auth_required")

// erpBridgeRemoteProductNotFoundError is returned when OpenWeb responded OK but no SKU row matched (ERP truth: not in catalog).
type erpBridgeRemoteProductNotFoundError struct {
	QueryID string
}

func (e *erpBridgeRemoteProductNotFoundError) Error() string {
	return fmt.Sprintf("jst openweb sku query: product not found for id %s", strings.TrimSpace(e.QueryID))
}

// erpRemoteFailureAllowsLocalFallback is true only for transient/upstream transport failures (hybrid mode).
func erpRemoteFailureAllowsLocalFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrERPRemoteOpenWebAuthRequired) {
		return false
	}
	var nf *erpBridgeRemoteProductNotFoundError
	if errors.As(err, &nf) {
		return false
	}
	var ow *erpBridgeOpenWebError
	if errors.As(err, &ow) {
		return false
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "jst sku query business code") ||
		strings.Contains(lower, "decode jst sku") ||
		strings.Contains(lower, "jst sku response root is not") {
		return false
	}
	var httpErr *erpBridgeHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusNotFound {
			return false
		}
		return httpErr.StatusCode >= 500 || httpErr.Retryable
	}
	var reqErr *erpBridgeRequestError
	if errors.As(err, &reqErr) {
		return reqErr.Timeout || reqErr.Retryable
	}
	return false
}

func classifyERPRemoteErr(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrERPRemoteOpenWebAuthRequired) {
		return "openweb_auth_required"
	}
	var nf *erpBridgeRemoteProductNotFoundError
	if errors.As(err, &nf) {
		return "remote_product_not_found"
	}
	var ow *erpBridgeOpenWebError
	if errors.As(err, &ow) {
		return "openweb_business_error"
	}
	var httpErr *erpBridgeHTTPError
	if errors.As(err, &httpErr) {
		return fmt.Sprintf("http_%d", httpErr.StatusCode)
	}
	var reqErr *erpBridgeRequestError
	if errors.As(err, &reqErr) {
		if reqErr.Timeout {
			return "request_timeout"
		}
		return "request_error"
	}
	return "other"
}

func NewRemoteERPBridgeClient(cfg ERPRemoteClientConfig) (ERPBridgeClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("remote erp base url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse remote erp base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = 600 * time.Millisecond
	}
	retryMax := cfg.RetryMax
	if retryMax < 0 {
		retryMax = 0
	}
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	upsertPath := strings.TrimSpace(cfg.UpsertPath)
	if upsertPath == "" {
		upsertPath = "/v1/erp/products/upsert"
	}
	itemStyleUpdatePath := strings.TrimSpace(cfg.ItemStyleUpdatePath)
	if itemStyleUpdatePath == "" {
		itemStyleUpdatePath = "/open/webapi/itemapi/itemskuim/itemupload"
	}
	shelveBatchPath := strings.TrimSpace(cfg.ShelveBatchPath)
	if shelveBatchPath == "" {
		shelveBatchPath = "/v1/erp/products/shelve/batch"
	}
	unshelveBatchPath := strings.TrimSpace(cfg.UnshelveBatchPath)
	if unshelveBatchPath == "" {
		unshelveBatchPath = "/v1/erp/products/unshelve/batch"
	}
	virtualQtyPath := strings.TrimSpace(cfg.VirtualQtyPath)
	if virtualQtyPath == "" {
		virtualQtyPath = "/v1/erp/inventory/virtual-qty"
	}
	syncLogsPath := strings.TrimSpace(cfg.SyncLogsPath)
	if syncLogsPath == "" {
		syncLogsPath = "/v1/erp/sync-logs"
	}
	getCompanyUsersPath := strings.TrimSpace(cfg.GetCompanyUsersPath)
	if getCompanyUsersPath == "" {
		getCompanyUsersPath = "/open/webapi/userapi/company/getcompanyusers"
	}
	client := &remoteERPBridgeClient{
		baseURL:                  parsed,
		upsertPath:               normalizeERPRemotePath(upsertPath),
		itemStyleUpdatePath:      normalizeERPRemotePath(itemStyleUpdatePath),
		shelveBatchPath:          normalizeERPRemotePath(shelveBatchPath),
		unshelveBatchPath:        normalizeERPRemotePath(unshelveBatchPath),
		virtualQtyPath:           normalizeERPRemotePath(virtualQtyPath),
		syncLogsPath:             normalizeERPRemotePath(syncLogsPath),
		getCompanyUsersPath:     normalizeERPRemotePath(getCompanyUsersPath),
		skuQueryPath:             normalizeERPRemotePath(firstNonEmptyString(strings.TrimSpace(cfg.SkuQueryPath), "/open/sku/query")),
		openWebCharset:           firstNonEmptyString(strings.TrimSpace(cfg.OpenWebCharset), "utf-8"),
		openWebVersion:           firstNonEmptyString(strings.TrimSpace(cfg.OpenWebVersion), "2"),
		httpClient:               &http.Client{Timeout: timeout},
		retryMax:                 retryMax,
		retryBackoff:             retryBackoff,
		authMode:                 strings.ToLower(strings.TrimSpace(cfg.AuthMode)),
		authHeaderToken:          strings.TrimSpace(cfg.AuthHeaderToken),
		appKey:                   strings.TrimSpace(cfg.AppKey),
		appSecret:                strings.TrimSpace(cfg.AppSecret),
		accessToken:              strings.TrimSpace(cfg.AccessToken),
		headerAppKey:             fallbackHeader(cfg.HeaderAppKey, "X-App-Key"),
		headerAccessToken:        fallbackHeader(cfg.HeaderAccessToken, "X-Access-Token"),
		headerTimestamp:          fallbackHeader(cfg.HeaderTimestamp, "X-Timestamp"),
		headerNonce:              fallbackHeader(cfg.HeaderNonce, "X-Nonce"),
		headerSignature:          fallbackHeader(cfg.HeaderSignature, "X-Signature"),
		signatureIncludeBodyHash: cfg.SignatureIncludeBodyHash,
		logger:                   logger,
	}
	if client.authMode == "" {
		client.authMode = "none"
	}
	if client.authMode == "app" && (client.appKey == "" || client.appSecret == "") {
		return nil, fmt.Errorf("remote erp app auth requires app key and app secret")
	}
	if client.authMode == "openweb" && (client.appKey == "" || client.appSecret == "" || client.accessToken == "") {
		return nil, fmt.Errorf("remote erp openweb auth requires app key, app secret, and access token")
	}
	return client, nil
}

func NewHybridERPBridgeClient(local ERPBridgeClient, remote ERPBridgeClient, enableFallback bool, logger *zap.Logger) ERPBridgeClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &hybridERPBridgeClient{
		localFallback:  local,
		remote:         remote,
		enableFallback: enableFallback,
		logger:         logger,
	}
}

func (c *hybridERPBridgeClient) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	tid := domain.TraceIDFromContext(ctx)
	kw := firstNonEmptyString(strings.TrimSpace(filter.Q), strings.TrimSpace(filter.Keyword), strings.TrimSpace(filter.SKUCode))
	if c.remote != nil {
		res, err := c.remote.SearchProducts(ctx, filter)
		if err == nil && res != nil {
			c.logger.Info("erp_bridge_product_search",
				zap.String("trace_id", tid),
				zap.String("erp_layer", "8081_remote"),
				zap.String("result", "remote_ok"),
				zap.Bool("fallback_used", false),
				zap.String("search_keyword", truncateLogString(kw, 120)),
				zap.Int("item_count", len(res.Items)),
			)
			return res, nil
		}
		if err != nil && !erpRemoteFailureAllowsLocalFallback(err) {
			c.logger.Warn("erp_bridge_product_search",
				zap.String("trace_id", tid),
				zap.String("erp_layer", "8081_remote"),
				zap.String("result", "remote_error_no_fallback"),
				zap.String("search_keyword", truncateLogString(kw, 120)),
				zap.String("error_class", classifyERPRemoteErr(err)),
				zap.Error(err),
			)
			return nil, err
		}
		if !c.enableFallback || c.localFallback == nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
		c.logger.Warn("erp_bridge_product_search",
			zap.String("trace_id", tid),
			zap.String("erp_layer", "8081_hybrid"),
			zap.String("result", "fallback_local_products"),
			zap.Bool("fallback_used", true),
			zap.String("fallback_reason", classifyERPRemoteErr(err)),
			zap.String("search_keyword", truncateLogString(kw, 120)),
			zap.Error(err),
		)
		out, locErr := c.localFallback.SearchProducts(ctx, filter)
		if locErr != nil {
			return nil, locErr
		}
		return out, nil
	}
	if c.localFallback == nil {
		return nil, fmt.Errorf("local fallback erp bridge client is unavailable")
	}
	c.logger.Info("erp_bridge_product_search",
		zap.String("trace_id", tid),
		zap.String("erp_layer", "8081_local_only"),
		zap.String("search_keyword", truncateLogString(kw, 120)),
	)
	return c.localFallback.SearchProducts(ctx, filter)
}

func (c *hybridERPBridgeClient) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error) {
	tid := domain.TraceIDFromContext(ctx)
	if c.remote != nil {
		res, err := c.remote.GetProductByID(ctx, id)
		if err == nil && res != nil {
			c.logger.Info("erp_bridge_product_by_id",
				zap.String("trace_id", tid),
				zap.String("erp_layer", "8081_remote"),
				zap.String("result", "remote_ok"),
				zap.Bool("fallback_used", false),
				zap.String("id", truncateLogString(strings.TrimSpace(id), 64)),
			)
			return res, nil
		}
		if err != nil && !erpRemoteFailureAllowsLocalFallback(err) {
			c.logger.Warn("erp_bridge_product_by_id",
				zap.String("trace_id", tid),
				zap.String("erp_layer", "8081_remote"),
				zap.String("result", "remote_error_no_fallback"),
				zap.String("id", truncateLogString(strings.TrimSpace(id), 64)),
				zap.String("error_class", classifyERPRemoteErr(err)),
				zap.Error(err),
			)
			return nil, err
		}
		if !c.enableFallback || c.localFallback == nil {
			if err != nil {
				return nil, err
			}
			return res, nil
		}
		c.logger.Warn("erp_bridge_product_by_id",
			zap.String("trace_id", tid),
			zap.String("erp_layer", "8081_hybrid"),
			zap.String("result", "fallback_local_products"),
			zap.Bool("fallback_used", true),
			zap.String("fallback_reason", classifyERPRemoteErr(err)),
			zap.String("id", truncateLogString(strings.TrimSpace(id), 64)),
			zap.Error(err),
		)
		out, locErr := c.localFallback.GetProductByID(ctx, id)
		if locErr != nil {
			return nil, locErr
		}
		return out, nil
	}
	if c.localFallback == nil {
		return nil, fmt.Errorf("local fallback erp bridge client is unavailable")
	}
	return c.localFallback.GetProductByID(ctx, id)
}

func (c *hybridERPBridgeClient) ListCategories(ctx context.Context) ([]*domain.ERPCategory, error) {
	if c.localFallback == nil {
		return []*domain.ERPCategory{}, nil
	}
	return c.localFallback.ListCategories(ctx)
}

func (c *hybridERPBridgeClient) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return &domain.ERPSyncLogListResponse{
				Items:      []*domain.ERPSyncLog{},
				Pagination: buildPaginationMeta(filter.Page, filter.PageSize, 0),
			}, nil
		}
		return c.localFallback.ListSyncLogs(ctx, filter)
	}
	result, err := c.remote.ListSyncLogs(ctx, filter)
	if err == nil {
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_sync_logs_failed_fallback_local", zap.Error(err))
	return c.localFallback.ListSyncLogs(ctx, filter)
}

func (c *hybridERPBridgeClient) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, nil
		}
		return c.localFallback.GetSyncLogByID(ctx, id)
	}
	result, err := c.remote.GetSyncLogByID(ctx, id)
	if err == nil {
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_sync_log_detail_failed_fallback_local", zap.Error(err))
	return c.localFallback.GetSyncLogByID(ctx, id)
}

func (c *hybridERPBridgeClient) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, fmt.Errorf("remote erp bridge client is unavailable")
		}
		c.logger.Info("erp_bridge_write_path_local", zap.String("operation", "upsert"))
		return c.localFallback.UpsertProduct(ctx, payload)
	}
	result, err := c.remote.UpsertProduct(ctx, payload)
	if err == nil {
		c.logger.Info("erp_bridge_write_path_remote", zap.String("operation", "upsert"))
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_upsert_failed_fallback_local", zap.Error(err))
	localResult, localErr := c.localFallback.UpsertProduct(ctx, payload)
	if localErr != nil {
		c.logger.Error("erp_remote_upsert_fallback_local_failed", zap.Error(localErr))
		return nil, localErr
	}
	c.logger.Info("erp_remote_upsert_fallback_local_success")
	return localResult, nil
}

func (c *hybridERPBridgeClient) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, fmt.Errorf("remote erp bridge client is unavailable")
		}
		c.logger.Info("erp_bridge_write_path_local", zap.String("operation", "item_style_update"))
		return c.localFallback.UpdateItemStyle(ctx, payload)
	}
	result, err := c.remote.UpdateItemStyle(ctx, payload)
	if err == nil {
		c.logger.Info("erp_bridge_write_path_remote", zap.String("operation", "item_style_update"))
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_item_style_update_failed_fallback_local", zap.Error(err))
	localResult, localErr := c.localFallback.UpdateItemStyle(ctx, payload)
	if localErr != nil {
		c.logger.Error("erp_remote_item_style_update_fallback_local_failed", zap.Error(localErr))
		return nil, localErr
	}
	c.logger.Info("erp_remote_item_style_update_fallback_local_success")
	return localResult, nil
}

func (c *hybridERPBridgeClient) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, fmt.Errorf("remote erp bridge client is unavailable")
		}
		c.logger.Info("erp_bridge_write_path_local", zap.String("operation", "shelve_batch"))
		return c.localFallback.ShelveProductsBatch(ctx, payload)
	}
	result, err := c.remote.ShelveProductsBatch(ctx, payload)
	if err == nil {
		c.logger.Info("erp_bridge_write_path_remote", zap.String("operation", "shelve_batch"))
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_shelve_batch_failed_fallback_local", zap.Error(err))
	localResult, localErr := c.localFallback.ShelveProductsBatch(ctx, payload)
	if localErr != nil {
		c.logger.Error("erp_remote_shelve_batch_fallback_local_failed", zap.Error(localErr))
		return nil, localErr
	}
	c.logger.Info("erp_remote_shelve_batch_fallback_local_success")
	return localResult, nil
}

func (c *hybridERPBridgeClient) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, fmt.Errorf("remote erp bridge client is unavailable")
		}
		c.logger.Info("erp_bridge_write_path_local", zap.String("operation", "unshelve_batch"))
		return c.localFallback.UnshelveProductsBatch(ctx, payload)
	}
	result, err := c.remote.UnshelveProductsBatch(ctx, payload)
	if err == nil {
		c.logger.Info("erp_bridge_write_path_remote", zap.String("operation", "unshelve_batch"))
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_unshelve_batch_failed_fallback_local", zap.Error(err))
	localResult, localErr := c.localFallback.UnshelveProductsBatch(ctx, payload)
	if localErr != nil {
		c.logger.Error("erp_remote_unshelve_batch_fallback_local_failed", zap.Error(localErr))
		return nil, localErr
	}
	c.logger.Info("erp_remote_unshelve_batch_fallback_local_success")
	return localResult, nil
}

func (c *hybridERPBridgeClient) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	if c.remote == nil {
		if c.localFallback == nil {
			return nil, fmt.Errorf("remote erp bridge client is unavailable")
		}
		c.logger.Info("erp_bridge_write_path_local", zap.String("operation", "virtual_inventory"))
		return c.localFallback.UpdateVirtualInventory(ctx, payload)
	}
	result, err := c.remote.UpdateVirtualInventory(ctx, payload)
	if err == nil {
		c.logger.Info("erp_bridge_write_path_remote", zap.String("operation", "virtual_inventory"))
		return result, nil
	}
	if !c.enableFallback || c.localFallback == nil {
		return nil, err
	}
	c.logger.Warn("erp_remote_virtual_inventory_failed_fallback_local", zap.Error(err))
	localResult, localErr := c.localFallback.UpdateVirtualInventory(ctx, payload)
	if localErr != nil {
		c.logger.Error("erp_remote_virtual_inventory_fallback_local_failed", zap.Error(localErr))
		return nil, localErr
	}
	c.logger.Info("erp_remote_virtual_inventory_fallback_local_success")
	return localResult, nil
}

func (c *hybridERPBridgeClient) GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	if c.remote == nil {
		return nil, fmt.Errorf("jst getcompanyusers requires remote erp bridge client")
	}
	return c.remote.GetCompanyUsers(ctx, filter)
}

func (c *remoteERPBridgeClient) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	if !strings.EqualFold(strings.TrimSpace(c.authMode), "openweb") {
		return nil, fmt.Errorf("%w: remote erp sku query requires ERP_REMOTE_AUTH_MODE=openweb", ErrERPRemoteOpenWebAuthRequired)
	}
	raw, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("marshal erp search filter: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.skuQueryPath, nil, raw, "jst_sku_query")
	if err != nil {
		return nil, err
	}
	rows, total, err := jstExtractSkuRows(respBody)
	if err != nil {
		return nil, err
	}
	kw := firstNonEmptyString(strings.TrimSpace(filter.Q), strings.TrimSpace(filter.Keyword), strings.TrimSpace(filter.SKUCode))
	items := jstMapsToERPProducts(rows, kw)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	ps := filter.PageSize
	if ps < 1 {
		ps = 20
	}
	if len(items) == 0 && len(rows) > 0 && kw != "" {
		items = jstMapsToERPProducts(rows, "")
	}
	return &domain.ERPProductListResponse{
		Items: items,
		Pagination: domain.PaginationMeta{
			Page:     page,
			PageSize: ps,
			Total:    total,
		},
	}, nil
}

func (c *remoteERPBridgeClient) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error) {
	if !strings.EqualFold(strings.TrimSpace(c.authMode), "openweb") {
		return nil, fmt.Errorf("%w: remote erp sku query requires ERP_REMOTE_AUTH_MODE=openweb", ErrERPRemoteOpenWebAuthRequired)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("product id is required")
	}
	raw, err := json.Marshal(map[string]string{"id": id})
	if err != nil {
		return nil, err
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.skuQueryPath, nil, raw, "jst_sku_query_by_id")
	if err != nil {
		return nil, err
	}
	rows, _, err := jstExtractSkuRows(respBody)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &erpBridgeRemoteProductNotFoundError{QueryID: id}
	}
	items := jstMapsToERPProducts(rows, "")
	for _, p := range items {
		if strings.TrimSpace(p.SKUID) == id || strings.TrimSpace(p.SKUCode) == id || strings.TrimSpace(p.ProductID) == id {
			return p, nil
		}
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return nil, &erpBridgeRemoteProductNotFoundError{QueryID: id}
}

func (c *remoteERPBridgeClient) ListCategories(ctx context.Context) ([]*domain.ERPCategory, error) {
	return []*domain.ERPCategory{}, nil
}

func (c *remoteERPBridgeClient) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
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
	respBody, err := c.doRequestWithRetry(ctx, http.MethodGet, c.syncLogsPath, query, nil, "sync_logs")
	if err != nil {
		return nil, err
	}
	return decodeERPSyncLogList(respBody, filter.Page, filter.PageSize)
}

func (c *remoteERPBridgeClient) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("remote erp sync log id is required")
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodGet, c.syncLogsPath+"/"+url.PathEscape(id), nil, nil, "sync_log_detail")
	if err != nil {
		return nil, err
	}
	return decodeERPSyncLog(respBody)
}

func (c *remoteERPBridgeClient) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp upsert payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.upsertPath, nil, raw, "upsert")
	if err != nil {
		return nil, err
	}
	return decodeERPProductUpsertResult(respBody, payload.Product)
}

func (c *remoteERPBridgeClient) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp item style payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.itemStyleUpdatePath, nil, raw, "item_style_update")
	if err != nil {
		return nil, err
	}
	return decodeERPItemStyleUpdateResult(respBody, payload)
}

func (c *remoteERPBridgeClient) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp shelve batch payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.shelveBatchPath, nil, raw, "shelve_batch")
	if err != nil {
		return nil, err
	}
	return decodeERPBatchMutationResult(respBody, "shelve", len(payload.Items))
}

func (c *remoteERPBridgeClient) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp unshelve batch payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.unshelveBatchPath, nil, raw, "unshelve_batch")
	if err != nil {
		return nil, err
	}
	return decodeERPBatchMutationResult(respBody, "unshelve", len(payload.Items))
}

func (c *remoteERPBridgeClient) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp virtual inventory payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.virtualQtyPath, nil, raw, "virtual_inventory")
	if err != nil {
		return nil, err
	}
	return decodeERPVirtualInventoryUpdateResult(respBody, len(payload.Items))
}

func (c *remoteERPBridgeClient) GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	raw, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp getcompanyusers payload: %w", err)
	}
	respBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.getCompanyUsersPath, nil, raw, "getcompanyusers")
	if err != nil {
		return nil, err
	}
	return decodeJSTUserListResponse(respBody)
}

func (c *remoteERPBridgeClient) doRequestWithRetry(ctx context.Context, method, requestPath string, query url.Values, body []byte, operation string) ([]byte, error) {
	var lastErr error
	attempts := c.retryMax + 1
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		respBody, err := c.doRequestOnce(ctx, method, requestPath, query, body, attempt, operation)
		if err == nil {
			return respBody, nil
		}
		lastErr = err
		if !isRetryableERPBridgeError(err) || attempt >= attempts {
			break
		}
		sleep := time.Duration(attempt) * c.retryBackoff
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(sleep):
		}
	}
	return nil, lastErr
}

func (c *remoteERPBridgeClient) doRequestOnce(ctx context.Context, method, requestPath string, query url.Values, body []byte, attempt int, operation string) ([]byte, error) {
	if c.authMode == "openweb" {
		return c.doOpenWebRequest(ctx, method, requestPath, body, attempt, operation)
	}

	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), requestPath)
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, target.String(), strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build remote erp request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.applyAuthHeaders(ctx, req, method, body, requestPath); err != nil {
		return nil, err
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
			Cause:     fmt.Errorf("do remote erp request: %w", err),
		}
		c.logger.Warn("remote_erp_request_failed",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("method", method),
			zap.String("url", target.String()),
			zap.Duration("duration", duration),
			zap.Bool("timeout", requestErr.Timeout),
			zap.Bool("retryable", requestErr.Retryable),
			zap.Error(requestErr.Cause),
		)
		return nil, requestErr
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		requestErr := &erpBridgeRequestError{
			URL:       target.String(),
			Duration:  duration,
			Retryable: true,
			Cause:     fmt.Errorf("read remote erp response: %w", err),
		}
		return nil, requestErr
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		httpErr := &erpBridgeHTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
			URL:        target.String(),
			Duration:   duration,
			Retryable:  isERPBridgeRetryableStatus(resp.StatusCode),
		}
		c.logger.Warn("remote_erp_upstream_status",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("method", method),
			zap.String("url", target.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Bool("retryable", httpErr.Retryable),
			zap.String("body", truncateLogString(httpErr.Body, 240)),
		)
		return nil, httpErr
	}
	c.logger.Info("remote_erp_request_completed",
		zap.Int("attempt", attempt),
		zap.String("operation", operation),
		zap.String("method", method),
		zap.String("url", target.String()),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
	)
	return respBody, nil
}

func (c *remoteERPBridgeClient) doOpenWebRequest(ctx context.Context, method, requestPath string, body []byte, attempt int, operation string) ([]byte, error) {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return nil, fmt.Errorf("remote erp openweb mode only supports POST requests")
	}
	bizPayload, err := buildERPRemoteOpenWebBiz(operation, body)
	if err != nil {
		return nil, err
	}
	bizRaw, err := json.Marshal(bizPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal remote erp openweb biz payload: %w", err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signParams := map[string]string{
		"app_key":      c.appKey,
		"access_token": c.accessToken,
		"timestamp":    timestamp,
		"charset":      c.openWebCharset,
		"version":      c.openWebVersion,
		"biz":          string(bizRaw),
	}
	signature := signERPRemoteOpenWeb(c.appSecret, signParams)

	formData := url.Values{}
	for key, value := range signParams {
		formData.Set(key, value)
	}
	formData.Set("sign", signature)

	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), requestPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build remote erp openweb request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Charset", "utf-8")

	c.logger.Info("remote_erp_openweb_request_started",
		zap.Int("attempt", attempt),
		zap.String("operation", operation),
		zap.String("url", target.String()),
	)
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		requestErr := &erpBridgeRequestError{
			URL:       target.String(),
			Duration:  duration,
			Timeout:   isERPBridgeTimeout(err),
			Retryable: isERPBridgeRetryableRequest(err),
			Cause:     fmt.Errorf("do remote erp openweb request: %w", err),
		}
		c.logger.Warn("remote_erp_openweb_request_failed",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("url", target.String()),
			zap.Duration("duration", duration),
			zap.Bool("timeout", requestErr.Timeout),
			zap.Bool("retryable", requestErr.Retryable),
			zap.Error(requestErr.Cause),
		)
		return nil, requestErr
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		requestErr := &erpBridgeRequestError{
			URL:       target.String(),
			Duration:  duration,
			Retryable: true,
			Cause:     fmt.Errorf("read remote erp openweb response: %w", err),
		}
		return nil, requestErr
	}
	if resp.StatusCode != http.StatusOK {
		httpErr := &erpBridgeHTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
			URL:        target.String(),
			Duration:   duration,
			Retryable:  isERPBridgeRetryableStatus(resp.StatusCode),
		}
		c.logger.Warn("remote_erp_openweb_upstream_status",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("url", target.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Bool("retryable", httpErr.Retryable),
			zap.String("body", truncateLogString(httpErr.Body, 240)),
		)
		return nil, httpErr
	}
	if openWebCode, openWebMsg, ok := parseERPRemoteOpenWebCodeMsg(respBody); ok && openWebCode != 0 {
		openWebErr := &erpBridgeOpenWebError{
			Code:      openWebCode,
			Message:   openWebMsg,
			Body:      strings.TrimSpace(string(respBody)),
			URL:       target.String(),
			Duration:  duration,
			Retryable: false,
		}
		c.logger.Warn("remote_erp_openweb_business_error",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("url", target.String()),
			zap.Int("openweb_code", openWebErr.Code),
			zap.String("openweb_message", truncateLogString(openWebErr.Message, 240)),
			zap.String("body", truncateLogString(openWebErr.Body, 240)),
		)
		return nil, openWebErr
	}
	if openWebErr := validateERPRemoteOpenWebResponse(operation, target.String(), duration, respBody); openWebErr != nil {
		c.logger.Warn("remote_erp_openweb_business_error",
			zap.Int("attempt", attempt),
			zap.String("operation", operation),
			zap.String("url", target.String()),
			zap.Int("openweb_code", openWebErr.Code),
			zap.String("openweb_message", truncateLogString(openWebErr.Message, 240)),
			zap.String("body", truncateLogString(openWebErr.Body, 240)),
		)
		return nil, openWebErr
	}
	c.logger.Info("remote_erp_openweb_request_completed",
		zap.Int("attempt", attempt),
		zap.String("operation", operation),
		zap.String("url", target.String()),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
	)
	return respBody, nil
}

func (c *remoteERPBridgeClient) applyAuthHeaders(ctx context.Context, req *http.Request, method string, body []byte, requestPath string) error {
	switch c.authMode {
	case "none":
		if bearerToken, ok := domain.RequestBearerTokenFromContext(ctx); ok {
			req.Header.Set("Authorization", "Bearer "+bearerToken)
		}
		return nil
	case "bearer":
		token := firstNonEmptyString(c.authHeaderToken, c.accessToken)
		if bearerToken, ok := domain.RequestBearerTokenFromContext(ctx); ok {
			token = bearerToken
		}
		if token == "" {
			return fmt.Errorf("remote erp bearer token is required")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	case "app":
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce, err := randomNonceHex(8)
		if err != nil {
			return fmt.Errorf("generate remote erp nonce: %w", err)
		}
		signInput := buildERPRemoteSignInput(method, requestPath, timestamp, nonce, body, c.signatureIncludeBodyHash)
		signature := hmacSHA256Hex(c.appSecret, signInput)
		req.Header.Set(c.headerAppKey, c.appKey)
		req.Header.Set(c.headerTimestamp, timestamp)
		req.Header.Set(c.headerNonce, nonce)
		req.Header.Set(c.headerSignature, signature)
		if c.accessToken != "" {
			req.Header.Set(c.headerAccessToken, c.accessToken)
		}
		return nil
	case "openweb":
		return nil
	default:
		return fmt.Errorf("unsupported remote erp auth mode: %s", c.authMode)
	}
}

func buildERPRemoteSignInput(method, reqPath, timestamp, nonce string, body []byte, includeBodyHash bool) string {
	part := strings.TrimSpace(string(body))
	if includeBodyHash {
		hash := sha256.Sum256(body)
		part = hex.EncodeToString(hash[:])
	}
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		strings.TrimSpace(reqPath),
		strings.TrimSpace(timestamp),
		strings.TrimSpace(nonce),
		part,
	}, "\n")
}

func hmacSHA256Hex(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomNonceHex(size int) (string, error) {
	if size <= 0 {
		size = 8
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func fallbackHeader(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	return raw
}

func normalizeERPRemotePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return raw
}

func isRetryableERPBridgeError(err error) bool {
	var requestErr *erpBridgeRequestError
	if strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "context deadline exceeded") {
		return true
	}
	if errors.As(err, &requestErr) {
		return requestErr.Retryable
	}
	var httpErr *erpBridgeHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable
	}
	var openWebErr *erpBridgeOpenWebError
	if errors.As(err, &openWebErr) {
		return openWebErr.Retryable
	}
	return false
}

func signERPRemoteOpenWeb(appSecret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if strings.EqualFold(strings.TrimSpace(key), "sign") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	builder.WriteString(strings.TrimSpace(appSecret))
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(params[key])
	}
	sum := md5.Sum([]byte(builder.String()))
	return fmt.Sprintf("%x", sum)
}

func parseERPRemoteOpenWebCodeMsg(payload []byte) (int, string, bool) {
	if len(payload) == 0 {
		return 0, "", false
	}
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return 0, "", false
	}
	code := firstInt(root, "code", "errcode", "error_code")
	msg := firstString(root, "msg", "message", "error_msg")
	return code, msg, true
}

func validateERPRemoteOpenWebResponse(operation, requestURL string, duration time.Duration, payload []byte) *erpBridgeOpenWebError {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil
	}
	if strings.TrimSpace(operation) != "virtual_inventory" {
		return nil
	}
	msg := strings.TrimSpace(firstString(root, "msg", "message", "error_msg"))
	if msg == "" {
		return nil
	}
	if strings.Contains(msg, "未获取到有效的传入数据") {
		return &erpBridgeOpenWebError{
			Code:      100,
			Message:   msg,
			Body:      strings.TrimSpace(string(payload)),
			URL:       requestURL,
			Duration:  duration,
			Retryable: false,
		}
	}
	container := unwrapERPBridgePayload(root)
	successCount := firstInt(container, "success_count", "updated_count", "updated", "success", "affected")
	if successCount == 0 {
		successCount = firstInt(root, "success_count", "updated_count", "updated", "success", "affected")
	}
	if successCount == 0 {
		return &erpBridgeOpenWebError{
			Code:      100,
			Message:   msg,
			Body:      strings.TrimSpace(string(payload)),
			URL:       requestURL,
			Duration:  duration,
			Retryable: false,
		}
	}
	return nil
}

func buildERPRemoteOpenWebBiz(operation string, rawBody []byte) (map[string]interface{}, error) {
	switch strings.TrimSpace(operation) {
	case "upsert":
		var payload domain.ERPProductUpsertPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return nil, fmt.Errorf("decode remote erp upsert payload: %w", err)
		}
		payload = normalizeERPProductUpsertPayload(payload)
		skuID := firstNonEmptyString(payload.SKUID, payload.SKUCode)
		item := map[string]interface{}{
			"sku_id": skuID,
			"name":   firstNonEmptyString(payload.Name, payload.ProductName, payload.ProductShortName, payload.SKUCode),
		}
		if iID := strings.TrimSpace(payload.IID); iID != "" {
			item["i_id"] = iID
		}
		if shortName := firstNonEmptyString(payload.ShortName, payload.ProductShortName); shortName != "" {
			item["short_name"] = shortName
		}
		if payload.SPrice != nil {
			item["s_price"] = *payload.SPrice
		}
		if categoryName := strings.TrimSpace(payload.CategoryName); categoryName != "" {
			item["category_name"] = categoryName
		}
		if payload.CostPrice != nil {
			item["cost_price"] = *payload.CostPrice
		} else if payload.BusinessInfo != nil && payload.BusinessInfo.CostPrice != nil {
			item["cost_price"] = *payload.BusinessInfo.CostPrice
		}
		if payload.Remark != "" {
			item["remark"] = payload.Remark
		}
		if _, ok := item["remark"]; !ok {
			remark := ""
			if payload.TaskContext != nil {
				remark = strings.TrimSpace(payload.TaskContext.Remark)
			}
			remark = firstNonEmptyString(remark, payload.Source)
			if remark != "" {
				item["remark"] = remark
			}
		}
		if payload.SupplierName != "" {
			item["supplier_name"] = payload.SupplierName
		}
		if payload.WMSCoID != "" {
			item["wms_co_id"] = payload.WMSCoID
		}
		for key, value := range map[string]string{
			"brand":            payload.Brand,
			"vc_name":          payload.VCName,
			"item_type":        payload.ItemType,
			"pic":              payload.Pic,
			"pic_big":          payload.PicBig,
			"sku_pic":          payload.SKUPic,
			"properties_value": payload.PropertiesValue,
			"supplier_sku_id":  payload.SupplierSKUID,
			"supplier_i_id":    payload.SupplierIID,
			"other_1":          payload.Other1,
			"other_2":          payload.Other2,
			"other_3":          payload.Other3,
			"other_4":          payload.Other4,
			"other_5":          payload.Other5,
		} {
			if strings.TrimSpace(value) != "" {
				item[key] = value
			}
		}
		for key, value := range map[string]*float64{
			"weight":        payload.Weight,
			"l":             payload.L,
			"w":             payload.W,
			"h":             payload.H,
			"market_price":  payload.MarketPrice,
			"other_price_1": payload.OtherPrice1,
			"other_price_2": payload.OtherPrice2,
			"other_price_3": payload.OtherPrice3,
			"other_price_4": payload.OtherPrice4,
			"other_price_5": payload.OtherPrice5,
		} {
			if value != nil {
				item[key] = *value
			}
		}
		for key, value := range map[string]*bool{
			"enabled":        payload.Enabled,
			"stock_disabled": payload.StockDisabled,
		} {
			if value != nil {
				item[key] = *value
			}
		}
		if payload.ProductID != "" {
			item["product_id"] = payload.ProductID
		}
		return map[string]interface{}{"items": []map[string]interface{}{item}}, nil
	case "item_style_update":
		var payload domain.ERPItemStyleUpdatePayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return nil, fmt.Errorf("decode remote erp item style payload: %w", err)
		}
		payload = normalizeERPItemStyleUpdatePayload(payload)
		item := map[string]interface{}{
			"i_id": payload.IID,
		}
		if payload.SKUID != "" {
			item["sku_id"] = payload.SKUID
		}
		for key, value := range map[string]string{
			"name":             payload.Name,
			"short_name":       payload.ShortName,
			"category_name":    payload.CategoryName,
			"pic":              payload.Pic,
			"pic_big":          payload.PicBig,
			"sku_pic":          payload.SKUPic,
			"properties_value": payload.PropertiesValue,
			"brand":            payload.Brand,
			"vc_name":          payload.VCName,
			"supplier_i_id":    payload.SupplierIID,
		} {
			if strings.TrimSpace(value) != "" {
				item[key] = value
			}
		}
		if payload.Enabled != nil {
			item["enabled"] = *payload.Enabled
		}
		return map[string]interface{}{"items": []map[string]interface{}{item}}, nil
	case "shelve_batch", "unshelve_batch":
		var payload domain.ERPProductBatchMutationPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return nil, fmt.Errorf("decode remote erp batch mutation payload: %w", err)
		}
		payload = normalizeERPBridgeBatchMutationPayload(payload)
		items := make([]map[string]interface{}, 0, len(payload.Items))
		skuCodes := make([]string, 0, len(payload.Items))
		for _, item := range payload.Items {
			skuID := firstNonEmptyString(item.SKUID, item.SKUCode)
			if skuID == "" {
				continue
			}
			skuCode := strings.TrimSpace(item.SKUCode)
			productID := strings.TrimSpace(item.ProductID)
			entry := map[string]interface{}{
				"sku_id": skuID,
			}
			if skuCode != "" {
				entry["sku_code"] = skuCode
			}
			if productID != "" {
				entry["product_id"] = productID
			}
			if wmsCoID := strings.TrimSpace(item.WMSCoID); wmsCoID != "" {
				entry["wms_co_id"] = wmsCoID
			}
			if binID := strings.TrimSpace(item.BinID); binID != "" {
				entry["bin_id"] = binID
			}
			if carryID := strings.TrimSpace(item.CarryID); carryID != "" {
				entry["carry_id"] = carryID
			}
			if boxNo := strings.TrimSpace(item.BoxNo); boxNo != "" {
				entry["box_no"] = boxNo
			}
			if item.Qty != nil {
				entry["qty"] = *item.Qty
			}
			items = append(items, entry)
			skuCodes = append(skuCodes, firstNonEmptyString(skuCode, skuID, productID))
		}
		return map[string]interface{}{
			"items":     items,
			"sku_codes": strings.Join(skuCodes, ","),
		}, nil
	case "virtual_inventory":
		var payload domain.ERPVirtualInventoryUpdatePayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			return nil, fmt.Errorf("decode remote erp virtual inventory payload: %w", err)
		}
		payload = normalizeERPBridgeVirtualInventoryPayload(payload)
		list := make([]map[string]interface{}, 0, len(payload.Items))
		for _, item := range payload.Items {
			skuID := firstNonEmptyString(item.SKUID, item.SKUCode)
			if skuID == "" {
				continue
			}
			skuCode := strings.TrimSpace(item.SKUCode)
			productID := strings.TrimSpace(item.ProductID)
			warehouseCode := strings.TrimSpace(item.WarehouseCode)
			wmsCoID := strings.TrimSpace(item.WMSCoID)
			if wmsCoID == "" {
				wmsCoID = warehouseCode
			}
			entry := map[string]interface{}{
				"sku_id":      skuID,
				"virtual_qty": item.VirtualQty,
				"qty":         item.VirtualQty,
			}
			if skuCode != "" {
				entry["sku_code"] = skuCode
			}
			if productID != "" {
				entry["product_id"] = productID
			}
			if strings.TrimSpace(item.IID) != "" {
				entry["i_id"] = strings.TrimSpace(item.IID)
			}
			if warehouseCode != "" {
				entry["warehouse_code"] = warehouseCode
			}
			if wmsCoID != "" {
				entry["wms_co_id"] = wmsCoID
				if wmsID, parseErr := strconv.ParseInt(wmsCoID, 10, 64); parseErr == nil && wmsID > 0 {
					entry["wms_co_id"] = wmsID
				}
			}
			list = append(list, entry)
		}
		return map[string]interface{}{"list": list, "items": list}, nil
	case "getcompanyusers":
		var filter domain.JSTUserListFilter
		if err := json.Unmarshal(rawBody, &filter); err != nil {
			return nil, fmt.Errorf("decode remote erp getcompanyusers payload: %w", err)
		}
		pageAction := filter.PageAction
		if pageAction < 0 || pageAction > 2 {
			pageAction = 0
		}
		currentPage := filter.CurrentPage
		if currentPage <= 0 {
			currentPage = 1
		}
		pageSize := filter.PageSize
		if pageSize <= 0 {
			pageSize = 50
		}
		version := filter.Version
		if version <= 0 {
			version = 2
		}
		biz := map[string]interface{}{
			"page_action":  pageAction,
			"current_page": currentPage,
			"page_size":    pageSize,
			"version":     version,
		}
		if filter.Enabled != nil {
			biz["enabled"] = *filter.Enabled
		}
		if strings.TrimSpace(filter.LoginID) != "" {
			biz["loginId"] = filter.LoginID
		}
		if strings.TrimSpace(filter.CreatedBegin) != "" {
			biz["creatd_begin"] = filter.CreatedBegin
		}
		if strings.TrimSpace(filter.CreatedEnd) != "" {
			biz["creatd_end"] = filter.CreatedEnd
		}
		return biz, nil
	case "jst_sku_query":
		var f domain.ERPProductSearchFilter
		if err := json.Unmarshal(rawBody, &f); err != nil {
			return nil, fmt.Errorf("decode jst sku query filter: %w", err)
		}
		return buildJSTSkuQueryBizFilter(f), nil
	case "jst_sku_query_by_id":
		var m struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawBody, &m); err != nil {
			return nil, fmt.Errorf("decode jst sku query id: %w", err)
		}
		id := strings.TrimSpace(m.ID)
		if id == "" {
			return nil, fmt.Errorf("jst sku query id is empty")
		}
		return map[string]interface{}{
			"page_index": "1",
			"page_size":  "50",
			"sku_ids":    id,
		}, nil
	default:
		trimmed := strings.TrimSpace(string(rawBody))
		if trimmed == "" {
			return map[string]interface{}{}, nil
		}
		var generic map[string]interface{}
		if err := json.Unmarshal(rawBody, &generic); err != nil {
			return nil, fmt.Errorf("decode remote erp payload: %w", err)
		}
		return generic, nil
	}
}
