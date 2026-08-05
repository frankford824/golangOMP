package service

import (
	"context"
	"reflect"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type taskFilterOptionsScopeRepo struct {
	repo.TaskRepo
	filter repo.TaskListFilter
}

func (r *taskFilterOptionsScopeRepo) ListFilterOptions(_ context.Context, filter repo.TaskListFilter) (*domain.TaskFilterOptions, error) {
	r.filter = filter
	return &domain.TaskFilterOptions{}, nil
}

func TestMainTaskReadScopeUsesExplicitStableOrganizationIDs(t *testing.T) {
	departmentID := int64(31)
	teamID := int64(42)
	roleID := int64(7)
	actor := domain.RequestActor{
		ID:           99,
		DepartmentID: &departmentID,
		TeamID:       &teamID,
		EffectiveAccess: &domain.EffectiveAccess{
			Permissions: []domain.PermissionCode{domain.PermissionTaskView},
			Assignments: []domain.AccessAssignment{
				{RoleID: roleID, ScopeMode: domain.AccessScopeSelf},
				{RoleID: roleID, ScopeMode: domain.AccessScopeOwnDepartment},
				{RoleID: roleID, ScopeMode: domain.AccessScopeOwnTeam},
			},
			Sources: []domain.EffectiveAccessNote{{RoleID: roleID, Permission: domain.PermissionTaskView}},
		},
	}

	scope := mainTaskReadScope(domain.WithRequestActor(context.Background(), actor))
	if scope.ViewAll {
		t.Fatal("ViewAll = true, want false")
	}
	if !reflect.DeepEqual(scope.UserIDs, []int64{actor.ID}) {
		t.Fatalf("UserIDs = %v", scope.UserIDs)
	}
	if !reflect.DeepEqual(scope.DepartmentIDs, []int64{departmentID}) {
		t.Fatalf("DepartmentIDs = %v", scope.DepartmentIDs)
	}
	if !reflect.DeepEqual(scope.TeamIDs, []int64{teamID}) {
		t.Fatalf("TeamIDs = %v", scope.TeamIDs)
	}
}

func TestMainTaskReadScopeFailsClosedWithoutExplicitTaskView(t *testing.T) {
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAdmin}}
	scope := mainTaskReadScope(domain.WithRequestActor(context.Background(), actor))
	if scope.ViewAll || len(scope.UserIDs) != 0 || len(scope.DepartmentIDs) != 0 || len(scope.TeamIDs) != 0 {
		t.Fatalf("scope unexpectedly grants access: %+v", scope)
	}
}

func TestListFilterOptionsUsesSameStableScopeAsTaskList(t *testing.T) {
	departmentID := int64(31)
	teamID := int64(42)
	roleID := int64(7)
	actor := domain.RequestActor{
		ID:           99,
		DepartmentID: &departmentID,
		TeamID:       &teamID,
		EffectiveAccess: &domain.EffectiveAccess{
			Permissions: []domain.PermissionCode{domain.PermissionTaskView},
			Assignments: []domain.AccessAssignment{
				{RoleID: roleID, ScopeMode: domain.AccessScopeSelf},
				{RoleID: roleID, ScopeMode: domain.AccessScopeOwnDepartment},
				{RoleID: roleID, ScopeMode: domain.AccessScopeOwnTeam},
			},
			Sources: []domain.EffectiveAccessNote{{RoleID: roleID, Permission: domain.PermissionTaskView}},
		},
	}
	repository := &taskFilterOptionsScopeRepo{}
	svc := &taskService{taskRepo: repository}

	options, appErr := svc.ListFilterOptions(domain.WithRequestActor(context.Background(), actor))
	if appErr != nil {
		t.Fatalf("ListFilterOptions() error = %+v", appErr)
	}
	if options == nil || options.Creators == nil || options.Designers == nil || options.OwnerDepartments == nil || options.OwnerTeams == nil {
		t.Fatalf("ListFilterOptions() did not normalize arrays: %+v", options)
	}
	if repository.filter.ScopeViewAll {
		t.Fatal("ScopeViewAll = true, want false")
	}
	if !reflect.DeepEqual(repository.filter.ScopeUserIDs, []int64{actor.ID}) {
		t.Fatalf("ScopeUserIDs = %v", repository.filter.ScopeUserIDs)
	}
	if !reflect.DeepEqual(repository.filter.ScopeDepartmentIDs, []int64{departmentID}) {
		t.Fatalf("ScopeDepartmentIDs = %v", repository.filter.ScopeDepartmentIDs)
	}
	if !reflect.DeepEqual(repository.filter.ScopeTeamIDs, []int64{teamID}) {
		t.Fatalf("ScopeTeamIDs = %v", repository.filter.ScopeTeamIDs)
	}
}
