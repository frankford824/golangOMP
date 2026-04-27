//go:build integration

package erp_product

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

// SA-C-I9 — GET /v1/erp/products/by-code
// Asserts: the thin by-code facade maps ERP success, not-found, and upstream
// failures to the MAIN response codes without reaching a real ERP upstream.
func TestSACI9_GetERPProductByCode_404OrUpstream502OrSuccess(t *testing.T) {
	ctx := context.Background()
	successProduct := &domain.ERPProduct{
		ProductID:   "erp-1001",
		SKUID:       "sku-1001",
		SKUCode:     "SAC-I9-CODE",
		ProductName: "SA-C I9 Product",
	}
	snapshot, appErr := NewService(saCERPBridgeStub{product: successProduct}).LookupByCode(ctx, " SAC-I9-CODE ")
	if appErr != nil {
		t.Fatalf("LookupByCode success appErr=%+v", appErr)
	}
	if snapshot == nil || snapshot.Code != "SAC-I9-CODE" || snapshot.ProductName != successProduct.ProductName {
		t.Fatalf("success snapshot=%+v", snapshot)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(snapshot.Snapshot, &raw); err != nil {
		t.Fatalf("snapshot json: %v", err)
	}
	if raw["sku_code"] != successProduct.SKUCode {
		t.Fatalf("snapshot sku_code=%v want %s", raw["sku_code"], successProduct.SKUCode)
	}

	_, appErr = NewService(saCERPBridgeStub{appErr: domain.NewAppError(domain.ErrCodeNotFound, "missing", nil)}).LookupByCode(ctx, "missing")
	if appErr == nil || appErr.Code != "erp_product_not_found" {
		t.Fatalf("not found appErr=%+v want erp_product_not_found", appErr)
	}

	_, appErr = NewService(saCERPBridgeStub{appErr: domain.NewAppError(domain.ErrCodeInternalError, "upstream", map[string]interface{}{"status": 502})}).LookupByCode(ctx, "timeout")
	if appErr == nil || appErr.Code != "erp_upstream_failure" {
		t.Fatalf("upstream appErr=%+v want erp_upstream_failure", appErr)
	}
}

type saCERPBridgeStub struct {
	product *domain.ERPProduct
	appErr  *domain.AppError
}

func (s saCERPBridgeStub) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	if s.appErr != nil {
		return nil, s.appErr
	}
	return s.product, nil
}

func (s saCERPBridgeStub) ListCategories(context.Context) ([]*domain.ERPCategory, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) ListWarehouses(context.Context) ([]domain.ERPWarehouse, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) EnsureLocalProduct(context.Context, repo.Tx, *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) UpsertProduct(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}

func (s saCERPBridgeStub) ListJSTUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInternalError, "not implemented in test stub", nil)
}
