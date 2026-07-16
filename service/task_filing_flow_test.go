package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestTriggerFilingSkipsDuplicatePayload(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			3: {
				ID:                  3,
				TaskNo:              "RW-003",
				SourceMode:          domain.TaskSourceModeExistingProduct,
				SKUCode:             "SKU-003",
				ProductNameSnapshot: "Original Product 3",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingWarehouseReceive,
			},
		},
		details: map[int64]*domain.TaskDetail{
			3: {
				TaskID:       3,
				CategoryCode: "CAT-3",
				SpecText:     "spec-3",
				CostPrice:    float64Ptr(39.9),
				ProductSelection: &domain.TaskProductSelectionContext{
					ERPProduct: &domain.ERPProductSelectionSnapshot{
						ProductID:   "ERP-3",
						SKUID:       "SKU-003",
						SKUCode:     "SKU-003",
						ProductName: "Original Product 3",
					},
				},
			},
		},
	}
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{ProductID: "ERP-3", SKUID: "SKU-003"},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	_, appErr := svc.TriggerFiling(context.Background(), TriggerTaskFilingParams{
		TaskID:     3,
		OperatorID: 7,
		Source:     TaskFilingTriggerSourceAuditFinalApproved,
	})
	if appErr != nil {
		t.Fatalf("TriggerFiling(first) unexpected error: %+v", appErr)
	}
	_, appErr = svc.TriggerFiling(context.Background(), TriggerTaskFilingParams{
		TaskID:     3,
		OperatorID: 7,
		Source:     TaskFilingTriggerSourceWarehouseCompletePrechk,
	})
	if appErr != nil {
		t.Fatalf("TriggerFiling(second) unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", bridgeStub.upsertCalls)
	}
}

func TestRetryFilingAfterFailure(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			4: {
				ID:                  4,
				TaskNo:              "RW-004",
				SourceMode:          domain.TaskSourceModeExistingProduct,
				SKUCode:             "SKU-004",
				ProductNameSnapshot: "Original Product 4",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingWarehouseReceive,
			},
		},
		details: map[int64]*domain.TaskDetail{
			4: {
				TaskID:       4,
				CategoryCode: "CAT-4",
				SpecText:     "spec-4",
				CostPrice:    float64Ptr(49.9),
				ProductSelection: &domain.TaskProductSelectionContext{
					ERPProduct: &domain.ERPProductSelectionSnapshot{
						ProductID:   "ERP-4",
						SKUID:       "SKU-004",
						SKUCode:     "SKU-004",
						ProductName: "Original Product 4",
					},
				},
			},
		},
	}
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertAppErr: domain.NewAppError(domain.ErrCodeInternalError, "bridge down", nil),
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	view, appErr := svc.TriggerFiling(context.Background(), TriggerTaskFilingParams{
		TaskID:     4,
		OperatorID: 9,
		Source:     TaskFilingTriggerSourceAuditFinalApproved,
	})
	if appErr != nil {
		t.Fatalf("TriggerFiling() unexpected error: %+v", appErr)
	}
	if view.FilingStatus != domain.FilingStatusFilingFailed {
		t.Fatalf("filing_status = %s, want filing_failed", view.FilingStatus)
	}

	bridgeStub.upsertAppErr = nil
	bridgeStub.upsertResult = &domain.ERPProductUpsertResult{ProductID: "ERP-4", SKUID: "SKU-004"}
	retried, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{
		TaskID:     4,
		OperatorID: 9,
		Remark:     "retry",
	})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if retried.FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("retry filing_status = %s, want filed", retried.FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
}

func TestBatchNewProductFilingUsesPerSKUProductIID(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{
			{IID: "I-1001", Label: "I-1001"},
			{IID: "I-1002", Label: "I-1002"},
		},
		upsertResult: &domain.ERPProductUpsertResult{Status: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:        domain.TaskTypeNewProductDevelopment,
		SourceMode:      domain.TaskSourceModeNewProduct,
		CreatorID:       11,
		OwnerTeam:       "设计组",
		DeadlineAt:      timePtr(),
		BatchSKUMode:    "multiple",
		SyncERPOnCreate: true,
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Batch A",
				DesignRequirement: "draw A",
				ProductIID:        "I-1001",
				CostPrice:         float64Ptr(5.1),
			},
			{
				ProductName:       "Batch B",
				DesignRequirement: "draw B",
				ProductIID:        "I-1002",
				CostPrice:         float64Ptr(6.2),
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
	if bridgeStub.upsertPayloads[0].IID != "I-1001" || bridgeStub.upsertPayloads[1].IID != "I-1002" {
		t.Fatalf("upsert iids = %s/%s, want I-1001/I-1002", bridgeStub.upsertPayloads[0].IID, bridgeStub.upsertPayloads[1].IID)
	}
	if got := bridgeStub.upsertPayloads[0].SPrice; got == nil || *got != 0 {
		t.Fatalf("batch upsert[0] s_price = %v, want explicit 0 to avoid ERP default sale price", got)
	}
	if got := bridgeStub.upsertPayloads[1].SPrice; got == nil || *got != 0 {
		t.Fatalf("batch upsert[1] s_price = %v, want explicit 0 to avoid ERP default sale price", got)
	}
	if got := bridgeStub.upsertPayloads[0].BusinessInfo.CostPrice; got == nil || *got != 5.1 {
		t.Fatalf("batch upsert[0] business_info.cost_price = %v, want 5.1", got)
	}
	if got := bridgeStub.upsertPayloads[1].BusinessInfo.CostPrice; got == nil || *got != 6.2 {
		t.Fatalf("batch upsert[1] business_info.cost_price = %v, want 6.2", got)
	}
	items := taskRepo.skuItems[task.ID]
	if len(items) != 2 || items[0].ProductIID != "I-1001" || items[1].ProductIID != "I-1002" {
		t.Fatalf("sku item product_i_id = %+v", items)
	}
}

func TestNewProductCreateExplicitSyncERPFalseSkipsCreateFiling(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		TopLevelNewSKU:      "NEW-SKU-OPT-OUT",
		ProductNameSnapshot: "Opt-out New Product",
		DesignRequirement:   "draw opt-out",
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductIID:          "定制海报",
		CostPriceMode:       string(domain.CostPriceModeManual),
		CostPrice:           float64Ptr(12.3),
		SyncERPOnCreateSet:  true,
		SyncERPOnCreate:     false,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 0 {
		t.Fatalf("upsert calls after create = %d, want 0", bridgeStub.upsertCalls)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusNotFiled {
		t.Fatalf("filing_status after create = %s, want not_filed", taskRepo.details[task.ID].FilingStatus)
	}
}

func TestBatchNewProductCreateExplicitSyncERPFalseSkipsCreateFiling(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{
			{IID: "I-1001", Label: "I-1001"},
			{IID: "I-1002", Label: "I-1002"},
		},
		upsertResult: &domain.ERPProductUpsertResult{Status: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:           domain.TaskTypeNewProductDevelopment,
		SourceMode:         domain.TaskSourceModeNewProduct,
		CreatorID:          11,
		OwnerTeam:          domain.AllValidTeams()[0],
		DeadlineAt:         timePtr(),
		BatchSKUMode:       "multiple",
		SyncERPOnCreateSet: true,
		SyncERPOnCreate:    false,
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Batch Opt-out A",
				DesignRequirement: "draw A",
				ProductIID:        "I-1001",
				CostPrice:         float64Ptr(5.1),
			},
			{
				ProductName:       "Batch Opt-out B",
				DesignRequirement: "draw B",
				ProductIID:        "I-1002",
				CostPrice:         float64Ptr(6.2),
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 0 {
		t.Fatalf("upsert calls after batch create = %d, want 0", bridgeStub.upsertCalls)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusNotFiled {
		t.Fatalf("filing_status after batch create = %s, want not_filed", taskRepo.details[task.ID].FilingStatus)
	}
	for _, item := range taskRepo.skuItems[task.ID] {
		if item.FilingStatus != domain.FilingStatusNotFiled {
			t.Fatalf("sku item filing_status = %s, want not_filed", item.FilingStatus)
		}
	}
}

func TestBatchSKUItemInfoCanBeUpdatedAfterAuditStarted(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			1373: {
				ID:             1373,
				TaskNo:         "RW-20260611-A-001370",
				TaskType:       domain.TaskTypeNewProductDevelopment,
				TaskStatus:     domain.TaskStatusPendingAuditA,
				IsBatchTask:    true,
				BatchItemCount: 3,
				BatchMode:      domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			1373: {TaskID: 1373},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			1373: {
				{
					ID:                  1371,
					TaskID:              1373,
					SequenceNo:          1,
					SKUCode:             "CGO000137",
					ProductNameSnapshot: "常规海报/升学宴//5条",
					DesignRequirement:   "旧需求",
					VariantJSON:         []byte(`{"product_i_id":"常规海报"}`),
					FilingStatus:        domain.FilingStatusFiled,
					ERPSyncStatus:       domain.FilingStatusFiled,
				},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
	)

	name := "常规海报/升学宴/更新后/5条"
	iid := "常规海报"
	requirement := "更新后的设计要求"
	updated, appErr := svc.UpdateSKUItemInfo(context.Background(), UpdateTaskSKUItemInfoParams{
		TaskID:            1373,
		SKUItemID:         1371,
		OperatorID:        11,
		ProductName:       &name,
		ProductIID:        &iid,
		DesignRequirement: &requirement,
		Remark:            "维护子项商品资料",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemInfo() unexpected error: %+v", appErr)
	}
	if updated.ProductNameSnapshot != name || updated.DesignRequirement != requirement || taskSKUItemProductIID(updated) != iid {
		t.Fatalf("updated sku item = %+v product_i_id=%q", updated, taskSKUItemProductIID(updated))
	}
}

func TestBatchNewProductCreateSyncAllowsMissingCost(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{
			{IID: "I-1001", Label: "I-1001"},
			{IID: "I-1002", Label: "I-1002"},
		},
		upsertResult: &domain.ERPProductUpsertResult{Status: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:        domain.TaskTypeNewProductDevelopment,
		SourceMode:      domain.TaskSourceModeNewProduct,
		CreatorID:       11,
		OwnerTeam:       domain.AllValidTeams()[0],
		DeadlineAt:      timePtr(),
		BatchSKUMode:    "multiple",
		SyncERPOnCreate: true,
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Batch A",
				DesignRequirement: "draw A",
				ProductIID:        "I-1001",
				CostPrice:         float64Ptr(5.1),
			},
			{
				ProductName:       "Batch B",
				DesignRequirement: "draw B",
				ProductIID:        "I-1002",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after create = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls after create = %d, want 2", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayloads[0].BusinessInfo.CostPrice; got == nil || *got != 5.1 {
		t.Fatalf("batch upsert[0] business_info.cost_price = %v, want 5.1", got)
	}
	if got := bridgeStub.upsertPayloads[1].BusinessInfo.CostPrice; got != nil {
		t.Fatalf("batch upsert[1] business_info.cost_price = %v, want nil for unknown cost", got)
	}
	if got := bridgeStub.upsertPayloads[1].CostPrice; got != nil {
		t.Fatalf("batch upsert[1] cost_price = %v, want nil for unknown cost", got)
	}
}

func legacyPurchaseFilingDoesNotRegressToPendingWhenBaseSalePriceMissingAfterCreateSync(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "NSCK000000",
		ProductNameSnapshot: "上线前采购单SKU任务",
		ProductIID:          "定制海报",
		CostPriceMode:       string(domain.CostPriceModeManual),
		CostPrice:           float64Ptr(22),
		Quantity:            int64Ptr(22),
		SyncERPOnCreate:     true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after create = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}

	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "上线前采购单SKU任务",
		ProductIID:         "定制海报",
		Category:           "定制海报",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(22),
		ManualCostOverride: true,
		Quantity:           int64Ptr(22),
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after business-info patch = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayloads[0].SPrice; got == nil || *got != 0 {
		t.Fatalf("create upsert s_price = %v, want explicit 0 to avoid ERP default sale price", got)
	}
	if got := bridgeStub.upsertPayloads[1].SPrice; got == nil || *got != 0 {
		t.Fatalf("refile upsert s_price = %v, want explicit 0 to avoid ERP default sale price", got)
	}
}

func legacyPurchaseUpdateBusinessInfoCostChangeRefilesERPAndAppendsCostEvent(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "NSCK000000",
		ProductNameSnapshot: "上线前采购单SKU任务",
		ProductIID:          "定制海报",
		CostPriceMode:       string(domain.CostPriceModeManual),
		CostPrice:           float64Ptr(22),
		Quantity:            int64Ptr(22),
		SyncERPOnCreate:     true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:                   task.ID,
		OperatorID:               11,
		ProductName:              "上线前采购单SKU任务",
		ProductIID:               "定制海报",
		Category:                 "定制海报",
		SpecText:                 "20*20",
		CostPrice:                float64Ptr(25),
		ManualCostOverride:       true,
		ManualCostOverrideReason: "仓库维护成本价",
		Quantity:                 int64Ptr(22),
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayloads[1].BusinessInfo.CostPrice; got == nil || *got != 25 {
		t.Fatalf("refile cost_price = %v, want 25", got)
	}

	var costEvent *domain.TaskEvent
	for _, event := range eventRepo.events {
		if event.EventType == domain.TaskEventCostUpdated {
			costEvent = event
			break
		}
	}
	if costEvent == nil {
		t.Fatalf("events = %+v, want cost updated event", eventRepo.events)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(costEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal cost event payload: %v", err)
	}
	if payload["manual_cost_override_reason"] != "仓库维护成本价" || payload["erp_sync_requested"] != true {
		t.Fatalf("cost event payload = %#v, want reason and erp_sync_requested", payload)
	}
}

func TestUpdateSingleSKUItemCostSyncsTaskDetailAndERPRefilingCost(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "铜版纸", Label: "铜版纸"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			692: {
				ID:                  692,
				TaskNo:              "RW-20260513-A-000689",
				SourceMode:          domain.TaskSourceModeNewProduct,
				SKUCode:             "NSCO000000",
				PrimarySKUCode:      "NSCO000000",
				ProductNameSnapshot: "真/常规250g铜版纸/双面/红字拱圆形桌牌/主桌1张/10*15cm",
				TaskType:            domain.TaskTypeNewProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			692: {
				TaskID:       692,
				Category:     "铜版纸",
				CategoryCode: "COPPER_PAPER",
				CategoryName: "铜版纸",
				SpecText:     "10*15cm",
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			692: {
				{
					ID:                  526,
					TaskID:              692,
					SequenceNo:          1,
					SKUCode:             "NSCO000000",
					ProductNameSnapshot: "真/常规250g铜版纸/双面/红字拱圆形桌牌/主桌1张/10*15cm",
					CategoryCode:        "COPPER_PAPER",
				},
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	updated, appErr := svc.UpdateSKUItemCostInfo(context.Background(), UpdateTaskSKUItemCostInfoParams{
		TaskID:             692,
		SKUItemID:          526,
		OperatorID:         1,
		CostPrice:          float64Ptr(0.6),
		ManualCostOverride: true,
		Remark:             "test single sku item cost",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemCostInfo() unexpected error: %+v", appErr)
	}
	if updated.CostPrice == nil || *updated.CostPrice != 0.6 {
		t.Fatalf("updated sku cost = %+v, want 0.6", updated.CostPrice)
	}
	if got := taskRepo.details[692].CostPrice; got == nil || *got != 0.6 {
		t.Fatalf("task detail cost = %+v, want synced 0.6", got)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want 1", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.BusinessInfo.CostPrice; got == nil || *got != 0.6 {
		t.Fatalf("erp business_info cost_price = %+v, want 0.6", got)
	}
	if got := bridgeStub.upsertPayload.CostPrice; got == nil || *got != 0.6 {
		t.Fatalf("erp top-level cost_price = %+v, want 0.6", got)
	}
}

func TestUpdateBatchSKUItemCostOnlyRefilesTargetSKU(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			703: {
				ID:                  703,
				TaskNo:              "RW-20260605-A-001143",
				SourceMode:          domain.TaskSourceModeNewProduct,
				ProductNameSnapshot: "CPT-定制海报/暑假班",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				IsBatchTask:         true,
				BatchItemCount:      3,
				BatchMode:           domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			703: {
				TaskID:             703,
				Category:           "GENERAL",
				CategoryCode:       "GENERAL",
				CategoryName:       "通用",
				FilingStatus:       domain.FilingStatusFilingFailed,
				FilingErrorMessage: "部分SKU同步失败",
				ERPSyncRequired:    true,
				ERPSyncVersion:     2,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			703: {
				{ID: 1001, TaskID: 703, SequenceNo: 1, SKUCode: "DZG000101", ProductNameSnapshot: "CPT-定制海报/暑假班/130*150cm", ProductIID: "定制海报", CostPrice: float64Ptr(10.725), FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true, FilingErrorMessage: "ERP频控"},
				{ID: 1002, TaskID: 703, SequenceNo: 2, SKUCode: "DZG000102", ProductNameSnapshot: "CPT-定制海报/暑假班/150*200cm", ProductIID: "定制海报", CostPrice: float64Ptr(16.5), FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true, FilingErrorMessage: "ERP频控"},
				{ID: 1003, TaskID: 703, SequenceNo: 3, SKUCode: "DZG000103", ProductNameSnapshot: "CPT-定制海报/暑假班/150*250cm", ProductIID: "定制海报", CostPrice: float64Ptr(20.625), FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	updated, appErr := svc.UpdateSKUItemCostInfo(context.Background(), UpdateTaskSKUItemCostInfoParams{
		TaskID:                   703,
		SKUItemID:                1002,
		OperatorID:               1,
		CostPrice:                float64Ptr(18.8),
		ManualCostOverride:       true,
		ManualCostOverrideReason: "运营单行修正成本",
		Remark:                   "target cost sync",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemCostInfo() unexpected error: %+v", appErr)
	}
	if updated.CostPrice == nil || *updated.CostPrice != 18.8 {
		t.Fatalf("updated cost = %+v, want 18.8", updated.CostPrice)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want only target SKU refiled", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.SKUID; got != "DZG000102" {
		t.Fatalf("upsert sku = %s, want DZG000102", got)
	}
	items := taskRepo.skuItems[703]
	if items[1].FilingStatus != domain.FilingStatusFiled || items[1].ERPSyncRequired {
		t.Fatalf("target item filing = %s required=%t, want filed false", items[1].FilingStatus, items[1].ERPSyncRequired)
	}
	if items[0].FilingStatus != domain.FilingStatusFilingFailed || !items[0].ERPSyncRequired {
		t.Fatalf("non-target failed item changed: status=%s required=%t", items[0].FilingStatus, items[0].ERPSyncRequired)
	}
	if taskRepo.details[703].FilingStatus != domain.FilingStatusFilingFailed {
		t.Fatalf("task detail filing_status = %s, want aggregated filing_failed", taskRepo.details[703].FilingStatus)
	}
	if !strings.Contains(taskRepo.details[703].FilingErrorMessage, "DZG000101") {
		t.Fatalf("task detail filing error = %q, want remaining failed SKU", taskRepo.details[703].FilingErrorMessage)
	}
}

func TestUpdateBatchSKUItemInfoRecomputesCostAndRefilesTargetSKU(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   77,
		CategoryCode: "PHOTO_CLOTH_CUSTOM",
		CategoryName: "定制写真布",
		DisplayName:  "定制写真布",
		CategoryType: domain.CategoryTypeCloth,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        7701,
			RuleVersion:   1,
			RuleName:      "定制写真布基础单价",
			CategoryCode:  "PHOTO_CLOTH_CUSTOM",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     float64Ptr(5),
			TaxMultiplier: float64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "phase_021_test",
		},
	}
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			704: {
				ID:                  704,
				TaskNo:              "RW-20260624-A-001740",
				SourceMode:          domain.TaskSourceModeNewProduct,
				ProductNameSnapshot: "CPT-定制海报/批量",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				IsBatchTask:         true,
				BatchItemCount:      3,
				BatchMode:           domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			704: {
				TaskID:             704,
				Category:           "GENERAL",
				CategoryCode:       "GENERAL",
				CategoryName:       "通用",
				FilingStatus:       domain.FilingStatusFilingFailed,
				FilingErrorMessage: "部分SKU同步失败",
				ERPSyncRequired:    true,
				ERPSyncVersion:     2,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			704: {
				{ID: 2001, TaskID: 704, SequenceNo: 1, SKUCode: "DZG000201", ProductNameSnapshot: "CPT-定制海报/旧/130*150cm", ProductIID: "定制海报", CostPrice: float64Ptr(10.725), FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true, FilingErrorMessage: "ERP频控"},
				{ID: 2002, TaskID: 704, SequenceNo: 2, SKUCode: "DZC000011", ProductNameSnapshot: "CPT-定制海报/旧", ProductIID: "定制海报", FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true, FilingErrorMessage: "ERP频控"},
				{ID: 2003, TaskID: 704, SequenceNo: 3, SKUCode: "DZG000203", ProductNameSnapshot: "CPT-定制海报/旧/150*250cm", ProductIID: "定制海报", CostPrice: float64Ptr(20.625), FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		costRuleRepo,
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	name := "露邱/定制海报/4rdhappybirthday白底西瓜彩条/100*150cm"
	updated, appErr := svc.UpdateSKUItemInfo(context.Background(), UpdateTaskSKUItemInfoParams{
		TaskID:      704,
		SKUItemID:   2002,
		OperatorID:  1,
		ProductName: &name,
		Remark:      "batch sku info cost refresh",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemInfo() unexpected error: %+v", appErr)
	}
	if updated.CostPrice == nil || !sameFloat64Ptr(updated.CostPrice, float64Ptr(8.25)) {
		if updated.CostPrice == nil {
			t.Fatalf("updated cost = nil, want 8.25")
		}
		t.Fatalf("updated cost = %.6f, want 8.25", *updated.CostPrice)
	}
	if updated.EstimatedCost == nil || !sameFloat64Ptr(updated.EstimatedCost, float64Ptr(8.25)) {
		if updated.EstimatedCost == nil {
			t.Fatalf("updated estimated cost = nil, want 8.25")
		}
		t.Fatalf("updated estimated cost = %.6f, want 8.25", *updated.EstimatedCost)
	}
	if updated.CostRuleName != "定制写真布基础单价" {
		t.Fatalf("cost_rule_name = %q, want 定制写真布基础单价", updated.CostRuleName)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want only target SKU refiled", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.SKUID; got != "DZC000011" {
		t.Fatalf("upsert sku = %s, want DZC000011", got)
	}
	if got := bridgeStub.upsertPayload.BusinessInfo.CostPrice; got == nil || !sameFloat64Ptr(got, float64Ptr(8.25)) {
		t.Fatalf("erp business_info cost_price = %+v, want 8.25", got)
	}
	items := taskRepo.skuItems[704]
	if items[1].FilingStatus != domain.FilingStatusFiled || items[1].ERPSyncRequired {
		t.Fatalf("target item filing = %s required=%t, want filed false", items[1].FilingStatus, items[1].ERPSyncRequired)
	}
	if items[0].FilingStatus != domain.FilingStatusFilingFailed || !items[0].ERPSyncRequired {
		t.Fatalf("non-target failed item changed: status=%s required=%t", items[0].FilingStatus, items[0].ERPSyncRequired)
	}
	var costEvent *domain.TaskEvent
	for _, event := range eventRepo.events {
		if event.EventType == domain.TaskEventCostUpdated {
			costEvent = event
			break
		}
	}
	if costEvent == nil {
		t.Fatalf("events = %+v, want cost updated event", eventRepo.events)
	}
}

func TestUpdateBatchSKUItemInfoRecomputesCostFromSubmittedSpec(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   77,
		CategoryCode: "PHOTO_CLOTH_CUSTOM",
		CategoryName: "定制写真布",
		DisplayName:  "定制写真布",
		CategoryType: domain.CategoryTypeCloth,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:        16,
			RuleVersion:   1,
			RuleName:      "定制写真布基础单价",
			CategoryCode:  "PHOTO_CLOTH_CUSTOM",
			RuleType:      domain.CostRuleTypeFixedUnitPrice,
			BasePrice:     float64Ptr(5),
			TaxMultiplier: float64Ptr(1.1),
			Priority:      10,
			IsActive:      true,
			Source:        "phase_021_test",
		},
	}
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "定制海报", Label: "定制海报"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			705: {
				ID:                  705,
				TaskNo:              "RW-20260624-A-001741",
				SourceMode:          domain.TaskSourceModeNewProduct,
				ProductNameSnapshot: "CPT-定制海报/批量",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				IsBatchTask:         true,
				BatchItemCount:      2,
				BatchMode:           domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			705: {
				TaskID:       705,
				Category:     "GENERAL",
				CategoryCode: "GENERAL",
				CategoryName: "通用",
				FilingStatus: domain.FilingStatusFiled,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			705: {
				{ID: 2011, TaskID: 705, SequenceNo: 1, SKUCode: "DZC000111", ProductNameSnapshot: "露邱/定制海报/4rdhappybirthday白底西瓜彩条", ProductIID: "定制海报", FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
				{ID: 2012, TaskID: 705, SequenceNo: 2, SKUCode: "DZC000112", ProductNameSnapshot: "露邱/定制海报/另一款", ProductIID: "定制海报", FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		costRuleRepo,
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	specText := "100*150cm"
	updated, appErr := svc.UpdateSKUItemInfo(context.Background(), UpdateTaskSKUItemInfoParams{
		TaskID:     705,
		SKUItemID:  2011,
		OperatorID: 1,
		SpecText:   &specText,
		Remark:     "batch sku spec cost refresh",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemInfo() unexpected error: %+v", appErr)
	}
	if updated.CostPrice == nil || !sameFloat64Ptr(updated.CostPrice, float64Ptr(8.25)) {
		t.Fatalf("updated cost = %+v, want 8.25", updated.CostPrice)
	}
	if updated.EstimatedCost == nil || !sameFloat64Ptr(updated.EstimatedCost, float64Ptr(8.25)) {
		t.Fatalf("updated estimated cost = %+v, want 8.25", updated.EstimatedCost)
	}
	if !strings.Contains(string(updated.VariantJSON), `"spec_text":"100*150cm"`) {
		t.Fatalf("variant_json = %s, want submitted spec_text", string(updated.VariantJSON))
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want only target SKU refiled", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.SKUID; got != "DZC000111" {
		t.Fatalf("upsert sku = %s, want DZC000111", got)
	}
	if got := bridgeStub.upsertPayload.BusinessInfo.CostPrice; got == nil || !sameFloat64Ptr(got, float64Ptr(8.25)) {
		t.Fatalf("erp business_info cost_price = %+v, want 8.25", got)
	}
}

func TestRetryFilingSyncsSingleSKUItemCostProjectionFromTaskDetail(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "KT_STANDARD", Label: "KT_STANDARD"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			794: {
				ID:                  794,
				TaskNo:              "RW-20260521-A-000791",
				SourceMode:          domain.TaskSourceModeNewProduct,
				SKUCode:             "CGK000031",
				PrimarySKUCode:      "CGK000031",
				ProductNameSnapshot: "真/常规kt板/爱心挂牌/30*34cm",
				TaskType:            domain.TaskTypeNewProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			794: {
				TaskID:                   794,
				Category:                 "常规kt板",
				CategoryName:             "常规kt板",
				SpecText:                 "30*34cm",
				CostPrice:                float64Ptr(11.98),
				EstimatedCost:            float64Ptr(11.98),
				CostRuleID:               int64Ptr(7),
				CostRuleName:             "常规KT板基础单价",
				CostRuleSource:           "system_auto",
				RequiresManualReview:     false,
				ManualCostOverride:       false,
				ManualCostOverrideReason: "",
				FilingStatus:             domain.FilingStatusFiled,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			794: {
				{
					ID:                  621,
					TaskID:              794,
					SequenceNo:          1,
					SKUCode:             "CGK000031",
					ProductNameSnapshot: "真/常规kt板/爱心挂牌/30*34cm",
				},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	_, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{
		TaskID:     794,
		OperatorID: 1,
		Remark:     "retry cost projection",
	})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	item := taskRepo.skuItems[794][0]
	if got := item.CostPrice; got == nil || *got != 11.98 {
		t.Fatalf("sku item cost_price = %v, want 11.98", got)
	}
	if got := item.EstimatedCost; got == nil || *got != 11.98 {
		t.Fatalf("sku item estimated_cost = %v, want 11.98", got)
	}
	if item.CostRuleName != "常规KT板基础单价" {
		t.Fatalf("sku item cost_rule_name = %q, want synced rule", item.CostRuleName)
	}
	if item.RequiresManualReview {
		t.Fatal("sku item requires_manual_review should be false after synced priced detail")
	}
}

func TestRetryFilingBatchMultiSKUDoesNotReportTopLevelIIDMissingOnReadbackNotFound(t *testing.T) {
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			CostVerification: &domain.ERPCostVerificationResult{
				Status: "readback_not_found",
			},
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			970: {
				ID:                  970,
				TaskNo:              "RW-970",
				SourceMode:          domain.TaskSourceModeNewProduct,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusInProgress,
				IsBatchTask:         true,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUCode:             "CGG000007",
				PrimarySKUCode:      "CGG000007",
				ProductNameSnapshot: "batch task",
			},
		},
		details: map[int64]*domain.TaskDetail{
			970: {
				TaskID:       970,
				FilingStatus: domain.FilingStatusFilingFailed,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			970: {
				{TaskID: 970, SequenceNo: 1, SKUCode: "CGG000007", ProductNameSnapshot: "A", ProductIID: "常规写真布"},
				{TaskID: 970, SequenceNo: 2, SKUCode: "CGG000008", ProductNameSnapshot: "B", VariantJSON: json.RawMessage(`{"i_id":"常规写真布"}`)},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	view, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{TaskID: 970, OperatorID: 1})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if view.FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want pending_filing", view.FilingStatus)
	}
	if strings.Contains(strings.Join(view.MissingFields, ","), "i_id") {
		t.Fatalf("missing_fields = %+v, should not contain top-level i_id for batch", view.MissingFields)
	}
	if got := strings.TrimSpace(view.FilingErrorMessage); got == "" || !strings.Contains(got, "等待系统回查确认") {
		t.Fatalf("filing_error_message = %q, want pending readback message", got)
	}
}

func TestRetryFilingRefreshesProductManagementProjection(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			CostVerification: &domain.ERPCostVerificationResult{
				Status: "matched",
			},
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			976: {
				ID:                  976,
				TaskNo:              "RW-976",
				SourceMode:          domain.TaskSourceModeNewProduct,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusInProgress,
				IsBatchTask:         true,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUCode:             "CGG000076",
				PrimarySKUCode:      "CGG000076",
				ProductNameSnapshot: "batch task",
			},
		},
		details: map[int64]*domain.TaskDetail{
			976: {
				TaskID:       976,
				FilingStatus: domain.FilingStatusPending,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			976: {
				{ID: 1976, TaskID: 976, SequenceNo: 1, SKUCode: "CGG000076", ProductNameSnapshot: "A", ProductIID: "常规海报", CostPrice: float64Ptr(12.3)},
			},
		},
	}
	productManagement := &productManagementCloseSyncerStub{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
		WithTaskProductManagementCloseSyncer(productManagement),
	).(*taskService)

	view, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{TaskID: 976, OperatorID: 1})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if view.FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status = %s, want filed", view.FilingStatus)
	}
	if productManagement.refreshCalls != 1 {
		t.Fatalf("product management refresh calls = %d, want 1", productManagement.refreshCalls)
	}
	if productManagement.baseQueueCalls != 1 {
		t.Fatalf("product management base queue calls = %d, want 1", productManagement.baseQueueCalls)
	}
	if productManagement.baseQueueTaskID != 976 {
		t.Fatalf("product management base queue task id = %d, want 976", productManagement.baseQueueTaskID)
	}
}

type productManagementCloseSyncerStub struct {
	refreshCalls    int
	baseQueueCalls  int
	baseQueueTaskID int64
}

func (s *productManagementCloseSyncerStub) AutoSyncImagesAfterTaskClosed(context.Context, int64, int64) *domain.AppError {
	return nil
}

func (s *productManagementCloseSyncerStub) RefreshReadModelNow(context.Context) *domain.AppError {
	s.refreshCalls++
	return nil
}

func (s *productManagementCloseSyncerStub) QueuePendingBaseSyncForTask(_ context.Context, taskID int64) (int, *domain.AppError) {
	s.refreshCalls++
	s.baseQueueCalls++
	s.baseQueueTaskID = taskID
	return 1, nil
}

func TestRetryFilingBatchMultiSKUReportsSKUScopedIIDMissing(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			971: {
				ID:                  971,
				TaskNo:              "RW-971",
				SourceMode:          domain.TaskSourceModeNewProduct,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusInProgress,
				IsBatchTask:         true,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUCode:             "CGG000009",
				PrimarySKUCode:      "CGG000009",
				ProductNameSnapshot: "batch task missing iid",
			},
		},
		details: map[int64]*domain.TaskDetail{
			971: {TaskID: 971},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			971: {
				{TaskID: 971, SequenceNo: 1, SKUCode: "CGG000009", ProductNameSnapshot: "A", ProductIID: "常规写真布"},
				{TaskID: 971, SequenceNo: 2, SKUCode: "CGG000010", ProductNameSnapshot: "B"},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{}),
	).(*taskService)

	view, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{TaskID: 971, OperatorID: 1})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if view.FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want pending", view.FilingStatus)
	}
	found := false
	for _, field := range view.MissingFields {
		if strings.Contains(field, "sku_items[1].product_i_id") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing_fields = %+v, want sku-scoped product_i_id missing", view.MissingFields)
	}
}

func TestRetryFilingBatchMultiSKUOnlyRetriesRowsNeedingSync(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			CostVerification: &domain.ERPCostVerificationResult{
				Status: "matched",
			},
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			972: {
				ID:                  972,
				TaskNo:              "RW-972",
				SourceMode:          domain.TaskSourceModeNewProduct,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusInProgress,
				IsBatchTask:         true,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUCode:             "CGG000011",
				PrimarySKUCode:      "CGG000011",
				ProductNameSnapshot: "batch retry only failed row",
			},
		},
		details: map[int64]*domain.TaskDetail{
			972: {
				TaskID:          972,
				FilingStatus:    domain.FilingStatusFilingFailed,
				ERPSyncRequired: true,
				ERPSyncVersion:  1,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			972: {
				{ID: 1, TaskID: 972, SequenceNo: 1, SKUCode: "CGG000011", ProductNameSnapshot: "A", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
				{ID: 2, TaskID: 972, SequenceNo: 2, SKUCode: "CGG000012", ProductNameSnapshot: "B", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true},
				{ID: 3, TaskID: 972, SequenceNo: 3, SKUCode: "CGG000013", ProductNameSnapshot: "C", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFiled, ERPSyncStatus: domain.FilingStatusFiled, ERPSyncRequired: false},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	view, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{TaskID: 972, OperatorID: 1})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls = %d, want only failed row retried", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.SKUID; got != "CGG000012" {
		t.Fatalf("retried sku = %s, want CGG000012", got)
	}
	if view.FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status = %s, want filed", view.FilingStatus)
	}
	for _, item := range taskRepo.skuItems[972] {
		if item.FilingStatus != domain.FilingStatusFiled || item.ERPSyncRequired {
			t.Fatalf("item %s filing = %s required=%t, want filed false", item.SKUCode, item.FilingStatus, item.ERPSyncRequired)
		}
	}
}

func TestRetryFilingBatchMultiSKURecordsPerSKUResultAndContinuesAfterFailure(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{}
	bridgeStub.upsertResultFn = func(call int) *domain.ERPProductUpsertResult {
		skuID := ""
		if bridgeStub.upsertPayload != nil {
			skuID = bridgeStub.upsertPayload.SKUID
		}
		if call == 1 {
			return &domain.ERPProductUpsertResult{
				Status:  "succeeded",
				Message: "ok",
				SKUID:   skuID,
				CostVerification: &domain.ERPCostVerificationResult{
					Status:  "unverified",
					Message: "upstream timeout",
				},
			}
		}
		return &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			SKUID:   skuID,
			CostVerification: &domain.ERPCostVerificationResult{
				Status: "matched",
			},
		}
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			973: {
				ID:                  973,
				TaskNo:              "RW-973",
				SourceMode:          domain.TaskSourceModeNewProduct,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusInProgress,
				IsBatchTask:         true,
				BatchMode:           domain.TaskBatchModeMultiSKU,
				SKUCode:             "CGG000014",
				PrimarySKUCode:      "CGG000014",
				ProductNameSnapshot: "batch partial failure",
			},
		},
		details: map[int64]*domain.TaskDetail{
			973: {
				TaskID:          973,
				FilingStatus:    domain.FilingStatusFilingFailed,
				ERPSyncRequired: true,
				ERPSyncVersion:  1,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			973: {
				{ID: 11, TaskID: 973, SequenceNo: 1, SKUCode: "CGG000014", ProductNameSnapshot: "A", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true},
				{ID: 12, TaskID: 973, SequenceNo: 2, SKUCode: "CGG000015", ProductNameSnapshot: "B", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true},
				{ID: 13, TaskID: 973, SequenceNo: 3, SKUCode: "CGG000016", ProductNameSnapshot: "C", ProductIID: "常规kt板", FilingStatus: domain.FilingStatusFilingFailed, ERPSyncStatus: domain.FilingStatusFilingFailed, ERPSyncRequired: true},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	).(*taskService)

	view, appErr := svc.RetryFiling(context.Background(), RetryTaskFilingParams{TaskID: 973, OperatorID: 1})
	if appErr != nil {
		t.Fatalf("RetryFiling() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertCalls != 3 {
		t.Fatalf("upsert calls = %d, want all target rows attempted", bridgeStub.upsertCalls)
	}
	if view.FilingStatus != domain.FilingStatusFilingFailed {
		t.Fatalf("filing_status = %s, want filing_failed", view.FilingStatus)
	}
	if !strings.Contains(view.FilingErrorMessage, "CGG000015") {
		t.Fatalf("filing_error_message = %q, want failed sku code", view.FilingErrorMessage)
	}
	want := map[string]domain.FilingStatus{
		"CGG000014": domain.FilingStatusFiled,
		"CGG000015": domain.FilingStatusFilingFailed,
		"CGG000016": domain.FilingStatusFiled,
	}
	for _, item := range taskRepo.skuItems[973] {
		if item.FilingStatus != want[item.SKUCode] {
			t.Fatalf("item %s filing_status = %s, want %s", item.SKUCode, item.FilingStatus, want[item.SKUCode])
		}
		if item.SKUCode == "CGG000015" && !item.ERPSyncRequired {
			t.Fatalf("failed item %s should remain erp_sync_required", item.SKUCode)
		}
		if item.SKUCode != "CGG000015" && item.ERPSyncRequired {
			t.Fatalf("successful item %s should not require erp sync", item.SKUCode)
		}
	}
}

func TestNewProductFilingDoesNotRegressToPendingWhenCostFieldsMissingAfterCreateSync(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions:   []*domain.ERPIIDOption{{IID: "KT_STANDARD", Label: "KT_STANDARD"}},
		upsertResult: &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "上线前新款单SKU任务",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
		SyncERPOnCreate:     true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after create = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 1 {
		t.Fatalf("upsert calls after create = %d, want 1", bridgeStub.upsertCalls)
	}
	if got := taskRepo.skuItems[task.ID][0].FilingStatus; got != domain.FilingStatusFiled {
		t.Fatalf("sku item filing_status after create = %s, want filed", got)
	}
	if got := taskRepo.skuItems[task.ID][0].ERPSyncStatus; got != domain.FilingStatusFiled {
		t.Fatalf("sku item erp_sync_status after create = %s, want filed", got)
	}
	if taskRepo.skuItems[task.ID][0].ERPSyncRequired {
		t.Fatal("sku item erp_sync_required should be false after create filing")
	}
	if got := bridgeStub.upsertPayload.CostPrice; got != nil {
		t.Fatalf("create erp cost_price = %v, want nil for unknown cost", got)
	}
	if got := bridgeStub.upsertPayload.BusinessInfo.CostPrice; got != nil {
		t.Fatalf("create erp business_info.cost_price = %v, want nil for unknown cost", got)
	}

	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "上线前新款单SKU任务3",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(5.69),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after business-info patch = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
	if got := bridgeStub.upsertPayload.CostPrice; got == nil || *got != 5.69 {
		t.Fatalf("erp cost_price = %v, want 5.69", got)
	}
	if got := bridgeStub.upsertPayload.BusinessInfo.CostPrice; got == nil || *got != 5.69 {
		t.Fatalf("erp business_info.cost_price = %v, want 5.69", got)
	}
	if got := taskRepo.skuItems[task.ID][0].CostPrice; got == nil || *got != 5.69 {
		t.Fatalf("sku item cost_price after business-info patch = %v, want 5.69", got)
	}
	if taskRepo.skuItems[task.ID][0].ERPSyncRequired {
		t.Fatal("sku item erp_sync_required should be false after forced cost refiling")
	}
}

func TestNewProductFilingFailsWhenERPCostReadbackDiffers(t *testing.T) {
	expected := 5.69
	actual := 0.96
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{{IID: "KT_STANDARD", Label: "KT_STANDARD"}},
		upsertResultFn: func(int) *domain.ERPProductUpsertResult {
			return &domain.ERPProductUpsertResult{
				Status:  "succeeded",
				Message: "ok",
				CostVerification: &domain.ERPCostVerificationResult{
					Status:       "mismatched",
					SKUID:        "NSKT000292",
					ExpectedCost: float64Ptr(expected),
					ActualCost:   float64Ptr(actual),
					Message:      "ERP cost readback mismatch",
				},
			}
		},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "成本回查失败测试",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	upsertsBefore := bridgeStub.upsertCalls
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "成本回查失败测试",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(expected),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFilingFailed {
		t.Fatalf("filing_status after cost mismatch = %s, want filing_failed", taskRepo.details[task.ID].FilingStatus)
	}
	if !strings.Contains(taskRepo.details[task.ID].FilingErrorMessage, "ERP成本回查不一致") {
		t.Fatalf("filing_error_message = %q, want ERP cost mismatch", taskRepo.details[task.ID].FilingErrorMessage)
	}
	if !strings.Contains(taskRepo.details[task.ID].FilingErrorMessage, "系统成本覆盖重试") {
		t.Fatalf("filing_error_message = %q, want retry exhaustion detail", taskRepo.details[task.ID].FilingErrorMessage)
	}
	maxUpserts := 1 + len(erpBridgeCostVerificationRetryDelays)
	if delta := bridgeStub.upsertCalls - upsertsBefore; delta != maxUpserts {
		t.Fatalf("upsert calls delta = %d, want %d after cost retry exhaustion", delta, maxUpserts)
	}
	if !taskRepo.details[task.ID].ERPSyncRequired {
		t.Fatal("erp_sync_required should stay true after cost readback mismatch")
	}
}

func TestNewProductFilingSucceedsWhenERPCostReadbackMatchesAfterRetry(t *testing.T) {
	expected := 3.3
	actual := 1.06
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	mismatchRemaining := 1
	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{{IID: "KT_STANDARD", Label: "KT_STANDARD"}},
	}
	bridgeStub.upsertResultFn = func(int) *domain.ERPProductUpsertResult {
		if bridgeStub.upsertPayload == nil || bridgeStub.upsertPayload.CostPrice == nil || *bridgeStub.upsertPayload.CostPrice != expected {
			return &domain.ERPProductUpsertResult{Status: "succeeded", Message: "ok"}
		}
		if mismatchRemaining > 0 {
			mismatchRemaining--
			return &domain.ERPProductUpsertResult{
				Status:  "succeeded",
				Message: "ok",
				CostVerification: &domain.ERPCostVerificationResult{
					Status:       "mismatched",
					ExpectedCost: float64Ptr(expected),
					ActualCost:   float64Ptr(actual),
				},
			}
		}
		return &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			CostVerification: &domain.ERPCostVerificationResult{
				Status:       "matched",
				ExpectedCost: float64Ptr(expected),
				ActualCost:   float64Ptr(expected),
			},
		}
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "成本回查重试成功测试",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	upsertsBefore := bridgeStub.upsertCalls
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "成本回查重试成功测试",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(expected),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status after cost retry match = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if delta := bridgeStub.upsertCalls - upsertsBefore; delta != 2 {
		t.Fatalf("upsert calls delta = %d, want 2 (mismatch then matched)", delta)
	}
	if taskRepo.details[task.ID].ERPSyncRequired {
		t.Fatal("erp_sync_required should be false after successful cost readback")
	}
}

func TestNewProductFilingSucceedsWhenERPCostReadback404ThenMatched(t *testing.T) {
	erpBridgeCostReadbackSleep = func(time.Duration) {}
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() {
		erpBridgeCostReadbackSleep = time.Sleep
		erpBridgeCostVerificationSleep = time.Sleep
	})

	const skuID = "CGK000000"
	expected := 9.9
	zero := 0.0
	readbackClient := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			skuID: {
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{product: &domain.ERPProduct{ProductID: skuID, SKUID: skuID, CostPrice: &zero}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{product: &domain.ERPProduct{ProductID: skuID, SKUID: skuID, CostPrice: float64Ptr(expected)}},
			},
		},
	}
	productRepo := &erpBridgeProductRepoStub{
		products: map[string]*domain.Product{
			"KT_STD": {
				ERPProductID: "KT_STD",
				ProductName:  "KT_STANDARD",
				SpecJSON:     `{"i_id":"KT_STANDARD"}`,
			},
		},
	}
	bridgeSvc := NewERPBridgeService(readbackClient, productRepo, nil)
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeSvc),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "成本回查404后成功测试",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != skuID {
		t.Fatalf("sku_code = %s, want %s", task.SKUCode, skuID)
	}

	upsertsBefore := readbackClient.upsertCalls
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "成本回查404后成功测试",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(expected),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status = %s, want filed (sku=%s err=%q)", taskRepo.details[task.ID].FilingStatus, task.SKUCode, taskRepo.details[task.ID].FilingErrorMessage)
	}
	if delta := readbackClient.upsertCalls - upsertsBefore; delta < 1 {
		t.Fatalf("upsert calls delta = %d, want at least 1", delta)
	}
}

func TestNewProductFilingStaysPendingWhenERPCostReadback404Exhausted(t *testing.T) {
	erpBridgeCostReadbackSleep = func(time.Duration) {}
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() {
		erpBridgeCostReadbackSleep = time.Sleep
		erpBridgeCostVerificationSleep = time.Sleep
	})

	const skuID = "CGK000000"
	readbackClient := &erpBridgeReadbackSequenceClient{
		getSteps: map[string][]erpBridgeReadbackStep{
			skuID: {
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
				{err: &erpBridgeHTTPError{StatusCode: http.StatusNotFound}},
			},
		},
	}
	productRepo := &erpBridgeProductRepoStub{
		products: map[string]*domain.Product{
			"KT_STD": {
				ERPProductID: "KT_STD",
				ProductName:  "KT_STANDARD",
				SpecJSON:     `{"i_id":"KT_STANDARD"}`,
			},
		},
	}
	bridgeSvc := NewERPBridgeService(readbackClient, productRepo, nil)
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeSvc),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "成本回查404耗尽测试",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != skuID {
		t.Fatalf("sku_code = %s, want %s", task.SKUCode, skuID)
	}

	upsertsBefore := readbackClient.upsertCalls
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "成本回查404耗尽测试",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(9.9),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want pending_filing", taskRepo.details[task.ID].FilingStatus)
	}
	wantMsg := "ERP已提交，等待系统回查确认"
	if taskRepo.details[task.ID].FilingErrorMessage != wantMsg {
		t.Fatalf("filing_error_message = %q, want %q", taskRepo.details[task.ID].FilingErrorMessage, wantMsg)
	}
	if delta := readbackClient.upsertCalls - upsertsBefore; delta != 1 {
		t.Fatalf("upsert calls delta = %d, want 1", delta)
	}
}

func TestNewProductFilingFailsWhenERPCostReadbackUnverified(t *testing.T) {
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	bridgeStub := &erpBridgeSelectionBinderStub{
		iidOptions: []*domain.ERPIIDOption{{IID: "KT_STANDARD", Label: "KT_STANDARD"}},
		upsertResult: &domain.ERPProductUpsertResult{
			Status:  "succeeded",
			Message: "ok",
			CostVerification: &domain.ERPCostVerificationResult{
				Status:  "unverified",
				Message: "ERP cost readback failed after upsert: upstream timeout",
			},
		},
	}
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "成本回查未验证失败测试",
		ProductIID:          "KT_STANDARD",
		CostPriceMode:       string(domain.CostPriceModeManual),
		DesignRequirement:   "设计要求",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	upsertsBefore := bridgeStub.upsertCalls
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:             task.ID,
		OperatorID:         11,
		ProductName:        "成本回查未验证失败测试",
		ProductIID:         "KT_STANDARD",
		Category:           "KT_STANDARD",
		SpecText:           "20*20",
		CostPrice:          float64Ptr(3.3),
		ManualCostOverride: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.details[task.ID].FilingStatus != domain.FilingStatusFilingFailed {
		t.Fatalf("filing_status = %s, want filing_failed", taskRepo.details[task.ID].FilingStatus)
	}
	if !strings.Contains(taskRepo.details[task.ID].FilingErrorMessage, "ERP成本回查失败") {
		t.Fatalf("filing_error_message = %q, want unverified failure", taskRepo.details[task.ID].FilingErrorMessage)
	}
	if delta := bridgeStub.upsertCalls - upsertsBefore; delta != 1 {
		t.Fatalf("upsert calls delta = %d, want 1 without mismatch retry", delta)
	}
}

func TestRetouchTaskFilingProjectionDoesNotRequireERPSync(t *testing.T) {
	task := &domain.Task{
		ID:       612,
		TaskType: domain.TaskTypeRetouchTask,
	}
	detail := &domain.TaskDetail{
		TaskID:              612,
		FilingStatus:        domain.FilingStatusNotFiled,
		ERPSyncRequired:     true,
		FilingTriggerSource: string(TaskFilingTriggerSourceCreate),
	}

	hydrateTaskDetailFilingProjection(task, detail)

	if detail.FilingStatus != domain.FilingStatusNotFiled {
		t.Fatalf("filing_status = %s, want not_filed", detail.FilingStatus)
	}
	if detail.ERPSyncRequired {
		t.Fatal("retouch_task should not require ERP sync")
	}
}

func TestBatchNewProductFilingPayloadUsesOnlyPublicPerSKUReferenceImage(t *testing.T) {
	task := &domain.Task{
		ID:                  77,
		TaskNo:              "RW-BATCH-77",
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		SKUCode:             "SKU-A",
		ProductNameSnapshot: "Batch A",
		IsBatchTask:         true,
	}
	detail := &domain.TaskDetail{TaskID: 77}
	payload, appErr := buildBatchSKUItemERPBridgeProductUpsertPayload(task, detail, &domain.TaskSKUItem{
		TaskID:              77,
		SequenceNo:          1,
		SKUCode:             "SKU-A",
		ProductNameSnapshot: "Batch A",
		ProductIID:          "I-1001",
		ReferenceFileRefs: []domain.ReferenceFileRef{
			{AssetID: "ref-a", DownloadURL: strPtr("https://cdn.example.com/ref-a.jpg")},
		},
	}, 11, "", string(TaskFilingTriggerSourceCreate))
	if appErr != nil {
		t.Fatalf("build payload unexpected error: %+v", appErr)
	}
	if payload.Pic != "https://cdn.example.com/ref-a.jpg" || payload.PicBig != "https://cdn.example.com/ref-a.jpg" || payload.SKUPic != "https://cdn.example.com/ref-a.jpg" {
		t.Fatalf("payload image fields = pic:%q pic_big:%q sku_pic:%q", payload.Pic, payload.PicBig, payload.SKUPic)
	}

	payload, appErr = buildBatchSKUItemERPBridgeProductUpsertPayload(task, detail, &domain.TaskSKUItem{
		TaskID:              77,
		SequenceNo:          2,
		SKUCode:             "SKU-B",
		ProductNameSnapshot: "Batch B",
		ProductIID:          "I-1002",
		ReferenceFileRefs: []domain.ReferenceFileRef{
			{AssetID: "ref-b", StorageKey: "tasks/ref-b.jpg"},
		},
	}, 11, "", string(TaskFilingTriggerSourceCreate))
	if appErr != nil {
		t.Fatalf("build payload storage key unexpected error: %+v", appErr)
	}
	if payload.Pic != "" || payload.PicBig != "" || payload.SKUPic != "" {
		t.Fatalf("relative asset routes must not be sent to ERP image fields: pic=%q pic_big=%q sku_pic=%q", payload.Pic, payload.PicBig, payload.SKUPic)
	}
}

func TestBatchNewProductFilingPayloadFallsBackCategoryNameToProductIID(t *testing.T) {
	payload, appErr := buildBatchSKUItemERPBridgeProductUpsertPayload(
		&domain.Task{
			ID:                  78,
			TaskNo:              "RW-BATCH-78",
			TaskType:            domain.TaskTypeNewProductDevelopment,
			SourceMode:          domain.TaskSourceModeNewProduct,
			SKUCode:             "SKU-C",
			ProductNameSnapshot: "Batch C",
			IsBatchTask:         true,
		},
		&domain.TaskDetail{TaskID: 78},
		&domain.TaskSKUItem{
			TaskID:              78,
			SequenceNo:          1,
			SKUCode:             "SKU-C",
			ProductNameSnapshot: "Batch C",
			ProductIID:          "河南kt板",
			CategoryCode:        "GENERAL",
		},
		11,
		"",
		string(TaskFilingTriggerSourceCreate),
	)
	if appErr != nil {
		t.Fatalf("build payload unexpected error: %+v", appErr)
	}
	if payload.CategoryName != "河南kt板" {
		t.Fatalf("CategoryName = %q, want product_i_id fallback", payload.CategoryName)
	}
	if payload.CategoryCode != "" {
		t.Fatalf("CategoryCode = %q, want GENERAL omitted from ERP payload", payload.CategoryCode)
	}
	if payload.BusinessInfo == nil || payload.BusinessInfo.CategoryName != "河南kt板" {
		t.Fatalf("BusinessInfo.CategoryName = %+v, want product_i_id fallback", payload.BusinessInfo)
	}
	if payload.BusinessInfo == nil || payload.BusinessInfo.CategoryCode != "" {
		t.Fatalf("BusinessInfo.CategoryCode = %+v, want GENERAL omitted from ERP payload", payload.BusinessInfo)
	}
}

func TestDefaultBatchItemCategoryCodePrefersProductIIDAndOnlyFallsBackGeneralForNewProduct(t *testing.T) {
	got := defaultBatchItemCategoryCode(
		CreateTaskParams{CategoryCode: "GENERAL", ProductIID: "常规kt板"},
		CreateTaskBatchSKUItemParams{CategoryCode: "GENERAL", ProductIID: "河南kt板"},
	)
	if got != "河南kt板" {
		t.Fatalf("defaultBatchItemCategoryCode() = %q, want item product_i_id", got)
	}

	got = defaultBatchItemCategoryCode(
		CreateTaskParams{CategoryCode: "GENERAL"},
		CreateTaskBatchSKUItemParams{CategoryCode: "GENERAL"},
	)
	if got != "" {
		t.Fatalf("defaultBatchItemCategoryCode() = %q, want empty instead of GENERAL fallback", got)
	}

	got = defaultBatchItemCategoryCode(
		CreateTaskParams{TaskType: domain.TaskTypeNewProductDevelopment},
		CreateTaskBatchSKUItemParams{},
	)
	if got != "GENERAL" {
		t.Fatalf("defaultBatchItemCategoryCode() = %q, want GENERAL fallback for new product batch", got)
	}
}
