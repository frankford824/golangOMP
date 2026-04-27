package service

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// TaskDetailAggregateService provides the frontend detail view model for tasks.
type TaskDetailAggregateService interface {
	GetByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetailAggregate, *domain.AppError)
}

type taskDetailAggregateService struct {
	taskRepo                repo.TaskRepo
	procurementRepo         repo.ProcurementRepo
	productRepo             repo.ProductRepo
	costRuleRepo            repo.CostRuleRepo
	auditV7Repo             repo.AuditV7Repo
	outsourceRepo           repo.OutsourceRepo
	taskAssetRepo           repo.TaskAssetRepo
	designAssetRepo         repo.DesignAssetRepo
	warehouseRepo           repo.WarehouseRepo
	taskEventRepo           repo.TaskEventRepo
	costOverrideEventRepo   repo.TaskCostOverrideEventRepo
	costOverrideReviewRepo  repo.TaskCostOverrideReviewRepo
	costFinanceFlagRepo     repo.TaskCostFinanceFlagRepo
	dataScopeResolver       DataScopeResolver
	scopeUserRepo           repo.UserRepo
	userDisplayNameResolver UserDisplayNameResolver
}

type TaskDetailAggregateServiceOption func(*taskDetailAggregateService)

func WithTaskDetailScopeUserRepo(userRepo repo.UserRepo) TaskDetailAggregateServiceOption {
	return func(s *taskDetailAggregateService) {
		s.scopeUserRepo = userRepo
	}
}

func WithTaskDetailUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskDetailAggregateServiceOption {
	return func(s *taskDetailAggregateService) {
		s.userDisplayNameResolver = resolver
	}
}

func WithTaskDetailDesignAssetReadModel(designAssetRepo repo.DesignAssetRepo) TaskDetailAggregateServiceOption {
	return func(s *taskDetailAggregateService) {
		s.designAssetRepo = designAssetRepo
	}
}

func NewTaskDetailAggregateService(
	taskRepo repo.TaskRepo,
	procurementRepo repo.ProcurementRepo,
	productRepo repo.ProductRepo,
	costRuleRepo repo.CostRuleRepo,
	auditV7Repo repo.AuditV7Repo,
	outsourceRepo repo.OutsourceRepo,
	taskAssetRepo repo.TaskAssetRepo,
	warehouseRepo repo.WarehouseRepo,
	taskEventRepo repo.TaskEventRepo,
	costOverrideEventRepo repo.TaskCostOverrideEventRepo,
	costOverrideReviewRepo repo.TaskCostOverrideReviewRepo,
	costFinanceFlagRepo repo.TaskCostFinanceFlagRepo,
	opts ...TaskDetailAggregateServiceOption,
) TaskDetailAggregateService {
	svc := &taskDetailAggregateService{
		taskRepo:               taskRepo,
		procurementRepo:        procurementRepo,
		productRepo:            productRepo,
		costRuleRepo:           costRuleRepo,
		auditV7Repo:            auditV7Repo,
		outsourceRepo:          outsourceRepo,
		taskAssetRepo:          taskAssetRepo,
		warehouseRepo:          warehouseRepo,
		taskEventRepo:          taskEventRepo,
		costOverrideEventRepo:  costOverrideEventRepo,
		costOverrideReviewRepo: costOverrideReviewRepo,
		costFinanceFlagRepo:    costFinanceFlagRepo,
		dataScopeResolver:      NewRoleBasedDataScopeResolver(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *taskDetailAggregateService) GetByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetailAggregate, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail aggregate task", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo).AuthorizeTaskAction(ctx, TaskActionReadDetail, task); appErr != nil {
		return nil, appErr
	}

	taskDetail, err := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail aggregate detail", err)
	}
	attachTaskProductSelection(taskDetail, task)
	hydrateTaskDetailFilingProjection(task, taskDetail)

	var product *domain.Product
	if task.ProductID != nil {
		product, err = s.productRepo.GetByID(ctx, *task.ProductID)
		if err != nil {
			return nil, infraError("get task detail aggregate product", err)
		}
	} else {
		sel := buildTaskProductSelectionContext(task, taskDetail)
		if sel != nil && sel.DeferLocalProductBinding && erpSnapshotSufficientForDeferredBinding(sel.ERPProduct) {
			product = syntheticProductFromDeferredERPSelection(task, sel)
		}
	}

	auditRecords, err := s.auditV7Repo.ListRecordsByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task detail aggregate audit records", err)
	}

	auditHandovers, err := s.auditV7Repo.ListHandoversByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task detail aggregate handovers", err)
	}

	outsourceOrders, _, err := s.outsourceRepo.List(ctx, repo.OutsourceListFilter{
		TaskID: &taskID,
		Page:   1,
		// Use a large enough cap for detail view; one task should normally have very few rows.
		PageSize: 100,
	})
	if err != nil {
		return nil, infraError("list task detail aggregate outsource orders", err)
	}

	assets, err := s.taskAssetRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task detail aggregate assets", err)
	}

	warehouseReceipt, err := s.warehouseRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail aggregate warehouse receipt", err)
	}
	procurement, err := s.procurementRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task detail aggregate procurement", err)
	}
	skuItems, appErr := loadTaskSKUItems(ctx, s.taskRepo, task, taskDetail)
	if appErr != nil {
		return nil, appErr
	}

	eventLogs, err := s.taskEventRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task detail aggregate events", err)
	}
	var overrideEvents []*domain.TaskCostOverrideAuditEvent
	if s.costOverrideEventRepo != nil {
		overrideEvents, err = s.costOverrideEventRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list task detail aggregate cost override events", err)
		}
	}
	var reviewRecords []*domain.TaskCostOverrideReviewRecord
	if s.costOverrideReviewRepo != nil {
		reviewRecords, err = s.costOverrideReviewRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list task detail aggregate cost override reviews", err)
		}
	}
	var financeFlags []*domain.TaskCostFinanceFlag
	if s.costFinanceFlagRepo != nil {
		financeFlags, err = s.costFinanceFlagRepo.ListByTaskID(ctx, taskID)
		if err != nil {
			return nil, infraError("list task detail aggregate cost finance flags", err)
		}
	}

	if auditRecords == nil {
		auditRecords = []*domain.AuditRecord{}
	}
	if auditHandovers == nil {
		auditHandovers = []*domain.AuditHandover{}
	}
	if outsourceOrders == nil {
		outsourceOrders = []*domain.OutsourceOrder{}
	}
	if assets == nil {
		assets = []*domain.TaskAsset{}
	}
	if eventLogs == nil {
		eventLogs = []*domain.TaskEvent{}
	}

	workflow := buildTaskWorkflowSnapshot(task, taskDetail, procurement, hasFinalTaskAsset(assets), warehouseReceipt)
	matchedRuleGovernance, overrideSummary, governanceAuditSummary, overrideBoundary, appErr := buildTaskGovernanceReadModels(ctx, s.costRuleRepo, taskDetail, eventLogs, overrideEvents, reviewRecords, financeFlags)
	if appErr != nil {
		return nil, appErr
	}
	designAssets, assetVersions, appErr := loadTaskDesignAssetReadModel(ctx, s.taskRepo, s.designAssetRepo, s.taskAssetRepo, task)
	if appErr != nil {
		return nil, appErr
	}

	aggregate := &domain.TaskDetailAggregate{
		Task:                   task,
		TaskDetail:             taskDetail,
		DesignAssets:           designAssets,
		AssetVersions:          assetVersions,
		SKUItems:               skuItems,
		Product:                product,
		Assets:                 assets,
		AuditRecords:           auditRecords,
		AuditHandovers:         auditHandovers,
		OutsourceOrders:        outsourceOrders,
		WarehouseReceipt:       warehouseReceipt,
		Procurement:            procurement,
		ProcurementSummary:     buildProcurementSummary(task, taskDetail, procurement, warehouseReceipt, workflow, matchedRuleGovernance, overrideSummary, governanceAuditSummary, overrideBoundary),
		ProductSelection:       buildTaskProductSelectionContext(task, taskDetail),
		MatchedRuleGovernance:  matchedRuleGovernance,
		OverrideSummary:        overrideSummary,
		GovernanceAuditSummary: governanceAuditSummary,
		OverrideBoundary:       overrideBoundary,
		EventLogs:              eventLogs,
		AvailableActions:       availableActionsForTask(task, taskDetail, procurement, assets, warehouseReceipt),
		Workflow:               workflow,
	}
	aggregate.CreatorID = &task.CreatorID
	aggregate.RequesterID = cloneInt64Ptr(task.RequesterID)
	aggregate.DesignerID = cloneInt64Ptr(task.DesignerID)
	aggregate.CurrentHandlerID = cloneInt64Ptr(task.CurrentHandlerID)
	aggregate.AssigneeID = cloneInt64Ptr(task.DesignerID)
	if s.userDisplayNameResolver != nil {
		if task.CreatorID != 0 {
			aggregate.CreatorName = s.userDisplayNameResolver.GetDisplayName(ctx, task.CreatorID)
		}
		if task.RequesterID != nil && *task.RequesterID != 0 {
			aggregate.RequesterName = s.userDisplayNameResolver.GetDisplayName(ctx, *task.RequesterID)
		}
		if task.DesignerID != nil && *task.DesignerID != 0 {
			aggregate.DesignerName = s.userDisplayNameResolver.GetDisplayName(ctx, *task.DesignerID)
			aggregate.AssigneeName = aggregate.DesignerName
		}
		if task.CurrentHandlerID != nil && *task.CurrentHandlerID != 0 {
			aggregate.CurrentHandlerName = s.userDisplayNameResolver.GetDisplayName(ctx, *task.CurrentHandlerID)
		}
	}
	domain.HydrateTaskDetailAggregatePolicy(aggregate)
	return aggregate, nil
}

// syntheticProductFromDeferredERPSelection fills aggregate.product when product_id is null (defer_local_product_binding).
func syntheticProductFromDeferredERPSelection(task *domain.Task, sel *domain.TaskProductSelectionContext) *domain.Product {
	if task == nil || sel == nil || sel.ERPProduct == nil {
		return nil
	}
	erp := normalizeERPProductSelectionSnapshot(sel.ERPProduct)
	sku := strings.TrimSpace(task.SKUCode)
	if sku == "" {
		sku = firstNonEmptyString(erp.SKUCode, erp.SKUID, erp.ProductID)
	}
	if sku == "" {
		return nil
	}
	name := strings.TrimSpace(task.ProductNameSnapshot)
	if name == "" {
		name = firstNonEmptyString(erp.ProductName, erp.Name, sku)
	}
	erpKey := firstNonEmptyString(strings.TrimSpace(erp.ProductID), strings.TrimSpace(erp.SKUID), sku)
	spec, _ := json.Marshal(map[string]interface{}{
		"deferred_local_product_binding": true,
		"erp_product":                    erp,
		"read_model":                     "erp_snapshot_only_pending_products_row",
	})
	return &domain.Product{
		ID:           0,
		ERPProductID: erpKey,
		SKUCode:      sku,
		ProductName:  name,
		Category:     strings.TrimSpace(erp.CategoryName),
		SpecJSON:     string(spec),
		Status:       "erp_snapshot",
	}
}

func availableActionsForTask(task *domain.Task, detail *domain.TaskDetail, procurement *domain.ProcurementRecord, assets []*domain.TaskAsset, warehouseReceipt *domain.WarehouseReceipt) []domain.AvailableAction {
	if task == nil {
		return []domain.AvailableAction{}
	}

	workflow := buildTaskWorkflowSnapshot(task, detail, procurement, hasFinalTaskAsset(assets), warehouseReceipt)
	actions := []domain.AvailableAction{}

	if workflow.CanPrepareWarehouse &&
		task.TaskStatus != domain.TaskStatusPendingWarehouseReceive &&
		task.TaskStatus != domain.TaskStatusPendingClose &&
		task.TaskStatus != domain.TaskStatusCompleted {
		actions = append(actions, domain.AvailableActionPrepareWarehouse)
	}

	if task.TaskStatus == domain.TaskStatusPendingClose {
		if workflow.Closable {
			actions = append(actions, domain.AvailableActionClose)
		}
		return actions
	}

	if task.TaskType == domain.TaskTypePurchaseTask {
		switch task.TaskStatus {
		case domain.TaskStatusPendingWarehouseReceive:
			if warehouseReceipt == nil || warehouseReceipt.Status == domain.WarehouseReceiptStatusRejected {
				return append(actions, domain.AvailableActionWarehouseReceive, domain.AvailableActionWarehouseReject)
			}
			if warehouseReceipt.Status == domain.WarehouseReceiptStatusReceived {
				return append(actions, domain.AvailableActionWarehouseReject, domain.AvailableActionWarehouseComplete)
			}
		}
		return actions
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingAssign:
		return append(actions, domain.AvailableActionAssign)
	case domain.TaskStatusInProgress, domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB:
		return append(actions, domain.AvailableActionSubmitDesign)
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB:
		return append(actions,
			domain.AvailableActionClaimAudit,
			domain.AvailableActionApproveAudit,
			domain.AvailableActionRejectAudit,
			domain.AvailableActionHandover,
		)
	case domain.TaskStatusPendingOutsource:
		return append(actions, domain.AvailableActionCreateOutsource)
	case domain.TaskStatusPendingOutsourceReview:
		return append(actions,
			domain.AvailableActionClaimAudit,
			domain.AvailableActionApproveAudit,
			domain.AvailableActionHandover,
		)
	case domain.TaskStatusPendingWarehouseReceive:
		if warehouseReceipt == nil || warehouseReceipt.Status == domain.WarehouseReceiptStatusRejected {
			return append(actions,
				domain.AvailableActionWarehouseReceive,
				domain.AvailableActionWarehouseReject,
			)
		}
		if warehouseReceipt.Status == domain.WarehouseReceiptStatusReceived {
			return append(actions,
				domain.AvailableActionWarehouseReject,
				domain.AvailableActionWarehouseComplete,
			)
		}
	}
	return actions
}
