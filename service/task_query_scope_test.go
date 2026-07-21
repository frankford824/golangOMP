package service

import (
	"context"
	"reflect"
	"testing"

	"workflow/domain"
)

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
