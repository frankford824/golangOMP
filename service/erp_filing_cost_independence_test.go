package service

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"workflow/domain"
)

func TestNewSKUFilesBeforeCostConfirmation(t *testing.T) {
	for _, batch := range []bool{false, true} {
		name := "single"
		if batch {
			name = "batch"
		}
		t.Run(name, func(t *testing.T) {
			task := &domain.Task{ID: 7562, TaskNo: "RW-TEST", TaskType: domain.TaskTypeNewProductDevelopment,
				SourceMode: domain.TaskSourceModeNewProduct, SKUCode: "CGK004864", ProductNameSnapshot: "国庆立牌", IsBatchTask: batch}
			detail := &domain.TaskDetail{TaskID: task.ID, Category: "常规kt板", RequiresManualReview: true}
			item := &domain.TaskSKUItem{ID: 10284, TaskID: task.ID, SKUCode: task.SKUCode,
				ProductNameSnapshot: task.ProductNameSnapshot, ProductIID: detail.Category, RequiresManualReview: true}
			r := &prdTaskRepo{skuItems: map[int64][]*domain.TaskSKUItem{task.ID: {item}}}
			svc := &taskService{taskRepo: r}
			build := func() domain.ERPProductUpsertPayload {
				t.Helper()
				payloads, missing, summary, appErr := svc.buildTaskERPBridgeFilingPayloads(context.Background(), task, detail, 1, "", string(TaskFilingTriggerSourceCreate), false, nil)
				if appErr != nil || len(missing) != 0 || len(payloads) != 1 {
					t.Fatalf("filing blocked: payloads=%d missing=%v summary=%s err=%+v", len(payloads), missing, summary, appErr)
				}
				if payloads[0].Payload.SKUID != task.SKUCode {
					t.Fatalf("wrong SKU: %+v", payloads[0].Payload)
				}
				return payloads[0].Payload
			}
			// Missing cost must not block identity creation.
			assertFilingCost(t, build(), nil)
			// A provisional positive cost must also be omitted, including the nested snapshot.
			item.CostPrice = float64Ptr(38)
			assertFilingCost(t, build(), nil)
			// Once manually confirmed, the same SKU can carry the cost on its next sync.
			item.ManualCostOverride = true
			assertFilingCost(t, build(), item.CostPrice)
			if !item.RequiresManualReview || !detail.RequiresManualReview || detail.CostPrice != nil {
				t.Fatal("filing mutated source pricing state")
			}
		})
	}
}

func assertFilingCost(t *testing.T, payload domain.ERPProductUpsertPayload, expected *float64) {
	t.Helper()
	if payload.BusinessInfo == nil {
		t.Fatal("missing business snapshot")
	}
	for _, actual := range []*float64{payload.CostPrice, payload.BusinessInfo.CostPrice} {
		if expected == nil {
			if actual != nil {
				t.Fatalf("unconfirmed cost leaked: %v", *actual)
			}
		} else if actual == nil || *actual != *expected {
			t.Fatalf("confirmed cost not retained: got=%v want=%v", actual, *expected)
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if expected == nil && strings.Contains(string(encoded), `"cost_price"`) {
		t.Fatalf("unconfirmed cost must be absent from JSON: %s", encoded)
	}
}

func TestProductManagementBaseSyncIndependentOfUnconfirmedCost(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		cost                 *float64
		review, manual, send bool
	}{
		{name: "missing", review: true},
		{name: "provisional", cost: float64Ptr(38), review: true},
		{name: "invalid", cost: float64Ptr(math.NaN()), manual: true},
		{name: "automatic_confirmed", cost: float64Ptr(38), send: true},
		{name: "manual_confirmed", cost: float64Ptr(38), review: true, manual: true, send: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Existing ERP cost differs from provisional local cost; a base-only
			// write must neither overwrite it nor demand a cost readback match.
			erpCost := float64Ptr(12)
			if tc.send {
				erpCost = tc.cost
			}
			bridge := &productManagementERPBridgeCapture{readbackProduct: &domain.ERPProduct{
				SKUCode: "CGK004864", IID: "常规kt板", ProductName: "国庆立牌", CostPrice: erpCost,
			}}
			svc := &productManagementService{erpBridge: bridge}
			record := &domain.ProductManagementRecord{ID: 245631, TaskID: 7562, SKUCode: "CGK004864",
				ProductIID: "常规kt板", ProductName: "国庆立牌", CostPrice: tc.cost,
				CostTrace: &domain.ProductManagementCostTrace{RequiresManualReview: tc.review, ManualCostOverride: tc.manual}}
			if appErr := svc.syncBaseRecordToERP(context.Background(), record); appErr != nil {
				t.Fatalf("base sync failed: %+v", appErr)
			}
			if bridge.upsertCalls != 1 {
				t.Fatalf("upsert calls=%d", bridge.upsertCalls)
			}
			if tc.send {
				if bridge.payload.CostPrice == nil || *bridge.payload.CostPrice != *tc.cost {
					t.Fatal("confirmed cost missing")
				}
			} else if bridge.payload.CostPrice != nil {
				t.Fatal("unconfirmed cost sent to ERP")
			}
		})
	}
}
