package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func newBatchParentBusinessInfoSvc(t *testing.T, taskID int64, detail *domain.TaskDetail) TaskService {
	t.Helper()
	categoryRepo := newCategoryRepoStub()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "SKU-BATCH-1",
				ProductNameSnapshot: "Batch Parent",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityNormal,
				IsBatchTask:         true,
				BatchItemCount:      2,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: detail,
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			taskID: {
				{ID: 1, TaskID: taskID, SequenceNo: 1, SKUCode: "SKU-BATCH-1", CategoryCode: "LIGHTBOX"},
				{ID: 2, TaskID: taskID, SequenceNo: 2, SKUCode: "SKU-BATCH-2", CategoryCode: "LIGHTBOX"},
			},
		},
	}
	return NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)
}

func TestUpdateBusinessInfoBatchParentInvalidStoredCategoryNoteOnly(t *testing.T) {
	const taskID int64 = 9101
	svc := newBatchParentBusinessInfoSvc(t, taskID, &domain.TaskDetail{
		TaskID:       taskID,
		CategoryCode: "BOGUS_CAT",
		Note:         "old note",
	})

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:     taskID,
		OperatorID: 1,
		Note:       "new ops note",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.Note != "new ops note" {
		t.Fatalf("note = %q, want new ops note", detail.Note)
	}
}

func TestUpdateBusinessInfoBatchParentInvalidStoredCategoryDesignRequirementOnly(t *testing.T) {
	const taskID int64 = 9102
	svc := newBatchParentBusinessInfoSvc(t, taskID, &domain.TaskDetail{
		TaskID:            taskID,
		CategoryCode:      "BOGUS_CAT",
		DesignRequirement: "old design",
	})

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:            taskID,
		OperatorID:        1,
		DesignRequirement: "updated design requirement",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.DesignRequirement != "updated design requirement" {
		t.Fatalf("design_requirement = %q", detail.DesignRequirement)
	}
}

func TestUpdateBusinessInfoBatchParentInvalidStoredCategoryCostOnly(t *testing.T) {
	const taskID int64 = 9103
	svc := newBatchParentBusinessInfoSvc(t, taskID, &domain.TaskDetail{
		TaskID:       taskID,
		CategoryCode: "BOGUS_CAT",
		CostPrice:    float64Ptr(1.0),
	})

	cost := 12.5
	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:                   taskID,
		OperatorID:               1,
		CostPrice:                &cost,
		ManualCostOverride:       true,
		ManualCostOverrideReason: "manual override in test",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.CostPrice == nil || *detail.CostPrice != cost {
		t.Fatalf("cost_price = %+v, want %v", detail.CostPrice, cost)
	}
	if !detail.ManualCostOverride {
		t.Fatal("expected manual_cost_override=true")
	}
}

func TestUpdateBusinessInfoRejectsExplicitInvalidCategoryCode(t *testing.T) {
	const taskID int64 = 9104
	svc := newBatchParentBusinessInfoSvc(t, taskID, &domain.TaskDetail{
		TaskID:       taskID,
		CategoryCode: "BOGUS_CAT",
	})

	_, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:        taskID,
		OperatorID:    1,
		CategoryCode:  "BOGUS_CAT",
		ApplyCategory: true,
	})
	if appErr == nil {
		t.Fatal("UpdateBusinessInfo() expected error for invalid category_code")
	}
	if !strings.Contains(appErr.Message, "category_code does not exist") {
		t.Fatalf("error message = %q", appErr.Message)
	}
}

func TestUpdateBusinessInfoPriorityOnly(t *testing.T) {
	const taskID int64 = 9105
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "SKU-PRI-1",
				ProductNameSnapshot: "Priority Task",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityNormal,
				IsBatchTask:         true,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {TaskID: taskID, CategoryCode: "BOGUS_CAT"},
		},
	}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:      taskID,
		OperatorID:  1,
		Priority:    domain.TaskPriorityHigh,
		PrioritySet: true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[taskID].Priority != domain.TaskPriorityHigh {
		t.Fatalf("priority = %q, want high", taskRepo.tasks[taskID].Priority)
	}
}

func TestUpdateBusinessInfoDeadlineOnly(t *testing.T) {
	const taskID int64 = 9107
	originalDeadline := time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)
	updatedDeadline := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "SKU-DUE-1",
				ProductNameSnapshot: "Deadline Task",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityNormal,
				DeadlineAt:          &originalDeadline,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {
				TaskID:                   taskID,
				CategoryCode:             "BOGUS_CAT",
				CostPrice:                float64Ptr(12.3),
				ManualCostOverride:       true,
				ManualCostOverrideReason: "warehouse maintained",
			},
		},
	}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:                   taskID,
		OperatorID:               1,
		CostPrice:                float64Ptr(12.3),
		ManualCostOverride:       true,
		ManualCostOverrideReason: "warehouse maintained",
		DeadlineAt:               &updatedDeadline,
		DeadlineAtSet:            true,
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[taskID].DeadlineAt == nil || !taskRepo.tasks[taskID].DeadlineAt.Equal(updatedDeadline) {
		t.Fatalf("deadline_at = %+v, want %s", taskRepo.tasks[taskID].DeadlineAt, updatedDeadline)
	}
	if got := taskRepo.details[taskID].CostPrice; got == nil || *got != 12.3 {
		t.Fatalf("cost_price changed to %+v, want 12.3", got)
	}
	if !taskRepo.details[taskID].ManualCostOverride {
		t.Fatal("manual_cost_override changed to false")
	}
}

func TestUpdateBusinessInfoRejectsInvalidPriority(t *testing.T) {
	const taskID int64 = 9106
	svc := newBatchParentBusinessInfoSvc(t, taskID, &domain.TaskDetail{TaskID: taskID})

	_, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:      taskID,
		OperatorID:  1,
		Priority:    domain.TaskPriority("urgent"),
		PrioritySet: true,
	})
	if appErr == nil {
		t.Fatal("UpdateBusinessInfo() expected error for invalid priority")
	}
	if appErr.Message != "task_priority_invalid" {
		t.Fatalf("error message = %q, want task_priority_invalid", appErr.Message)
	}
}

func TestUpdateBusinessInfoProductNameUpdatesShortNameAndQueuesProductManagementSync(t *testing.T) {
	const taskID int64 = 9110
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "CGO_TEST_001",
				ProductNameSnapshot: "旧商品名称",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityNormal,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {
				TaskID:           taskID,
				CategoryCode:     "BOGUS_CAT",
				ProductShortName: "旧商品名称",
			},
		},
	}
	productManagement := &productManagementQueueStub{}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskProductManagementCloseSyncer(productManagement),
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:      taskID,
		OperatorID:  1,
		ProductName: "新商品名称",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[taskID].ProductNameSnapshot != "新商品名称" {
		t.Fatalf("ProductNameSnapshot = %q, want 新商品名称", taskRepo.tasks[taskID].ProductNameSnapshot)
	}
	if detail.ProductShortName != "新商品名称" {
		t.Fatalf("ProductShortName = %q, want 新商品名称", detail.ProductShortName)
	}
	if got := productManagement.queuedTasks; len(got) != 1 || got[0] != taskID {
		t.Fatalf("queued product management tasks = %+v, want [%d]", got, taskID)
	}
}

func TestUpdateSKUItemInfoProductNameUpdatesShortNameAndQueuesProductManagementSync(t *testing.T) {
	const taskID int64 = 9111
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:                  taskID,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "CGO_PARENT",
				ProductNameSnapshot: "批量母任务",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityNormal,
				IsBatchTask:         true,
				BatchItemCount:      1,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {TaskID: taskID, CategoryCode: "BOGUS_CAT"},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			taskID: {
				{
					ID:                  17,
					TaskID:              taskID,
					SequenceNo:          1,
					SKUCode:             "CGO_TEST_017",
					ProductNameSnapshot: "旧子项名称",
					ProductShortName:    "旧子项名称",
				},
			},
		},
	}
	productManagement := &productManagementQueueStub{}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskProductManagementCloseSyncer(productManagement),
	)

	name := "新子项名称"
	item, appErr := svc.UpdateSKUItemInfo(context.Background(), UpdateTaskSKUItemInfoParams{
		TaskID:      taskID,
		SKUItemID:   17,
		OperatorID:  1,
		ProductName: &name,
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemInfo() unexpected error: %+v", appErr)
	}
	if item.ProductNameSnapshot != "新子项名称" {
		t.Fatalf("ProductNameSnapshot = %q, want 新子项名称", item.ProductNameSnapshot)
	}
	if item.ProductShortName != "新子项名称" {
		t.Fatalf("ProductShortName = %q, want 新子项名称", item.ProductShortName)
	}
	if got := productManagement.queuedTasks; len(got) != 1 || got[0] != taskID {
		t.Fatalf("queued product management tasks = %+v, want [%d]", got, taskID)
	}
}

type productManagementQueueStub struct {
	queuedTasks  []int64
	refreshCalls int
}

func (s *productManagementQueueStub) AutoSyncImagesAfterTaskClosed(context.Context, int64, int64) *domain.AppError {
	return nil
}

func (s *productManagementQueueStub) RefreshReadModelNow(context.Context) *domain.AppError {
	s.refreshCalls++
	return nil
}

func (s *productManagementQueueStub) QueuePendingBaseSyncForTask(_ context.Context, taskID int64) (int, *domain.AppError) {
	s.queuedTasks = append(s.queuedTasks, taskID)
	return 1, nil
}
