package workers

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

func TestProductManagementSyncWorkerDefaultsAreConservative(t *testing.T) {
	worker := NewProductManagementSyncWorker(productManagementServiceStub{}, zap.NewNop(), 0, 0)
	if worker.interval != 30*time.Second {
		t.Fatalf("interval = %s, want 30s", worker.interval)
	}
	if worker.limit != 2 {
		t.Fatalf("limit = %d, want 2", worker.limit)
	}
}

func TestGroupProductManagementWorkerUsesResponsiveBatch(t *testing.T) {
	worker := NewProductManagementSyncWorker(productManagementServiceStub{}, zap.NewNop(), 15*time.Second, 8)
	if worker.interval != 15*time.Second {
		t.Fatalf("interval = %s, want 15s", worker.interval)
	}
	if worker.limit != 8 {
		t.Fatalf("limit = %d, want 8", worker.limit)
	}
}

func TestGroupShouldStartProductManagementSyncWorkerRequiresERPEnabled(t *testing.T) {
	svc := productManagementServiceStub{}
	if (&Group{erpEnabled: false, productMgmt: svc}).shouldStartProductManagementSyncWorker() {
		t.Fatal("product management worker must not start when ERP sync is disabled")
	}
	if (&Group{erpEnabled: true}).shouldStartProductManagementSyncWorker() {
		t.Fatal("product management worker must not start without service")
	}
	if !(&Group{erpEnabled: true, productMgmt: svc}).shouldStartProductManagementSyncWorker() {
		t.Fatal("product management worker should start when ERP sync is enabled and service exists")
	}
}

func TestGroupShouldStartSKUComboSyncWorkerRequiresERPEnabled(t *testing.T) {
	svc := skuComboSyncServiceStub{}
	if (&Group{erpEnabled: false, skuComboSync: svc}).shouldStartSKUComboSyncWorker() {
		t.Fatal("sku combo worker must not start when ERP sync is disabled")
	}
	if (&Group{erpEnabled: true}).shouldStartSKUComboSyncWorker() {
		t.Fatal("sku combo worker must not start without service")
	}
	if !(&Group{erpEnabled: true, skuComboSync: svc}).shouldStartSKUComboSyncWorker() {
		t.Fatal("sku combo worker should start when ERP sync is enabled and service exists")
	}
}

type skuComboSyncServiceStub struct{}

func (skuComboSyncServiceStub) ProcessNextPage(context.Context) (int, *domain.AppError) {
	return 0, nil
}

type productManagementServiceStub struct{}

func (productManagementServiceStub) List(context.Context, repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (productManagementServiceStub) ListComboTree(context.Context, repo.ProductManagementListFilter) (*domain.ProductManagementComboTreeResponse, *domain.AppError) {
	return &domain.ProductManagementComboTreeResponse{}, nil
}

func (productManagementServiceStub) GetByTaskID(context.Context, int64) ([]*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) ListImageCandidates(context.Context, domain.RequestActor, int64) ([]*domain.ProductManagementImageCandidate, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) ReparseImage(context.Context, domain.RequestActor, int64) (*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) SetManualImage(context.Context, domain.RequestActor, int64, int64) (*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) RequestSync(context.Context, domain.RequestActor, int64, bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) RequestBaseSync(context.Context, domain.RequestActor, int64, bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) RequestImageSync(context.Context, domain.RequestActor, int64, bool) (*domain.ProductManagementRecord, *domain.AppError) {
	return nil, nil
}

func (productManagementServiceStub) AutoSyncImagesAfterTaskClosed(context.Context, int64, int64) *domain.AppError {
	return nil
}

func (productManagementServiceStub) RefreshReadModelNow(context.Context) *domain.AppError {
	return nil
}

func (productManagementServiceStub) QueuePendingBaseSyncForTask(context.Context, int64) (int, *domain.AppError) {
	return 0, nil
}

func (productManagementServiceStub) ProcessQueuedERPSync(context.Context, int) (int, *domain.AppError) {
	return 0, nil
}
