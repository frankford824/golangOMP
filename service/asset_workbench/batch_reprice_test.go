package assetworkbench

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type batchRepriceRepo struct {
	repo.AssetWorkbenchRepo
	rule       *domain.AssetWorkbenchPriceMatrix
	items      []*domain.AssetWorkbenchSubmissionItem
	beforeLock func()
	month      string
	updates    []*domain.AssetWorkbenchSubmissionItem
	refreshes  []int64
	events     []*domain.AssetWorkbenchEvent
}

func (r *batchRepriceRepo) GetPriceMatrix(context.Context, int64) (*domain.AssetWorkbenchPriceMatrix, error) {
	rule := *r.rule
	return &rule, nil
}

func (r *batchRepriceRepo) GetPriceMatrixForUpdate(ctx context.Context, _ repo.Tx, id int64) (*domain.AssetWorkbenchPriceMatrix, error) {
	return r.GetPriceMatrix(ctx, id)
}

func (r *batchRepriceRepo) ListSubmissionItemsByMonth(context.Context, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	items := make([]*domain.AssetWorkbenchSubmissionItem, 0, len(r.items))
	for _, item := range r.items {
		copy := *item
		items = append(items, &copy)
	}
	return items, nil
}

func (r *batchRepriceRepo) FindActivePrice(_ context.Context, _, _, _ string, date time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if date.Format("2006-01-02") == "2026-11-20" {
		other := *r.rule
		other.ID++
		return &other, nil
	}
	rule := *r.rule
	return &rule, nil
}

func (*batchRepriceRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return nil, nil
}

func (r *batchRepriceRepo) RunSettlementMonthTx(_ context.Context, month string, fn func(repo.Tx) error) error {
	r.month = month
	if r.beforeLock != nil {
		r.beforeLock()
	}
	err := fn(assetWorkbenchTestTx{})
	if err != nil {
		r.updates = nil
		r.refreshes = nil
		r.events = nil
	}
	return err
}

func (r *batchRepriceRepo) GetSubmissionItemForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}

func (r *batchRepriceRepo) UpdateSubmissionItemPricing(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	r.updates = append(r.updates, item)
	return item, nil
}

func (r *batchRepriceRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, id int64) error {
	r.refreshes = append(r.refreshes, id)
	return nil
}

func (r *batchRepriceRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	r.events = append(r.events, event)
	return event, nil
}

func batchRepriceTestData() (*batchRepriceRepo, domain.RequestActor, BatchRepriceParams) {
	item := func(id int64, day time.Time) *domain.AssetWorkbenchSubmissionItem {
		return &domain.AssetWorkbenchSubmissionItem{
			ID: id, SubmissionID: 100, PayeeUserID: 52,
			BusinessMonth: "2026-11", SubmittedAt: day, EntryKind: domain.AssetWorkbenchSubmissionEntryKindNormal,
			DifficultyClass: "C", WorkerTypeSnapshot: "fulltime", JobGradeSnapshot: "P1", PageCount: 1, ItemCount: 1,
			PricingStatus: domain.AssetWorkbenchPricingStatusUnpriced,
			QCStatus:      domain.AssetWorkbenchSubmissionStatusSubmitted, SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
		}
	}
	nov18 := time.Date(2026, 11, 18, 4, 30, 0, 0, time.UTC)
	eligible := item(1, nov18)
	shanghaiNov1 := item(2, time.Date(2026, 10, 31, 16, 1, 0, 0, time.UTC))
	shanghaiOct31 := item(3, time.Date(2026, 10, 31, 15, 59, 0, 0, time.UTC))
	locked := item(4, nov18)
	batchID := int64(101)
	locked.CurrentSettlementBatchID = &batchID
	voided := item(5, nov18)
	voided.QCStatus = domain.AssetWorkbenchSubmissionStatusVoided
	priced := item(6, nov18)
	priced.PricingStatus, priced.GrossAmount = domain.AssetWorkbenchPricingStatusPriced, 0.10
	differentPayee := item(7, nov18)
	differentPayee.PayeeUserID++
	otherRule := item(8, time.Date(2026, 11, 20, 4, 30, 0, 0, time.UTC))
	nonBillable := item(9, nov18)
	nonBillable.DifficultyClass = "0"
	r := &batchRepriceRepo{
		rule: &domain.AssetWorkbenchPriceMatrix{ID: 21, WorkerType: "fulltime", JobGrade: "P1", DifficultyClass: "C",
			Enabled: true, UnitPrice: 0.17, EffectiveFrom: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		items: []*domain.AssetWorkbenchSubmissionItem{otherRule, shanghaiNov1, eligible, shanghaiOct31, locked, voided, priced, differentPayee, nonBillable},
	}
	actor := domain.RequestActor{ID: 51, EffectiveAccess: &domain.EffectiveAccess{Permissions: []domain.PermissionCode{domain.PermissionAssetWorkbenchSettlement}}}
	return r, actor, BatchRepriceParams{RuleID: 21, BusinessMonth: "2026-11", PayeeUserID: 52, SubmittedFrom: "2026-11-01"}
}

func TestBatchRepriceUsesOriginalShanghaiDatesAndOnlyTheMatchingRule(t *testing.T) {
	ctx := context.Background()
	r, actor, scope := batchRepriceTestData()
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	preview, appErr := svc.PreviewBatchReprice(ctx, actor, scope)
	if appErr != nil || preview.Count != 2 || preview.OldGross != 0 || preview.NewGross != 0.34 {
		t.Fatalf("preview=%+v err=%+v", preview, appErr)
	}
	if preview.Items[0].ItemID != 1 || preview.Items[0].SubmittedOn != "2026-11-18" ||
		preview.Items[1].ItemID != 2 || preview.Items[1].SubmittedOn != "2026-11-01" {
		t.Fatalf("unexpected original dates or items: %+v", preview.Items)
	}
	if len(r.updates) != 0 || len(r.events) != 0 {
		t.Fatal("preview made changes")
	}
	result, appErr := svc.ConfirmBatchReprice(ctx, actor, ConfirmBatchRepriceParams{BatchRepriceParams: scope, PreviewToken: preview.PreviewToken})
	if appErr != nil || result.Updated != 2 || result.NewGross != 0.34 || r.month != "2026-11" {
		t.Fatalf("confirm=%+v err=%+v month=%s", result, appErr, r.month)
	}
	if len(r.updates) != 2 || len(r.events) != 2 || len(r.refreshes) != 1 || r.refreshes[0] != 100 {
		t.Fatalf("wanted two pricing updates and events, one submission total refresh")
	}
	for _, changed := range r.updates {
		if changed.PayeeUserID != 52 || changed.BusinessMonth != "2026-11" || changed.PageCount != 1 ||
			changed.BasePriceRuleID == nil || *changed.BasePriceRuleID != 21 || changed.GrossAmount != 0.17 {
			t.Fatalf("unexpected pricing change: %+v", changed)
		}
	}
	scope.IncludePriced = true
	preview, appErr = svc.PreviewBatchReprice(ctx, actor, scope)
	if appErr != nil || preview.Count != 3 || preview.OldGross != 0.10 || preview.NewGross != 0.51 {
		t.Fatalf("include priced preview=%+v err=%+v", preview, appErr)
	}
}

func TestBatchRepriceRejectsStalePreviewsAndSettlementRaces(t *testing.T) {
	ctx := context.Background()
	r, actor, scope := batchRepriceTestData()
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	preview, appErr := svc.PreviewBatchReprice(ctx, actor, scope)
	if appErr != nil {
		t.Fatal(appErr)
	}
	changedScope := scope
	changedScope.SubmittedFrom = "2026-11-18"
	if _, err := svc.ConfirmBatchReprice(ctx, actor, ConfirmBatchRepriceParams{BatchRepriceParams: changedScope, PreviewToken: preview.PreviewToken}); err == nil || err.Code != domain.ErrCodeConflict {
		t.Fatalf("changed scope accepted: %+v", err)
	}
	r.beforeLock = func() {
		for _, item := range r.items {
			if item.ID == 2 {
				item.SettlementStatus = domain.AssetWorkbenchSettlementStatusInBatch
			}
		}
	}
	if _, err := svc.ConfirmBatchReprice(ctx, actor, ConfirmBatchRepriceParams{BatchRepriceParams: scope, PreviewToken: preview.PreviewToken}); err == nil || err.Code != domain.ErrCodeConflict {
		t.Fatalf("settlement race accepted: %+v", err)
	}
	if len(r.updates) != 0 || len(r.events) != 0 || len(r.refreshes) != 0 {
		t.Fatal("failed batch should roll back pricing and audit changes")
	}
	actor.EffectiveAccess = &domain.EffectiveAccess{Permissions: []domain.PermissionCode{domain.PermissionAssetWorkbenchTemplates}}
	if _, err := svc.PreviewBatchReprice(ctx, actor, scope); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("template-only actor reached repricing: %+v", err)
	}
}

func TestBatchRepricePersonnelListAndConfirmation(t *testing.T) {
	ctx := context.Background()
	r, actor, scope := batchRepriceTestData()
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	scope.PayeeUserIDs = []int64{52, 53}
	if _, err := svc.PreviewBatchReprice(ctx, actor, scope); err == nil {
		t.Fatal("mixed scopes accepted")
	}
	scope.PayeeUserID = 0
	for _, invalid := range [][]int64{{}, {52, 0}} {
		scope.PayeeUserIDs = invalid
		if _, err := svc.PreviewBatchReprice(ctx, actor, scope); err == nil {
			t.Fatal("invalid explicit selection accepted")
		}
	}
	scope.PayeeUserIDs = []int64{53, 52, 52}
	preview, err := svc.PreviewBatchReprice(ctx, actor, scope)
	if err != nil || preview.Count != 3 {
		t.Fatalf("preview=%+v error=%v", preview, err)
	}
	scope.PayeeUserIDs = []int64{52}
	if _, err := svc.ConfirmBatchReprice(ctx, actor, ConfirmBatchRepriceParams{BatchRepriceParams: scope, PreviewToken: preview.PreviewToken}); err == nil {
		t.Fatal("narrowed scope confirmed")
	}
	scope.PayeeUserIDs = []int64{52, 53}
	result, err := svc.ConfirmBatchReprice(ctx, actor, ConfirmBatchRepriceParams{BatchRepriceParams: scope, PreviewToken: preview.PreviewToken})
	if err != nil || result.Updated != 3 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func (*batchRepriceRepo) GetProfileByUserID(_ context.Context, id int64) (*domain.AssetWorkbenchProfile, error) {
	return &domain.AssetWorkbenchProfile{UserID: id, WorkerType: "fulltime", JobGrade: "P1", Status: domain.AssetWorkbenchProfileStatusActive}, nil
}

func TestBatchRepricePendingGradeAfterProfileCompleted(t *testing.T) {
	r, actor, scope := batchRepriceTestData()
	item := r.items[2]
	item.PricingStatus = domain.AssetWorkbenchPricingStatusPendingGrade
	item.JobGradeSnapshot = ""
	r.items = []*domain.AssetWorkbenchSubmissionItem{item}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	preview, err := svc.PreviewBatchReprice(context.Background(), actor, scope)
	if err != nil || preview.Count != 1 || preview.NewGross != 0.17 || len(r.updates) != 0 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
}
