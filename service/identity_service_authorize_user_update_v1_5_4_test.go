package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestIdentityServiceAuthorizeUserUpdateV154Matrix(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}).(*identityService)
	target := &domain.User{
		ID:         30001,
		Department: domain.DepartmentOperations,
		Team:       "淘系一组",
		Roles:      []domain.Role{domain.RoleMember},
	}
	roles := []struct {
		name      string
		role      domain.Role
		wantAllow map[string]bool
	}{
		{
			name: "SuperAdmin", role: domain.RoleSuperAdmin,
			wantAllow: map[string]bool{"profile": true, "department": true, "team": true, "roles": true, "status": true, "employment": true, "managed_scope": true},
		},
		{
			name: "HRAdmin", role: domain.RoleHRAdmin,
			wantAllow: map[string]bool{"profile": true, "department": true, "team": true, "roles": true, "status": true, "employment": true, "managed_scope": true},
		},
		{
			name: "DeptAdmin", role: domain.RoleDeptAdmin,
			wantAllow: map[string]bool{"profile": true, "department": true, "team": true, "roles": true, "status": true, "employment": true, "managed_scope": false},
		},
		{
			name: "TeamLead", role: domain.RoleTeamLead,
			wantAllow: map[string]bool{"profile": false, "department": false, "team": false, "roles": false, "status": true, "employment": false, "managed_scope": false},
		},
	}
	fields := []string{"profile", "department", "team", "roles", "status", "employment", "managed_scope"}
	for _, roleCase := range roles {
		for _, field := range fields {
			t.Run(roleCase.name+"/"+field, func(t *testing.T) {
				ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
					ID:                 7,
					Username:           roleCase.name,
					Roles:              []domain.Role{roleCase.role},
					Department:         string(domain.DepartmentOperations),
					Team:               "淘系一组",
					ManagedDepartments: []string{string(domain.DepartmentOperations)},
					ManagedTeams:       []string{"淘系一组"},
					Source:             domain.RequestActorSourceSessionToken,
					AuthMode:           domain.AuthModeSessionTokenRoleEnforced,
				})
				appErr := authorizeMatrixField(ctx, svc, target, field)
				wantAllow := roleCase.wantAllow[field]
				if wantAllow && appErr != nil {
					t.Fatalf("field %s denied: %+v", field, appErr)
				}
				if !wantAllow && appErr == nil {
					t.Fatalf("field %s allowed, want deny", field)
				}
			})
		}
	}
}

func TestIdentityServiceAuthorizeUserUpdateV154DenyEdges(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}).(*identityService)
	target := &domain.User{ID: 30002, Department: domain.DepartmentOperations, Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}}

	hrCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleHRAdmin}, Department: string(domain.DepartmentOperations), Team: "淘系一组", Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced})
	if appErr := svc.authorizeUserRoleChange(hrCtx, target, []domain.Role{domain.RoleSuperAdmin}); appErr == nil || appErrorDenyCode(appErr) != "role_assignment_denied_by_scope" {
		t.Fatalf("HRAdmin assigning SuperAdmin appErr = %+v", appErr)
	}

	deptCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 2, Roles: []domain.Role{domain.RoleDeptAdmin}, Department: string(domain.DepartmentOperations), ManagedDepartments: []string{string(domain.DepartmentOperations)}, Team: "淘系一组", Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced})
	if appErr := svc.authorizeUserRoleChange(deptCtx, target, []domain.Role{domain.RoleDeptAdmin}); appErr == nil || appErrorDenyCode(appErr) != "role_assignment_denied_by_scope" {
		t.Fatalf("DeptAdmin assigning DeptAdmin appErr = %+v", appErr)
	}
	crossDept := domain.DepartmentAudit
	if appErr := svc.authorizeUserUpdate(deptCtx, target, UpdateUserParams{Department: &crossDept}, crossDept, target.Team); appErr == nil || appErrorDenyCode(appErr) != "user_update_field_denied_by_scope" {
		t.Fatalf("DeptAdmin cross-department update appErr = %+v", appErr)
	}

	teamCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 3, Roles: []domain.Role{domain.RoleTeamLead}, Department: string(domain.DepartmentOperations), Team: "淘系二组", Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced})
	if appErr := svc.authorizeUserStatusEndpoint(teamCtx, target); appErr == nil || appErrorDenyCode(appErr) != "user_update_field_denied_by_scope" {
		t.Fatalf("TeamLead cross-team status appErr = %+v", appErr)
	}
}

func authorizeMatrixField(ctx context.Context, svc *identityService, target *domain.User, field string) *domain.AppError {
	switch field {
	case "profile":
		name := "New Name"
		return svc.authorizeUserUpdate(ctx, target, UpdateUserParams{DisplayName: &name}, target.Department, target.Team)
	case "department":
		department := target.Department
		return svc.authorizeUserUpdate(ctx, target, UpdateUserParams{Department: &department}, department, target.Team)
	case "team":
		team := "淘系二组"
		return svc.authorizeUserUpdate(ctx, target, UpdateUserParams{Team: &team}, target.Department, team)
	case "roles":
		return svc.authorizeUserRoleChange(ctx, target, []domain.Role{domain.RoleOps})
	case "status":
		return svc.authorizeUserStatusEndpoint(ctx, target)
	case "employment":
		employmentType := domain.EmploymentTypePartTime
		return svc.authorizeUserUpdate(ctx, target, UpdateUserParams{EmploymentType: &employmentType}, target.Department, target.Team)
	case "managed_scope":
		managed := []string{string(domain.DepartmentOperations)}
		return svc.authorizeUserUpdate(ctx, target, UpdateUserParams{ManagedDepartments: &managed}, target.Department, target.Team)
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "unknown field", nil)
	}
}
