package service

import (
	"context"
	"sort"

	"workflow/domain"
	"workflow/repo"
)

func (s *taskService) listTasks(ctx context.Context, filter TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	normalized, appErr := normalizeTaskFilter(filter)
	if appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}

	repoFilter := taskFilterToRepoTaskListFilter(normalized, normalized.Page, normalized.PageSize, mainTaskReadScope())
	items, total, err := s.taskRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list tasks", err)
	}
	items = hydrateTaskListItems(items)
	return items, buildPaginationMeta(normalized.Page, normalized.PageSize, total), nil
}

func (s *taskService) listBoardCandidates(ctx context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError) {
	if len(presets) == 0 {
		return []*domain.TaskListItem{}, nil
	}

	normalized, appErr := normalizeTaskFilter(filter)
	if appErr != nil {
		return nil, appErr
	}

	items, err := s.taskRepo.ListBoardCandidates(ctx, repo.TaskBoardCandidateFilter{
		TaskListFilter:   taskFilterToRepoTaskListFilter(normalized, 0, 0, mainTaskReadScope()),
		CandidateFilters: append([]domain.TaskQueryFilterDefinition(nil), presets...),
	})
	if err != nil {
		return nil, infraError("list board candidates", err)
	}
	return hydrateTaskListItems(items), nil
}

func mainTaskReadScope() *DataScope {
	return &DataScope{ViewAll: true}
}

func normalizeTaskFilter(filter TaskFilter) (TaskFilter, *domain.AppError) {
	if filter.SubStatusScope != nil {
		if len(filter.SubStatusCodes) == 0 {
			return TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "sub_status_scope requires sub_status_code", nil)
		}
		if !filter.SubStatusScope.Valid() {
			return TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "sub_status_scope must be design/audit/procurement/warehouse/customization/outsource/production", nil)
		}
	}
	if len(filter.SubStatusCodes) == 0 {
		filter.SubStatusScope = nil
	}
	return filter, nil
}

func taskFilterToRepoTaskListFilter(filter TaskFilter, page, pageSize int, scope *DataScope) repo.TaskListFilter {
	repoFilter := repo.TaskListFilter{
		TaskQueryFilterDefinition: filter.TaskQueryFilterDefinition,
		CreatorID:                 filter.CreatorID,
		DesignerID:                filter.DesignerID,
		NeedOutsource:             filter.NeedOutsource,
		Overdue:                   filter.Overdue,
		Keyword:                   filter.Keyword,
		Page:                      page,
		PageSize:                  pageSize,
	}
	return applyTaskOrgVisibilityScope(repoFilter, scope)
}

func hydrateTaskListItems(items []*domain.TaskListItem) []*domain.TaskListItem {
	if items == nil {
		return []*domain.TaskListItem{}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		applyTaskListItemReadModelOrgOwnership(item)
		item.Workflow = buildTaskWorkflowSnapshotFromListItem(item)
		if selection := buildTaskProductSelectionSummaryFromListItem(item); selection != nil {
			item.ProductSelection = selection
		}
		item.ProcurementSummary = buildProcurementSummaryFromListItem(item)
		domain.HydrateTaskListItemPolicy(item)
	}
	return items
}

func matchesTaskFilter(item *domain.TaskListItem, filter TaskFilter) bool {
	if item == nil {
		return false
	}
	if len(filter.Statuses) > 0 && !containsTaskStatus(filter.Statuses, item.TaskStatus) {
		return false
	}
	if len(filter.TaskTypes) > 0 && !containsTaskType(filter.TaskTypes, item.TaskType) {
		return false
	}
	if len(filter.SourceModes) > 0 && !containsTaskSourceMode(filter.SourceModes, item.SourceMode) {
		return false
	}
	if len(filter.WorkflowLanes) > 0 && !containsWorkflowLane(filter.WorkflowLanes, item.WorkflowLane) {
		return false
	}
	if len(filter.MainStatuses) > 0 && !containsTaskMainStatus(filter.MainStatuses, item.Workflow.MainStatus) {
		return false
	}
	if len(filter.SubStatusCodes) > 0 {
		if filter.SubStatusScope != nil {
			code := taskSubStatusCodeByScope(item, *filter.SubStatusScope)
			if !containsTaskSubStatusCode(filter.SubStatusCodes, code) {
				return false
			}
		} else if !matchesAnyTaskSubStatusCode(item, filter.SubStatusCodes) {
			return false
		}
	}
	if len(filter.CoordinationStatuses) > 0 {
		status, ok := taskCoordinationStatus(item)
		if !ok || !containsCoordinationStatus(filter.CoordinationStatuses, status) {
			return false
		}
	}
	if len(filter.OwnerDepartments) > 0 && !containsStringValue(filter.OwnerDepartments, item.OwnerDepartment) {
		return false
	}
	if len(filter.OwnerOrgTeams) > 0 && !containsStringValue(filter.OwnerOrgTeams, item.OwnerOrgTeam) {
		return false
	}
	if filter.WarehousePrepareReady != nil {
		if item.Workflow.CanPrepareWarehouse != *filter.WarehousePrepareReady {
			return false
		}
	}
	if filter.WarehouseReceiveReady != nil {
		if taskWarehouseReceiveReady(item) != *filter.WarehouseReceiveReady {
			return false
		}
	}
	if len(filter.WarehouseBlockingReasonCodes) > 0 && !hasAnyWorkflowReasonCode(item, filter.WarehouseBlockingReasonCodes) {
		return false
	}
	return true
}

func taskSubStatusCodeByScope(item *domain.TaskListItem, scope domain.TaskSubStatusScope) domain.TaskSubStatusCode {
	switch scope {
	case domain.TaskSubStatusScopeDesign:
		return item.Workflow.SubStatus.Design.Code
	case domain.TaskSubStatusScopeAudit:
		return item.Workflow.SubStatus.Audit.Code
	case domain.TaskSubStatusScopeProcurement:
		return item.Workflow.SubStatus.Procurement.Code
	case domain.TaskSubStatusScopeWarehouse:
		return item.Workflow.SubStatus.Warehouse.Code
	case domain.TaskSubStatusScopeCustomization:
		return item.Workflow.SubStatus.Customization.Code
	case domain.TaskSubStatusScopeOutsource:
		if item.Workflow.SubStatus.Customization.Code != "" {
			return item.Workflow.SubStatus.Customization.Code
		}
		return item.Workflow.SubStatus.Outsource.Code
	case domain.TaskSubStatusScopeProduction:
		return item.Workflow.SubStatus.Production.Code
	default:
		return ""
	}
}

func matchesAnyTaskSubStatusCode(item *domain.TaskListItem, codes []domain.TaskSubStatusCode) bool {
	candidates := []domain.TaskSubStatusCode{
		item.Workflow.SubStatus.Design.Code,
		item.Workflow.SubStatus.Audit.Code,
		item.Workflow.SubStatus.Procurement.Code,
		item.Workflow.SubStatus.Warehouse.Code,
		item.Workflow.SubStatus.Customization.Code,
		item.Workflow.SubStatus.Outsource.Code,
		item.Workflow.SubStatus.Production.Code,
	}
	for _, candidate := range candidates {
		if containsTaskSubStatusCode(codes, candidate) {
			return true
		}
	}
	return false
}

func taskCoordinationStatus(item *domain.TaskListItem) (domain.ProcurementCoordinationStatus, bool) {
	if item == nil || item.TaskType != domain.TaskTypePurchaseTask {
		return "", false
	}
	if item.ProcurementSummary != nil {
		return item.ProcurementSummary.CoordinationStatus, true
	}
	task := &domain.Task{
		TaskType:   item.TaskType,
		TaskStatus: item.TaskStatus,
	}
	var record *domain.ProcurementRecord
	if item.ProcurementStatus != nil {
		record = &domain.ProcurementRecord{
			Status:             *item.ProcurementStatus,
			ProcurementPrice:   item.ProcurementPrice,
			Quantity:           item.ProcurementQuantity,
			SupplierName:       item.SupplierName,
			ExpectedDeliveryAt: item.ExpectedDeliveryAt,
		}
	}
	var warehouse *domain.WarehouseReceipt
	if item.WarehouseStatus != nil {
		warehouse = &domain.WarehouseReceipt{Status: *item.WarehouseStatus}
	}
	return deriveProcurementCoordinationStatus(task, record, warehouse), true
}

func taskWarehouseReceiveReady(item *domain.TaskListItem) bool {
	if item == nil {
		return false
	}
	task := &domain.Task{
		TaskType:   item.TaskType,
		TaskStatus: item.TaskStatus,
	}
	var warehouse *domain.WarehouseReceipt
	if item.WarehouseStatus != nil {
		warehouse = &domain.WarehouseReceipt{Status: *item.WarehouseStatus}
	}
	return canReceiveInWarehouse(task, warehouse)
}

func hasAnyWorkflowReasonCode(item *domain.TaskListItem, codes []domain.WorkflowReasonCode) bool {
	for _, code := range codes {
		for _, reason := range item.Workflow.WarehouseBlockingReasons {
			if reason.Code == code {
				return true
			}
		}
	}
	return false
}

func paginateTaskListItems(items []*domain.TaskListItem, page, pageSize int) ([]*domain.TaskListItem, domain.PaginationMeta) {
	pagination := buildPaginationMeta(page, pageSize, int64(len(items)))
	start := (pagination.Page - 1) * pagination.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pagination.PageSize
	if end > len(items) {
		end = len(items)
	}
	pageItems := make([]*domain.TaskListItem, 0, end-start)
	pageItems = append(pageItems, items[start:end]...)
	return pageItems, pagination
}

func sortTaskListItems(items []*domain.TaskListItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func containsTaskStatus(values []domain.TaskStatus, want domain.TaskStatus) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsTaskType(values []domain.TaskType, want domain.TaskType) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsTaskSourceMode(values []domain.TaskSourceMode, want domain.TaskSourceMode) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsTaskMainStatus(values []domain.TaskMainStatus, want domain.TaskMainStatus) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsWorkflowLane(values []domain.WorkflowLane, want domain.WorkflowLane) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsTaskSubStatusCode(values []domain.TaskSubStatusCode, want domain.TaskSubStatusCode) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsCoordinationStatus(values []domain.ProcurementCoordinationStatus, want domain.ProcurementCoordinationStatus) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
