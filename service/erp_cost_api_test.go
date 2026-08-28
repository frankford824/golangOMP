package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type erpCostReadRepoStub struct {
	inventoryWatermark time.Time
	feedPages          func(repo.ERPCostFeedPageQuery) []domain.ERPCostSKU
	batchItems         []domain.ERPCostSKU
	batchWatermark     time.Time
	changeWatermark    int64
	changePages        func(repo.ERPCostChangePageQuery) []domain.ERPCostChange
}

func (s *erpCostReadRepoStub) InventoryWatermark(context.Context) (time.Time, error) {
	return s.inventoryWatermark, nil
}

func (s *erpCostReadRepoStub) ListInventoryCosts(_ context.Context, query repo.ERPCostFeedPageQuery) ([]domain.ERPCostSKU, error) {
	if s.feedPages == nil {
		return []domain.ERPCostSKU{}, nil
	}
	return s.feedPages(query), nil
}

func (s *erpCostReadRepoStub) BatchInventoryCosts(context.Context, []string) ([]domain.ERPCostSKU, time.Time, error) {
	return append([]domain.ERPCostSKU(nil), s.batchItems...), s.batchWatermark, nil
}

func (s *erpCostReadRepoStub) CostChangeWatermark(context.Context) (int64, error) {
	return s.changeWatermark, nil
}

func (s *erpCostReadRepoStub) ListCostChanges(_ context.Context, query repo.ERPCostChangePageQuery) ([]domain.ERPCostChange, error) {
	if s.changePages == nil {
		return []domain.ERPCostChange{}, nil
	}
	return s.changePages(query), nil
}

type historyCostProviderStub struct{ periods []domain.JSTHistoryCostPeriod }

func (s historyCostProviderStub) QueryHistoryCosts(_ context.Context, query domain.JSTHistoryCostQuery) (*domain.JSTHistoryCostResponse, error) {
	wanted := make(map[string]struct{}, len(query.SKUIDs))
	for _, skuID := range query.SKUIDs {
		wanted[skuID] = struct{}{}
	}
	periods := make([]domain.JSTHistoryCostPeriod, 0)
	for _, period := range s.periods {
		if _, ok := wanted[period.SKUID]; ok {
			periods = append(periods, period)
		}
	}
	return &domain.JSTHistoryCostResponse{Periods: periods}, nil
}

func TestERPCostFeedKeepsWatermarkAcrossSignedCursorPages(t *testing.T) {
	watermark := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	t1 := watermark.Add(-2 * time.Hour)
	t2 := watermark.Add(-time.Hour)
	cost1, cost2, cost3 := "5.3", "6.2500", "7"
	page := 0
	repoStub := &erpCostReadRepoStub{
		inventoryWatermark: watermark,
		feedPages: func(query repo.ERPCostFeedPageQuery) []domain.ERPCostSKU {
			if !query.Watermark.Equal(watermark) {
				t.Fatalf("watermark = %s, want %s", query.Watermark, watermark)
			}
			page++
			if page == 1 {
				if query.LastSKUID != "" {
					t.Fatalf("first page last sku = %q", query.LastSKUID)
				}
				return []domain.ERPCostSKU{
					{SKUID: "SKU-1", CostPrice: &cost1, ModifiedAt: t1},
					{SKUID: "SKU-2", CostPrice: &cost2, ModifiedAt: t2},
					{SKUID: "SKU-3", CostPrice: &cost3, ModifiedAt: watermark},
				}
			}
			if query.LastSKUID != "SKU-2" || !query.LastModifiedAt.Equal(t2) {
				t.Fatalf("cursor position = %s/%s", query.LastModifiedAt, query.LastSKUID)
			}
			return []domain.ERPCostSKU{{SKUID: "SKU-3", CostPrice: &cost3, ModifiedAt: watermark}}
		},
	}
	svc := NewERPCostAPIService(repoStub, nil, "cost-secret")
	first, appErr := svc.Feed(context.Background(), "2026-08-26T00:00:00Z", "", 2)
	if appErr != nil {
		t.Fatalf("first Feed() appErr = %+v", appErr)
	}
	if len(first.Data) != 2 || first.NextCursor == "" || *first.Data[0].CostPrice != "5.3000" {
		t.Fatalf("first result = %+v", first)
	}
	second, appErr := svc.Feed(context.Background(), "2026-08-26T00:00:00Z", first.NextCursor, 2)
	if appErr != nil {
		t.Fatalf("second Feed() appErr = %+v", appErr)
	}
	if len(second.Data) != 1 || second.Data[0].SKUID != "SKU-3" || second.NextCursor != "" {
		t.Fatalf("second result = %+v", second)
	}
	if first.SnapshotVersion != second.SnapshotVersion {
		t.Fatalf("snapshot changed across pages: %s != %s", first.SnapshotVersion, second.SnapshotVersion)
	}
}

func TestERPCostBatchQueryPreservesInputOrderAndMissingIDs(t *testing.T) {
	watermark := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	costA, costB := "1.2", "2.3456"
	svc := NewERPCostAPIService(&erpCostReadRepoStub{
		batchWatermark: watermark,
		batchItems: []domain.ERPCostSKU{
			{SKUID: "SKU-A", CostPrice: &costA, ModifiedAt: watermark},
			{SKUID: "sku-b", CostPrice: &costB, ModifiedAt: watermark},
		},
	}, nil, "cost-secret")
	result, appErr := svc.BatchQuery(context.Background(), []string{"SKU-B", "MISSING", "SKU-A", "SKU-B"})
	if appErr != nil {
		t.Fatalf("BatchQuery() appErr = %+v", appErr)
	}
	if len(result.Data) != 2 || result.Data[0].SKUID != "sku-b" || result.Data[1].SKUID != "SKU-A" {
		t.Fatalf("data order = %+v", result.Data)
	}
	if len(result.MissingSKUIDs) != 1 || result.MissingSKUIDs[0] != "MISSING" {
		t.Fatalf("missing = %+v", result.MissingSKUIDs)
	}
	if *result.Data[1].CostPrice != "1.2000" {
		t.Fatalf("cost precision = %q", *result.Data[1].CostPrice)
	}
}

func TestERPCostHistorySelectsEffectivePeriodPerWarehouse(t *testing.T) {
	costOld, costNew, costOther := "4.5", "5.3619", "6"
	svc := NewERPCostAPIService(&erpCostReadRepoStub{}, historyCostProviderStub{periods: []domain.JSTHistoryCostPeriod{
		{WMSCoID: "1", SKUID: "COMBO-1", CostPrice: &costOld, BeginDate: "2026-01-01", EndDate: "2026-05-31"},
		{WMSCoID: "1", SKUID: "COMBO-1", CostPrice: &costNew, BeginDate: "2026-06-01"},
		{WMSCoID: "2", SKUID: "COMBO-1", CostPrice: &costOther, BeginDate: "2026-01-01"},
	}}, "cost-secret")
	result, appErr := svc.History(context.Background(), []string{"COMBO-1", "MISSING"}, "2026-08-27", nil)
	if appErr != nil {
		t.Fatalf("History() appErr = %+v", appErr)
	}
	if len(result.Data) != 2 || *result.Data[0].CostPrice != "5.3619" || *result.Data[1].CostPrice != "6.0000" {
		t.Fatalf("history data = %+v", result.Data)
	}
	if len(result.MissingSKUIDs) != 1 || result.MissingSKUIDs[0] != "MISSING" {
		t.Fatalf("missing = %+v", result.MissingSKUIDs)
	}
}

func TestERPCostChangesKeepsIDWatermarkAndFourDecimals(t *testing.T) {
	changedAt := time.Date(2026, 8, 27, 13, 0, 0, 0, time.UTC)
	oldCost, newCost := "5", "5.3619"
	page := 0
	svc := NewERPCostAPIService(&erpCostReadRepoStub{
		changeWatermark: 12,
		changePages: func(query repo.ERPCostChangePageQuery) []domain.ERPCostChange {
			if query.WatermarkID != 12 {
				t.Fatalf("watermark id = %d", query.WatermarkID)
			}
			page++
			if page == 1 {
				return []domain.ERPCostChange{
					{ID: 10, SKUID: "SKU-1", OldCostPrice: &oldCost, NewCostPrice: &newCost, ChangedAt: changedAt},
					{ID: 11, SKUID: "SKU-2", OldCostPrice: &oldCost, NewCostPrice: &newCost, ChangedAt: changedAt},
				}
			}
			if query.LastID != 10 {
				t.Fatalf("last id = %d, want 10", query.LastID)
			}
			return []domain.ERPCostChange{{ID: 11, SKUID: "SKU-2", OldCostPrice: &oldCost, NewCostPrice: &newCost, ChangedAt: changedAt}}
		},
	}, nil, "cost-secret")
	first, appErr := svc.Changes(context.Background(), "2026-08-27T00:00:00Z", "", 1)
	if appErr != nil {
		t.Fatalf("first Changes() appErr = %+v", appErr)
	}
	if len(first.Data) != 1 || first.NextCursor == "" || *first.Data[0].OldCostPrice != "5.0000" {
		t.Fatalf("first result = %+v", first)
	}
	second, appErr := svc.Changes(context.Background(), "2026-08-27T00:00:00Z", first.NextCursor, 1)
	if appErr != nil {
		t.Fatalf("second Changes() appErr = %+v", appErr)
	}
	if len(second.Data) != 1 || second.Data[0].ID != 11 || second.SnapshotVersion != first.SnapshotVersion {
		t.Fatalf("second result = %+v", second)
	}
}
