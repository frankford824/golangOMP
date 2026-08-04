package service

import (
	"context"
	"sort"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func (s *taskService) listTasks(ctx context.Context, filter TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	normalized, appErr := normalizeTaskFilter(filter)
	if appErr != nil {
		return nil, domain.PaginationMeta{}, appErr
	}

	repoFilter := taskFilterToRepoTaskListFilter(normalized, normalized.Page, normalized.PageSize, mainTaskReadScope(ctx))
	items, total, err := s.taskRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list tasks", err)
	}
	items = hydrateTaskListItems(items)
	return items, buildPaginationMeta(normalized.Page, normalized.PageSize, total), nil
}

func mainTaskReadScope(ctx context.Context) *DataScope {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		// Internal callers and repository-focused tests do not carry an HTTP
		// identity. Public routes always inject an effective-access actor.
		return &DataScope{ViewAll: true}
	}
	access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)
	scope := &DataScope{ViewAll: access.Global}
	if access.Self && actor.ID > 0 {
		scope.UserIDs = []int64{actor.ID}
	}
	scope.DepartmentIDs = append([]int64(nil), access.DepartmentIDs...)
	scope.TeamIDs = append([]int64(nil), access.TeamIDs...)
	return scope
}

func normalizeTaskFilter(filter TaskFilter) (TaskFilter, *domain.AppError) {
	return filter, nil
}

func taskFilterToRepoTaskListFilter(filter TaskFilter, page, pageSize int, scope *DataScope) repo.TaskListFilter {
	repoFilter := repo.TaskListFilter{
		TaskQueryFilterDefinition: filter.TaskQueryFilterDefinition,
		CreatorID:                 filter.CreatorID,
		MineActorID:               filter.MineActorID,
		DesignerID:                filter.DesignerID,
		DesignerEmpty:             filter.DesignerEmpty,
		Overdue:                   filter.Overdue,
		OperationalBucket:         filter.OperationalBucket,
		CreatedFrom:               filter.CreatedFrom,
		CreatedTo:                 filter.CreatedTo,
		Keyword:                   filter.Keyword,
		Sort:                      filter.Sort,
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
		if selection := buildTaskProductSelectionSummaryFromListItem(item); selection != nil {
			item.ProductSelection = selection
		}
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
	if len(filter.Priorities) > 0 && !containsTaskPriority(filter.Priorities, item.Priority) {
		return false
	}
	if len(filter.TaskTypes) > 0 && !containsTaskType(filter.TaskTypes, item.TaskType) {
		return false
	}
	if len(filter.SourceModes) > 0 && !containsTaskSourceMode(filter.SourceModes, item.SourceMode) {
		return false
	}
	if len(filter.BusinessLanes) > 0 && !containsBusinessLane(filter.BusinessLanes, item.BusinessLane) {
		return false
	}
	if len(filter.OwnerDepartments) > 0 && !containsTaskOwnerDepartment(filter.OwnerDepartments, item.OwnerDepartment) {
		return false
	}
	if len(filter.OwnerOrgTeams) > 0 && !containsTaskOwnerOrgTeam(filter.OwnerOrgTeams, item.OwnerOrgTeam) {
		return false
	}
	if filter.DesignerEmpty != nil && *filter.DesignerEmpty {
		if item.DesignerID != nil && *item.DesignerID > 0 {
			return false
		}
	}
	if filter.CreatedFrom != nil && item.CreatedAt.Before(*filter.CreatedFrom) {
		return false
	}
	if filter.CreatedTo != nil && item.CreatedAt.After(*filter.CreatedTo) {
		return false
	}
	return true
}

func containsTaskOwnerDepartment(values []string, target string) bool {
	for _, value := range values {
		if domain.OrgDepartmentsEquivalent(value, target) || strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func containsTaskOwnerOrgTeam(values []string, target string) bool {
	for _, value := range values {
		if domain.OrgTeamsEquivalent(value, target) || strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
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

func containsTaskPriority(values []domain.TaskPriority, want domain.TaskPriority) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func containsBusinessLane(values []domain.TaskBusinessLane, want domain.TaskBusinessLane) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
