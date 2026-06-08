package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

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
	bridge := &productManagementERPBridgeCapture{
		readbackProduct: &domain.ERPProduct{
			SKUCode:  "CGK000181",
			ImageURL: "https://images-erp.sursung.com/prod/erp/ItemSku/CGK000181.jpg",
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

type productManagementERPBridgeCapture struct {
	payload         domain.ERPProductUpsertPayload
	readbackProduct *domain.ERPProduct
	readbackErr     *domain.AppError
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
