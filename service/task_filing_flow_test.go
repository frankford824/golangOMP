package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestAuditApproveFinalStageTriggersOriginalFiling(t *testing.T) {
	auditorID := int64(41)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			1: {
				ID:                  1,
				TaskNo:              "RW-001",
				SourceMode:          domain.TaskSourceModeExistingProduct,
				SKUCode:             "SKU-001",
				ProductNameSnapshot: "Original Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAuditB,
				CurrentHandlerID:    &auditorID,
			},
		},
		details: map[int64]*domain.TaskDetail{
			1: {
				TaskID:       1,
				CategoryCode: "CAT-1",
				SpecText:     "spec-1",
				CostPrice:    float64Ptr(19.9),
				ProductSelection: &domain.TaskProductSelectionContext{
					ERPProduct: &domain.ERPProductSelectionSnapshot{
						ProductID:   "ERP-1",
						SKUID:       "SKU-001",
						SKUCode:     "SKU-001",
						ProductName: "Original Product",
					},
				},
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{ProductID: "ERP-1", SKUID: "SKU-001"},
	}
	taskSvc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	)
	auditSvc := NewAuditV7Service(
		taskRepo,
		&auditV7RepoStub{},
		eventRepo,
		prdCodeRuleService{},
		step04TxRunner{},
		WithAuditV7FilingTrigger(taskSvc),
	)

	appErr := auditSvc.Approve(context.Background(), ApproveAuditParams{
		TaskID:     1,
		AuditorID:  auditorID,
		Stage:      domain.AuditRecordStageB,
		NextStatus: domain.TaskStatusPendingWarehouseReceive,
		Comment:    "approve and auto-file",
	})
	if appErr != nil {
		t.Fatalf("Approve() unexpected error: %+v", appErr)
	}
	if bridgeStub.upsertPayload == nil {
		t.Fatal("expected ERP filing payload on final approval")
	}
	if bridgeStub.upsertPayload.Source != string(TaskFilingTriggerSourceAuditFinalApproved) {
		t.Fatalf("payload source = %s, want %s", bridgeStub.upsertPayload.Source, TaskFilingTriggerSourceAuditFinalApproved)
	}
	if taskRepo.details[1].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("filing_status = %s, want filed", taskRepo.details[1].FilingStatus)
	}
}

func TestWarehouseCompletePrecheckTriggersOriginalFiling(t *testing.T) {
	taskID := int64(2)
	receiverID := int64(52)
	receivedAt := timePtr()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskNo:              "RW-002",
				SourceMode:          domain.TaskSourceModeExistingProduct,
				SKUCode:             "SKU-002",
				ProductNameSnapshot: "Original Product 2",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingWarehouseReceive,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {
				TaskID:       taskID,
				CategoryCode: "CAT-2",
				SpecText:     "spec-2",
				CostPrice:    float64Ptr(29.9),
				ProductSelection: &domain.TaskProductSelectionContext{
					ERPProduct: &domain.ERPProductSelectionSnapshot{
						ProductID:   "ERP-2",
						SKUID:       "SKU-002",
						SKUCode:     "SKU-002",
						ProductName: "Original Product 2",
					},
				},
			},
		},
	}
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{ProductID: "ERP-2", SKUID: "SKU-002"},
	}
	taskSvc := NewTaskService(
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
	warehouseRepo := &prdWarehouseRepo{
		receipts: map[int64]*domain.WarehouseReceipt{
			taskID: {
				TaskID:     taskID,
				ReceiptNo:  "WR-2",
				Status:     domain.WarehouseReceiptStatusReceived,
				ReceiverID: &receiverID,
				ReceivedAt: receivedAt,
			},
		},
	}
	warehouseSvc := NewWarehouseService(
		taskRepo,
		&prdTaskAssetRepo{},
		warehouseRepo,
		&prdTaskEventRepo{},
		step04TxRunner{},
		WithWarehouseFilingTrigger(taskSvc),
	)

	receipt, appErr := warehouseSvc.Complete(context.Background(), CompleteWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     "complete with precheck filing",
	})
	if appErr != nil {
		t.Fatalf("Complete() unexpected error: %+v", appErr)
	}
	if receipt.Status != domain.WarehouseReceiptStatusCompleted {
		t.Fatalf("receipt status = %s, want completed", receipt.Status)
	}
	if bridgeStub.upsertPayload == nil {
		t.Fatal("expected ERP filing payload on warehouse precheck")
	}
	if bridgeStub.upsertPayload.Source != string(TaskFilingTriggerSourceWarehouseCompletePrechk) {
		t.Fatalf("payload source = %s, want %s", bridgeStub.upsertPayload.Source, TaskFilingTriggerSourceWarehouseCompletePrechk)
	}
}

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

func TestNewAndPurchaseTaskPendingThenAutoFilingOnPatch(t *testing.T) {
	bridgeStub := &erpBridgeSelectionBinderStub{
		upsertResult: &domain.ERPProductUpsertResult{ProductID: "ERP-X", SKUID: "SKU-X"},
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
		step04TxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
	)

	newTask, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		CreatorID:           11,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		SKUCode:             "NEW-PENDING-001",
		ProductNameSnapshot: "New Product A",
		ProductShortName:    "NPA",
		DesignRequirement:   "new product",
		CategoryCode:        "",
	})
	if appErr != nil {
		t.Fatalf("Create(new) unexpected error: %+v", appErr)
	}
	if taskRepo.details[newTask.ID].FilingStatus != domain.FilingStatusPending {
		t.Fatalf("new task filing_status = %s, want pending_filing", taskRepo.details[newTask.ID].FilingStatus)
	}
	_, appErr = svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:       newTask.ID,
		OperatorID:   11,
		CategoryCode: "CAT-NEW",
		Category:     "CAT-NEW",
		Remark:       "fill missing category",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo(new) unexpected error: %+v", appErr)
	}
	if taskRepo.details[newTask.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("new task filing_status after patch = %s, want filed", taskRepo.details[newTask.ID].FilingStatus)
	}

	purchaseTask, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		CreatorID:           12,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "PUR-001",
		ProductNameSnapshot: "Purchase Product A",
		CostPriceMode:       string(domain.CostPriceModeTemplate),
		BaseSalePrice:       float64Ptr(16.8),
	})
	if appErr != nil {
		t.Fatalf("Create(purchase) unexpected error: %+v", appErr)
	}
	if taskRepo.details[purchaseTask.ID].FilingStatus != domain.FilingStatusPending {
		t.Fatalf("purchase task filing_status = %s, want pending_filing", taskRepo.details[purchaseTask.ID].FilingStatus)
	}
	_, appErr = svc.UpdateProcurement(context.Background(), UpdateTaskProcurementParams{
		TaskID:           purchaseTask.ID,
		OperatorID:       12,
		Status:           domain.ProcurementStatusDraft,
		Quantity:         int64Ptr(10),
		ProcurementPrice: float64Ptr(8.2),
		SupplierName:     "supplier-a",
		Remark:           "fill quantity",
	})
	if appErr != nil {
		t.Fatalf("UpdateProcurement(purchase) unexpected error: %+v", appErr)
	}
	if taskRepo.details[purchaseTask.ID].FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("purchase task filing_status after update = %s, want filed", taskRepo.details[purchaseTask.ID].FilingStatus)
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
		OwnerTeam:       domain.AllValidTeams()[0],
		DeadlineAt:      timePtr(),
		BatchSKUMode:    "multiple",
		SyncERPOnCreate: true,
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Batch A",
				DesignRequirement: "draw A",
				ProductIID:        "I-1001",
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
		t.Fatalf("filing_status = %s, want filed", taskRepo.details[task.ID].FilingStatus)
	}
	if bridgeStub.upsertCalls != 2 {
		t.Fatalf("upsert calls = %d, want 2", bridgeStub.upsertCalls)
	}
	if bridgeStub.upsertPayloads[0].IID != "I-1001" || bridgeStub.upsertPayloads[1].IID != "I-1002" {
		t.Fatalf("upsert iids = %s/%s, want I-1001/I-1002", bridgeStub.upsertPayloads[0].IID, bridgeStub.upsertPayloads[1].IID)
	}
	items := taskRepo.skuItems[task.ID]
	if len(items) != 2 || items[0].ProductIID != "I-1001" || items[1].ProductIID != "I-1002" {
		t.Fatalf("sku item product_i_id = %+v", items)
	}
}

func TestBatchNewProductFilingPayloadUsesPerSKUReferenceImage(t *testing.T) {
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
			{AssetID: "ref-a", DownloadURL: strPtr("/v1/assets/files/ref-a.jpg")},
		},
	}, 11, "", string(TaskFilingTriggerSourceCreate))
	if appErr != nil {
		t.Fatalf("build payload unexpected error: %+v", appErr)
	}
	if payload.Pic != "/v1/assets/files/ref-a.jpg" || payload.PicBig != "/v1/assets/files/ref-a.jpg" || payload.SKUPic != "/v1/assets/files/ref-a.jpg" {
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
	if payload.SKUPic != "/v1/assets/files/tasks/ref-b.jpg" {
		t.Fatalf("payload sku_pic = %q, want storage key file route", payload.SKUPic)
	}
}
