package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type repairBindingStub struct {
	repo.CostRuleBindingRepo
	enabled bool
}

func (s repairBindingStub) GetActiveByNormalizedIID(context.Context, string) (*domain.CostRuleBinding, error) {
	if !s.enabled {
		return nil, nil
	}
	return &domain.CostRuleBinding{RuleGroup: "KT_STANDARD", IsActive: true}, nil
}

type repairStoreStub struct {
	manualCostStoreStub
	staged bool
}

func (s *repairStoreStub) QuarantineRestoredCost(context.Context, repo.Tx, string, int64, string) error {
	s.staged = true
	return nil
}

func (s *repairStoreStub) StageRestoredCost(context.Context, repo.Tx, string, *float64, int64, string) error {
	s.staged = true
	return nil
}

func TestLegacyCostRepairPreservesManualPricesAndChecksInputs(t *testing.T) {
	ctx := context.Background()
	item := &domain.TaskSKUItem{ID: 1, TaskID: 1, SKUCode: "CGK005285", ProductIID: "常规kt板", CategoryCode: "KT_STANDARD", ProductNameSnapshot: "两块立牌", VariantJSON: json.RawMessage(`{"area":0.86,"product_i_id":"常规kt板"}`), CostPrice: float64Ptr(72.084), ManualCostOverride: true, OverrideActor: "erp_cost_sync"}
	tasks := &prdTaskRepo{skuByCode: map[string]*domain.TaskSKUItem{item.SKUCode: item}}
	store := &repairStoreStub{manualCostStoreStub: manualCostStoreStub{costSyncStoreStub: costSyncStoreStub{state: &domain.CostSyncState{SKUCode: item.SKUCode, Revision: 2, LocalCost: item.CostPrice, LocalConfirmed: true, ManualLock: true, ManualOrigin: "erp"}}}}
	rules := &costRuleRepoStub{rules: []*domain.CostRule{{RuleID: 34, RuleVersion: 2, CategoryCode: "KT_STANDARD", RuleType: domain.CostRuleTypeFixedUnitPrice, BasePrice: float64Ptr(11), TaxMultiplier: float64Ptr(1.1), IsActive: true, Source: LegacyCostRestoreSource}}}
	svc := NewLegacyCostRepairService(tasks, rules, repairBindingStub{enabled: true}, nil, step04TxRunner{}, store)
	p, err := svc.Preview(ctx, item.SKUCode)
	if err != nil || p.SkipReason != "" || p.NewCost == nil || *p.NewCost != 10.406 {
		t.Fatalf("preview=%+v err=%v", p, err)
	}
	item.OverrideActor = "operator:246"
	protected, err := svc.Preview(ctx, item.SKUCode)
	if err != nil || protected.SkipReason != "preserved_manual_price" {
		t.Fatalf("manual price not protected: %+v %v", protected, err)
	}
	item.OverrideActor = "erp_cost_sync"
	item.VariantJSON = json.RawMessage(`{"area":2.07,"product_i_id":"常规kt板"}`)
	if err = svc.Apply(ctx, p, float64Ptr(72.084), 1, "reviewed-plan"); err == nil || !strings.Contains(err.Error(), "changed since preview") || store.staged {
		t.Fatalf("stale specification applied: %v", err)
	}
	svc.bindings = repairBindingStub{}
	missing, err := svc.Preview(ctx, item.SKUCode)
	if err != nil || missing.SkipReason != "missing_exact_binding" {
		t.Fatalf("unbound repair must not guess: %+v %v", missing, err)
	}
	svc.bindings = repairBindingStub{enabled: true}
	item.VariantJSON = json.RawMessage(`{"product_i_id":"常规kt板"}`)
	missing, err = svc.Preview(ctx, item.SKUCode)
	if err != nil || missing.SkipReason != "incomplete_tariff_or_specification" {
		t.Fatalf("missing dimensions: %+v %v", missing, err)
	}
	if !missing.NeedsQuarantine() {
		t.Fatal("unreviewed ERP price with missing specifications must not remain certified")
	}
	if protected.NeedsQuarantine() {
		t.Fatal("a human price must never be quarantined by this recovery")
	}
}
