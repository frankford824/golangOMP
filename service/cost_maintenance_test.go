package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
	"workflow/domain"
	"workflow/repo"
)

type maintenanceStoreStub struct {
	repo.CostMaintenanceRepo
	row           *domain.CostMaintenanceRow
	saved, logged int
	business      bool
}

func (m *maintenanceStoreStub) LockRow(context.Context, repo.Tx, int64) (*domain.CostMaintenanceRow, error) {
	v := *m.row
	return &v, nil
}
func (m *maintenanceStoreStub) Save(_ context.Context, _ repo.Tx, item *domain.TaskSKUItem, _ bool, business bool) error {
	m.saved++
	m.business = business
	m.row.CostPrice = cloneFloat64Ptr(item.CostPrice)
	return nil
}
func (m *maintenanceStoreStub) Project(context.Context, repo.Tx, *domain.TaskSKUItem) error {
	return nil
}
func (m *maintenanceStoreStub) AppendLog(context.Context, repo.Tx, int64, int64, string, string, string, int64, interface{}) error {
	m.logged++
	return nil
}

func TestMaintenanceConfirmationIsAtomicIntentAndNeverBusinessReplay(t *testing.T) {
	raw := json.RawMessage(`{"width":20,"height":30,"product_i_id":"KT"}`)
	row := &domain.CostMaintenanceRow{ID: 10, TaskID: 71, SKUCode: "SKU", ProductName: "原名称", VariantJSON: raw, Revision: 2, CostPrice: float64Ptr(3.3), ERPCost: float64Ptr(9.9), UpdatedAt: time.Now()}
	r := &prdTaskRepo{tasks: map[int64]*domain.Task{71: {ID: 71, TaskStatus: domain.TaskStatusCompleted, PrimarySKUCode: "SKU"}}, details: map[int64]*domain.TaskDetail{71: {TaskID: 71}}, skuItems: map[int64][]*domain.TaskSKUItem{71: {{ID: 10, TaskID: 71, SKUCode: "SKU", ProductNameSnapshot: "原名称", VariantJSON: raw, CostPrice: float64Ptr(3.3)}}}}
	for _, tc := range []struct {
		name   string
		action string
		price  *float64
		token  string
		erp    *float64
		want   bool
	}{
		{"confirm current", "confirm", nil, "", float64Ptr(9.9), true},
		{"price edit", "modify", float64Ptr(4.4), "", float64Ptr(9.9), true},
		{"stale list", "confirm", nil, "stale", float64Ptr(9.9), false},
		{"changed ERP", "confirm", nil, "", float64Ptr(10), false},
		{"empty modification", "modify", nil, "", float64Ptr(9.9), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &maintenanceStoreStub{row: row}
			store := &manualCostStoreStub{costSyncStoreStub: costSyncStoreStub{state: &domain.CostSyncState{Revision: 2, LocalCost: float64Ptr(3.3)}}}
			sync := NewCostSyncService(store, costObserverStub{get: func() (*domain.ERPProduct, error) { return &domain.ERPProduct{SKUID: "SKU", CostPrice: tc.erp}, nil }})
			s := NewTaskService(r, &prdTaskAssetRepo{}, &prdTaskEventRepo{}, nil, prdCodeRuleService{}, step04TxRunner{}, WithTaskCostSyncService(sync), WithTaskCostMaintenanceRepo(m)).(*taskService)
			actor := taskActionTestActor(801, domain.PermissionCatalogManage, domain.AccessScopeGlobal)
			actor.Permissions = append(actor.Permissions, domain.PermissionERPManage)
			ctx := domain.WithRequestActor(context.Background(), actor)
			token := maintenanceToken(row)
			if tc.token != "" {
				token = tc.token
			}
			_, err := s.MaintainCost(ctx, 10, domain.CostMaintenanceRequest{Action: tc.action, Token: token, Revision: 2, ERPCost: float64Ptr(9.9), ReasonCode: "cost_incorrect", CostPrice: tc.price})
			if (err == nil) != tc.want {
				t.Fatalf("unexpected result %v", err)
			}
			if tc.want && (m.saved != 1 || m.logged != 1 || m.business) {
				t.Fatalf("intent/audit/business replay: %+v", m)
			}
			if !tc.want && (m.saved != 0 || m.logged != 0) {
				t.Fatal("rejected operation wrote data")
			}
			row.CostPrice = float64Ptr(3.3)
		})
	}
}

func TestMaintenanceValidationAndDimensionUnits(t *testing.T) {
	for _, p := range []domain.CostMaintenanceRequest{
		{Action: "modify", Token: "a", Revision: 1, ReasonCode: "other"},
		{Action: "modify", Token: "a", Revision: 1, ReasonCode: "unknown"},
		{Action: "confirm", Token: "a", Revision: 1, CostPrice: float64Ptr(2)},
		{Action: "modify", Token: "a", Revision: 1, ReasonCode: "area_incorrect", AreaM2: float64Ptr(math.NaN())},
	} {
		if validateMaintenanceRequest(p) == nil {
			t.Fatalf("invalid request accepted %+v", p)
		}
	}
	r := &domain.CostMaintenanceRow{VariantJSON: json.RawMessage(`{"width":20,"height":30}`), DetailArea: float64Ptr(99)}
	hydrateMaintenanceRow(r)
	if r.AreaM2 == nil || math.Abs(*r.AreaM2-.06) > 1e-9 {
		t.Fatalf("stale aggregate area leaked %+v", r)
	}
	r = &domain.CostMaintenanceRow{VariantJSON: json.RawMessage(`{"cost_input":{"area_mode":"faces","pieces":6,"faces":[{"width_m":0.2,"height_m":0.3,"count":2}]}}`)}
	hydrateMaintenanceRow(r)
	if r.AreaM2 == nil || math.Abs(*r.AreaM2-.12) > 1e-9 || r.WidthCM != nil {
		t.Fatal("faces counted twice or fabricated dimensions")
	}
	token := r.Token
	r.ProductName = "new"
	if maintenanceToken(r) == token {
		t.Fatal("name change did not invalidate edit")
	}
}

func TestMaintenanceRequiresBothPermissionsBeforeAnyRead(t *testing.T) {
	s := &taskService{}
	ctx := domain.WithRequestActor(context.Background(), taskActionTestActor(1, domain.PermissionCatalogManage, domain.AccessScopeGlobal))
	if _, err := s.MaintainCost(ctx, 1, domain.CostMaintenanceRequest{}); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("permission bypass %v", err)
	}
}
