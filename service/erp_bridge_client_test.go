package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestERPBridgeClientSearchProductsParsesCommonEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/erp/products" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("category_id"); got != "12" {
			t.Fatalf("category_id = %s", got)
		}
		if got := r.URL.Query().Get("sku_code"); got != "CF-001" {
			t.Fatalf("sku_code = %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":            "1001",
						"sku_id":        "2002",
						"sku_code":      "CF-001",
						"name":          "ERP Product",
						"category_name": "Banner",
						"image_url":     "https://img.example.com/1.png",
						"price":         18.5,
					},
				},
				"page":      1,
				"page_size": 20,
				"total":     1,
			},
		})
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	resp, err := client.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:          "banner",
		SKUCode:    "CF-001",
		CategoryID: "12",
		Page:       1,
		PageSize:   20,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if resp.Pagination.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("SearchProducts() response = %+v", resp)
	}
	if resp.Items[0].ProductID != "1001" || resp.Items[0].SKUID != "2002" {
		t.Fatalf("SearchProducts() ids = %+v", resp.Items[0])
	}
	if resp.Items[0].Price == nil || *resp.Items[0].Price != 18.5 {
		t.Fatalf("SearchProducts() price = %+v", resp.Items[0].Price)
	}
}

func TestERPBridgeClientSearchProductsForwardsBearerTokenFromContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer session-token-1" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "1001", "name": "ERP Product"},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	ctx := domain.WithRequestBearerToken(context.Background(), "session-token-1")
	resp, err := client.SearchProducts(ctx, domain.ERPProductSearchFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ProductID != "1001" {
		t.Fatalf("SearchProducts() response = %+v", resp)
	}
}

func TestERPBridgeClientSearchProductsMergesDuplicateRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"rows": []map[string]interface{}{
					{"id": "1001", "sku_code": "CF-001"},
					{"id": "1001", "name": "ERP Product", "category_name": "Banner", "sale_price": "20.50"},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	resp, err := client.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:        "ERP Product",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("SearchProducts() items = %+v", resp.Items)
	}
	if resp.Items[0].ProductName != "ERP Product" || resp.Items[0].SKUCode != "CF-001" {
		t.Fatalf("SearchProducts() merged item = %+v", resp.Items[0])
	}
	if resp.Items[0].Price == nil || *resp.Items[0].Price != 20.5 {
		t.Fatalf("SearchProducts() merged price = %+v", resp.Items[0].Price)
	}
}

func TestERPBridgeClientSearchProductsUsesCodeLikeIDAsSKUCodeFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":            "HQT21413",
						"name":          "ERP Product",
						"category_name": "Poster",
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	resp, err := client.SearchProducts(context.Background(), domain.ERPProductSearchFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("SearchProducts() items = %+v", resp.Items)
	}
	if resp.Items[0].ProductID != "HQT21413" || resp.Items[0].SKUCode != "HQT21413" {
		t.Fatalf("SearchProducts() fallback identifiers = %+v", resp.Items[0])
	}
}

func TestERPBridgeClientParsesDetailAndCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/erp/products/1001":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"product_id":    "1001",
					"sku_code":      "CF-001",
					"product_name":  "ERP Product",
					"category_name": "Banner",
					"image":         "https://img.example.com/detail.png",
					"sale_price":    "19.80",
				},
			})
		case "/v1/erp/categories":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "10", "name": "Banner", "parent_id": "1", "level": 2},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	product, err := client.GetProductByID(context.Background(), "1001")
	if err != nil {
		t.Fatalf("GetProductByID() error = %v", err)
	}
	if product == nil || product.ProductName != "ERP Product" {
		t.Fatalf("GetProductByID() product = %+v", product)
	}
	if product.Price == nil || *product.Price != 19.8 {
		t.Fatalf("GetProductByID() price = %+v", product.Price)
	}

	categories, err := client.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(categories) != 1 || categories[0].CategoryName != "Banner" {
		t.Fatalf("ListCategories() categories = %+v", categories)
	}
}

func TestERPBridgeClientUpsertProductPostsNormalizedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/v1/erp/products/upsert" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["product_id"] != "ERP-3001" {
			t.Fatalf("payload product_id = %+v", payload["product_id"])
		}
		if payload["source"] != "task_business_info_filing" {
			t.Fatalf("payload source = %+v", payload["source"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"product": map[string]interface{}{
					"product_id":   "ERP-3001",
					"sku_code":     "CF-3001",
					"product_name": "Filed Product",
				},
				"sync_log": map[string]interface{}{
					"id":      "sync-3001",
					"status":  "accepted",
					"message": "queued",
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	result, err := client.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		ProductID:   "ERP-3001",
		SKUCode:     "CF-3001",
		ProductName: "Filed Product",
		Source:      "task_business_info_filing",
		Product: &domain.ERPProductSelectionSnapshot{
			ProductID:   "ERP-3001",
			SKUCode:     "CF-3001",
			ProductName: "Filed Product",
		},
	})
	if err != nil {
		t.Fatalf("UpsertProduct() error = %v", err)
	}
	if result.SyncLogID != "sync-3001" || result.Status != "accepted" {
		t.Fatalf("UpsertProduct() result = %+v", result)
	}
}

func TestERPBridgeClientUpsertProductAllowsEmptySuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/erp/products/upsert" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}

	result, err := client.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		ProductID: "ERP-3002",
		Product: &domain.ERPProductSelectionSnapshot{
			ProductID:   "ERP-3002",
			SKUCode:     "CF-3002",
			ProductName: "Fallback Product",
		},
	})
	if err != nil {
		t.Fatalf("UpsertProduct() error = %v", err)
	}
	if result.ProductID != "ERP-3002" || result.SKUCode != "CF-3002" {
		t.Fatalf("UpsertProduct() fallback result = %+v", result)
	}
}

func TestERPBridgeClientSyncLogsAndMutations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/erp/sync-logs":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"items": []map[string]interface{}{
						{
							"sync_log_id": "501",
							"connector":   "erp_bridge_product_shelve_batch",
							"operation":   "erp.bridge.products.shelve.batch",
							"status":      "succeeded",
						},
					},
					"page":      1,
					"page_size": 20,
					"total":     1,
				},
			})
		case "/v1/erp/sync-logs/501":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sync_log_id": "501",
					"status":      "succeeded",
				},
			})
		case "/v1/erp/products/shelve/batch":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sync_log_id": "601",
					"status":      "accepted",
					"total":       2,
					"accepted":    2,
				},
			})
		case "/v1/erp/products/unshelve/batch":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sync_log_id": "602",
					"status":      "accepted",
					"total":       2,
					"accepted":    2,
				},
			})
		case "/v1/erp/inventory/virtual-qty":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sync_log_id": "603",
					"status":      "accepted",
					"updated":     1,
					"total":       1,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewERPBridgeClient(ERPBridgeClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewERPBridgeClient() error = %v", err)
	}
	logList, err := client.ListSyncLogs(context.Background(), domain.ERPSyncLogFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListSyncLogs() error = %v", err)
	}
	if len(logList.Items) != 1 || logList.Items[0].SyncLogID != "501" {
		t.Fatalf("ListSyncLogs() items = %+v", logList.Items)
	}
	logDetail, err := client.GetSyncLogByID(context.Background(), "501")
	if err != nil {
		t.Fatalf("GetSyncLogByID() error = %v", err)
	}
	if logDetail == nil || logDetail.SyncLogID != "501" {
		t.Fatalf("GetSyncLogByID() detail = %+v", logDetail)
	}

	shelveResult, err := client.ShelveProductsBatch(context.Background(), domain.ERPProductBatchMutationPayload{
		Items: []domain.ERPProductBatchMutationItem{{ProductID: "ERP-1"}, {ProductID: "ERP-2"}},
	})
	if err != nil {
		t.Fatalf("ShelveProductsBatch() error = %v", err)
	}
	if shelveResult.SyncLogID != "601" || shelveResult.Accepted != 2 {
		t.Fatalf("ShelveProductsBatch() result = %+v", shelveResult)
	}

	unshelveResult, err := client.UnshelveProductsBatch(context.Background(), domain.ERPProductBatchMutationPayload{
		Items: []domain.ERPProductBatchMutationItem{{ProductID: "ERP-1"}, {ProductID: "ERP-2"}},
	})
	if err != nil {
		t.Fatalf("UnshelveProductsBatch() error = %v", err)
	}
	if unshelveResult.SyncLogID != "602" || unshelveResult.Accepted != 2 {
		t.Fatalf("UnshelveProductsBatch() result = %+v", unshelveResult)
	}

	qtyResult, err := client.UpdateVirtualInventory(context.Background(), domain.ERPVirtualInventoryUpdatePayload{
		Items: []domain.ERPVirtualInventoryUpdateItem{{ProductID: "ERP-1", VirtualQty: 9}},
	})
	if err != nil {
		t.Fatalf("UpdateVirtualInventory() error = %v", err)
	}
	if qtyResult.SyncLogID != "603" || qtyResult.Accepted != 1 {
		t.Fatalf("UpdateVirtualInventory() result = %+v", qtyResult)
	}
}

func TestERPBridgeServiceEnsureLocalProductCachesSnapshot(t *testing.T) {
	productRepo := &erpBridgeProductRepoStub{products: map[string]*domain.Product{}}
	svc := NewERPBridgeService(nil, productRepo, erpBridgeTxRunner{})

	product, appErr := svc.EnsureLocalProduct(context.Background(), nil, &domain.ERPProductSelectionSnapshot{
		ProductID:    "ERP-1001",
		SKUID:        "SKU-2002",
		SKUCode:      "CF-001",
		ProductName:  "ERP Product",
		CategoryName: "Banner",
		ImageURL:     "https://img.example.com/1.png",
		Price:        float64Ptr(12.3),
	})
	if appErr != nil {
		t.Fatalf("EnsureLocalProduct() error = %+v", appErr)
	}
	if product == nil || product.ID == 0 {
		t.Fatalf("EnsureLocalProduct() product = %+v", product)
	}
	if stored := productRepo.products["ERP-1001"]; stored == nil || stored.ProductName != "ERP Product" {
		t.Fatalf("EnsureLocalProduct() stored = %+v", stored)
	}
}

func TestERPBridgeServiceEnsureLocalProductMergesExistingSnapshot(t *testing.T) {
	raw, err := json.Marshal(&domain.ERPProductSelectionSnapshot{
		ProductID:    "ERP-1001",
		SKUID:        "SKU-2002",
		SKUCode:      "CF-001",
		ProductName:  "Existing Name",
		CategoryName: "Banner",
		ImageURL:     "https://img.example.com/existing.png",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	productRepo := &erpBridgeProductRepoStub{
		products: map[string]*domain.Product{
			"ERP-1001": {
				ID:           8,
				ERPProductID: "ERP-1001",
				SKUCode:      "CF-001",
				ProductName:  "Existing Name",
				Category:     "Banner",
				SpecJSON:     string(raw),
				Status:       "active",
			},
		},
		nextID: 8,
	}
	svc := NewERPBridgeService(nil, productRepo, erpBridgeTxRunner{})

	product, appErr := svc.EnsureLocalProduct(context.Background(), nil, &domain.ERPProductSelectionSnapshot{
		ProductID: "ERP-1001",
		Price:     float64Ptr(18.8),
	})
	if appErr != nil {
		t.Fatalf("EnsureLocalProduct() error = %+v", appErr)
	}
	if product == nil || product.ID != 8 {
		t.Fatalf("EnsureLocalProduct() product = %+v", product)
	}
	stored := erpProductSnapshotFromSpecJSON(productRepo.products["ERP-1001"].SpecJSON)
	if stored == nil || stored.ProductName != "Existing Name" || stored.ImageURL != "https://img.example.com/existing.png" {
		t.Fatalf("EnsureLocalProduct() merged snapshot = %+v", stored)
	}
	if stored.Price == nil || *stored.Price != 18.8 {
		t.Fatalf("EnsureLocalProduct() merged price = %+v", stored)
	}
}

func TestERPBridgeServiceSearchProductsRejectsUnknownCategory(t *testing.T) {
	client := &erpBridgeClientStub{
		searchResponses: map[string]*domain.ERPProductListResponse{
			"page=1": {
				Items:      []*domain.ERPProduct{{ProductID: "ERP-1", ProductName: "Fallback"}},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 1},
			},
		},
		categories: []*domain.ERPCategory{
			{CategoryID: "poster", CategoryName: "Poster"},
		},
	}
	svc := NewERPBridgeService(client, nil, nil)

	resp, appErr := svc.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		CategoryName: "missing-category",
		Page:         1,
		PageSize:     20,
	})
	if appErr != nil {
		t.Fatalf("SearchProducts() appErr = %+v", appErr)
	}
	if len(resp.Items) != 0 || resp.Pagination.Total != 0 {
		t.Fatalf("SearchProducts() resp = %+v", resp)
	}
}

func TestERPBridgeServiceSearchProductsRefinesCategoryAndPaginationLocally(t *testing.T) {
	client := &erpBridgeClientStub{
		searchResponses: map[string]*domain.ERPProductListResponse{
			"category_id=poster&category_name=Poster&page=1": {
				Items: []*domain.ERPProduct{
					{ProductID: "A-1", ProductName: "A1", CategoryName: ""},
					{ProductID: "B-1", ProductName: "B1", CategoryName: "Poster"},
				},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 2, Total: 2},
			},
			"category_id=poster&category_name=Poster&page=2": {
				Items: []*domain.ERPProduct{
					{ProductID: "B-2", ProductName: "B2", CategoryName: "Poster"},
				},
				Pagination: domain.PaginationMeta{Page: 2, PageSize: 2, Total: 1},
			},
		},
		categories: []*domain.ERPCategory{
			{CategoryID: "poster", CategoryName: "Poster"},
		},
	}
	svc := NewERPBridgeService(client, nil, nil)

	resp, appErr := svc.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		CategoryName: "Poster",
		Page:         1,
		PageSize:     2,
	})
	if appErr != nil {
		t.Fatalf("SearchProducts() appErr = %+v", appErr)
	}
	if len(resp.Items) != 2 || resp.Pagination.Total != 2 {
		t.Fatalf("SearchProducts() resp = %+v", resp)
	}
	if resp.Items[0].CategoryID != "poster" || resp.Items[0].CategoryCode != "poster" {
		t.Fatalf("SearchProducts() enriched category = %+v", resp.Items[0])
	}
	if resp.NormalizedFilters == nil || resp.NormalizedFilters.CategoryID != "poster" {
		t.Fatalf("SearchProducts() normalized_filters = %+v", resp.NormalizedFilters)
	}
}

func TestERPBridgeServiceSearchProductsStabilizesShortPageTotal(t *testing.T) {
	client := &erpBridgeClientStub{
		searchResponses: map[string]*domain.ERPProductListResponse{
			"page=1&q=bamboo": {
				Items: []*domain.ERPProduct{
					{ProductID: "A-1", ProductName: "A1"},
					{ProductID: "A-2", ProductName: "A2"},
					{ProductID: "A-3", ProductName: "A3"},
					{ProductID: "A-4", ProductName: "A4"},
				},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 5, Total: 5},
			},
		},
		categories: []*domain.ERPCategory{},
	}
	svc := NewERPBridgeService(client, nil, nil)

	resp, appErr := svc.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:        "bamboo",
		Page:     1,
		PageSize: 5,
	})
	if appErr != nil {
		t.Fatalf("SearchProducts() appErr = %+v", appErr)
	}
	if len(resp.Items) != 4 {
		t.Fatalf("SearchProducts() items = %+v", resp.Items)
	}
	if resp.Pagination.Total != 4 {
		t.Fatalf("SearchProducts() pagination = %+v", resp.Pagination)
	}
}

func TestERPBridgeServiceSearchProductsFallsBackWhenKeywordSearchTimesOut(t *testing.T) {
	client := &erpBridgeClientStub{
		searchResponses: map[string]*domain.ERPProductListResponse{
			"page=1": {
				Items: []*domain.ERPProduct{
					{ProductID: "SKU-1001", SKUCode: "SKU-1001", ProductName: "SKU board"},
					{ProductID: "OTHER-1", SKUCode: "OTHER-1", ProductName: "Poster"},
				},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 2},
			},
		},
		searchErrs: map[string]error{
			"page=1&q=sku": &erpBridgeRequestError{
				URL:       "http://127.0.0.1:8081/v1/erp/products?page=1&page_size=20&q=sku",
				Duration:  15 * time.Second,
				Timeout:   true,
				Retryable: true,
				Cause:     context.DeadlineExceeded,
			},
		},
		categories: []*domain.ERPCategory{},
	}
	svc := NewERPBridgeService(client, nil, nil)

	resp, appErr := svc.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:        "sku",
		Page:     1,
		PageSize: 20,
	})
	if appErr != nil {
		t.Fatalf("SearchProducts() appErr = %+v", appErr)
	}
	if len(resp.Items) != 1 || resp.Items[0].SKUCode != "SKU-1001" {
		t.Fatalf("SearchProducts() fallback items = %+v", resp.Items)
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("SearchProducts() fallback pagination = %+v", resp.Pagination)
	}
}

func TestERPBridgeServiceSearchProductsFallsBackToLocalReplicaWhenRemoteKeywordEmpty(t *testing.T) {
	client := &erpBridgeClientStub{
		searchResponses: map[string]*domain.ERPProductListResponse{
			"page=1&q=常规kt板/唱片牌/毕业快乐特别鸣谢一起成长的你们/直径30cm": {
				Items:      []*domain.ERPProduct{},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0},
			},
		},
		categories: []*domain.ERPCategory{},
	}
	productRepo := &erpBridgeProductRepoStub{
		products: map[string]*domain.Product{
			"HSC06315": {
				ID:           45334310,
				ERPProductID: "HSC06315",
				SKUCode:      "HSC06315",
				ProductName:  "常规kt板/唱片牌/毕业快乐特别鸣谢一起成长的你们/直径30cm",
				Category:     "常规KT板",
			},
		},
	}
	svc := NewERPBridgeService(client, productRepo, nil)

	resp, appErr := svc.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:        "常规kt板/唱片牌/毕业快乐特别鸣谢一起成长的你们/直径30cm",
		Page:     1,
		PageSize: 20,
	})
	if appErr != nil {
		t.Fatalf("SearchProducts() appErr = %+v", appErr)
	}
	if len(resp.Items) != 1 || resp.Items[0].SKUCode != "HSC06315" {
		t.Fatalf("SearchProducts() local fallback items = %+v", resp.Items)
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("SearchProducts() local fallback pagination = %+v", resp.Pagination)
	}
	if resp.NormalizedFilters == nil || resp.NormalizedFilters.QueryMode != "keyword" {
		t.Fatalf("SearchProducts() normalized filters = %+v", resp.NormalizedFilters)
	}
}

func TestERPBridgeServiceGetProductByIDFallsBackToSearchReference(t *testing.T) {
	const productRef = "NAME-REF-1"
	client := &erpBridgeClientStub{
		getProductErrs: map[string]error{
			productRef: &erpBridgeHTTPError{StatusCode: http.StatusNotFound},
		},
		searchResponses: map[string]*domain.ERPProductListResponse{
			"page=1&q=" + productRef: {
				Items: []*domain.ERPProduct{
					{ProductID: productRef, ProductName: productRef},
				},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 1},
			},
			"page=1&q=" + productRef + "&sku_code=" + productRef: {
				Items:      []*domain.ERPProduct{},
				Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0},
			},
		},
		categories: []*domain.ERPCategory{},
	}
	svc := NewERPBridgeService(client, nil, nil)

	product, appErr := svc.GetProductByID(context.Background(), productRef)
	if appErr != nil {
		t.Fatalf("GetProductByID() appErr = %+v", appErr)
	}
	if product == nil || product.ProductID != productRef {
		t.Fatalf("GetProductByID() product = %+v", product)
	}
}

type erpBridgeTx struct{}

func (erpBridgeTx) IsTx() {}

type erpBridgeTxRunner struct{}

func (erpBridgeTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(erpBridgeTx{})
}

type erpBridgeProductRepoStub struct {
	products map[string]*domain.Product
	nextID   int64
}

type erpBridgeClientStub struct {
	searchResponses map[string]*domain.ERPProductListResponse
	searchErrs      map[string]error
	searchErr       error
	getProducts     map[string]*domain.ERPProduct
	getProductErrs  map[string]error
	categories      []*domain.ERPCategory
	categoryErr     error
}

func (s *erpBridgeClientStub) SearchProducts(_ context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	key := erpBridgeClientStubSearchKey(filter)
	if err, ok := s.searchErrs[key]; ok {
		return nil, err
	}
	if resp, ok := s.searchResponses[key]; ok {
		return resp, nil
	}
	return &domain.ERPProductListResponse{
		Items:      []*domain.ERPProduct{},
		Pagination: domain.PaginationMeta{Page: filter.Page, PageSize: filter.PageSize},
	}, nil
}

func (s *erpBridgeClientStub) GetProductByID(_ context.Context, id string) (*domain.ERPProduct, error) {
	if err, ok := s.getProductErrs[id]; ok {
		return nil, err
	}
	if product, ok := s.getProducts[id]; ok {
		return product, nil
	}
	return nil, nil
}

func (s *erpBridgeClientStub) ListCategories(context.Context) ([]*domain.ERPCategory, error) {
	if s.categoryErr != nil {
		return nil, s.categoryErr
	}
	return s.categories, nil
}

func (s *erpBridgeClientStub) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	return &domain.ERPSyncLogListResponse{
		Items:      []*domain.ERPSyncLog{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0},
	}, nil
}

func (s *erpBridgeClientStub) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, error) {
	return nil, nil
}

func (s *erpBridgeClientStub) UpsertProduct(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	return &domain.ERPProductUpsertResult{}, nil
}

func (s *erpBridgeClientStub) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	return &domain.ERPItemStyleUpdateResult{Status: "accepted"}, nil
}

func (s *erpBridgeClientStub) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return &domain.ERPProductBatchMutationResult{Action: "shelve", Status: "accepted"}, nil
}

func (s *erpBridgeClientStub) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return &domain.ERPProductBatchMutationResult{Action: "unshelve", Status: "accepted"}, nil
}

func (s *erpBridgeClientStub) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	return &domain.ERPVirtualInventoryUpdateResult{Status: "accepted"}, nil
}

func (s *erpBridgeClientStub) GetCompanyUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	return &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}, nil
}

func erpBridgeClientStubSearchKey(filter domain.ERPProductSearchFilter) string {
	parts := make([]string, 0, 5)
	if filter.CategoryID != "" {
		parts = append(parts, "category_id="+filter.CategoryID)
	}
	if filter.CategoryName != "" {
		parts = append(parts, "category_name="+filter.CategoryName)
	}
	parts = append(parts, "page="+strconv.Itoa(filter.Page))
	if filter.Q != "" {
		parts = append(parts, "q="+filter.Q)
	}
	if filter.SKUCode != "" {
		parts = append(parts, "sku_code="+filter.SKUCode)
	}
	return strings.Join(parts, "&")
}

func (r *erpBridgeProductRepoStub) GetByID(_ context.Context, id int64) (*domain.Product, error) {
	for _, product := range r.products {
		if product.ID == id {
			copyProduct := *product
			return &copyProduct, nil
		}
	}
	return nil, nil
}

func (r *erpBridgeProductRepoStub) GetByERPProductID(_ context.Context, erpProductID string) (*domain.Product, error) {
	if product, ok := r.products[erpProductID]; ok {
		copyProduct := *product
		return &copyProduct, nil
	}
	return nil, nil
}

func (r *erpBridgeProductRepoStub) Search(_ context.Context, filter repo.ProductSearchFilter) ([]*domain.Product, int64, error) {
	if r.products == nil {
		return []*domain.Product{}, 0, nil
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	category := strings.ToLower(strings.TrimSpace(filter.Category))
	matches := make([]*domain.Product, 0, len(r.products))
	for _, product := range r.products {
		if product == nil {
			continue
		}
		if keyword != "" &&
			!strings.Contains(strings.ToLower(product.ProductName), keyword) &&
			!strings.Contains(strings.ToLower(product.SKUCode), keyword) {
			continue
		}
		if category != "" && !strings.Contains(strings.ToLower(product.Category), category) {
			continue
		}
		copied := *product
		matches = append(matches, &copied)
	}
	total := int64(len(matches))
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(matches) {
		return []*domain.Product{}, total, nil
	}
	end := start + pageSize
	if end > len(matches) {
		end = len(matches)
	}
	return matches[start:end], total, nil
}

func (r *erpBridgeProductRepoStub) ListIIDs(_ context.Context, filter repo.ProductIIDListFilter) ([]*domain.ERPIIDOption, int64, error) {
	if r.products == nil {
		return []*domain.ERPIIDOption{}, 0, nil
	}
	q := strings.ToLower(strings.TrimSpace(filter.Q))
	counts := map[string]int64{}
	for _, product := range r.products {
		if product == nil {
			continue
		}
		snapshot := erpProductSnapshotFromSpecJSON(product.SpecJSON)
		iid := ""
		if snapshot != nil {
			iid = strings.TrimSpace(snapshot.IID)
		}
		if iid == "" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(iid), q) && !strings.Contains(strings.ToLower(product.ProductName), q) {
			continue
		}
		counts[iid]++
	}
	items := make([]*domain.ERPIIDOption, 0, len(counts))
	for iid, count := range counts {
		items = append(items, &domain.ERPIIDOption{IID: iid, Label: iid, ProductCount: count})
	}
	return items, int64(len(items)), nil
}

func (r *erpBridgeProductRepoStub) UpsertBatch(_ context.Context, _ repo.Tx, products []*domain.Product) (int64, error) {
	for _, product := range products {
		copied := *product
		if existing, ok := r.products[product.ERPProductID]; ok {
			copied.ID = existing.ID
		} else {
			r.nextID++
			copied.ID = r.nextID
		}
		r.products[product.ERPProductID] = &copied
	}
	return int64(len(products)), nil
}
