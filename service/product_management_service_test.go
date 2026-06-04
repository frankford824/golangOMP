package service

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"workflow/domain"
	"workflow/repo"
)

func TestProductManagementSyncRecordToERPUsesShortERPShortName(t *testing.T) {
	assetID := int64(7301)
	versionID := int64(4395)
	asset := erpImageProxyTestAsset(versionID, "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/image.jpg")
	asset.AssetID = &assetID
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	bridge := &productManagementERPBridgeCapture{}
	svc := &productManagementService{
		assetSearch: productManagementAssetSearchStub{
			versionRow: &repo.TaskAssetSearchRow{Asset: asset},
		},
		imageProxy: signer,
		erpBridge:  bridge,
		now:        time.Now,
	}

	productName := strings.Repeat("长", 45)
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
	if !utf8.ValidString(bridge.payload.ShortName) {
		t.Fatalf("ShortName is invalid UTF-8: %q", bridge.payload.ShortName)
	}
	if got := len(bridge.payload.ShortName); got > ERPProductShortNameMaxBytes {
		t.Fatalf("ShortName byte length = %d, want <= %d: %q", got, ERPProductShortNameMaxBytes, bridge.payload.ShortName)
	}
	if bridge.payload.ProductShortName != bridge.payload.ShortName {
		t.Fatalf("ProductShortName = %q, want ShortName %q", bridge.payload.ProductShortName, bridge.payload.ShortName)
	}
}

type productManagementERPBridgeCapture struct {
	payload domain.ERPProductUpsertPayload
}

func (s *productManagementERPBridgeCapture) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) ListIIDs(context.Context, domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	return nil, nil
}

func (s *productManagementERPBridgeCapture) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	return nil, nil
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
	return &domain.ERPProductUpsertResult{Status: "accepted"}, nil
}

func (s *productManagementERPBridgeCapture) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	return nil, nil
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
