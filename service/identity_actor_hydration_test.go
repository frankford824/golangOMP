package service

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"workflow/domain"
)

// newIdentityServiceWithObservedLogger builds an identity service whose
// Round 0.5 telemetry is captured by a zaptest observer at warn+ level.
func newIdentityServiceWithObservedLogger(t *testing.T, userRepo *identityUserRepoStub, sessionRepo *identitySessionRepoStub) (IdentityService, *observer.ObservedLogs) {
	t.Helper()
	core, observed := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	svc := NewIdentityService(
		userRepo,
		sessionRepo,
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityLogger(logger),
	)
	return svc, observed
}

// registerActorForHydrationTest bootstraps a user via Register so we get a
// session token, then overrides the persisted role slice (raw DB view) to the
// supplied rawRoles. The returned user is the raw registration response,
// including the generated session token.
func registerActorForHydrationTest(
	t *testing.T,
	svc IdentityService,
	userRepo *identityUserRepoStub,
	params RegisterUserParams,
	rawRoles []string,
) *domain.AuthResult {
	t.Helper()
	result, appErr := svc.Register(context.Background(), params)
	if appErr != nil {
		t.Fatalf("Register(%s) error = %+v", params.Username, appErr)
	}
	if result == nil || result.User == nil || result.Session == nil {
		t.Fatalf("Register(%s) returned empty result", params.Username)
	}
	userRepo.setRawRolesForTest(result.User.ID, rawRoles)
	return result
}

// TestResolveRequestActorZeroKnownRolesEmitsDegradedHydration covers
// diagnostic case (a): DB ListRoles returns zero known roles. The actor
// must expose a canonical role slice that matches user.FrontendAccess.Roles
// (both defaulted to [Member]), and the telemetry event must fire.
func TestResolveRequestActorZeroKnownRolesEmitsDegradedHydration(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	reg := registerActorForHydrationTest(t, svc, userRepo, RegisterUserParams{
		Username:    "zero_roles_member",
		DisplayName: "空角色成员",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000710",
		Password:    "Pass1234",
	}, []string{})

	actor, appErr := svc.ResolveRequestActor(context.Background(), reg.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	if actor == nil {
		t.Fatal("ResolveRequestActor() returned nil actor")
	}

	if len(actor.Roles) != 1 || actor.Roles[0] != domain.RoleMember {
		t.Fatalf("actor.Roles = %+v, want [%s]", actor.Roles, domain.RoleMember)
	}
	if !containsString(actor.FrontendAccess.Roles, "member") {
		t.Fatalf("actor.FrontendAccess.Roles = %+v, want to contain member", actor.FrontendAccess.Roles)
	}

	actorRoleSet := map[string]struct{}{}
	for _, role := range actor.Roles {
		actorRoleSet[string(role)] = struct{}{}
	}
	if _, ok := actorRoleSet[string(domain.RoleMember)]; !ok {
		t.Fatalf("actor.Roles %+v does not contain %s", actor.Roles, domain.RoleMember)
	}

	entries := observed.FilterMessage("actor_role_hydration_degraded").All()
	if len(entries) != 1 {
		t.Fatalf("actor_role_hydration_degraded entries = %d, want 1 (all=%+v)", len(entries), observed.All())
	}
	entry := entries[0]
	fields := entry.ContextMap()
	if fields["event"] != "actor_role_hydration_degraded" {
		t.Fatalf("event field = %v", fields["event"])
	}
	if gotZero, _ := fields["zero_known_roles"].(bool); !gotZero {
		t.Fatalf("zero_known_roles field = %v, want true", fields["zero_known_roles"])
	}
	if rawCount, _ := fields["raw_roles_count"].(int64); rawCount != 0 {
		t.Fatalf("raw_roles_count = %v, want 0", fields["raw_roles_count"])
	}
	if userID, _ := fields["user_id"].(int64); userID != reg.User.ID {
		t.Fatalf("user_id field = %v, want %d", fields["user_id"], reg.User.ID)
	}
}

// TestResolveRequestActorDropsUnknownRolesConsistently covers diagnostic
// case (b): a mix of known and unknown raw role strings. The unknown string
// must be dropped from both RequestActor.Roles and FrontendAccess, and the
// telemetry event must record it under dropped_roles.
func TestResolveRequestActorDropsUnknownRolesConsistently(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	reg := registerActorForHydrationTest(t, svc, userRepo, RegisterUserParams{
		Username:    "mixed_roles_user",
		DisplayName: "混合角色用户",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000711",
		Password:    "Pass1234",
	}, []string{string(domain.RoleMember), string(domain.RoleWarehouse), "legacy_ghost_role"})

	actor, appErr := svc.ResolveRequestActor(context.Background(), reg.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}

	for _, role := range actor.Roles {
		if string(role) == "legacy_ghost_role" {
			t.Fatalf("actor.Roles = %+v, want legacy_ghost_role dropped", actor.Roles)
		}
	}
	if !containsRoleValue(actor.Roles, domain.RoleMember) || !containsRoleValue(actor.Roles, domain.RoleWarehouse) {
		t.Fatalf("actor.Roles = %+v, want [member, warehouse]", actor.Roles)
	}
	if containsString(actor.FrontendAccess.Roles, "legacy_ghost_role") {
		t.Fatalf("FrontendAccess.Roles = %+v, want legacy_ghost_role absent", actor.FrontendAccess.Roles)
	}
	if !containsString(actor.FrontendAccess.Roles, "warehouse") {
		t.Fatalf("FrontendAccess.Roles = %+v, want warehouse retained", actor.FrontendAccess.Roles)
	}

	entries := observed.FilterMessage("actor_role_hydration_degraded").All()
	if len(entries) != 1 {
		t.Fatalf("actor_role_hydration_degraded entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	dropped, ok := fields["dropped_roles"].([]interface{})
	if !ok {
		if droppedStr, okStr := fields["dropped_roles"].([]string); okStr {
			dropped = make([]interface{}, 0, len(droppedStr))
			for _, s := range droppedStr {
				dropped = append(dropped, s)
			}
		}
	}
	foundGhost := false
	for _, d := range dropped {
		if s, ok := d.(string); ok && s == "legacy_ghost_role" {
			foundGhost = true
			break
		}
	}
	if !foundGhost {
		t.Fatalf("dropped_roles = %+v, want to contain legacy_ghost_role (full entry = %+v)", fields["dropped_roles"], fields)
	}
	if rawCount, _ := fields["raw_roles_count"].(int64); rawCount != 3 {
		t.Fatalf("raw_roles_count = %v, want 3", fields["raw_roles_count"])
	}
}

// TestResolveRequestActorCleanRolesEmitsNoTelemetry guards that the warn
// emission path is not triggered when the raw slice normalizes without
// drops and yields at least one known role. This is the non-degraded
// baseline and should produce zero telemetry entries.
func TestResolveRequestActorCleanRolesEmitsNoTelemetry(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	reg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "clean_member",
		DisplayName: "干净成员",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000712",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}

	if _, appErr := svc.ResolveRequestActor(context.Background(), reg.Session.Token); appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	if got := observed.FilterMessage("actor_role_hydration_degraded").Len(); got != 0 {
		t.Fatalf("actor_role_hydration_degraded entries = %d, want 0", got)
	}
}

// TestAuthorizeUserReadDepartmentAdminSameDepartmentIsSilent covers
// diagnostic case (c): a DepartmentAdmin reading a target in the same
// department must succeed and must not emit authorize_user_read_denied.
func TestAuthorizeUserReadDepartmentAdminSameDepartmentIsSilent(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	dept := domain.DepartmentProcurement
	team := "采购组"
	adminReg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "procurement_admin",
		DisplayName: "采购主管",
		Department:  dept,
		Team:        team,
		Mobile:      "13800000720",
		Password:    "Pass1234",
		AdminKey:    "superAdmin",
	})
	if appErr != nil {
		t.Fatalf("Register(admin) error = %+v", appErr)
	}
	if !containsRoleValue(adminReg.User.Roles, domain.RoleDeptAdmin) {
		t.Fatalf("admin roles = %+v, want DepartmentAdmin", adminReg.User.Roles)
	}

	targetReg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "procurement_member",
		DisplayName: "采购成员",
		Department:  dept,
		Team:        team,
		Mobile:      "13800000721",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(target) error = %+v", appErr)
	}

	actor, appErr := svc.ResolveRequestActor(context.Background(), adminReg.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	ctx := domain.WithRequestActor(context.Background(), *actor)

	fetched, appErr := svc.GetUser(ctx, targetReg.User.ID)
	if appErr != nil {
		t.Fatalf("GetUser() error = %+v", appErr)
	}
	if fetched == nil || fetched.ID != targetReg.User.ID {
		t.Fatalf("GetUser() user = %+v, want id %d", fetched, targetReg.User.ID)
	}
	if observed.FilterMessage("authorize_user_read_denied").Len() != 0 {
		t.Fatalf("authorize_user_read_denied must not emit for same-department admin (entries=%+v)", observed.All())
	}
}

// TestAuthorizeUserReadNonManagementDenyEmitsTelemetry covers diagnostic
// case (d): a non-management actor ([Warehouse, Member]) hitting
// authorizeUserRead must receive management_access_required and emit the
// new default-deny telemetry with the full actor role list.
func TestAuthorizeUserReadNonManagementDenyEmitsTelemetry(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	actorReg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "warehouse_worker",
		DisplayName: "仓库工",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000730",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(actor) error = %+v", appErr)
	}
	if err := userRepo.ReplaceRoles(context.Background(), nil, actorReg.User.ID, []domain.Role{
		domain.RoleWarehouse,
		domain.RoleMember,
	}); err != nil {
		t.Fatalf("ReplaceRoles(actor) error = %v", err)
	}

	targetReg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "design_member",
		DisplayName: "设计成员",
		Department:  domain.DepartmentDesign,
		Team:        "设计审核组",
		Mobile:      "13800000731",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(target) error = %+v", appErr)
	}

	actor, appErr := svc.ResolveRequestActor(context.Background(), actorReg.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	if !containsRoleValue(actor.Roles, domain.RoleWarehouse) || !containsRoleValue(actor.Roles, domain.RoleMember) {
		t.Fatalf("actor.Roles = %+v, want [warehouse, member]", actor.Roles)
	}
	ctx := domain.WithRequestActor(context.Background(), *actor)

	_, appErr = svc.GetUser(ctx, targetReg.User.ID)
	if appErr == nil {
		t.Fatal("GetUser() want management_access_required deny, got nil")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("appErr.Code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "management_access_required" {
		t.Fatalf("appErr deny_code = %q, want management_access_required", denyCode)
	}

	entries := observed.FilterMessage("authorize_user_read_denied").All()
	if len(entries) != 1 {
		t.Fatalf("authorize_user_read_denied entries = %d, want 1 (all=%+v)", len(entries), observed.All())
	}
	fields := entries[0].ContextMap()
	if fields["deny_code"] != "management_access_required" {
		t.Fatalf("deny_code field = %v", fields["deny_code"])
	}
	if gotActorID, _ := fields["actor_id"].(int64); gotActorID != actor.ID {
		t.Fatalf("actor_id field = %v, want %d", fields["actor_id"], actor.ID)
	}
	if gotTargetID, _ := fields["target_user_id"].(int64); gotTargetID != targetReg.User.ID {
		t.Fatalf("target_user_id field = %v, want %d", fields["target_user_id"], targetReg.User.ID)
	}
	roles := extractStringSliceField(fields["actor_roles"])
	if !containsString(roles, string(domain.RoleWarehouse)) || !containsString(roles, string(domain.RoleMember)) {
		t.Fatalf("actor_roles = %+v, want %s+%s", roles, domain.RoleWarehouse, domain.RoleMember)
	}
}

// TestAuthorizeUserListFilterNonManagementDenyEmitsTelemetry exercises the
// sibling default-deny path on authorizeUserListFilter using the same
// non-management actor profile. The deny semantics and telemetry shape must
// match the user_read path (minus the target_* fields).
func TestAuthorizeUserListFilterNonManagementDenyEmitsTelemetry(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc, observed := newIdentityServiceWithObservedLogger(t, userRepo, sessionRepo)

	actorReg, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "warehouse_worker_list",
		DisplayName: "仓库工2",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000732",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(actor) error = %+v", appErr)
	}
	if err := userRepo.ReplaceRoles(context.Background(), nil, actorReg.User.ID, []domain.Role{
		domain.RoleWarehouse,
		domain.RoleMember,
	}); err != nil {
		t.Fatalf("ReplaceRoles(actor) error = %v", err)
	}

	actor, appErr := svc.ResolveRequestActor(context.Background(), actorReg.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	ctx := domain.WithRequestActor(context.Background(), *actor)

	_, _, appErr = svc.ListUsers(ctx, UserFilter{})
	if appErr == nil {
		t.Fatal("ListUsers() want management_access_required deny, got nil")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "management_access_required" {
		t.Fatalf("appErr deny_code = %q, want management_access_required", denyCode)
	}

	entries := observed.FilterMessage("authorize_user_list_filter_denied").All()
	if len(entries) != 1 {
		t.Fatalf("authorize_user_list_filter_denied entries = %d, want 1 (all=%+v)", len(entries), observed.All())
	}
	fields := entries[0].ContextMap()
	if fields["deny_code"] != "management_access_required" {
		t.Fatalf("deny_code field = %v", fields["deny_code"])
	}
	if gotActorID, _ := fields["actor_id"].(int64); gotActorID != actor.ID {
		t.Fatalf("actor_id field = %v, want %d", fields["actor_id"], actor.ID)
	}
	if _, hasTarget := fields["target_user_id"]; hasTarget {
		t.Fatalf("target_user_id must not be present on list-filter telemetry (fields=%+v)", fields)
	}
}

func appErrorDenyCode(err *domain.AppError) string {
	if err == nil {
		return ""
	}
	details, ok := err.Details.(map[string]interface{})
	if !ok {
		return ""
	}
	code, _ := details["deny_code"].(string)
	return code
}

func extractStringSliceField(v interface{}) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
