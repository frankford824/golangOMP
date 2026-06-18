package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type skuComboRepoFake struct {
	relations []*domain.OMPSKUComboRelationWithRecord

	state       *domain.OMPSKUComboSyncState
	claimed     bool
	claimCalls  int
	successDone bool
	successPage int

	upsertedRecords   []string
	upsertedRelations []domain.OMPSKUComboRelation
	deletedComboCode  string
	deletedChildren   []string
}

func (f *skuComboRepoFake) UpsertComboRecord(_ context.Context, _ repo.Tx, record *domain.OMPSKUComboRecord) error {
	if record != nil {
		f.upsertedRecords = append(f.upsertedRecords, record.ComboSKUCode)
	}
	return nil
}

func (f *skuComboRepoFake) UpsertComboRelation(_ context.Context, _ repo.Tx, relation *domain.OMPSKUComboRelation) error {
	if relation != nil {
		f.upsertedRelations = append(f.upsertedRelations, *relation)
	}
	return nil
}

func (f *skuComboRepoFake) DeleteStaleComboRelations(_ context.Context, _ repo.Tx, comboSKUCode string, _ string, currentChildSKUs []string) error {
	f.deletedComboCode = comboSKUCode
	f.deletedChildren = append([]string{}, currentChildSKUs...)
	return nil
}

func (f *skuComboRepoFake) ListRelationsByChildSKUs(context.Context, []string) ([]*domain.OMPSKUComboRelationWithRecord, error) {
	return f.relations, nil
}

func (f *skuComboRepoFake) GetLatestSyncState(context.Context) (*domain.OMPSKUComboSyncState, error) {
	return f.state, nil
}

func (f *skuComboRepoFake) EnsureNextSyncWindow(context.Context, time.Time, time.Duration) (*domain.OMPSKUComboSyncState, error) {
	return f.state, nil
}

func (f *skuComboRepoFake) ClaimSyncState(context.Context, repo.Tx, int64, time.Time) (bool, error) {
	f.claimCalls++
	return f.claimed, nil
}

func (f *skuComboRepoFake) MarkSyncStateSuccess(_ context.Context, _ repo.Tx, _ int64, nextPage int, _ int, finished bool, _ time.Time) error {
	f.successPage = nextPage
	f.successDone = finished
	return nil
}

func (f *skuComboRepoFake) MarkSyncStateFailed(context.Context, repo.Tx, int64, string, time.Time) error {
	return nil
}

type testTx struct{}

func (testTx) IsTx() {}

type txRunnerFake struct{}

func (txRunnerFake) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(testTx{})
}

func TestProductManagementComboGroupsPreserveRecordOrder(t *testing.T) {
	t.Parallel()
	now := time.Now()
	svc := &productManagementService{
		skuCombos: &skuComboRepoFake{
			relations: []*domain.OMPSKUComboRelationWithRecord{
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-A", ChildSKUCode: "SKU-A", Quantity: 2},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-A", Name: "组合 A", LastSyncedAt: now},
				},
				{
					Relation: domain.OMPSKUComboRelation{ComboSKUCode: "COMBO-Z", ChildSKUCode: "SKU-Z", Quantity: 1},
					Record:   &domain.OMPSKUComboRecord{ComboSKUCode: "COMBO-Z", Name: "组合 Z", LastSyncedAt: now},
				},
			},
		},
	}
	records := []*domain.ProductManagementRecord{
		{ID: 1, SKUCode: "SKU-Z"},
		{ID: 2, SKUCode: "SKU-A"},
		{ID: 3, SKUCode: "SKU-SINGLE"},
	}

	groups := svc.productManagementComboGroups(context.Background(), records)
	if len(groups) != 3 {
		t.Fatalf("groups length = %d, want 3", len(groups))
	}
	gotKeys := []string{groups[0].GroupKey, groups[1].GroupKey, groups[2].GroupKey}
	wantKeys := []string{"combo:COMBO-Z", "combo:COMBO-A", "single:3"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("group keys = %#v, want %#v", gotKeys, wantKeys)
	}
	if got := groups[0].Children[0].Record.SKUCode; got != "SKU-Z" {
		t.Fatalf("first combo child sku = %q, want SKU-Z", got)
	}
	if got := groups[1].Children[0].Quantity; got != 2 {
		t.Fatalf("second combo quantity = %v, want 2", got)
	}
}

func TestSKUComboSyncServiceDoesNotCallERPWhenClaimMisses(t *testing.T) {
	t.Parallel()
	repoFake := &skuComboRepoFake{
		state: &domain.OMPSKUComboSyncState{
			ID:          1,
			WindowBegin: time.Date(2026, 6, 1, 0, 0, 0, 0, jstOpenWebLocation),
			WindowEnd:   time.Date(2026, 6, 8, 0, 0, 0, 0, jstOpenWebLocation),
			PageIndex:   1,
			PageSize:    50,
			Status:      "pending",
		},
		claimed: false,
	}
	bridge := &productManagementERPBridgeCapture{}
	svc := &skuComboSyncService{
		erpBridge:  bridge,
		skuCombos:  repoFake,
		txRunner:   txRunnerFake{},
		now:        func() time.Time { return time.Date(2026, 6, 8, 12, 0, 0, 0, jstOpenWebLocation) },
		windowSize: 7 * 24 * time.Hour,
	}

	processed, appErr := svc.ProcessNextPage(context.Background())
	if appErr != nil {
		t.Fatalf("ProcessNextPage() appErr = %+v", appErr)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}
	if bridge.combineCalls != 0 {
		t.Fatalf("combine calls = %d, want 0", bridge.combineCalls)
	}
	if repoFake.claimCalls != 1 {
		t.Fatalf("claim calls = %d, want 1", repoFake.claimCalls)
	}
}

func TestSKUComboSyncServiceDeletesStaleRelationsForFetchedCombo(t *testing.T) {
	t.Parallel()
	repoFake := &skuComboRepoFake{
		state: &domain.OMPSKUComboSyncState{
			ID:          2,
			WindowBegin: time.Date(2026, 6, 1, 0, 0, 0, 0, jstOpenWebLocation),
			WindowEnd:   time.Date(2026, 6, 8, 0, 0, 0, 0, jstOpenWebLocation),
			PageIndex:   1,
			PageSize:    50,
			Status:      "pending",
		},
		claimed: true,
	}
	bridge := &productManagementERPBridgeCapture{
		combineResponse: &domain.JSTCombineSKUListResponse{
			Items: []domain.JSTCombineSKUItem{
				{
					ComboSKUCode: "COMBO-1",
					Children: []domain.JSTCombineSKUChild{
						{SKUCode: "SKU-1", Quantity: 2},
						{SKUCode: "SKU-2", Quantity: 1},
					},
				},
			},
			Pagination: domain.PaginationMeta{Page: 1, PageSize: 50, Total: 1},
		},
	}
	svc := &skuComboSyncService{
		erpBridge:  bridge,
		skuCombos:  repoFake,
		txRunner:   txRunnerFake{},
		now:        func() time.Time { return time.Date(2026, 6, 8, 12, 0, 0, 0, jstOpenWebLocation) },
		windowSize: 7 * 24 * time.Hour,
	}

	processed, appErr := svc.ProcessNextPage(context.Background())
	if appErr != nil {
		t.Fatalf("ProcessNextPage() appErr = %+v", appErr)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if repoFake.deletedComboCode != "COMBO-1" {
		t.Fatalf("deleted combo = %q, want COMBO-1", repoFake.deletedComboCode)
	}
	if !reflect.DeepEqual(repoFake.deletedChildren, []string{"SKU-1", "SKU-2"}) {
		t.Fatalf("deleted children keep-list = %#v", repoFake.deletedChildren)
	}
	if !repoFake.successDone || repoFake.successPage != 1 {
		t.Fatalf("success state done=%v nextPage=%d, want done true nextPage 1", repoFake.successDone, repoFake.successPage)
	}
}
