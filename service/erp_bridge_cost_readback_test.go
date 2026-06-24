package service

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"workflow/domain"
)

type erpBridgeReadbackSequenceClient struct {
	mu sync.Mutex

	upsertCalls int
	getCalls    map[string]int
	getSteps    map[string][]erpBridgeReadbackStep
}

type erpBridgeReadbackStep struct {
	product *domain.ERPProduct
	err     error
}

func (c *erpBridgeReadbackSequenceClient) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	return &domain.ERPProductListResponse{
		Items:      []*domain.ERPProduct{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0},
	}, nil
}

func (c *erpBridgeReadbackSequenceClient) GetProductByID(_ context.Context, id string) (*domain.ERPProduct, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getCalls == nil {
		c.getCalls = map[string]int{}
	}
	if c.getSteps == nil {
		c.getSteps = map[string][]erpBridgeReadbackStep{}
	}
	idx := c.getCalls[id]
	c.getCalls[id] = idx + 1
	steps := c.getSteps[id]
	if idx >= len(steps) {
		return nil, &erpBridgeHTTPError{StatusCode: http.StatusNotFound}
	}
	step := steps[idx]
	return step.product, step.err
}

func (c *erpBridgeReadbackSequenceClient) QueryCombineSKUs(context.Context, domain.JSTCombineSKUFilter) (*domain.JSTCombineSKUListResponse, error) {
	return &domain.JSTCombineSKUListResponse{
		Items:      []domain.JSTCombineSKUItem{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 50, Total: 0},
	}, nil
}

func (c *erpBridgeReadbackSequenceClient) ListCategories(context.Context) ([]*domain.ERPCategory, error) {
	return []*domain.ERPCategory{}, nil
}

func (c *erpBridgeReadbackSequenceClient) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	return &domain.ERPSyncLogListResponse{
		Items:      []*domain.ERPSyncLog{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0},
	}, nil
}

func (c *erpBridgeReadbackSequenceClient) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, error) {
	return nil, nil
}

func (c *erpBridgeReadbackSequenceClient) QueryOrderActionLogs(context.Context, domain.ERPOrderActionLogFilter) (*domain.ERPOrderActionLogListResponse, error) {
	return &domain.ERPOrderActionLogListResponse{
		Items:      []*domain.ERPOrderActionLog{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 30, Total: 0},
	}, nil
}

func (c *erpBridgeReadbackSequenceClient) UpsertProduct(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	c.mu.Lock()
	c.upsertCalls++
	c.mu.Unlock()
	return &domain.ERPProductUpsertResult{Status: "accepted"}, nil
}

func (c *erpBridgeReadbackSequenceClient) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	return &domain.ERPItemStyleUpdateResult{Status: "accepted"}, nil
}

func (c *erpBridgeReadbackSequenceClient) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return &domain.ERPProductBatchMutationResult{Action: "shelve", Status: "accepted"}, nil
}

func (c *erpBridgeReadbackSequenceClient) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return &domain.ERPProductBatchMutationResult{Action: "unshelve", Status: "accepted"}, nil
}

func (c *erpBridgeReadbackSequenceClient) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	return &domain.ERPVirtualInventoryUpdateResult{Status: "accepted"}, nil
}

func (c *erpBridgeReadbackSequenceClient) GetCompanyUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	return &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}, nil
}

func TestERPBridgeServiceUpsertProductCostReadback404ThenMatched(t *testing.T) {
	erpBridgeCostReadbackSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostReadbackSleep = time.Sleep })

	const skuID = "CGP000017"
	expected := 9.9
	client := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			skuID: {
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{product: &domain.ERPProduct{ProductID: skuID, SKUID: skuID, CostPrice: float64Ptr(expected)}},
			},
		},
	}
	svc := NewERPBridgeService(client, nil, nil)

	result, appErr := svc.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		SKUID:     skuID,
		CostPrice: float64Ptr(expected),
	})
	if appErr != nil {
		t.Fatalf("UpsertProduct() appErr = %+v", appErr)
	}
	if result == nil || result.CostVerification == nil {
		t.Fatalf("missing cost verification: %+v", result)
	}
	if result.CostVerification.Status != erpBridgeCostVerificationStatusMatched {
		t.Fatalf("status = %s, want matched", result.CostVerification.Status)
	}
	client.mu.Lock()
	upserts := client.upsertCalls
	gets := client.getCalls[skuID]
	client.mu.Unlock()
	if upserts != 1 {
		t.Fatalf("upsert calls = %d, want 1", upserts)
	}
	if gets != 3 {
		t.Fatalf("readback get calls = %d, want 3", gets)
	}
}

func TestERPBridgeServiceUpsertProductCostReadback404Exhausted(t *testing.T) {
	erpBridgeCostReadbackSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostReadbackSleep = time.Sleep })

	const skuID = "CGP000018"
	client := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			skuID: {
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
			},
		},
	}
	svc := NewERPBridgeService(client, nil, nil)

	result, appErr := svc.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		SKUID:     skuID,
		CostPrice: float64Ptr(9.9),
	})
	if appErr != nil {
		t.Fatalf("UpsertProduct() appErr = %+v", appErr)
	}
	if result.CostVerification.Status != erpBridgeCostVerificationStatusReadbackNotFound {
		t.Fatalf("status = %s, want readback_not_found", result.CostVerification.Status)
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, 1); failure != "" {
		t.Fatalf("failure = %q, want empty because readback_not_found is pending confirmation", failure)
	}
	if got := erpBridgeCostVerificationPendingMessage(result); got != "ERP已提交，等待系统回查确认" {
		t.Fatalf("pending message = %q", got)
	}
	if client.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", client.upsertCalls)
	}
}

func TestERPBridgeServiceUpsertProductCostReadback404ThenMismatchTriggersMismatchOnly(t *testing.T) {
	erpBridgeCostReadbackSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostReadbackSleep = time.Sleep })

	const skuID = "CGP000019"
	expected := 9.9
	actual := 1.2
	client := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			skuID: {
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{product: &domain.ERPProduct{ProductID: skuID, SKUID: skuID, CostPrice: float64Ptr(actual)}},
			},
		},
	}
	svc := NewERPBridgeService(client, nil, nil)

	result, appErr := svc.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		SKUID:     skuID,
		CostPrice: float64Ptr(expected),
	})
	if appErr != nil {
		t.Fatalf("UpsertProduct() appErr = %+v", appErr)
	}
	if result.CostVerification.Status != erpBridgeCostVerificationStatusMismatched {
		t.Fatalf("status = %s, want mismatched", result.CostVerification.Status)
	}
	if client.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1 (readback retry must not upsert again)", client.upsertCalls)
	}
}

func TestERPBridgeServiceUpsertProductUpsertFailureStillFails(t *testing.T) {
	client := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			"SKU-FAIL": {{product: &domain.ERPProduct{SKUID: "SKU-FAIL", CostPrice: float64Ptr(1)}}},
		},
	}
	clientWithUpsertErr := &erpBridgeUpsertFailureClient{inner: client, upsertErr: &erpBridgeHTTPError{StatusCode: http.StatusBadGateway}}
	svc := NewERPBridgeService(clientWithUpsertErr, nil, nil)

	_, appErr := svc.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		SKUID:     "SKU-FAIL",
		CostPrice: float64Ptr(9.9),
	})
	if appErr == nil {
		t.Fatal("expected upsert failure")
	}
	if clientWithUpsertErr.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", clientWithUpsertErr.upsertCalls)
	}
}

type erpBridgeUpsertFailureClient struct {
	inner       *erpBridgeReadbackSequenceClient
	upsertErr   error
	upsertCalls int
}

func (c *erpBridgeUpsertFailureClient) SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	return c.inner.SearchProducts(ctx, filter)
}
func (c *erpBridgeUpsertFailureClient) GetProductByID(ctx context.Context, id string) (*domain.ERPProduct, error) {
	return c.inner.GetProductByID(ctx, id)
}
func (c *erpBridgeUpsertFailureClient) QueryCombineSKUs(ctx context.Context, filter domain.JSTCombineSKUFilter) (*domain.JSTCombineSKUListResponse, error) {
	return c.inner.QueryCombineSKUs(ctx, filter)
}
func (c *erpBridgeUpsertFailureClient) ListCategories(ctx context.Context) ([]*domain.ERPCategory, error) {
	return c.inner.ListCategories(ctx)
}
func (c *erpBridgeUpsertFailureClient) ListSyncLogs(ctx context.Context, filter domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	return c.inner.ListSyncLogs(ctx, filter)
}
func (c *erpBridgeUpsertFailureClient) GetSyncLogByID(ctx context.Context, id string) (*domain.ERPSyncLog, error) {
	return c.inner.GetSyncLogByID(ctx, id)
}
func (c *erpBridgeUpsertFailureClient) QueryOrderActionLogs(ctx context.Context, filter domain.ERPOrderActionLogFilter) (*domain.ERPOrderActionLogListResponse, error) {
	return c.inner.QueryOrderActionLogs(ctx, filter)
}
func (c *erpBridgeUpsertFailureClient) UpsertProduct(ctx context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	c.upsertCalls++
	return nil, c.upsertErr
}
func (c *erpBridgeUpsertFailureClient) UpdateItemStyle(ctx context.Context, payload domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	return c.inner.UpdateItemStyle(ctx, payload)
}
func (c *erpBridgeUpsertFailureClient) ShelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return c.inner.ShelveProductsBatch(ctx, payload)
}
func (c *erpBridgeUpsertFailureClient) UnshelveProductsBatch(ctx context.Context, payload domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return c.inner.UnshelveProductsBatch(ctx, payload)
}
func (c *erpBridgeUpsertFailureClient) UpdateVirtualInventory(ctx context.Context, payload domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	return c.inner.UpdateVirtualInventory(ctx, payload)
}
func (c *erpBridgeUpsertFailureClient) GetCompanyUsers(ctx context.Context, filter domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	return c.inner.GetCompanyUsers(ctx, filter)
}
