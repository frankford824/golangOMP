package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestAuthorizeUserRoleChangeRoleMatrix(t *testing.T) {
	testCases := []struct {
		name       string
		actorRoles []domain.Role
		wantAllow  bool
	}{
		{name: "hr_admin_allowed", actorRoles: []domain.Role{domain.RoleHRAdmin}, wantAllow: true},
		{name: "super_admin_allowed", actorRoles: []domain.Role{domain.RoleSuperAdmin}, wantAllow: true},
		{name: "admin_denied", actorRoles: []domain.Role{domain.RoleAdmin}, wantAllow: false},
		{name: "role_admin_denied", actorRoles: []domain.Role{domain.RoleRoleAdmin}, wantAllow: false},
		{name: "org_admin_denied", actorRoles: []domain.Role{domain.RoleOrgAdmin}, wantAllow: false},
		{name: "department_admin_denied", actorRoles: []domain.Role{domain.RoleDeptAdmin}, wantAllow: false},
		{name: "team_lead_denied", actorRoles: []domain.Role{domain.RoleTeamLead}, wantAllow: false},
		{name: "member_denied", actorRoles: []domain.Role{domain.RoleMember}, wantAllow: false},
		{name: "hr_admin_mixed_allowed", actorRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleMember, domain.RoleDesigner}, wantAllow: true},
		{name: "admin_member_denied", actorRoles: []domain.Role{domain.RoleAdmin, domain.RoleMember}, wantAllow: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			userRepo := newIdentityUserRepo()
			sessionRepo := &identitySessionRepoStub{}
			svcInterface, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)
			svc := svcInterface.(*identityService)

			targetUser := &domain.User{
				ID:         42,
				Department: domain.DepartmentDesign,
				Team:       "Design Review",
			}
			actor := domain.RequestActor{
				ID:         7,
				Username:   tc.name,
				Roles:      tc.actorRoles,
				Source:     domain.RequestActorSourceSessionToken,
				AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
				Department: string(domain.DepartmentOperations),
				Team:       "Ops A",
			}
			ctx := domain.WithRequestActor(context.Background(), actor)

			appErr := svc.authorizeUserRoleChange(ctx, targetUser, []domain.Role{domain.RoleOps})
			if tc.wantAllow {
				if appErr != nil {
					t.Fatalf("authorizeUserRoleChange() error = %+v, want nil", appErr)
				}
				if observed.FilterMessage("authorize_user_role_change_denied").Len() != 0 {
					t.Fatalf("authorize_user_role_change_denied must not emit on allow (entries=%+v)", observed.All())
				}
				return
			}

			if appErr == nil {
				t.Fatal("authorizeUserRoleChange() error = nil, want deny")
			}
			if appErr.Code != domain.ErrCodePermissionDenied {
				t.Fatalf("appErr.Code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
			}
			if denyCode := appErrorDenyCode(appErr); denyCode != "role_assignment_denied_by_scope" {
				t.Fatalf("appErr deny_code = %q, want role_assignment_denied_by_scope", denyCode)
			}

			entries := observed.FilterMessage("authorize_user_role_change_denied").All()
			if len(entries) > 1 {
				t.Fatalf("authorize_user_role_change_denied entries = %d, want at most 1 (all=%+v)", len(entries), observed.All())
			}
		})
	}
}
