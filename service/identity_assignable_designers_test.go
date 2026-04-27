package service

import (
	"context"
	"testing"

	"workflow/domain"
)

// seedDesigner inserts a user via the stub repo with the given username,
// department, status, and role set. Used by the assignable-designer
// regression tests below.
func seedAssignableDesignerUser(
	t *testing.T,
	repo *identityUserRepoStub,
	username, displayName string,
	department domain.Department,
	team string,
	status domain.UserStatus,
	roles ...domain.Role,
) int64 {
	t.Helper()
	id, err := repo.Create(context.Background(), identityTx{}, &domain.User{
		Username:       username,
		DisplayName:    displayName,
		Department:     department,
		Team:           team,
		Status:         status,
		EmploymentType: domain.EmploymentTypeFullTime,
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	if err := repo.ReplaceRoles(context.Background(), identityTx{}, id, roles); err != nil {
		t.Fatalf("seed roles for %s: %v", username, err)
	}
	return id
}

func collectUsernames(users []*domain.User) []string {
	out := make([]string, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		out = append(out, user.Username)
	}
	return out
}

func assertUsernamesExact(t *testing.T, users []*domain.User, want []string) {
	t.Helper()
	got := collectUsernames(users)
	if len(got) != len(want) {
		t.Fatalf("usernames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("usernames = %v, want %v", got, want)
		}
	}
}

func assignableDesignersOpsActor() *domain.RequestActor {
	return &domain.RequestActor{
		ID:         100,
		Username:   "ops_user",
		Roles:      []domain.Role{domain.RoleOps},
		Department: string(domain.DepartmentOperations),
		Team:       "运营一组",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	}
}

func seedAssignableLaneUsers(t *testing.T, repo *identityUserRepoStub) {
	t.Helper()
	seedAssignableDesignerUser(t, repo, "designer_a", "设计A", domain.DepartmentDesignRD, "研发默认组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, repo, "designer_b", "设计B", domain.DepartmentDesignRD, "研发默认组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, repo, "custom_operator_a", "定制A", domain.DepartmentCustomizationArt, "定制默认组", domain.UserStatusActive, domain.RoleCustomizationOperator)
	seedAssignableDesignerUser(t, repo, "custom_operator_b", "定制B", domain.DepartmentCustomizationArt, "定制默认组", domain.UserStatusActive, domain.RoleCustomizationOperator)
	seedAssignableDesignerUser(t, repo, "custom_operator_disabled", "定制停用", domain.DepartmentCustomizationArt, "定制默认组", domain.UserStatusDisabled, domain.RoleCustomizationOperator)
}

func TestListAssignableDesigners_CustomizationLaneReturnsOperators(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	seedAssignableLaneUsers(t, userRepo)

	users, appErr := svc.ListAssignableDesigners(context.Background(), assignableDesignersOpsActor(), AssignableLaneCustomization)
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(customization) error = %+v", appErr)
	}
	assertUsernamesExact(t, users, []string{"custom_operator_b", "custom_operator_a"})
}

func TestListAssignableDesigners_NormalLaneBackwardCompatible(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	seedAssignableLaneUsers(t, userRepo)

	users, appErr := svc.ListAssignableDesigners(context.Background(), assignableDesignersOpsActor(), AssignableLaneNormal)
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(normal) error = %+v", appErr)
	}
	assertUsernamesExact(t, users, []string{"designer_b", "designer_a"})

	defaultUsers, appErr := svc.ListAssignableDesigners(context.Background(), assignableDesignersOpsActor(), "")
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(default) error = %+v", appErr)
	}
	assertUsernamesExact(t, defaultUsers, []string{"designer_b", "designer_a"})
}

func TestListAssignableDesigners_AllLaneReturnsUnionDeduped(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	seedAssignableLaneUsers(t, userRepo)
	seedAssignableDesignerUser(t, userRepo, "dual_lane_user", "双角色", domain.DepartmentCustomizationArt, "定制默认组", domain.UserStatusActive, domain.RoleDesigner, domain.RoleCustomizationOperator)

	users, appErr := svc.ListAssignableDesigners(context.Background(), assignableDesignersOpsActor(), AssignableLaneAll)
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(all) error = %+v", appErr)
	}
	assertUsernamesExact(t, users, []string{"dual_lane_user", "designer_b", "designer_a", "custom_operator_b", "custom_operator_a"})
}

// TestListAssignableDesigners_OpsActorReturnsFullList proves Round D's key
// outcome: an Ops-only actor whose department is 运营部 can still see
// designers located in 设计研发部 — i.e. the assignment-candidate-pool service
// method bypasses authorizeUserListFilter and ignores the actor's
// department scope. This is the exact regression that blocked UAT before
// Round D.
func TestListAssignableDesigners_OpsActorReturnsFullList(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	seedAssignableDesignerUser(t, userRepo, "designer_a", "设计A", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, userRepo, "designer_b", "设计B", domain.DepartmentDesign, "设计二组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, userRepo, "ops_peer", "运营同事", domain.DepartmentOperations, "运营一组", domain.UserStatusActive, domain.RoleOps)

	actor := &domain.RequestActor{
		ID:         100,
		Username:   "ops_user",
		Roles:      []domain.Role{domain.RoleOps},
		Department: string(domain.DepartmentOperations),
		Team:       "运营一组",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	}

	users, appErr := svc.ListAssignableDesigners(context.Background(), actor, "")
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(ops) error = %+v", appErr)
	}
	names := collectUsernames(users)
	if len(names) != 2 {
		t.Fatalf("ListAssignableDesigners(ops) usernames = %v, want 2 designers", names)
	}
	if !containsString(names, "designer_a") || !containsString(names, "designer_b") {
		t.Fatalf("ListAssignableDesigners(ops) usernames = %v, expected both designer_a and designer_b", names)
	}
	for _, user := range users {
		if user.FrontendAccess.Roles == nil {
			t.Fatalf("ListAssignableDesigners(ops) user %s missing frontend_access.roles — prepareUserForResponse was not applied", user.Username)
		}
		if user.Account != user.Username {
			t.Fatalf("ListAssignableDesigners(ops) user %s account alias missing", user.Username)
		}
	}
}

// TestListAssignableDesigners_DesignerSelfActorReturnsFullList proves that a
// Designer-only actor (who under the pre-Round-D filter would be rejected
// as "management access required") can still enumerate peer designers
// through the assignment-candidate-pool path.
func TestListAssignableDesigners_DesignerSelfActorReturnsFullList(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	seedAssignableDesignerUser(t, userRepo, "designer_a", "设计A", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, userRepo, "designer_b", "设计B", domain.DepartmentDesign, "设计二组", domain.UserStatusActive, domain.RoleDesigner)

	actor := &domain.RequestActor{
		ID:         7,
		Username:   "designer_a",
		Roles:      []domain.Role{domain.RoleDesigner},
		Department: string(domain.DepartmentDesign),
		Team:       "设计一组",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	}

	users, appErr := svc.ListAssignableDesigners(context.Background(), actor, "")
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners(designer) error = %+v", appErr)
	}
	names := collectUsernames(users)
	if len(names) != 2 {
		t.Fatalf("ListAssignableDesigners(designer) usernames = %v, want 2 designers", names)
	}
}

// TestListAssignableDesigners_ExcludesDisabledUsers proves disabled designers
// must not appear in the assignment candidate pool, mirroring frontend
// expectations.
func TestListAssignableDesigners_ExcludesDisabledUsers(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	seedAssignableDesignerUser(t, userRepo, "designer_active", "设计A", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, userRepo, "designer_disabled", "设计B", domain.DepartmentDesign, "设计一组", domain.UserStatusDisabled, domain.RoleDesigner)

	actor := &domain.RequestActor{
		ID:       1,
		Username: "admin",
		Roles:    []domain.Role{domain.RoleAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}

	users, appErr := svc.ListAssignableDesigners(context.Background(), actor, "")
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners() error = %+v", appErr)
	}
	names := collectUsernames(users)
	if len(names) != 1 || names[0] != "designer_active" {
		t.Fatalf("ListAssignableDesigners() usernames = %v, want only designer_active", names)
	}
}

// TestListAssignableDesigners_IgnoresNonDesignerRoles proves the role filter
// is strict: users whose roles are Ops/Warehouse/etc. must not appear,
// even if they share a department with a real designer.
func TestListAssignableDesigners_IgnoresNonDesignerRoles(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	seedAssignableDesignerUser(t, userRepo, "designer_only", "设计", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleDesigner)
	seedAssignableDesignerUser(t, userRepo, "ops_in_design", "伪装设计的运营", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleOps)
	seedAssignableDesignerUser(t, userRepo, "warehouse_in_design", "伪装设计的云仓", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleWarehouse)

	actor := &domain.RequestActor{
		ID:       1,
		Username: "admin",
		Roles:    []domain.Role{domain.RoleAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}

	users, appErr := svc.ListAssignableDesigners(context.Background(), actor, "")
	if appErr != nil {
		t.Fatalf("ListAssignableDesigners() error = %+v", appErr)
	}
	names := collectUsernames(users)
	if len(names) != 1 || names[0] != "designer_only" {
		t.Fatalf("ListAssignableDesigners() usernames = %v, want only designer_only", names)
	}
}

// TestListAssignableDesigners_NilActorRejected proves the method refuses nil
// / zero-id actors without touching the repo. The route layer is normally
// responsible for establishing the actor; the service-level guard exists
// as defense in depth.
func TestListAssignableDesigners_NilActorRejected(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	seedAssignableDesignerUser(t, userRepo, "designer_a", "设计A", domain.DepartmentDesign, "设计一组", domain.UserStatusActive, domain.RoleDesigner)

	users, appErr := svc.ListAssignableDesigners(context.Background(), nil, "")
	if appErr == nil {
		t.Fatalf("ListAssignableDesigners(nil) expected error, got users=%+v", users)
	}
	if appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("ListAssignableDesigners(nil) error code = %s, want %s", appErr.Code, domain.ErrCodeUnauthorized)
	}
	if userRepo.listRolesCalls != 0 && userRepo.listRolesByUserIDsCalls != 0 {
		t.Fatalf("ListAssignableDesigners(nil) should not touch the repo (listRoles=%d, batchListRoles=%d)",
			userRepo.listRolesCalls, userRepo.listRolesByUserIDsCalls)
	}

	zeroActor := &domain.RequestActor{ID: 0}
	users, appErr = svc.ListAssignableDesigners(context.Background(), zeroActor, "")
	if appErr == nil {
		t.Fatalf("ListAssignableDesigners(zero id) expected error, got users=%+v", users)
	}
}
