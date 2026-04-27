package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestTaskBoardServiceSummaryBuildsRoleQueues(t *testing.T) {
	stub := &taskBoardCandidateListerStub{
		items: taskBoardFixtureItems(),
	}
	svc := NewTaskBoardService(stub).(*taskBoardService)
	svc.nowFn = func() time.Time { return time.Date(2026, 3, 9, 8, 0, 0, 0, time.UTC) }

	result, appErr := svc.GetSummary(context.Background(), TaskBoardFilter{
		BoardView:   domain.TaskBoardViewProcurement,
		PreviewSize: 2,
	})
	if appErr != nil {
		t.Fatalf("GetSummary() unexpected error: %+v", appErr)
	}
	if result.BoardView != domain.TaskBoardViewProcurement {
		t.Fatalf("GetSummary() board_view = %s", result.BoardView)
	}
	if result.PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || result.PolicyScopeSummary == nil {
		t.Fatalf("GetSummary() policy scaffolding = mode:%s summary:%+v", result.PolicyMode, result.PolicyScopeSummary)
	}
	if len(result.Queues) != 3 {
		t.Fatalf("GetSummary() queue count = %d, want 3", len(result.Queues))
	}

	byKey := map[string]domain.TaskBoardQueueSummary{}
	for _, queue := range result.Queues {
		byKey[queue.QueueKey] = queue
	}

	if got := byKey["procurement_pending_followup"].Count; got != 2 {
		t.Fatalf("procurement_pending_followup count = %d, want 2", got)
	}
	if got := byKey["awaiting_arrival"].Count; got != 1 {
		t.Fatalf("awaiting_arrival count = %d, want 1", got)
	}
	if got := byKey["warehouse_pending_prepare"].Count; got != 1 {
		t.Fatalf("warehouse_pending_prepare count = %d, want 1", got)
	}
	if len(byKey["procurement_pending_followup"].SampleTasks) != 2 {
		t.Fatalf("procurement_pending_followup sample size = %d, want 2", len(byKey["procurement_pending_followup"].SampleTasks))
	}
	if byKey["procurement_pending_followup"].Filters.SubStatusScope == nil || *byKey["procurement_pending_followup"].Filters.SubStatusScope != domain.TaskSubStatusScopeProcurement {
		t.Fatalf("procurement_pending_followup filter scope = %+v", byKey["procurement_pending_followup"].Filters.SubStatusScope)
	}
	if byKey["procurement_pending_followup"].QueryTemplate.TaskType != "purchase_task" {
		t.Fatalf("procurement_pending_followup query_template.task_type = %q", byKey["procurement_pending_followup"].QueryTemplate.TaskType)
	}
	if byKey["procurement_pending_followup"].QueryTemplate.SubStatusCode != "not_started,preparing,pending_inbound" {
		t.Fatalf("procurement_pending_followup query_template.sub_status_code = %q", byKey["procurement_pending_followup"].QueryTemplate.SubStatusCode)
	}
	if len(byKey["procurement_pending_followup"].SuggestedRoles) == 0 || byKey["procurement_pending_followup"].SuggestedRoles[0] != domain.RoleOps {
		t.Fatalf("procurement_pending_followup suggested_roles = %+v", byKey["procurement_pending_followup"].SuggestedRoles)
	}
	if byKey["procurement_pending_followup"].OwnershipHint == "" {
		t.Fatal("procurement_pending_followup ownership_hint should not be empty")
	}
	if byKey["procurement_pending_followup"].PolicyMode != domain.PolicyModeRouteRoleVisibilityScaffolding || byKey["procurement_pending_followup"].PolicyScopeSummary == nil {
		t.Fatalf("queue policy scaffolding = mode:%s summary:%+v", byKey["procurement_pending_followup"].PolicyMode, byKey["procurement_pending_followup"].PolicyScopeSummary)
	}
	if len(byKey["procurement_pending_followup"].SampleTasks) == 0 || byKey["procurement_pending_followup"].SampleTasks[0].ProductSelection == nil {
		t.Fatalf("procurement_pending_followup sample product_selection = %+v", byKey["procurement_pending_followup"].SampleTasks)
	}
	if len(byKey["procurement_pending_followup"].SampleTasks) == 0 || byKey["procurement_pending_followup"].SampleTasks[0].PlatformEntryBoundary == nil {
		t.Fatalf("procurement_pending_followup sample platform_entry_boundary = %+v", byKey["procurement_pending_followup"].SampleTasks)
	}
	if byKey["procurement_pending_followup"].SampleTasks[0].PlatformEntryBoundary.FinanceEntrySummary == nil {
		t.Fatalf("procurement_pending_followup finance_entry_summary = %+v", byKey["procurement_pending_followup"].SampleTasks[0].PlatformEntryBoundary)
	}
	if len(stub.lastCandidateFilters) != 3 {
		t.Fatalf("board candidate preset count = %d, want 3", len(stub.lastCandidateFilters))
	}
	if stub.calls != 1 {
		t.Fatalf("GetSummary() candidate scan call count = %d, want 1", stub.calls)
	}
}

func TestTaskBoardServiceQueuesPaginatesAndFiltersQueueKey(t *testing.T) {
	stub := &taskBoardCandidateListerStub{
		items: taskBoardFixtureItems(),
	}
	svc := NewTaskBoardService(stub).(*taskBoardService)
	svc.nowFn = func() time.Time { return time.Date(2026, 3, 9, 8, 0, 0, 0, time.UTC) }

	result, appErr := svc.GetQueues(context.Background(), TaskBoardFilter{
		BoardView: domain.TaskBoardViewDesigner,
		QueueKey:  "design_pending_submit",
		TaskFilter: TaskFilter{
			Page:     1,
			PageSize: 1,
		},
	})
	if appErr != nil {
		t.Fatalf("GetQueues() unexpected error: %+v", appErr)
	}
	if len(result.Queues) != 1 {
		t.Fatalf("GetQueues() queue count = %d, want 1", len(result.Queues))
	}
	queue := result.Queues[0]
	if queue.Count != 2 {
		t.Fatalf("design_pending_submit count = %d, want 2", queue.Count)
	}
	if queue.SuggestedActorType != "assigned_actor" {
		t.Fatalf("design_pending_submit suggested_actor_type = %q", queue.SuggestedActorType)
	}
	if queue.Pagination.Total != 2 || queue.Pagination.PageSize != 1 {
		t.Fatalf("design_pending_submit pagination = %+v", queue.Pagination)
	}
	if len(queue.Tasks) != 1 {
		t.Fatalf("design_pending_submit tasks len = %d, want 1", len(queue.Tasks))
	}
	if queue.Tasks[0].ID != 2 {
		t.Fatalf("design_pending_submit first task id = %d, want 2", queue.Tasks[0].ID)
	}
	if len(stub.lastCandidateFilters) != 1 {
		t.Fatalf("candidate preset count = %d, want 1", len(stub.lastCandidateFilters))
	}
	if stub.calls != 1 {
		t.Fatalf("GetQueues() candidate scan call count = %d, want 1", stub.calls)
	}
}

func TestTaskBoardServiceSummaryAppliesGlobalConvergedFilterAcrossQueues(t *testing.T) {
	stub := &taskBoardCandidateListerStub{
		items: taskBoardFixtureItems(),
	}
	svc := NewTaskBoardService(stub).(*taskBoardService)

	result, appErr := svc.GetSummary(context.Background(), TaskBoardFilter{
		BoardView: domain.TaskBoardViewProcurement,
		TaskFilter: TaskFilter{
			TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
				CoordinationStatuses: []domain.ProcurementCoordinationStatus{
					domain.ProcurementCoordinationStatusAwaitingArrival,
				},
			},
		},
	})
	if appErr != nil {
		t.Fatalf("GetSummary() unexpected error: %+v", appErr)
	}
	if len(result.Queues) != 2 {
		t.Fatalf("GetSummary() queue count = %d, want 2", len(result.Queues))
	}

	byKey := map[string]domain.TaskBoardQueueSummary{}
	for _, queue := range result.Queues {
		byKey[queue.QueueKey] = queue
	}

	if got := byKey["procurement_pending_followup"].Count; got != 1 {
		t.Fatalf("procurement_pending_followup count = %d, want 1", got)
	}
	if got := byKey["awaiting_arrival"].Count; got != 1 {
		t.Fatalf("awaiting_arrival count = %d, want 1", got)
	}
	if _, exists := byKey["warehouse_pending_prepare"]; exists {
		t.Fatal("warehouse_pending_prepare should be filtered out by the global converged filter")
	}
	if stub.lastFilter.CoordinationStatuses[0] != domain.ProcurementCoordinationStatusAwaitingArrival {
		t.Fatalf("candidate scan coordination_status = %s, want %s", stub.lastFilter.CoordinationStatuses[0], domain.ProcurementCoordinationStatusAwaitingArrival)
	}
	if stub.calls != 1 {
		t.Fatalf("GetSummary() candidate scan call count = %d, want 1", stub.calls)
	}
}

func TestTaskBoardServiceRejectsInvalidBoardView(t *testing.T) {
	svc := NewTaskBoardService(&taskBoardCandidateListerStub{}).(*taskBoardService)

	_, appErr := svc.GetSummary(context.Background(), TaskBoardFilter{
		BoardView: domain.TaskBoardView("finance"),
	})
	if appErr == nil {
		t.Fatal("GetSummary() expected error, got nil")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("GetSummary() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

type taskBoardCandidateListerStub struct {
	items                []*domain.TaskListItem
	calls                int
	lastFilter           TaskFilter
	lastCandidateFilters []domain.TaskQueryFilterDefinition
}

func (s *taskBoardCandidateListerStub) ListBoardCandidates(_ context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError) {
	s.calls++
	s.lastFilter = filter
	s.lastCandidateFilters = append([]domain.TaskQueryFilterDefinition(nil), presets...)
	filtered := make([]*domain.TaskListItem, 0, len(s.items))
	for _, item := range s.items {
		if !matchesTaskFilter(item, filter) {
			continue
		}
		for _, preset := range presets {
			effective, ok := mergeTaskBoardFilter(filter, preset)
			if !ok {
				continue
			}
			if matchesTaskFilter(item, effective) {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered, nil
}

func taskBoardFixtureItems() []*domain.TaskListItem {
	designerID := int64(200)
	now := time.Date(2026, 3, 9, 9, 0, 0, 0, time.UTC)
	return []*domain.TaskListItem{
		{
			ID:         1,
			TaskNo:     "T-OPS-1",
			TaskType:   domain.TaskTypeOriginalProductDevelopment,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  101,
			TaskStatus: domain.TaskStatusPendingAssign,
			UpdatedAt:  now.Add(-9 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusCreated,
				WarehouseBlockingReasons: []domain.WorkflowReason{
					{Code: domain.WorkflowReasonCategoryMissing, Message: "Category missing."},
				},
			},
		},
		{
			ID:         2,
			TaskNo:     "T-DESIGN-1",
			TaskType:   domain.TaskTypeOriginalProductDevelopment,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  102,
			DesignerID: &designerID,
			TaskStatus: domain.TaskStatusInProgress,
			UpdatedAt:  now.Add(-1 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusFiled,
				SubStatus: domain.TaskSubStatusSnapshot{
					Design: domain.TaskSubStatusItem{Code: domain.TaskSubStatusDesigning, Label: "Designing", Source: domain.TaskSubStatusSourceTaskStatus},
				},
			},
		},
		{
			ID:         9,
			TaskNo:     "T-DESIGN-2",
			TaskType:   domain.TaskTypeOriginalProductDevelopment,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  102,
			DesignerID: &designerID,
			TaskStatus: domain.TaskStatusRejectedByAuditA,
			UpdatedAt:  now.Add(-5 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusFiled,
				SubStatus: domain.TaskSubStatusSnapshot{
					Design: domain.TaskSubStatusItem{Code: domain.TaskSubStatusReworkRequired, Label: "Rework required", Source: domain.TaskSubStatusSourceTaskStatus},
				},
			},
		},
		{
			ID:         3,
			TaskNo:     "T-AUDIT-1",
			TaskType:   domain.TaskTypeOriginalProductDevelopment,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  103,
			TaskStatus: domain.TaskStatusPendingAuditA,
			UpdatedAt:  now.Add(-2 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusFiled,
				SubStatus: domain.TaskSubStatusSnapshot{
					Audit: domain.TaskSubStatusItem{Code: domain.TaskSubStatusInReview, Label: "In review", Source: domain.TaskSubStatusSourceTaskStatus},
				},
			},
		},
		{
			ID:         4,
			TaskNo:     "T-PROC-1",
			TaskType:   domain.TaskTypePurchaseTask,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  104,
			TaskStatus: domain.TaskStatusPendingAssign,
			UpdatedAt:  now.Add(-4 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusFiled,
				SubStatus: domain.TaskSubStatusSnapshot{
					Procurement: domain.TaskSubStatusItem{Code: domain.TaskSubStatusNotStarted, Label: "Not started", Source: domain.TaskSubStatusSourceTaskType},
				},
			},
			ProductSelection: &domain.TaskProductSelectionSummary{
				SelectedProductID:      int64Ptr(5004),
				SelectedProductName:    "Procurement Pending Product",
				SelectedProductSKUCode: "SKU-5004",
				MatchedCategoryCode:    "KT",
				MatchedSearchEntryCode: "KT",
				SourceProductID:        int64Ptr(5004),
				SourceProductName:      "Procurement Pending Product",
				SourceMatchType:        taskProductSelectionMatchMapped,
				SourceMatchRule:        "KT",
				SourceSearchEntryCode:  "KT",
			},
		},
		{
			ID:         5,
			TaskNo:     "T-PROC-2",
			TaskType:   domain.TaskTypePurchaseTask,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  104,
			TaskStatus: domain.TaskStatusPendingAssign,
			UpdatedAt:  now.Add(-3 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusFiled,
				SubStatus: domain.TaskSubStatusSnapshot{
					Procurement: domain.TaskSubStatusItem{Code: domain.TaskSubStatusPendingInbound, Label: "Awaiting arrival", Source: domain.TaskSubStatusSourceProcurement},
				},
			},
			ProcurementSummary: &domain.ProcurementSummary{
				Status:             domain.ProcurementStatusInProgress,
				CoordinationStatus: domain.ProcurementCoordinationStatusAwaitingArrival,
			},
			ProductSelection: &domain.TaskProductSelectionSummary{
				SelectedProductID:      int64Ptr(5005),
				SelectedProductName:    "Procurement Existing Product",
				SelectedProductSKUCode: "SKU-5005",
				MatchedCategoryCode:    "KT",
				MatchedSearchEntryCode: "KT",
				SourceProductID:        int64Ptr(5005),
				SourceProductName:      "Procurement Existing Product",
				SourceMatchType:        taskProductSelectionMatchMapped,
				SourceMatchRule:        "KT",
				SourceSearchEntryCode:  "KT",
			},
		},
		{
			ID:         6,
			TaskNo:     "T-PROC-3",
			TaskType:   domain.TaskTypePurchaseTask,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  104,
			TaskStatus: domain.TaskStatusPendingAssign,
			UpdatedAt:  now.Add(-6 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus:          domain.TaskMainStatusFiled,
				CanPrepareWarehouse: true,
				SubStatus: domain.TaskSubStatusSnapshot{
					Procurement: domain.TaskSubStatusItem{Code: domain.TaskSubStatusReady, Label: "Ready for warehouse", Source: domain.TaskSubStatusSourceProcurement},
				},
			},
			ProcurementSummary: &domain.ProcurementSummary{
				Status:             domain.ProcurementStatusCompleted,
				CoordinationStatus: domain.ProcurementCoordinationStatusReadyForWarehouse,
			},
		},
		{
			ID:         7,
			TaskNo:     "T-WH-1",
			TaskType:   domain.TaskTypePurchaseTask,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  105,
			TaskStatus: domain.TaskStatusPendingWarehouseReceive,
			UpdatedAt:  now.Add(-7 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusPendingWarehouseReceive,
				SubStatus: domain.TaskSubStatusSnapshot{
					Warehouse: domain.TaskSubStatusItem{Code: domain.TaskSubStatusPendingReceive, Label: "Pending receive", Source: domain.TaskSubStatusSourceTaskStatus},
				},
			},
		},
		{
			ID:         8,
			TaskNo:     "T-CLOSE-1",
			TaskType:   domain.TaskTypePurchaseTask,
			SourceMode: domain.TaskSourceModeExistingProduct,
			CreatorID:  106,
			TaskStatus: domain.TaskStatusPendingClose,
			UpdatedAt:  now.Add(-8 * time.Hour),
			Workflow: domain.TaskWorkflowSnapshot{
				MainStatus: domain.TaskMainStatusPendingClose,
			},
		},
	}
}
