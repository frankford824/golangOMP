package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestERPSyncServiceRunManualUpsertsNewProducts(t *testing.T) {
	ctx := context.Background()
	stubPath := writeERPStubFile(t, []map[string]interface{}{
		{
			"erp_product_id":    "ERP-1",
			"sku_code":          "SKU-1",
			"product_name":      "Product 1",
			"category":          "poster",
			"spec_json":         map[string]interface{}{"size": "A4"},
			"status":            "active",
			"source_updated_at": "2026-03-09T00:00:00Z",
		},
	})

	productRepo := newERPSyncProductRepo()
	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(productRepo, runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(stubPath), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         stubPath,
		Timeout:          30 * time.Second,
	})

	result, appErr := svc.RunManual(ctx)
	if appErr != nil {
		t.Fatalf("RunManual() unexpected error: %+v", appErr)
	}
	if result.Status != domain.ERPSyncStatusSuccess {
		t.Fatalf("RunManual() status = %s, want success", result.Status)
	}
	if got := productRepo.products["ERP-1"]; got == nil || got.ProductName != "Product 1" {
		t.Fatalf("RunManual() missing upserted product: %+v", got)
	}
	if runRepo.latestRun == nil || runRepo.latestRun.TotalUpserted != 1 {
		t.Fatalf("RunManual() latest run = %+v, want total_upserted=1", runRepo.latestRun)
	}
}

func TestERPSyncServiceRunManualUpdatesExistingProducts(t *testing.T) {
	ctx := context.Background()
	stubPath := writeERPStubFile(t, []map[string]interface{}{
		{
			"erp_product_id": "ERP-2",
			"sku_code":       "SKU-2-NEW",
			"product_name":   "Updated Name",
			"category":       "banner",
			"spec_json":      map[string]interface{}{"color": "red"},
			"status":         "active",
		},
	})

	productRepo := newERPSyncProductRepo()
	productRepo.products["ERP-2"] = &domain.Product{
		ERPProductID: "ERP-2",
		SKUCode:      "SKU-2-OLD",
		ProductName:  "Old Name",
		Category:     "old",
		SpecJSON:     "{}",
		Status:       "inactive",
	}
	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(productRepo, runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(stubPath), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         stubPath,
		Timeout:          30 * time.Second,
	})

	result, appErr := svc.RunManual(ctx)
	if appErr != nil {
		t.Fatalf("RunManual() unexpected error: %+v", appErr)
	}
	if result.TotalUpserted != 1 {
		t.Fatalf("RunManual() total_upserted = %d, want 1", result.TotalUpserted)
	}
	got := productRepo.products["ERP-2"]
	if got.SKUCode != "SKU-2-NEW" || got.ProductName != "Updated Name" {
		t.Fatalf("RunManual() product = %+v, want updated fields", got)
	}
}

func TestERPSyncServiceRunManualMissingStubReturnsNoop(t *testing.T) {
	ctx := context.Background()
	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(newERPSyncProductRepo(), runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(filepath.Join(t.TempDir(), "missing.json")), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         "missing.json",
		Timeout:          30 * time.Second,
	})

	result, appErr := svc.RunManual(ctx)
	if appErr != nil {
		t.Fatalf("RunManual() unexpected error: %+v", appErr)
	}
	if result.Status != domain.ERPSyncStatusNoop {
		t.Fatalf("RunManual() status = %s, want noop", result.Status)
	}
	if runRepo.latestRun == nil || runRepo.latestRun.Status != domain.ERPSyncStatusNoop {
		t.Fatalf("RunManual() latest run = %+v, want noop", runRepo.latestRun)
	}
}

func TestERPSyncServiceRunManualInvalidStubReturnsFailed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	stubPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(stubPath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(newERPSyncProductRepo(), runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(stubPath), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         stubPath,
		Timeout:          30 * time.Second,
	})

	result, appErr := svc.RunManual(ctx)
	if appErr != nil {
		t.Fatalf("RunManual() unexpected error: %+v", appErr)
	}
	if result.Status != domain.ERPSyncStatusFailed {
		t.Fatalf("RunManual() status = %s, want failed", result.Status)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Fatalf("RunManual() error_message = %+v, want non-empty", result.ErrorMessage)
	}
}

func TestERPSyncServiceGetStatusResolvesStubFile(t *testing.T) {
	ctx := context.Background()
	stubPath := writeERPStubFile(t, []map[string]interface{}{
		{
			"erp_product_id": "ERP-3",
			"sku_code":       "SKU-3",
			"product_name":   "Product 3",
		},
	})
	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(newERPSyncProductRepo(), runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(stubPath), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         stubPath,
		Timeout:          30 * time.Second,
	})

	status, appErr := svc.GetStatus(ctx)
	if appErr != nil {
		t.Fatalf("GetStatus() unexpected error: %+v", appErr)
	}
	if status.ResolvedStubFile != stubPath {
		t.Fatalf("GetStatus() resolved_stub_file = %q, want %q", status.ResolvedStubFile, stubPath)
	}
	if !status.StubFileExists {
		t.Fatalf("GetStatus() stub_file_exists = false, want true")
	}
}

func TestERPSyncServiceGetStatusMarksMissingStubFile(t *testing.T) {
	ctx := context.Background()
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	runRepo := &erpSyncRunRepoStub{}
	svc := NewERPSyncService(newERPSyncProductRepo(), runRepo, erpSyncTxRunner{}, NewStubERPProductProvider(missingPath), ERPSyncOptions{
		SchedulerEnabled: true,
		Interval:         5 * time.Minute,
		SourceMode:       "stub",
		StubFile:         missingPath,
		Timeout:          30 * time.Second,
	})

	status, appErr := svc.GetStatus(ctx)
	if appErr != nil {
		t.Fatalf("GetStatus() unexpected error: %+v", appErr)
	}
	if status.ResolvedStubFile != missingPath {
		t.Fatalf("GetStatus() resolved_stub_file = %q, want %q", status.ResolvedStubFile, missingPath)
	}
	if status.StubFileExists {
		t.Fatalf("GetStatus() stub_file_exists = true, want false")
	}
}

type erpSyncTx struct{}

func (erpSyncTx) IsTx() {}

type erpSyncTxRunner struct{}

func (erpSyncTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(erpSyncTx{})
}

type erpSyncProductRepoStub struct {
	products map[string]*domain.Product
}

func newERPSyncProductRepo() *erpSyncProductRepoStub {
	return &erpSyncProductRepoStub{products: map[string]*domain.Product{}}
}

func (r *erpSyncProductRepoStub) GetByID(_ context.Context, _ int64) (*domain.Product, error) {
	return nil, nil
}

func (r *erpSyncProductRepoStub) GetByERPProductID(_ context.Context, erpProductID string) (*domain.Product, error) {
	return r.products[erpProductID], nil
}

func (r *erpSyncProductRepoStub) Search(_ context.Context, _ repo.ProductSearchFilter) ([]*domain.Product, int64, error) {
	return []*domain.Product{}, 0, nil
}

func (r *erpSyncProductRepoStub) ListIIDs(context.Context, repo.ProductIIDListFilter) ([]*domain.ERPIIDOption, int64, error) {
	return []*domain.ERPIIDOption{}, 0, nil
}

func (r *erpSyncProductRepoStub) UpsertBatch(_ context.Context, _ repo.Tx, products []*domain.Product) (int64, error) {
	for _, product := range products {
		copied := *product
		r.products[product.ERPProductID] = &copied
	}
	return int64(len(products)), nil
}

type erpSyncRunRepoStub struct {
	latestRun *domain.ERPSyncRun
}

func (r *erpSyncRunRepoStub) Create(_ context.Context, _ repo.Tx, run *domain.ERPSyncRun) (int64, error) {
	copied := *run
	r.latestRun = &copied
	return 1, nil
}

func (r *erpSyncRunRepoStub) GetLatest(_ context.Context) (*domain.ERPSyncRun, error) {
	return r.latestRun, nil
}

func writeERPStubFile(t *testing.T, records []map[string]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "erp_stub.json")
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
