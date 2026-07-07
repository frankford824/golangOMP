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

func TestListRolesMarksAssignableByCurrentActor(t *testing.T) {
	svc := &identityService{}
	hrCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       7,
		Username: "hr",
		Roles:    []domain.Role{domain.RoleHRAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	entries := svc.ListRoles(hrCtx)

	if roleCatalogEntryForTest(entries, domain.RoleOutsource).Assignable {
		t.Fatal("Outsource assignable = true, want false")
	}
	if roleCatalogEntryForTest(entries, domain.RoleAuditB).Assignable {
		t.Fatal("Audit_B assignable = true, want false")
	}
	if roleCatalogEntryForTest(entries, domain.RoleAuditB).AssignableByCurrentActor {
		t.Fatal("Audit_B assignable_by_current_actor = true, want false")
	}
	if roleCatalogEntryForTest(entries, domain.RoleOutsource).AssignableByCurrentActor {
		t.Fatal("Outsource assignable_by_current_actor = true, want false")
	}
	if roleCatalogEntryForTest(entries, domain.RoleRoleAdmin).AssignableByCurrentActor {
		t.Fatal("RoleAdmin assignable_by_current_actor = true, want false")
	}
	if roleCatalogEntryForTest(entries, domain.RoleHRAdmin).AssignableByCurrentActor {
		t.Fatal("HRAdmin assignable_by_current_actor = true for HR actor, want false")
	}
	if !roleCatalogEntryForTest(entries, domain.RoleCustomizationReviewer).AssignableByCurrentActor {
		t.Fatal("CustomizationReviewer assignable_by_current_actor = false for HR actor, want true")
	}
}

func TestAuthorizeAssignableRoleAdditionsRejectsCompatibilityRoles(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       7,
		Username: "super",
		Roles:    []domain.Role{domain.RoleSuperAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	appErr := authorizeAssignableRoleAdditions(ctx, domain.DepartmentAudit, []domain.Role{domain.RoleOutsource})
	if appErr == nil {
		t.Fatal("authorizeAssignableRoleAdditions(Outsource) appErr = nil, want deny")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "role_not_assignable" {
		t.Fatalf("deny_code = %q, want role_not_assignable", denyCode)
	}
	appErr = authorizeAssignableRoleAdditions(ctx, domain.DepartmentAudit, []domain.Role{domain.RoleAuditB})
	if appErr == nil {
		t.Fatal("authorizeAssignableRoleAdditions(Audit_B) appErr = nil, want deny")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "role_not_assignable" {
		t.Fatalf("deny_code = %q, want role_not_assignable", denyCode)
	}
}

func TestAuthorizeAssignableRoleAdditionsDeptAdminUsesDepartmentDefaults(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         7,
		Username:   "dept",
		Roles:      []domain.Role{domain.RoleDeptAdmin},
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
		Department: string(domain.DepartmentDesign),
	})
	if appErr := authorizeAssignableRoleAdditions(ctx, domain.DepartmentDesign, []domain.Role{domain.RoleDesigner, domain.RoleTeamLead, domain.RoleMember}); appErr != nil {
		t.Fatalf("authorizeAssignableRoleAdditions(design defaults) appErr = %+v", appErr)
	}
	appErr := authorizeAssignableRoleAdditions(ctx, domain.DepartmentDesign, []domain.Role{domain.RoleDesignReviewer})
	if appErr == nil {
		t.Fatal("authorizeAssignableRoleAdditions(DesignReviewer) appErr = nil, want deny")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "role_not_assignable" {
		t.Fatalf("deny_code = %q, want role_not_assignable", denyCode)
	}
}

func TestEnsureAdminRoleSafetyProtectsLastSuperAdmin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[1] = &domain.User{ID: 1, Username: "super", Status: domain.UserStatusActive}
	userRepo.roles[1] = []domain.Role{domain.RoleSuperAdmin}
	svc := &identityService{userRepo: userRepo}

	appErr := svc.ensureAdminRoleSafety(context.Background(), []domain.Role{domain.RoleSuperAdmin}, []domain.Role{domain.RoleMember})
	if appErr == nil {
		t.Fatal("ensureAdminRoleSafety() appErr = nil, want deny")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "last_super_admin_removal_denied" {
		t.Fatalf("deny_code = %q, want last_super_admin_removal_denied", denyCode)
	}
}

func roleCatalogEntryForTest(entries []domain.RoleCatalogEntry, role domain.Role) domain.RoleCatalogEntry {
	for _, entry := range entries {
		if entry.Role == role {
			return entry
		}
	}
	return domain.RoleCatalogEntry{}
}
