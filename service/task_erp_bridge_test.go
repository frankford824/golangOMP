package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskServiceCreateBindsERPBridgeSelectionIntoMainline(t *testing.T) {
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
		WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{
			product: &domain.Product{
				ID:           501,
				ERPProductID: "ERP-501",
				SKUCode:      "CF-501",
				ProductName:  "定制车缝旗帜",
			},
		}),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		CreatorID:     9,
		OwnerTeam:     domain.AllValidTeams()[0],
		DeadlineAt:    timePtr(),
		ChangeRequest: "bind erp bridge selection into mainline",
		ProductSelection: &domain.TaskProductSelectionContext{
			SourceMatchType: "erp_bridge_keyword_search",
			SourceMatchRule: "定制车缝",
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:    "ERP-501",
				SKUID:        "SKU-501",
				SKUCode:      "CF-501",
				ProductName:  "定制车缝旗帜",
				CategoryName: "旗帜",
				ImageURL:     "https://img.example.com/501.png",
				Price:        float64Ptr(19.8),
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.ProductID == nil || *task.ProductID != 501 {
		t.Fatalf("Create() product_id = %+v, want 501", task.ProductID)
	}
	if task.SourceMode != domain.TaskSourceModeExistingProduct {
		t.Fatalf("Create() source_mode = %s, want %s", task.SourceMode, domain.TaskSourceModeExistingProduct)
	}
	if task.SKUCode != "CF-501" || task.ProductNameSnapshot != "定制车缝旗帜" {
		t.Fatalf("Create() task binding = %s / %s", task.SKUCode, task.ProductNameSnapshot)
	}

	detail := taskRepo.details[task.ID]
	if detail == nil || detail.ProductSelection == nil {
		t.Fatalf("Create() detail = %+v", detail)
	}
	if detail.ProductSelection.ERPProduct == nil || detail.ProductSelection.ERPProduct.ProductID != "ERP-501" {
		t.Fatalf("Create() erp product snapshot = %+v", detail.ProductSelection)
	}
	if detail.ProductSelection.SourceMatchType != "erp_bridge_keyword_search" || detail.ProductSelection.SourceMatchRule != "定制车缝" {
		t.Fatalf("Create() selection provenance = %+v", detail.ProductSelection)
	}
}

func TestTaskServiceUpdateBusinessInfoRebindsERPBridgeSelectionIntoMainline(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			150: {
				ID:                  150,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(10),
				SKUCode:             "OLD-010",
				ProductNameSnapshot: "Old Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			150: {TaskID: 150},
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
		step04TxRunner{},
		WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{
			product: &domain.Product{
				ID:           611,
				ERPProductID: "ERP-611",
				SKUCode:      "CF-611",
				ProductName:  "ERP Bridge Product",
			},
		}),
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:     150,
		OperatorID: 7,
		ProductSelection: &domain.TaskProductSelectionContext{
			SourceMatchType: "erp_bridge_keyword_search",
			SourceMatchRule: "定制车缝",
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:    "ERP-611",
				SKUCode:      "CF-611",
				ProductName:  "ERP Bridge Product",
				CategoryName: "旗帜",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[150].ProductID == nil || *taskRepo.tasks[150].ProductID != 611 {
		t.Fatalf("UpdateBusinessInfo() task product_id = %+v, want 611", taskRepo.tasks[150].ProductID)
	}
	if detail.ProductSelection == nil || detail.ProductSelection.ERPProduct == nil {
		t.Fatalf("UpdateBusinessInfo() detail selection = %+v", detail.ProductSelection)
	}
	if detail.ProductSelection.ERPProduct.ProductID != "ERP-611" {
		t.Fatalf("UpdateBusinessInfo() erp snapshot = %+v", detail.ProductSelection.ERPProduct)
	}
	if detail.ProductSelection.ERPProduct.SKUCode != "CF-611" || detail.ProductSelection.ERPProduct.ProductName != "ERP Bridge Product" {
		t.Fatalf("UpdateBusinessInfo() erp snapshot backfill = %+v", detail.ProductSelection.ERPProduct)
	}
}

func TestTaskServiceUpdateBusinessInfoFilesERPBridgeAtFiledBoundary(t *testing.T) {
	filedAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			220: {
				ID:                  220,
				TaskNo:              "TSK-220",
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(10),
				SKUCode:             "OLD-220",
				ProductNameSnapshot: "Old Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			220: {TaskID: 220},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	callLogRepo := newIntegrationCallLogRepoStub()
	bridgeStub := &erpBridgeSelectionBinderStub{
		product: &domain.Product{
			ID:           622,
			ERPProductID: "ERP-622",
			SKUCode:      "CF-622",
			ProductName:  "ERP Filed Product",
		},
		upsertResult: &domain.ERPProductUpsertResult{
			ProductID: "ERP-622",
			SKUCode:   "CF-622",
			SyncLogID: "sync-622",
			Status:    "accepted",
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithERPBridgeSelectionBinding(bridgeStub),
		WithTaskERPBridgeFilingTrace(callLogRepo),
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:       220,
		OperatorID:   7,
		Category:     "Banner",
		CategoryCode: "BANNER",
		SpecText:     "board filing",
		CostPrice:    float64Ptr(12.8),
		FiledAt:      &filedAt,
		ProductSelection: &domain.TaskProductSelectionContext{
			SourceMatchType: "erp_bridge_keyword_search",
			SourceMatchRule: "filed",
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:    "ERP-622",
				SKUCode:      "CF-622",
				ProductName:  "ERP Filed Product",
				CategoryName: "Banner",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.FilingStatus != domain.FilingStatusFiled {
		t.Fatalf("detail.FilingStatus = %s, want %s", detail.FilingStatus, domain.FilingStatusFiled)
	}
	if detail.FiledAt == nil {
		t.Fatalf("detail.FiledAt = %+v, want non-nil", detail.FiledAt)
	}
	if bridgeStub.upsertPayload == nil || bridgeStub.upsertPayload.ProductID != "ERP-622" {
		t.Fatalf("upsert payload = %+v", bridgeStub.upsertPayload)
	}
	if bridgeStub.upsertPayload.TaskContext == nil || bridgeStub.upsertPayload.TaskContext.TaskID != 220 {
		t.Fatalf("upsert task_context = %+v", bridgeStub.upsertPayload.TaskContext)
	}
	if len(callLogRepo.logs) != 1 {
		t.Fatalf("call logs = %+v", callLogRepo.logs)
	}
	var storedLog *domain.IntegrationCallLog
	for _, item := range callLogRepo.logs {
		storedLog = item
	}
	if storedLog == nil || storedLog.Status != domain.IntegrationCallStatusSucceeded {
		t.Fatalf("stored call log = %+v", storedLog)
	}
	if storedLog.ConnectorKey != domain.IntegrationConnectorKeyERPBridgeProductUpsert {
		t.Fatalf("connector_key = %s", storedLog.ConnectorKey)
	}
	if len(eventRepo.events) < 2 {
		t.Fatalf("events = %+v", eventRepo.events)
	}
	var filingEvent *domain.TaskEvent
	for _, ev := range eventRepo.events {
		if ev != nil && ev.EventType == domain.TaskEventFilingTriggered {
			filingEvent = ev
			break
		}
	}
	if filingEvent == nil {
		t.Fatalf("filing event not found in events=%+v", eventRepo.events)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(filingEvent.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(event payload) error = %v", err)
	}
	filing, ok := payload["erp_filing"].(map[string]interface{})
	if !ok {
		t.Fatalf("erp_filing payload = %+v", payload["erp_filing"])
	}
	if filing["sync_log_id"] != "sync-622" {
		t.Fatalf("erp_filing sync_log_id = %+v", filing["sync_log_id"])
	}
}

func TestTaskServiceUpdateBusinessInfoMarksPendingFilingWithoutERPSelection(t *testing.T) {
	filedAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			221: {
				ID:                  221,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(10),
				SKUCode:             "OLD-221",
				ProductNameSnapshot: "Legacy Local Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			221: {TaskID: 221},
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
		step04TxRunner{},
		WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{}),
	)

	_, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:     221,
		OperatorID: 7,
		SpecText:   "legacy filing",
		FiledAt:    &filedAt,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	detail := taskRepo.details[221]
	if detail == nil {
		t.Fatal("task detail not found")
	}
	if detail.FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want %s", detail.FilingStatus, domain.FilingStatusPending)
	}
}

func TestTaskServiceCreateRejectsMismatchedSelectedProductIDForERPSelection(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{
			product: &domain.Product{
				ID:           700,
				ERPProductID: "ERP-700",
				SKUCode:      "CF-700",
				ProductName:  "Resolved Product",
			},
		}),
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		SourceMode: domain.TaskSourceModeExistingProduct,
		TaskType:   domain.TaskTypeOriginalProductDevelopment,
		CreatorID:  9,
		OwnerTeam:  domain.AllValidTeams()[0],
		DeadlineAt: timePtr(),
		ProductSelection: &domain.TaskProductSelectionContext{
			SelectedProductID: int64Ptr(701),
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:   "ERP-700",
				SKUCode:     "CF-700",
				ProductName: "Resolved Product",
			},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected mismatched selected_product_id validation error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("appErr = %+v", appErr)
	}
}

type erpBridgeSelectionBinderStub struct {
	product       *domain.Product
	upsertPayload *domain.ERPProductUpsertPayload
	upsertResult  *domain.ERPProductUpsertResult
	upsertAppErr  *domain.AppError
	upsertCalls   int
}

func (s *erpBridgeSelectionBinderStub) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) ListIIDs(context.Context, domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError) {
	return &domain.ERPIIDListResponse{Items: []*domain.ERPIIDOption{}}, nil
}

func (s *erpBridgeSelectionBinderStub) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) ListCategories(context.Context) ([]*domain.ERPCategory, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) ListWarehouses(context.Context) ([]domain.ERPWarehouse, *domain.AppError) {
	return []domain.ERPWarehouse{}, nil
}

func (s *erpBridgeSelectionBinderStub) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, *domain.AppError) {
	return &domain.ERPSyncLogListResponse{Items: []*domain.ERPSyncLog{}, Pagination: domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0}}, nil
}

func (s *erpBridgeSelectionBinderStub) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) EnsureLocalProduct(context.Context, repo.Tx, *domain.ERPProductSelectionSnapshot) (*domain.Product, *domain.AppError) {
	return s.product, nil
}

func (s *erpBridgeSelectionBinderStub) UpsertProduct(_ context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	s.upsertCalls++
	copyPayload := payload
	s.upsertPayload = &copyPayload
	return s.upsertResult, s.upsertAppErr
}

func (s *erpBridgeSelectionBinderStub) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, *domain.AppError) {
	return &domain.ERPItemStyleUpdateResult{Status: "accepted"}, nil
}

func (s *erpBridgeSelectionBinderStub) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, *domain.AppError) {
	return nil, nil
}

func (s *erpBridgeSelectionBinderStub) ListJSTUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, *domain.AppError) {
	return &domain.JSTUserListResponse{Datas: []*domain.JSTUser{}}, nil
}
