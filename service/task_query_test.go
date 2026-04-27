package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskQueryStageVisibilityUATMatrix(t *testing.T) {
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	taskRepo := &taskQueryVisibilityRepo{
		prdTaskRepo: prdTaskRepo{
			listItems: []*domain.TaskListItem{
				newScopedTaskListItem(1, "T-AUDIT-A", domain.TaskStatusPendingAuditA, domain.WorkflowLaneNormal, now),
				newScopedTaskListItem(2, "T-AUDIT-B", domain.TaskStatusPendingAuditB, domain.WorkflowLaneNormal, now.Add(-1*time.Minute)),
				newScopedTaskListItem(3, "T-CR", domain.TaskStatusPendingCustomizationReview, domain.WorkflowLaneCustomization, now.Add(-2*time.Minute)),
				newScopedTaskListItem(4, "T-ER", domain.TaskStatusPendingEffectReview, domain.WorkflowLaneCustomization, now.Add(-3*time.Minute)),
				newScopedTaskListItem(5, "T-CPROD", domain.TaskStatusPendingCustomizationProduction, domain.WorkflowLaneCustomization, now.Add(-4*time.Minute)),
				newScopedTaskListItem(6, "T-WHQC", domain.TaskStatusPendingWarehouseQC, domain.WorkflowLaneNormal, now.Add(-5*time.Minute)),
				newScopedTaskListItem(7, "T-WHRECV", domain.TaskStatusPendingWarehouseReceive, domain.WorkflowLaneCustomization, now.Add(-6*time.Minute)),
				newScopedTaskListItem(8, "T-INPROG-N", domain.TaskStatusInProgress, domain.WorkflowLaneNormal, now.Add(-7*time.Minute)),
				newScopedTaskListItem(9, "T-INPROG-C", domain.TaskStatusInProgress, domain.WorkflowLaneCustomization, now.Add(-8*time.Minute)),
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
		step04TxRunner{},
	)

	boardPresets := []domain.TaskQueryFilterDefinition{
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingAuditA}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneNormal}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingAuditB}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneNormal}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingCustomizationReview}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingEffectReview}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingCustomizationProduction}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingWarehouseQC}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneNormal}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusPendingWarehouseReceive}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusInProgress}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneNormal}},
		{Statuses: []domain.TaskStatus{domain.TaskStatusInProgress}, WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization}},
	}

	tests := []struct {
		name      string
		actor     domain.RequestActor
		wantCount int
	}{
		{
			name: "super_admin sees all",
			actor: domain.RequestActor{
				ID:    1001,
				Roles: []domain.Role{domain.RoleSuperAdmin},
			},
			wantCount: 9,
		},
		{
			name: "operations dept admin plus ops unchanged by owner department",
			actor: domain.RequestActor{
				ID:         1002,
				Department: string(domain.DepartmentOperations),
				Roles:      []domain.Role{domain.RoleDeptAdmin, domain.RoleOps},
			},
			wantCount: 9,
		},
		{
			name: "audit department admin gets both audit stages and customization review stages",
			actor: domain.RequestActor{
				ID:         1003,
				Department: string(domain.DepartmentAudit),
				Roles: []domain.Role{
					domain.RoleDeptAdmin,
					domain.RoleAuditA,
					domain.RoleAuditB,
					domain.RoleCustomizationReviewer,
				},
			},
			wantCount: 4,
		},
		{
			name: "normal audit group gets audit a and b only",
			actor: domain.RequestActor{
				ID:         1004,
				Department: string(domain.DepartmentAudit),
				Roles:      []domain.Role{domain.RoleAuditA, domain.RoleAuditB},
			},
			wantCount: 2,
		},
		{
			name: "customization reviewer gets customization review stages only",
			actor: domain.RequestActor{
				ID:         1005,
				Department: string(domain.DepartmentAudit),
				Roles:      []domain.Role{domain.RoleCustomizationReviewer},
			},
			wantCount: 2,
		},
		{
			name: "cloud warehouse dept admin plus warehouse sees both warehouse lanes",
			actor: domain.RequestActor{
				ID:         1006,
				Department: string(domain.DepartmentCloudWarehouse),
				Roles:      []domain.Role{domain.RoleDeptAdmin, domain.RoleWarehouse},
			},
			wantCount: 2,
		},
		{
			name: "customization art dept admin plus operator sees customization review and production stages",
			actor: domain.RequestActor{
				ID:         1007,
				Department: string(domain.DepartmentCustomizationArt),
				Roles:      []domain.Role{domain.RoleDeptAdmin, domain.RoleCustomizationOperator},
			},
			wantCount: 4,
		},
		{
			name: "customization operator sees customization production and in progress customization only",
			actor: domain.RequestActor{
				ID:         1008,
				Department: string(domain.DepartmentCustomizationArt),
				Roles:      []domain.Role{domain.RoleCustomizationOperator},
			},
			wantCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := domain.WithRequestActor(context.Background(), tc.actor)

			items, _, appErr := svc.List(ctx, TaskFilter{Page: 1, PageSize: 50})
			if appErr != nil {
				t.Fatalf("List() error = %+v", appErr)
			}
			if len(items) != tc.wantCount {
				t.Fatalf("List() len = %d, want %d, task_nos=%v", len(items), tc.wantCount, collectTaskNos(items))
			}

			candidates, appErr := svc.ListBoardCandidates(ctx, TaskFilter{}, boardPresets)
			if appErr != nil {
				t.Fatalf("ListBoardCandidates() error = %+v", appErr)
			}
			if len(candidates) != tc.wantCount {
				t.Fatalf("ListBoardCandidates() len = %d, want %d, task_nos=%v", len(candidates), tc.wantCount, collectTaskNos(candidates))
			}
		})
	}
}

type taskQueryVisibilityRepo struct {
	prdTaskRepo
}

func (r *taskQueryVisibilityRepo) List(_ context.Context, filter repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	r.lastListFilter = filter
	r.listCalls++

	taskFilter := TaskFilter{
		TaskQueryFilterDefinition: filter.TaskQueryFilterDefinition,
		CreatorID:                 filter.CreatorID,
		DesignerID:                filter.DesignerID,
		NeedOutsource:             filter.NeedOutsource,
		Overdue:                   filter.Overdue,
		Keyword:                   filter.Keyword,
		Page:                      filter.Page,
		PageSize:                  filter.PageSize,
	}

	filtered := make([]*domain.TaskListItem, 0, len(r.listItems))
	for _, item := range r.listItems {
		if item == nil {
			continue
		}
		copied := *item
		applyTaskListItemReadModelOrgOwnership(&copied)
		if !taskListItemVisibleToScope(&copied, filter) {
			continue
		}
		copied.Workflow = buildTaskWorkflowSnapshotFromListItem(&copied)
		copied.ProcurementSummary = buildProcurementSummaryFromListItem(&copied)
		if matchesTaskFilter(&copied, taskFilter) {
			filtered = append(filtered, &copied)
		}
	}

	pagination := buildPaginationMeta(filter.Page, filter.PageSize, int64(len(filtered)))
	start := (pagination.Page - 1) * pagination.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pagination.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], int64(len(filtered)), nil
}

func (r *taskQueryVisibilityRepo) ListBoardCandidates(_ context.Context, filter repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	r.lastListFilter = filter.TaskListFilter
	r.listCalls++

	taskFilter := TaskFilter{
		TaskQueryFilterDefinition: filter.TaskListFilter.TaskQueryFilterDefinition,
		CreatorID:                 filter.CreatorID,
		DesignerID:                filter.DesignerID,
		NeedOutsource:             filter.NeedOutsource,
		Overdue:                   filter.Overdue,
		Keyword:                   filter.Keyword,
	}

	filtered := make([]*domain.TaskListItem, 0, len(r.listItems))
	for _, item := range r.listItems {
		if item == nil {
			continue
		}
		copied := *item
		applyTaskListItemReadModelOrgOwnership(&copied)
		if !taskListItemVisibleToScope(&copied, filter.TaskListFilter) {
			continue
		}
		copied.Workflow = buildTaskWorkflowSnapshotFromListItem(&copied)
		copied.ProcurementSummary = buildProcurementSummaryFromListItem(&copied)
		if !matchesTaskFilter(&copied, taskFilter) {
			continue
		}
		for _, preset := range filter.CandidateFilters {
			effective, ok := mergeTaskBoardFilter(taskFilter, preset)
			if !ok {
				continue
			}
			if matchesTaskFilter(&copied, effective) {
				filtered = append(filtered, &copied)
				break
			}
		}
	}
	return filtered, nil
}

func taskListItemVisibleToScope(item *domain.TaskListItem, filter repo.TaskListFilter) bool {
	scope := &DataScope{
		ViewAll:           filter.ScopeViewAll,
		DepartmentCodes:   append([]string(nil), filter.ScopeDepartmentCodes...),
		TeamCodes:         append([]string(nil), filter.ScopeTeamCodes...),
		UserIDs:           append([]int64(nil), filter.ScopeUserIDs...),
		StageVisibilities: make([]StageVisibility, 0, len(filter.ScopeStageVisibilities)),
	}
	for _, visibility := range filter.ScopeStageVisibilities {
		scope.StageVisibilities = append(scope.StageVisibilities, StageVisibility{
			Statuses: append([]domain.TaskStatus(nil), visibility.Statuses...),
			Lane:     cloneWorkflowLane(visibility.Lane),
		})
	}
	return canViewTaskWithScope(taskFromListItemForScope(item), scope)
}

func taskFromListItemForScope(item *domain.TaskListItem) *domain.Task {
	if item == nil {
		return nil
	}
	customizationRequired := item.CustomizationRequired || item.WorkflowLane == domain.WorkflowLaneCustomization
	return &domain.Task{
		ID:                    item.ID,
		CreatorID:             item.CreatorID,
		DesignerID:            item.DesignerID,
		CurrentHandlerID:      item.CurrentHandlerID,
		OwnerDepartment:       item.OwnerDepartment,
		OwnerOrgTeam:          item.OwnerOrgTeam,
		TaskStatus:            item.TaskStatus,
		CustomizationRequired: customizationRequired,
	}
}

func newScopedTaskListItem(id int64, taskNo string, status domain.TaskStatus, lane domain.WorkflowLane, updatedAt time.Time) *domain.TaskListItem {
	customization := lane == domain.WorkflowLaneCustomization
	return &domain.TaskListItem{
		ID:                    id,
		TaskNo:                taskNo,
		SKUCode:               taskNo + "-SKU",
		ProductNameSnapshot:   taskNo,
		TaskType:              domain.TaskTypeOriginalProductDevelopment,
		SourceMode:            domain.TaskSourceModeExistingProduct,
		OwnerDepartment:       string(domain.DepartmentOperations),
		TaskStatus:            status,
		Priority:              domain.TaskPriorityNormal,
		CreatorID:             8000 + id,
		CreatedAt:             updatedAt,
		UpdatedAt:             updatedAt,
		BatchMode:             domain.TaskBatchModeSingle,
		BatchItemCount:        1,
		CustomizationRequired: customization,
		WorkflowLane:          lane,
	}
}

func collectTaskNos(items []*domain.TaskListItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, item.TaskNo)
	}
	return out
}
