package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

func TestERPBridgeHandlerSearchProductsReturnsPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewERPBridgeHandler(&erpBridgeServiceStub{
		searchResponse: &domain.ERPProductListResponse{
			Items: []*domain.ERPProduct{
				{
					ProductID:   "ERP-1",
					SKUID:       "SKU-1",
					SKUCode:     "CF-001",
					ProductName: "定制车缝旗帜",
				},
			},
			Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 1},
			NormalizedFilters: &domain.ERPProductSearchFilter{
				Q:         "定制车缝",
				Page:      1,
				PageSize:  20,
				QueryMode: "keyword",
			},
		},
	})
	router.GET("/v1/erp/products", handler.SearchProducts)

	req := httptest.NewRequest(http.MethodGet, "/v1/erp/products?q=%E5%AE%9A%E5%88%B6%E8%BD%A6%E7%BC%9D&page=1&page_size=20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/erp/products code = %d, want 200", rec.Code)
	}
	var resp struct {
		Data              []domain.ERPProduct            `json:"data"`
		Pagination        domain.PaginationMeta          `json:"pagination"`
		NormalizedFilters *domain.ERPProductSearchFilter `json:"normalized_filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ProductID != "ERP-1" {
		t.Fatalf("response data = %+v", resp.Data)
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("pagination = %+v", resp.Pagination)
	}
	if resp.NormalizedFilters == nil || resp.NormalizedFilters.QueryMode != "keyword" {
		t.Fatalf("normalized_filters = %+v", resp.NormalizedFilters)
	}
}

func TestERPBridgeHandlerSearchProductsRejectsInvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewERPBridgeHandler(&erpBridgeServiceStub{})
	router.GET("/v1/erp/products", handler.SearchProducts)

	req := httptest.NewRequest(http.MethodGet, "/v1/erp/products?page=x", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/erp/products invalid page code = %d, want 400", rec.Code)
	}
}

func TestERPBridgeHandlerGetProductByIDReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewERPBridgeHandler(&erpBridgeServiceStub{appErr: domain.ErrNotFound})
	router.GET("/v1/erp/products/*id", handler.GetProductByID)

	req := httptest.NewRequest(http.MethodGet, "/v1/erp/products/404", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/erp/products/:id code = %d, want 404", rec.Code)
	}
}

func TestERPBridgeHandlerGetProductByIDAcceptsSlashContainingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewERPBridgeHandler(&erpBridgeServiceStub{
		product: &domain.ERPProduct{ProductID: "name/with/slash", ProductName: "Slash Product"},
	})
	router.GET("/v1/erp/products/*id", handler.GetProductByID)

	req := httptest.NewRequest(http.MethodGet, "/v1/erp/products/name%2Fwith%2Fslash", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/erp/products/*id code = %d, want 200", rec.Code)
	}
}

type erpBridgeServiceStub struct {
	searchResponse *domain.ERPProductListResponse
	product        *domain.ERPProduct
	categories     []*domain.ERPCategory
	appErr         *domain.AppError
}

func (s *erpBridgeServiceStub) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	return s.searchResponse, s.appErr
}

func (s *erpBridgeServiceStub) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	return s.product, s.appErr
}

func (s *erpBridgeServiceStub) ListCategories(context.Context) ([]*domain.ERPCategory, *domain.AppError) {
	return s.categories, s.appErr
}

func (s *erpBridgeServiceStub) ListWarehouses(context.Context) ([]domain.ERPWarehouse, *domain.AppError) {
	return []domain.ERPWarehouse{}, s.appErr
}

func (s *erpBridgeServiceStub) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError) {
	return &domain.ERPSyncLogListResponse{Items: []*domain.ERPSyncLog{}, Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0}}, s.appErr
}

func (s *erpBridgeServiceStub) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) EnsureLocalProduct(context.Context, repo.Tx, *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) UpsertProduct(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError) {
	return nil, s.appErr
}

func (s *erpBridgeServiceStub) ListJSTUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError) {
	return &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}, nil
}

var _ service.ERPBridgeService = (*erpBridgeServiceStub)(nil)
