package service

import (
	"context"
	"encoding/json"
	"fmt"
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
func (m *maintenanceStoreStub) AppendLog(_ context.Context, _ repo.Tx, _, _ int64, _, _, _ string, _ int64, payload interface{}) error {
	m.logged++
	m.row.ReviewJSON, _ = json.Marshal(payload)
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
			copy := *row
			m := &maintenanceStoreStub{row: &copy}
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
			result, err := s.MaintainCost(ctx, 10, domain.CostMaintenanceRequest{Action: tc.action, Token: token, Revision: 2, ERPCost: float64Ptr(9.9), ReasonCode: "cost_incorrect", CostPrice: tc.price})
			if (err == nil) != tc.want {
				t.Fatalf("unexpected result %v", err)
			}
			if tc.want && (m.saved != 1 || m.logged != 1 || m.business) {
				t.Fatalf("intent/audit/business replay: %+v", m)
			}
			if tc.want && !result.Reviewed {
				t.Fatal("committed human review was not returned to the list")
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

func TestMaintenanceReviewTracksHumanAuditAndCurrentBusinessValues(t *testing.T) {
	original := domain.CostMaintenanceRow{ID: 10, TaskID: 71, SKUCode: "SKU", ProductName: "原名称", ProductIID: "KT", VariantJSON: json.RawMessage(`{"width":20,"height":30}`), CostPrice: float64Ptr(3.3), ERPCost: float64Ptr(3.3), Revision: 2, Status: "synced", UpdatedAt: time.Now()}
	hydrateMaintenanceRow(&original)
	if original.Reviewed {
		t.Fatal("ERP convergence must not imply human review")
	}
	for _, action := range []string{"confirm", "modify"} {
		audit, err := json.Marshal(map[string]interface{}{"action": action, "after": original})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name   string
			change func(*domain.CostMaintenanceRow)
			want   bool
		}{
			{"unchanged after reload", func(*domain.CostMaintenanceRow) {}, true},
			{"delivery pending", func(r *domain.CostMaintenanceRow) {
				r.Status = "pending"
				r.ERPCost = float64Ptr(9.9)
				r.Revision++
				r.UpdatedAt = r.UpdatedAt.Add(time.Hour)
				r.BaseSyncRequired = true
			}, true},
			{"name changed", func(r *domain.CostMaintenanceRow) { r.ProductName = "新名称" }, false},
			{"style changed", func(r *domain.CostMaintenanceRow) { r.ProductIID = "NEW" }, false},
			{"dimensions changed", func(r *domain.CostMaintenanceRow) { r.VariantJSON = json.RawMessage(`{"width":40,"height":30}`) }, false},
			{"area changed", func(r *domain.CostMaintenanceRow) {
				r.VariantJSON = json.RawMessage(`{"width":20,"height":30,"area":0.12}`)
			}, false},
			{"specification changed", func(r *domain.CostMaintenanceRow) {
				r.VariantJSON = json.RawMessage(`{"cost_input":{"area_mode":"total","area_m2":0.12,"pieces":1}}`)
			}, false},
			{"price changed", func(r *domain.CostMaintenanceRow) { r.CostPrice = float64Ptr(4.4) }, false},
			{"price removed", func(r *domain.CostMaintenanceRow) { r.CostPrice = nil }, false},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				r := original
				r.ReviewJSON = audit
				tc.change(&r)
				hydrateMaintenanceRow(&r)
				if r.Reviewed != tc.want {
					t.Fatalf("reviewed=%v want=%v", r.Reviewed, tc.want)
				}
			})
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"action":"confirm"}`, `{"action":"automatic","after":{}}`, `invalid`} {
		r := original
		r.ReviewJSON = json.RawMessage(raw)
		hydrateMaintenanceRow(&r)
		if r.Reviewed {
			t.Fatalf("invalid or incomplete audit counted as human review: %s", raw)
		}
	}
}

func TestMaintenanceRequiresBothPermissionsBeforeAnyRead(t *testing.T) {
	s := &taskService{}
	ctx := domain.WithRequestActor(context.Background(), taskActionTestActor(1, domain.PermissionCatalogManage, domain.AccessScopeGlobal))
	if _, err := s.MaintainCost(ctx, 1, domain.CostMaintenanceRequest{}); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("permission bypass %v", err)
	}
}

func TestMaintenanceFilingUsesCorrectedMemberDimensions(t *testing.T) {
	detail := &domain.TaskDetail{Width: float64Ptr(999), Height: float64Ptr(888), Area: float64Ptr(777)}
	item := &domain.TaskSKUItem{ID: 10, SKUCode: "SKU", ProductNameSnapshot: "corrected", VariantJSON: json.RawMessage(`{"product_i_id":"KT","width":20,"height":30,"area":0.06,"size_text":"20×30cm"}`)}
	payload, err := buildBatchSKUItemERPBridgeProductUpsertPayload(&domain.Task{ID: 71, TaskType: domain.TaskTypeNewProductDevelopment}, detail, item, 1, "", "business_info_patch")
	if err != nil || payload.BusinessInfo == nil || *payload.BusinessInfo.Width != 20 || *payload.BusinessInfo.Height != 30 || *payload.BusinessInfo.Area != .06 || *detail.Width != 999 {
		t.Fatalf("wrong member dimensions or mutated header: %+v %v", payload.BusinessInfo, err)
	}
}

type maintenanceListingStub struct {
	repo.CostMaintenanceRepo
	rows  []*domain.CostMaintenanceRow
	calls []repo.CostMaintenanceFilter
}

func (m *maintenanceListingStub) List(ctx context.Context, f repo.CostMaintenanceFilter) ([]*domain.CostMaintenanceRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.calls = append(m.calls, f)
	out := []*domain.CostMaintenanceRow{}
	for _, r := range m.rows {
		if f.BeforeID > 0 && r.SortID >= f.BeforeID {
			continue
		}
		c := *r
		out = append(out, &c)
		if len(out) == f.PageSize+1 {
			break
		}
	}
	return out, nil
}
func TestMaintenanceReviewFilteringPrecedesKeysetPagination(t *testing.T) {
	m := &maintenanceListingStub{}
	for id := int64(230); id >= 1; id-- {
		row := &domain.CostMaintenanceRow{ID: id, TaskID: id, SortID: id, SKUCode: fmt.Sprint(id), ProductName: "name", ProductIID: "KT", CostPrice: float64Ptr(3.3), Status: "synced"}
		hydrateMaintenanceRow(row)
		if id >= 26 {
			row.ReviewJSON, _ = json.Marshal(map[string]interface{}{"action": "confirm", "after": row})
		}
		if id == 200 {
			row.Status = "conflict"
		}
		m.rows = append(m.rows, row)
	}
	s := &taskService{costMaintenance: m}
	seen := map[int64]bool{}
	f := repo.CostMaintenanceFilter{PageSize: 10}
	for {
		page, err := s.ListCostMaintenance(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Data) == 0 {
			t.Fatal("filtered pagination produced an empty page before EOF")
		}
		for _, row := range page.Data {
			if seen[row.ID] {
				t.Fatalf("duplicate SKU %d", row.ID)
			}
			if row.Reviewed && row.Status != "conflict" {
				t.Fatal("default list leaked a reviewed SKU")
			}
			seen[row.ID] = true
		}
		if page.NextBeforeID == 0 {
			break
		}
		if f.BeforeID > 0 && page.NextBeforeID >= f.BeforeID {
			t.Fatal("cursor did not advance")
		}
		f.BeforeID = page.NextBeforeID
	}
	if len(seen) != 26 || len(m.calls) < 3 {
		t.Fatalf("lost pending SKUs after reviewed prefix: count=%d calls=%d", len(seen), len(m.calls))
	}
	checked, err := s.ListCostMaintenance(context.Background(), repo.CostMaintenanceFilter{PageSize: 100, ReviewState: "reviewed"})
	if err != nil || len(checked.Data) != 100 {
		t.Fatalf("reviewed list: %v %v", checked, err)
	}
	for _, row := range checked.Data {
		if !row.Reviewed || row.Status == "conflict" || seen[row.ID] {
			t.Fatal("reviewed and pending scopes overlap")
		}
	}
	all, err := s.ListCostMaintenance(context.Background(), repo.CostMaintenanceFilter{PageSize: 100, ReviewState: "all"})
	if err != nil || len(all.Data) != 100 || all.NextBeforeID != 131 {
		t.Fatalf("all list pagination changed: %v %v", all, err)
	}
	_, err = s.ListCostMaintenance(context.Background(), repo.CostMaintenanceFilter{ReviewState: "unknown"})
	if err == nil || err.Code != domain.ErrCodeInvalidRequest {
		t.Fatal("invalid review filter accepted")
	}
}
