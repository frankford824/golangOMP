package service

import (
	"context"
	"errors"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type manualCostStoreStub struct {
	costSyncStoreStub
	releases int
}

func (s *manualCostStoreStub) Lock(context.Context, string) (func(), error) {
	return func() { s.releases++ }, nil
}
func (s *manualCostStoreStub) LockManualRevision(context.Context, repo.Tx, string, int64) error {
	return nil
}
func (s *manualCostStoreStub) StageManualCost(context.Context, repo.Tx, string, *float64, int64, string) error {
	return nil
}

func TestManualCostEditBaseline(t *testing.T) {
	for _, tc := range []struct {
		name     string
		revision int64
		erp      *float64
		readErr  error
		want     bool
	}{
		{"first ERP price reviewed", 2, float64Ptr(9.9), nil, true},
		{"zero reviewed", 2, float64Ptr(0), nil, true},
		{"empty reviewed", 2, nil, nil, true},
		{"local concurrent change", 1, float64Ptr(9.9), nil, false},
		{"ERP concurrent change", 2, float64Ptr(10), nil, false},
		{"ERP unavailable", 2, float64Ptr(9.9), errors.New("offline"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &manualCostStoreStub{costSyncStoreStub: costSyncStoreStub{state: &domain.CostSyncState{SKUCode: "SKU", Revision: 2, Status: "conflict", LocalCost: float64Ptr(3.3)}}}
			baseline := &domain.CostEditBaseline{Revision: tc.revision, ERPCost: float64Ptr(9.9)}
			if tc.name == "zero reviewed" || tc.name == "empty reviewed" {
				baseline.ERPCost = tc.erp
			}
			svc := NewCostSyncService(store, costObserverStub{get: func() (*domain.ERPProduct, error) {
				return &domain.ERPProduct{SKUID: "SKU", CostPrice: tc.erp}, tc.readErr
			}})
			guard, err := svc.PrepareManualEdit(context.Background(), "SKU", baseline)
			if (err == nil) != tc.want {
				t.Fatalf("guard=%v err=%v", guard, err)
			}
			guard.Close()
			if store.releases != 1 {
				t.Fatal("SKU lock leaked")
			}
			if store.state.Status != "conflict" {
				t.Fatal("preflight modified persisted state")
			}
		})
	}
}

func TestTaskCostReadDoesNotTreatUnavailableAsEmptyPrice(t *testing.T) {
	store := &costSyncStoreStub{state: &domain.CostSyncState{Revision: 1}}
	svc := NewCostSyncService(store, costObserverStub{get: func() (*domain.ERPProduct, error) { return nil, errors.New("private upstream error") }})
	v, err := svc.TaskView(context.Background(), "SKU", true)
	if err != nil || v.ERPAvailable || v.Baseline != nil || v.Message == "" {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestTaskCostSyncReadEnforcesTaskScopeAndSKUMembership(t *testing.T) {
	r := &prdTaskRepo{
		tasks:    map[int64]*domain.Task{71: {ID: 71, CreatorID: 801, TaskStatus: domain.TaskStatusInProgress}},
		details:  map[int64]*domain.TaskDetail{71: {TaskID: 71}},
		skuItems: map[int64][]*domain.TaskSKUItem{71: {{ID: 10, TaskID: 71, SKUCode: "SKU"}}},
	}
	reads := 0
	cs := NewCostSyncService(&costSyncStoreStub{state: &domain.CostSyncState{Revision: 1}}, costObserverStub{get: func() (*domain.ERPProduct, error) {
		reads++
		return &domain.ERPProduct{SKUID: "SKU", CostPrice: float64Ptr(9.9)}, nil
	}})
	s := NewTaskService(r, &prdTaskAssetRepo{}, &prdTaskEventRepo{}, nil, prdCodeRuleService{}, step04TxRunner{}, WithTaskCostSyncService(cs)).(*taskService)
	other := domain.WithRequestActor(context.Background(), taskActionTestActor(802, domain.PermissionTaskView, domain.AccessScopeSelf))
	if _, err := s.GetSKUCostSync(other, 71, 10, true); err == nil {
		t.Fatal("other creator scope bypassed")
	}
	allowed := domain.WithRequestActor(context.Background(), taskActionTestActor(801, domain.PermissionTaskView, domain.AccessScopeGlobal))
	if _, err := s.GetSKUCostSync(allowed, 71, 11, true); err == nil {
		t.Fatal("unrelated SKU accepted")
	}
	if reads != 0 {
		t.Fatal("ERP data fetched before scope/membership checks")
	}
	v, err := s.GetSKUCostSync(allowed, 71, 10, true)
	if err != nil || v.Baseline == nil || reads != 1 {
		t.Fatalf("allowed read failed %+v %v", v, err)
	}
}
