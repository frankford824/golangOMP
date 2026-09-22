package service

import (
	"context"
	"encoding/json"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type costSyncStoreStub struct {
	repo.CostSyncRepo
	state *domain.CostSyncState
	acked bool
}

func (s *costSyncStoreStub) Get(context.Context, string) (*domain.CostSyncState, error) {
	return s.state, nil
}
func (s *costSyncStoreStub) Lock(context.Context, string) (func(), error)      { return func() {}, nil }
func (s *costSyncStoreStub) Fail(context.Context, string, int64, string) error { return nil }
func (s *costSyncStoreStub) Observe(_ context.Context, _ string, p *float64) error {
	action := domain.DecideCostObservation(s.state, p)
	s.state.ERPCost = p
	if action == "conflict" {
		s.state.Status = "conflict"
	}
	if action == "agree" {
		s.state.Status = "synced"
		s.state.AckRevision = s.state.Revision
	}
	return nil
}
func (s *costSyncStoreStub) Acknowledge(context.Context, string, int64, *float64) error {
	s.acked = true
	return nil
}

type costObserverStub struct {
	ERPBridgeClient
	get func() (*domain.ERPProduct, error)
}

func (s costObserverStub) GetProductByID(context.Context, string) (*domain.ERPProduct, error) {
	return s.get()
}
func TestCostGuardStripsConflictCostButStillFilesIdentity(t *testing.T) {
	store := &costSyncStoreStub{state: &domain.CostSyncState{LocalConfirmed: true, SKUCode: "SKU", LocalCost: float64Ptr(10), ERPCost: float64Ptr(10), Revision: 1, AckRevision: 1, ManualLock: true, Status: "synced"}}
	svc := NewCostSyncService(store, costObserverStub{get: func() (*domain.ERPProduct, error) {
		return &domain.ERPProduct{SKUID: "SKU", CostPrice: float64Ptr(12)}, nil
	}})
	p := domain.ERPProductUpsertPayload{SKUID: "SKU", Name: "保留商品", CostPrice: float64Ptr(10), BusinessInfo: &domain.ERPTaskBusinessInfoSnapshot{CostPrice: float64Ptr(10)}}
	_, err := svc.Protect(context.Background(), p, func(_ context.Context, p domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		if p.CostPrice != nil || p.BusinessInfo.CostPrice != nil || p.Name != "保留商品" {
			t.Fatal("cost leaked or identity lost")
		}
		return &domain.ERPProductUpsertResult{}, nil
	})
	if err != nil || store.acked || store.state.Status != "conflict" {
		t.Fatalf("bad conflict handling %+v", err)
	}
	if p.BusinessInfo.CostPrice == nil {
		t.Fatal("input mutated")
	}
}
func TestCostOnlyPayloadCannotRewriteStyleNameOrDimensions(t *testing.T) {
	p := normalizeERPProductUpsertPayload(domain.ERPProductUpsertPayload{SKUID: "SKU", Name: "do not send", IID: "wrong", CostPrice: float64Ptr(3.7967), Operation: "cost_sync", H: float64Ptr(5.5)})
	raw, _ := json.Marshal(p)
	biz, err := buildERPRemoteOpenWebBiz("upsert", raw)
	if err != nil {
		t.Fatal(err)
	}
	item := biz["items"].([]map[string]interface{})[0]
	if len(item) != 3 || item["sku_id"] != "SKU" || item["cost_price"] != 3.7967 {
		t.Fatalf("unexpected cost-only payload %+v", item)
	}
}
func TestCostGuardRejectsStaleLocalPayload(t *testing.T) {
	store := &costSyncStoreStub{state: &domain.CostSyncState{LocalCost: float64Ptr(20), Revision: 2, Status: "pending"}}
	svc := NewCostSyncService(store, nil)
	_, err := svc.Protect(context.Background(), domain.ERPProductUpsertPayload{SKUID: "SKU", CostPrice: float64Ptr(10), Operation: "cost_sync"}, func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		t.Fatal("stale write sent")
		return nil, nil
	})
	if err == nil {
		t.Fatal("stale write accepted")
	}
}

func TestCostGuardNewSKUStillCreatesWithConfirmedPrice(t *testing.T) {
	store := &costSyncStoreStub{state: &domain.CostSyncState{SKUCode: "SKU", LocalCost: float64Ptr(10), Revision: 1, Status: "pending", LocalConfirmed: true}}
	calls := 0
	observer := costObserverStub{get: func() (*domain.ERPProduct, error) {
		calls++
		if calls == 1 {
			return nil, &erpBridgeRemoteProductNotFoundError{QueryID: "SKU"}
		}
		return &domain.ERPProduct{SKUID: "SKU", CostPrice: float64Ptr(10)}, nil
	}}
	svc := NewCostSyncService(store, observer)
	_, err := svc.Protect(context.Background(), domain.ERPProductUpsertPayload{SKUID: "SKU", Name: "new", CostPrice: float64Ptr(10)}, func(_ context.Context, p domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		if p.CostPrice == nil {
			t.Fatal("confirmed new-SKU price lost")
		}
		return &domain.ERPProductUpsertResult{}, nil
	})
	if err != nil || !store.acked {
		t.Fatalf("creation not confirmed %v", err)
	}
}
func TestCostGuardDoesNotAcknowledgeMismatchedReadback(t *testing.T) {
	store := &costSyncStoreStub{state: &domain.CostSyncState{SKUCode: "SKU", LocalCost: float64Ptr(12), ERPCost: float64Ptr(10), Revision: 2, AckRevision: 1, Status: "pending", LocalConfirmed: true}}
	calls := 0
	observer := costObserverStub{get: func() (*domain.ERPProduct, error) {
		calls++
		price := 10.0
		if calls > 1 {
			price = 13
		}
		return &domain.ERPProduct{SKUID: "SKU", CostPrice: &price}, nil
	}}
	svc := NewCostSyncService(store, observer)
	_, err := svc.Protect(context.Background(), domain.ERPProductUpsertPayload{SKUID: "SKU", CostPrice: float64Ptr(12), Operation: "cost_sync"}, func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		return &domain.ERPProductUpsertResult{}, nil
	})
	if err == nil || store.acked || store.state.Status != "conflict" {
		t.Fatal("mismatched readback falsely completed")
	}
}
func TestFilingComparisonIgnoresOnlyCostNotBusinessChanges(t *testing.T) {
	old := domain.ERPProductUpsertPayload{SKUID: "SKU", Name: "A", CostPrice: float64Ptr(10), H: float64Ptr(5.5)}
	raw, _ := json.Marshal(old)
	next := old
	next.CostPrice = float64Ptr(12)
	if !sameFilingBusinessIgnoringCost(string(raw), []taskFilingPayload{{Payload: next}}) {
		t.Fatal("a price-only edit should not replay the product profile")
	}
	next.H = float64Ptr(8)
	if sameFilingBusinessIgnoringCost(string(raw), []taskFilingPayload{{Payload: next}}) {
		t.Fatal("a dimension change must remain an explicit profile change")
	}
}
