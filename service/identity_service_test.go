package service

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"workflow/domain"
	"workflow/repo"
)

func TestIdentityServiceRegisterCreatesOnlyExplicitMember(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	logRepo := &identityPermissionLogRepoStub{}
	assignmentWriter := &identityAccessAssignmentWriterStub{}
	svc := NewIdentityService(userRepo, sessionRepo, logRepo, identityTxRunner{}, WithIdentityAccessAssignmentWriter(assignmentWriter))

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "designer_admin",
		DisplayName: "设计主管",
		Department:  domain.DepartmentDesign,
		Team:        "设计审核组",
		Mobile:      "13800000001",
		Email:       "designer@example.com",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if registerResult.User == nil || registerResult.Session == nil {
		t.Fatalf("Register() result = %+v", registerResult)
	}
	if !reflect.DeepEqual(registerResult.User.Roles, []domain.Role{domain.RoleMember}) {
		t.Fatalf("Register() roles = %+v, want only Member", registerResult.User.Roles)
	}
	if registerResult.User.Account != "designer_admin" || registerResult.User.Name != "设计主管" {
		t.Fatalf("Register() user aliases = %+v", registerResult.User)
	}
	if registerResult.User.Department != domain.DepartmentDesign || registerResult.User.Team != "设计审核组" {
		t.Fatalf("Register() profile = %+v", registerResult.User)
	}
	if registerResult.User.FrontendAccess.IsDepartmentAdmin || len(registerResult.User.ManagedDepartments) != 0 {
		t.Fatalf("Register() must not infer management from organization profile: %+v", registerResult.User)
	}
	if !containsString(registerResult.User.FrontendAccess.Roles, "member") {
		t.Fatalf("Register() frontend_access.roles = %+v", registerResult.User.FrontendAccess.Roles)
	}

	loginResult, appErr := svc.Login(context.Background(), LoginParams{
		Username: "designer_admin",
		Password: "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Login() error = %+v", appErr)
	}
	if loginResult.Session.Token == "" {
		t.Fatal("Login() token is empty")
	}
	actor, appErr := svc.ResolveRequestActor(context.Background(), loginResult.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	renewedSession := sessionRepo.sessions[hashToken(loginResult.Session.Token)]
	if renewedSession == nil || !renewedSession.ExpiresAt.After(loginResult.Session.ExpiresAt) {
		t.Fatalf("ResolveRequestActor() session expiry = %+v, want later than login expiry %s", renewedSession, loginResult.Session.ExpiresAt)
	}
	sessionCtx := domain.WithRequestActor(context.Background(), *actor)
	currentUser, appErr := svc.GetCurrentUser(sessionCtx)
	if appErr != nil {
		t.Fatalf("GetCurrentUser() error = %+v", appErr)
	}
	if currentUser.Department != domain.DepartmentDesign || currentUser.Email != "designer@example.com" {
		t.Fatalf("GetCurrentUser() user = %+v", currentUser)
	}
	if currentUser.FrontendAccess.IsDepartmentAdmin || containsString(currentUser.FrontendAccess.Pages, "department_users") {
		t.Fatalf("GetCurrentUser() leaked department-admin access = %+v", currentUser.FrontendAccess)
	}
	if len(assignmentWriter.calls) != 1 || assignmentWriter.calls[0].roleCode != "member" || assignmentWriter.calls[0].scopeMode != domain.AccessScopeSelf {
		t.Fatalf("explicit assignments = %+v, want member/self", assignmentWriter.calls)
	}
}

func TestIdentityServiceRegisterAssetWorkbenchUserOnlyGrantsSubmitter(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	logRepo := &identityPermissionLogRepoStub{}
	assignmentWriter := &identityAccessAssignmentWriterStub{}
	svc := NewIdentityService(userRepo, sessionRepo, logRepo, identityTxRunner{}, WithIdentityAccessAssignmentWriter(assignmentWriter))

	result, appErr := svc.RegisterAssetWorkbenchUser(context.Background(), RegisterAssetWorkbenchUserParams{
		Username:    "piece_worker",
		DisplayName: "计件人员",
		Mobile:      "13800000991",
		Email:       "piece-worker@example.com",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("RegisterAssetWorkbenchUser() error = %+v", appErr)
	}
	if result == nil || result.User == nil || result.Session == nil || result.Session.Token == "" {
		t.Fatalf("RegisterAssetWorkbenchUser() result = %+v", result)
	}
	if result.User.Department != domain.DepartmentUnassigned {
		t.Fatalf("department = %q, want unassigned", result.User.Department)
	}
	if result.User.Team != "未分配池" {
		t.Fatalf("team = %q, want 未分配池", result.User.Team)
	}
	if result.User.EmploymentType != domain.EmploymentTypePartTime {
		t.Fatalf("employment_type = %q, want part_time", result.User.EmploymentType)
	}
	if !reflect.DeepEqual(result.User.Roles, []domain.Role{domain.RoleAssetSubmitter}) {
		t.Fatalf("roles = %+v, want only AssetSubmitter", result.User.Roles)
	}
	if containsRoleValue(result.User.Roles, domain.RoleMember) || containsRoleValue(result.User.Roles, domain.RoleDesigner) {
		t.Fatalf("workbench self-registration leaked main-site roles: %+v", result.User.Roles)
	}
	if len(logRepo.logs) != 1 || logRepo.logs[0].RoutePath != "/v1/asset-workbench/register" {
		t.Fatalf("permission logs = %+v", logRepo.logs)
	}
	if got := assignmentWriter.roleCodes(); !reflect.DeepEqual(got, []string{"member", "asset_submitter"}) {
		t.Fatalf("explicit assignments = %+v, want member + asset_submitter", assignmentWriter.calls)
	}
}

func TestIdentityServiceRegisterAbortsBeforeSessionWhenExplicitAssignmentFails(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	assignmentWriter := &identityAccessAssignmentWriterStub{err: errors.New("assignment write failed")}
	svc := NewIdentityService(
		userRepo,
		sessionRepo,
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithIdentityAccessAssignmentWriter(assignmentWriter),
	)

	result, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member_assignment_failure",
		DisplayName: "授权失败用户",
		Department:  domain.DepartmentOperations,
		Team:        "运营一组",
		Mobile:      "13800000992",
		Password:    "Pass1234",
	})
	if appErr == nil || result != nil {
		t.Fatalf("Register() = result:%+v error:%+v, want transaction failure", result, appErr)
	}
	if len(sessionRepo.sessions) != 0 {
		t.Fatalf("sessions = %+v, assignment failure must abort before session creation", sessionRepo.sessions)
	}
}

func TestIdentityServiceGetCurrentUserAllowsNonManagementSessionUser(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "warehouse_member",
		DisplayName: "云仓成员",
		Department:  domain.DepartmentCloudWarehouse,
		Team:        "默认组",
		Mobile:      "13800000999",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if err := userRepo.ReplaceRoles(context.Background(), nil, registerResult.User.ID, []domain.Role{
		domain.RoleMember,
		domain.RoleWarehouse,
	}); err != nil {
		t.Fatalf("ReplaceRoles() error = %v", err)
	}

	actor, appErr := svc.ResolveRequestActor(context.Background(), registerResult.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}

	currentUser, appErr := svc.GetCurrentUser(domain.WithRequestActor(context.Background(), *actor))
	if appErr != nil {
		t.Fatalf("GetCurrentUser() error = %+v", appErr)
	}
	if currentUser == nil || currentUser.ID != registerResult.User.ID {
		t.Fatalf("GetCurrentUser() user = %+v, want id %d", currentUser, registerResult.User.ID)
	}
	if !containsRoleValue(currentUser.Roles, domain.RoleWarehouse) || !containsRoleValue(currentUser.Roles, domain.RoleMember) {
		t.Fatalf("GetCurrentUser() roles = %+v", currentUser.Roles)
	}
	if containsString(currentUser.FrontendAccess.Pages, "warehouse_receive") {
		t.Fatalf("GetCurrentUser() frontend_access = %+v, retired Warehouse role must not hydrate warehouse pages", currentUser.FrontendAccess)
	}
}

func TestIdentityServiceUpdateMyAvatarIsDedicatedManagedPath(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "avatar_member",
		DisplayName: "头像成员",
		Department:  domain.DepartmentOperations,
		Team:        "淘系一组",
		Mobile:      "13800000998",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	actor, appErr := svc.ResolveRequestActor(context.Background(), registerResult.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	ctx := domain.WithRequestActor(context.Background(), *actor)

	if _, appErr := svc.UpdateMyAvatar(ctx, UpdateMyAvatarParams{AvatarURL: "https://example.com/avatar.png", Method: "POST"}); appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UpdateMyAvatar(external) appErr = %+v, want invalid request", appErr)
	}
	afterReject, err := userRepo.GetByID(context.Background(), registerResult.User.ID)
	if err != nil {
		t.Fatalf("GetByID() after reject error = %v", err)
	}
	if afterReject.AvatarURL != "" {
		t.Fatalf("avatar after rejected update = %q, want empty", afterReject.AvatarURL)
	}

	avatarURL := "/v1/me/avatar-files/avatar-" + strings.Repeat("a", 32) + ".png"
	updated, appErr := svc.UpdateMyAvatar(ctx, UpdateMyAvatarParams{AvatarURL: avatarURL, Method: "POST"})
	if appErr != nil {
		t.Fatalf("UpdateMyAvatar(managed) error = %+v", appErr)
	}
	if updated.AvatarURL != avatarURL || updated.Avatar != avatarURL {
		t.Fatalf("UpdateMyAvatar(managed) avatar = url:%q alias:%q, want %q", updated.AvatarURL, updated.Avatar, avatarURL)
	}

	nextName := "头像成员改名"
	profileUpdated, appErr := svc.UpdateMe(ctx, UpdateMeParams{DisplayName: &nextName})
	if appErr != nil {
		t.Fatalf("UpdateMe(profile) error = %+v", appErr)
	}
	if profileUpdated.DisplayName != nextName || profileUpdated.AvatarURL != avatarURL {
		t.Fatalf("UpdateMe(profile) user = %+v, want name changed and avatar preserved", profileUpdated)
	}

	cleared, appErr := svc.UpdateMyAvatar(ctx, UpdateMyAvatarParams{AvatarURL: "", Method: "DELETE"})
	if appErr != nil {
		t.Fatalf("UpdateMyAvatar(clear) error = %+v", appErr)
	}
	if cleared.AvatarURL != "" || cleared.Avatar != "" {
		t.Fatalf("UpdateMyAvatar(clear) avatar = url:%q alias:%q, want empty", cleared.AvatarURL, cleared.Avatar)
	}
}

func TestIdentityServiceRegistrationOptionsAndTeamValidation(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	options, appErr := svc.GetRegistrationOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetRegistrationOptions() error = %+v", appErr)
	}
	if len(options.Departments) == 0 {
		t.Fatal("GetRegistrationOptions() returned no departments")
	}

	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member1",
		DisplayName: "成员1",
		Department:  domain.DepartmentDesign,
		Mobile:      "13800000002",
		Password:    "Pass1234",
	}); appErr == nil || appErr.Message != "team is required" {
		t.Fatalf("Register(missing team) appErr = %+v", appErr)
	}

	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member2",
		DisplayName: "成员2",
		Department:  domain.DepartmentDesign,
		Team:        "错误组",
		Mobile:      "13800000003",
		Password:    "Pass1234",
	}); appErr == nil || appErr.Message != "team must belong to department" {
		t.Fatalf("Register(invalid team) appErr = %+v", appErr)
	}

	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member3",
		DisplayName: "成员3",
		Department:  domain.DepartmentUnassigned,
		Mobile:      "13800000004",
		Password:    "Pass1234",
	}); appErr == nil || appErr.Message != "team is required" {
		t.Fatalf("Register(unassigned missing team) appErr = %+v", appErr)
	}
}

func TestIdentityServiceRegisterRejectsInvalidDepartmentAndDuplicateMobile(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member1",
		DisplayName: "成员1",
		Department:  domain.DepartmentProcurement,
		Team:        "采购组",
		Mobile:      "13800000005",
		Password:    "Pass1234",
	}); appErr != nil {
		t.Fatalf("Register(first) error = %+v", appErr)
	}
	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member2",
		DisplayName: "成员2",
		Department:  domain.Department("未知部门"),
		Mobile:      "13800000006",
		Password:    "Pass1234",
	}); appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Register(invalid department) appErr = %+v", appErr)
	}
	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "member3",
		DisplayName: "成员3",
		Department:  domain.DepartmentProcurement,
		Team:        "采购组",
		Mobile:      "13800000005",
		Password:    "Pass1234",
	}); appErr == nil || appErr.Message != "mobile already exists" {
		t.Fatalf("Register(duplicate mobile) appErr = %+v", appErr)
	}
}

func TestIdentityServiceRegisterOrdinaryMemberDoesNotReceiveDepartmentBusinessBundle(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "ordinary_ops_member",
		DisplayName: "普通运营成员",
		Department:  domain.DepartmentOperations,
		Team:        "运营一组",
		Mobile:      "13800000066",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if containsRoleValue(registerResult.User.Roles, domain.RoleDeptAdmin) || containsRoleValue(registerResult.User.Roles, domain.RoleOps) {
		t.Fatalf("Register() roles = %+v, ordinary member should not receive admin/business bundle", registerResult.User.Roles)
	}
	if containsString(registerResult.User.FrontendAccess.Menus, "task_board") || containsString(registerResult.User.FrontendAccess.Menus, "user_admin") {
		t.Fatalf("Register() frontend_access menus = %+v, ordinary member should not receive business/admin menus", registerResult.User.FrontendAccess.Menus)
	}
}

func TestIdentityService_RegisterDoesNotAutoFillManagedDepartments(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "auto_fill_dept_admin",
		DisplayName: "Auto Fill Dept Admin",
		Department:  domain.DepartmentDesignRD,
		Team:        "默认组",
		Mobile:      "13800001091",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if got := registerResult.User.ManagedDepartments; len(got) != 0 {
		t.Fatalf("ManagedDepartments = %+v, want empty", got)
	}
	if containsRoleValue(registerResult.User.Roles, domain.RoleDeptAdmin) {
		t.Fatalf("roles = %+v, registration must not grant DepartmentAdmin", registerResult.User.Roles)
	}
}

func TestIdentityService_RegisterMember_NoAutoFill(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "no_auto_fill_member",
		DisplayName: "No Auto Fill Member",
		Department:  domain.DepartmentOperations,
		Team:        "淘系一组",
		Mobile:      "13800001093",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if len(registerResult.User.ManagedDepartments) != 0 {
		t.Fatalf("ManagedDepartments = %+v, want empty", registerResult.User.ManagedDepartments)
	}
}

func TestIdentityServiceChangePasswordAndRelogin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "ops_user",
		DisplayName: "运营",
		Department:  domain.DepartmentOperations,
		Team:        "运营一组",
		Mobile:      "13800000007",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	actor, appErr := svc.ResolveRequestActor(context.Background(), registerResult.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	sessionCtx := domain.WithRequestActor(context.Background(), *actor)
	if appErr := svc.ChangePassword(sessionCtx, ChangePasswordParams{
		OldPassword: "Pass1234",
		NewPassword: "Next1234",
	}); appErr != nil {
		t.Fatalf("ChangePassword() error = %+v", appErr)
	}
	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "ops_user",
		Password: "Pass1234",
	}); appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("Login(old password) appErr = %+v", appErr)
	}
	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "ops_user",
		Password: "Next1234",
	}); appErr != nil {
		t.Fatalf("Login(new password) error = %+v", appErr)
	}
}

func TestIdentityServiceCreateManagedUserSupportsInitialPasswordOrgAndMemberAccess(t *testing.T) {
	userRepo := newIdentityUserRepo()
	logRepo := &identityPermissionLogRepoStub{}
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, logRepo, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}

	adminCtx := identityGlobalAccessManageContext(1)
	user, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "ops_new",
		EmployeeNo:  intPtr(1020),
		DisplayName: "Ops New",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001020",
		Email:       "ops_new@example.com",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() error = %+v", appErr)
	}
	if user.Username != "ops_new" || user.DisplayName != "Ops New" {
		t.Fatalf("CreateManagedUser() user = %+v", user)
	}
	if user.Department != domain.DepartmentOperations || user.Team != opsTeam {
		t.Fatalf("CreateManagedUser() org = %+v", user)
	}
	if !slices.Equal(user.Roles, []domain.Role{domain.RoleMember}) {
		t.Fatalf("CreateManagedUser() roles = %+v, want [Member]", user.Roles)
	}
	if containsString(user.FrontendAccess.Actions, "task.create") {
		t.Fatalf("CreateManagedUser() must not infer task.create from account creation: %+v", user.FrontendAccess)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionUserCreated, user.ID) {
		t.Fatalf("permission logs = %+v, want user_created entry", logRepo.logs)
	}

	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "ops_new",
		Password: "Init1234",
	}); appErr != nil {
		t.Fatalf("Login(initial password) error = %+v", appErr)
	}
}

func TestIdentityServiceEmployeeNoMustBeUniqueWithBusinessMessage(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}

	adminCtx := identityGlobalAccessManageContext(1)
	first, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "employee_no_owner",
		EmployeeNo:  intPtr(88),
		DisplayName: "工号占用人",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001880",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(first) error = %+v", appErr)
	}

	_, appErr = svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "employee_no_conflict",
		EmployeeNo:  intPtr(88),
		DisplayName: "重复工号",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001881",
		Password:    "Init1234",
	})
	if appErr == nil || appErrorDenyCode(appErr) != "employee_no_conflict" {
		t.Fatalf("CreateManagedUser(duplicate employee_no) appErr = %+v, want employee_no_conflict", appErr)
	}
	if !strings.Contains(appErr.Message, "工号 88 已被 工号占用人") {
		t.Fatalf("duplicate employee_no message = %q", appErr.Message)
	}

	updateNo := 88
	_, appErr = svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID:     first.ID,
		EmployeeNo: &updateNo,
	})
	if appErr != nil {
		t.Fatalf("UpdateUser(same employee_no) error = %+v", appErr)
	}
}

func TestIdentityServiceResetUserPasswordAllowsReloginWithNewPassword(t *testing.T) {
	userRepo := newIdentityUserRepo()
	logRepo := &identityPermissionLogRepoStub{}
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, logRepo, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	procurementTeam, ok := findDepartmentTeam(options, string(domain.DepartmentProcurement))
	if !ok {
		t.Fatalf("missing procurement team in options: %+v", options.Departments)
	}

	created, appErr := svc.CreateManagedUser(identityGlobalAccessManageContext(1), CreateManagedUserParams{
		Username:    "reset_user",
		EmployeeNo:  intPtr(1021),
		DisplayName: "Reset User",
		Department:  domain.DepartmentProcurement,
		Team:        procurementTeam,
		Mobile:      "13800001021",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() error = %+v", appErr)
	}

	adminCtx := identityGlobalAccessManageContext(1)
	if _, appErr := svc.ResetUserPassword(adminCtx, ResetUserPasswordParams{
		UserID:      created.ID,
		NewPassword: "Reset1234",
	}); appErr != nil {
		t.Fatalf("ResetUserPassword() error = %+v", appErr)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionPasswordReset, created.ID) {
		t.Fatalf("permission logs = %+v, want password_reset entry", logRepo.logs)
	}
	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "reset_user",
		Password: "Init1234",
	}); appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("Login(old password) appErr = %+v", appErr)
	}
	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "reset_user",
		Password: "Reset1234",
	}); appErr != nil {
		t.Fatalf("Login(new password) error = %+v", appErr)
	}
}

func TestIdentityServiceSyncConfiguredAuthSeedsDefaultAdmin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() error = %+v", appErr)
	}
	loginResult, appErr := svc.Login(context.Background(), LoginParams{
		Username: "admin",
		Password: "ChangeMeAdmin123",
	})
	if appErr != nil {
		t.Fatalf("Login(admin) error = %+v", appErr)
	}
	if !containsRoleValue(loginResult.User.Roles, domain.RoleSuperAdmin) || containsRoleValue(loginResult.User.Roles, domain.RoleAdmin) {
		t.Fatalf("Login(admin) roles = %+v, want SuperAdmin without legacy Admin", loginResult.User.Roles)
	}
	if loginResult.User.FrontendAccess.IsSuperAdmin {
		t.Fatalf("Login(admin) frontend_access = %+v, legacy role must not grant explicit SuperAdmin access", loginResult.User.FrontendAccess)
	}
	if loginResult.User.Team != "未分配池" {
		t.Fatalf("Login(admin) team = %s", loginResult.User.Team)
	}
}

func TestSeedHRAdminConfigSuperAdmin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() error = %+v", appErr)
	}

	user, err := userRepo.GetByUsername(context.Background(), "HRAdmin")
	if err != nil {
		t.Fatalf("GetByUsername(HRAdmin) error = %v", err)
	}
	if user == nil {
		t.Fatal("GetByUsername(HRAdmin) returned nil")
	}
	if !slices.Equal(user.Roles, []domain.Role{domain.RoleHRAdmin, domain.RoleOrgAdmin}) {
		t.Fatalf("HRAdmin roles = %+v, want [HRAdmin OrgAdmin]", user.Roles)
	}
	if user.Department != domain.DepartmentHR {
		t.Fatalf("HRAdmin department = %q, want %q", user.Department, domain.DepartmentHR)
	}
	if user.Status != domain.UserStatusActive {
		t.Fatalf("HRAdmin status = %q, want active", user.Status)
	}
	if user.EmploymentType != domain.EmploymentTypeFullTime {
		t.Fatalf("HRAdmin employment_type = %q, want full_time", user.EmploymentType)
	}
	if !user.IsConfigSuperAdmin {
		t.Fatal("HRAdmin IsConfigSuperAdmin = false, want true")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("ChangeMeAdmin123")); err != nil {
		t.Fatalf("HRAdmin password hash does not match: %v", err)
	}
}

func TestIdentityServiceGetOrgOptionsIncludesUnassignedPool(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	if !options.UnassignedPoolEnabled {
		t.Fatal("GetOrgOptions() expected unassigned pool to be enabled")
	}
	if !containsDepartmentOption(options.Departments, "未分配", "未分配池") {
		t.Fatalf("GetOrgOptions() departments = %+v", options.Departments)
	}
}

func TestIdentityServiceGetOrgOptionsReturnsDefensiveCopy(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	first, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() first error = %+v", appErr)
	}
	if len(first.Departments) == 0 {
		t.Fatal("GetOrgOptions() expected at least one department option")
	}
	first.Departments[0].Name = "MUTATED"
	first.TeamsByDepartment = map[string][]string{"MUTATED": {"x"}}

	second, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() second error = %+v", appErr)
	}
	if second.Departments[0].Name == "MUTATED" {
		t.Fatalf("GetOrgOptions() second call leaked previous mutation: %+v", second.Departments[0])
	}
	if _, ok := second.TeamsByDepartment["MUTATED"]; ok {
		t.Fatalf("GetOrgOptions() teams map leaked previous mutation: %+v", second.TeamsByDepartment)
	}
}

func TestIdentityServiceGetOrgOptionsCanonicalDepartmentsCarryTeams(t *testing.T) {
	svc := NewIdentityService(newIdentityUserRepo(), &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	if len(options.Departments) == 0 {
		t.Fatal("GetOrgOptions() expected at least one department option")
	}
	for _, department := range options.Departments {
		if len(department.Teams) == 0 {
			continue
		}
		compatTeams := options.TeamsByDepartment[department.Name]
		if len(compatTeams) == 0 {
			t.Fatalf("GetOrgOptions() compatibility mirror missing department %q", department.Name)
		}
		if !slices.Equal(department.Teams, compatTeams) {
			t.Fatalf("GetOrgOptions() department teams mismatch for %q: canonical=%v compatibility=%v", department.Name, department.Teams, compatTeams)
		}
		return
	}
	t.Fatalf("GetOrgOptions() expected at least one department with populated teams: %+v", options.Departments)
}

func TestIdentityServiceListUsersBatchAttachRoles(t *testing.T) {
	userRepo := newIdentityUserRepo()
	logRepo := &identityPermissionLogRepoStub{}
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, logRepo, identityTxRunner{})

	options, appErr := svc.GetRegistrationOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetRegistrationOptions() error = %+v", appErr)
	}
	if len(options.Departments) == 0 || len(options.Departments[0].Teams) == 0 {
		t.Fatalf("GetRegistrationOptions() invalid departments = %+v", options.Departments)
	}
	department := domain.Department(options.Departments[0].Name)
	team := options.Departments[0].Teams[0]

	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "u_a",
		DisplayName: "User A",
		Department:  department,
		Team:        team,
		Mobile:      "13800001001",
		Password:    "Pass1234",
	}); appErr != nil {
		t.Fatalf("Register(u_a) error = %+v", appErr)
	}
	if _, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "u_b",
		DisplayName: "User B",
		Department:  department,
		Team:        team,
		Mobile:      "13800001002",
		Password:    "Pass1234",
	}); appErr != nil {
		t.Fatalf("Register(u_b) error = %+v", appErr)
	}
	for userID := range userRepo.users {
		if userID == 0 {
			continue
		}
		_ = userRepo.ReplaceRoles(context.Background(), identityTx{}, userID, []domain.Role{domain.RoleMember, domain.RoleDesigner})
	}

	users, _, appErr := svc.ListUsers(context.Background(), UserFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("ListUsers() error = %+v", appErr)
	}
	if len(users) < 2 {
		t.Fatalf("ListUsers() users len = %d, want >=2", len(users))
	}
	if userRepo.listRolesByUserIDsCalls == 0 {
		t.Fatal("ListUsers() expected batch role query to be used")
	}
	if userRepo.listRolesCalls != 0 {
		t.Fatalf("ListUsers() fallback single-role reads should not be used, got %d calls", userRepo.listRolesCalls)
	}
	for _, user := range users {
		if user == nil || len(user.Roles) == 0 {
			t.Fatalf("ListUsers() user roles missing: %+v", user)
		}
	}
}

func TestIdentityServiceListUsersSupportsDepartmentTeamRoleAndKeywordFilters(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}
	warehouseTeam, ok := findDepartmentTeam(options, string(domain.DepartmentWarehouse))
	if !ok {
		t.Fatalf("missing warehouse team in options: %+v", options.Departments)
	}

	if _, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "ops_filter",
		EmployeeNo:  intPtr(1022),
		DisplayName: "Ops Filter",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001022",
		Password:    "Init1234",
	}); appErr != nil {
		t.Fatalf("CreateManagedUser(ops_filter) error = %+v", appErr)
	}
	if _, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "warehouse_filter",
		EmployeeNo:  intPtr(1023),
		DisplayName: "Warehouse Filter",
		Department:  domain.DepartmentWarehouse,
		Team:        warehouseTeam,
		Mobile:      "13800001023",
		Password:    "Init1234",
	}); appErr != nil {
		t.Fatalf("CreateManagedUser(warehouse_filter) error = %+v", appErr)
	}

	role := domain.RoleMember
	department := domain.DepartmentOperations
	users, pagination, appErr := svc.ListUsers(context.Background(), UserFilter{
		Keyword:    "ops",
		Role:       &role,
		Department: &department,
		Team:       opsTeam,
		Page:       1,
		PageSize:   10,
	})
	if appErr != nil {
		t.Fatalf("ListUsers() error = %+v", appErr)
	}
	if pagination.Total != 1 || len(users) != 1 {
		t.Fatalf("ListUsers() pagination/data = %+v / %+v", pagination, users)
	}
	if users[0].Username != "ops_filter" {
		t.Fatalf("ListUsers() user = %+v, want ops_filter", users[0])
	}
}

func TestIdentityServiceListUsersInvalidOrgFilterReturnsEmptyPage(t *testing.T) {
	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	orgRepo.departments[1] = &domain.OrgDepartment{ID: 1, Name: "不存在部门", Enabled: false}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithOrgRepo(orgRepo),
	)

	department := domain.Department("不存在部门")
	users, pagination, appErr := svc.ListUsers(context.Background(), UserFilter{
		Department: &department,
		Team:       "不存在小组",
		Page:       1,
		PageSize:   20,
	})
	if appErr != nil {
		t.Fatalf("ListUsers() error = %+v", appErr)
	}
	if len(users) != 0 || pagination.Total != 0 {
		t.Fatalf("ListUsers() data = %+v pagination = %+v, want empty page", users, pagination)
	}
	if len(userRepo.listFilters) != 0 {
		t.Fatalf("ListUsers() should not hit user repo for invalid org filter, got filters %+v", userRepo.listFilters)
	}
}

func TestIdentityServiceUpdateUserSupportsOrgAndManagedScopes(t *testing.T) {
	userRepo := newIdentityUserRepo()
	logRepo := &identityPermissionLogRepoStub{}
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, logRepo, identityTxRunner{})

	member, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "pool_user",
		DisplayName: "待分配成员",
		Department:  domain.DepartmentUnassigned,
		Team:        "未分配池",
		Mobile:      "13800000009",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(pool_user) error = %+v", appErr)
	}

	displayName := "定制美工组成员"
	department := domain.DepartmentDesign
	team := "定制美工组"
	managedDepartments := []string{string(domain.DepartmentDesign)}
	managedTeams := []string{"定制美工组"}
	adminCtx := identityGlobalAccessManageContext(1)

	updated, appErr := svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID:             member.User.ID,
		DisplayName:        &displayName,
		Department:         &department,
		Team:               &team,
		ManagedDepartments: &managedDepartments,
		ManagedTeams:       &managedTeams,
	})
	if appErr != nil {
		t.Fatalf("UpdateUser() error = %+v", appErr)
	}
	if updated.Department != domain.DepartmentDesign || updated.Team != "定制美工组" {
		t.Fatalf("UpdateUser() org = %+v", updated)
	}
	if !containsString(updated.ManagedDepartments, string(domain.DepartmentDesign)) || !containsString(updated.ManagedTeams, "定制美工组") {
		t.Fatalf("UpdateUser() managed scopes = departments:%+v teams:%+v", updated.ManagedDepartments, updated.ManagedTeams)
	}
	if len(updated.FrontendAccess.ManagedDepartments) != 0 || len(updated.FrontendAccess.ManagedTeams) != 0 {
		t.Fatalf("UpdateUser() frontend_access = %+v, legacy managed fields must not grant frontend scope", updated.FrontendAccess)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionPoolAssigned, member.User.ID) {
		t.Fatalf("permission logs = %+v, want user_pool_assigned entry", logRepo.logs)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionUserOrgChanged, member.User.ID) {
		t.Fatalf("permission logs = %+v, want user_org_changed entry", logRepo.logs)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionUserScopeChanged, member.User.ID) {
		t.Fatalf("permission logs = %+v, want user_scope_changed entry", logRepo.logs)
	}
}

func TestIdentityServiceUpdateUserPatchKeepsUnspecifiedFields(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	targetDepartment, targetTeams, ok := pickOrgDepartmentWithTeams(options, string(domain.DepartmentUnassigned), 1)
	if !ok {
		t.Fatalf("pick target department failed: %+v", options.Departments)
	}
	originDepartment, originTeams, ok := pickOrgDepartmentWithTeams(options, targetDepartment, 1)
	if !ok {
		t.Fatalf("pick origin department failed: %+v", options.Departments)
	}

	user, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "patch_preserve_user",
		EmployeeNo:  intPtr(1080),
		DisplayName: "Patch Preserve User",
		Department:  domain.Department(originDepartment),
		Team:        originTeams[0],
		Mobile:      "13800001080",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() error = %+v", appErr)
	}

	adminCtx := identityGlobalAccessManageContext(1)
	department := domain.Department(targetDepartment)
	team := targetTeams[0]
	updated, appErr := svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID:     user.ID,
		Department: &department,
		Team:       &team,
	})
	if appErr != nil {
		t.Fatalf("UpdateUser() error = %+v", appErr)
	}
	if updated.Department != department || updated.Team != team {
		t.Fatalf("UpdateUser() org = department:%q team:%q, want department:%q team:%q", updated.Department, updated.Team, department, team)
	}
	if updated.DisplayName != user.DisplayName {
		t.Fatalf("UpdateUser() display_name = %q, want preserved %q", updated.DisplayName, user.DisplayName)
	}
	if updated.Status != user.Status {
		t.Fatalf("UpdateUser() status = %q, want preserved %q", updated.Status, user.Status)
	}

	reloaded, appErr := svc.GetUser(adminCtx, user.ID)
	if appErr != nil {
		t.Fatalf("GetUser() error = %+v", appErr)
	}
	if reloaded.DisplayName != user.DisplayName || reloaded.Status != user.Status {
		t.Fatalf("GetUser() preserved fields mismatch: display_name=%q status=%q", reloaded.DisplayName, reloaded.Status)
	}
}

func TestIdentityServiceUpdateUserSupportsCanonicalTeamAndUngrouped(t *testing.T) {
	userRepo := newIdentityUserRepo()
	logRepo := &identityPermissionLogRepoStub{}
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, logRepo, identityTxRunner{})

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	formalDepartment, formalTeams, ok := pickOrgDepartmentWithTeams(options, string(domain.DepartmentUnassigned), 2)
	if !ok {
		t.Fatalf("pick formal department failed: %+v", options.Departments)
	}
	unassignedTeam, ok := findDepartmentTeam(options, string(domain.DepartmentUnassigned))
	if !ok {
		t.Fatalf("missing unassigned team in options: %+v", options.Departments)
	}

	member, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "group_alias_user",
		DisplayName: "Group Alias User",
		Department:  domain.Department(formalDepartment),
		Team:        formalTeams[0],
		Mobile:      "13800001009",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register(group_alias_user) error = %+v", appErr)
	}

	adminCtx := identityGlobalAccessManageContext(1)

	selectedTeam := formalTeams[1]
	updated, appErr := svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID: member.User.ID,
		Team:   &selectedTeam,
	})
	if appErr != nil {
		t.Fatalf("UpdateUser(team) error = %+v", appErr)
	}
	if updated.Team != selectedTeam {
		t.Fatalf("UpdateUser(team) team = %q, want %q", updated.Team, selectedTeam)
	}

	ungrouped := "ungrouped"
	updated, appErr = svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID: member.User.ID,
		Team:   &ungrouped,
	})
	if appErr != nil {
		t.Fatalf("UpdateUser(ungrouped) error = %+v", appErr)
	}
	if updated.Department != domain.DepartmentUnassigned || updated.Team != unassignedTeam {
		t.Fatalf("UpdateUser(ungrouped) org = department:%q team:%q, want department:%q team:%q", updated.Department, updated.Team, domain.DepartmentUnassigned, unassignedTeam)
	}
	if !hasPermissionAction(logRepo.logs, domain.PermissionActionUserOrgChanged, member.User.ID) {
		t.Fatalf("permission logs = %+v, want user_org_changed entry", logRepo.logs)
	}
}

func TestIdentityServiceUpdateUserDisableBlocksLogin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	designTeam, ok := findDepartmentTeam(options, string(domain.DepartmentDesign))
	if !ok {
		t.Fatalf("missing design team in options: %+v", options.Departments)
	}

	user, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "disabled_user",
		EmployeeNo:  intPtr(1024),
		DisplayName: "Disabled User",
		Department:  domain.DepartmentDesign,
		Team:        designTeam,
		Mobile:      "13800001024",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() error = %+v", appErr)
	}

	disabled := domain.UserStatusDisabled
	adminCtx := identityGlobalAccessManageContext(1)
	if _, appErr := svc.UpdateUser(adminCtx, UpdateUserParams{
		UserID: user.ID,
		Status: &disabled,
	}); appErr != nil {
		t.Fatalf("UpdateUser(disable) error = %+v", appErr)
	}
	if _, appErr := svc.Login(context.Background(), LoginParams{
		Username: "disabled_user",
		Password: "Init1234",
	}); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Login(disabled user) appErr = %+v", appErr)
	}
}

func TestIdentityServiceResolveRequestActorHydratesFrontendAccess(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})

	registerResult, appErr := svc.Register(context.Background(), RegisterUserParams{
		Username:    "hydrated_dept_admin",
		DisplayName: "Hydrated Department Admin",
		Department:  domain.DepartmentDesignRD,
		Team:        "默认组",
		Mobile:      "13800001025",
		Password:    "Pass1234",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if err := userRepo.ReplaceRoles(context.Background(), identityTx{}, registerResult.User.ID, []domain.Role{domain.RoleMember, domain.RoleDeptAdmin}); err != nil {
		t.Fatalf("ReplaceRoles() error = %v", err)
	}
	userRepo.users[registerResult.User.ID].ManagedDepartments = []string{string(domain.DepartmentDesignRD)}

	actor, appErr := svc.ResolveRequestActor(context.Background(), registerResult.Session.Token)
	if appErr != nil {
		t.Fatalf("ResolveRequestActor() error = %+v", appErr)
	}
	if actor == nil {
		t.Fatal("ResolveRequestActor() returned nil actor")
	}
	if actor.FrontendAccess.Department == "" || actor.FrontendAccess.Team == "" {
		t.Fatalf("ResolveRequestActor() frontend_access org = %+v", actor.FrontendAccess)
	}
	if len(actor.FrontendAccess.Pages) == 0 || len(actor.FrontendAccess.Actions) == 0 {
		t.Fatalf("ResolveRequestActor() frontend_access = %+v, want hydrated pages/actions", actor.FrontendAccess)
	}
	if !actor.FrontendAccess.IsDepartmentAdmin {
		t.Fatalf("ResolveRequestActor() frontend_access = %+v, want department admin marker", actor.FrontendAccess)
	}
}

func TestIdentityServiceDepartmentAdminCannotCreateUserOutsideOwnDepartment(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	designTeam, ok := findDepartmentTeam(options, string(domain.DepartmentDesignRD))
	if !ok {
		t.Fatalf("missing design-rd team in options: %+v", options.Departments)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         1,
		Username:   "design_admin",
		Roles:      []domain.Role{domain.RoleDeptAdmin},
		Department: string(domain.DepartmentDesignRD),
		Team:       designTeam,
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr = svc.CreateManagedUser(ctx, CreateManagedUserParams{
		Username:    "ops_member_by_design_admin",
		EmployeeNo:  intPtr(1026),
		DisplayName: "Ops Member",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001026",
		Password:    "Init1234",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateManagedUser() appErr = %+v", appErr)
	}
}

func TestIdentityServiceTeamLeadCannotCreateManagedUser(t *testing.T) {
	userRepo := newIdentityUserRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         2,
		Username:   "ops_team_lead",
		Roles:      []domain.Role{domain.RoleTeamLead},
		Department: string(domain.DepartmentOperations),
		Team:       opsTeam,
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})
	_, appErr = svc.CreateManagedUser(ctx, CreateManagedUserParams{
		Username:    "ops_member_by_lead",
		EmployeeNo:  intPtr(1027),
		DisplayName: "Ops Member By Lead",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001027",
		Password:    "Init1234",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateManagedUser() appErr = %+v", appErr)
	}
}

func TestIdentityServiceScopedAccessPasswordResetAndCrossDepartmentBoundaries(t *testing.T) {
	userRepo := newIdentityUserRepo()
	sessionRepo := &identitySessionRepoStub{}
	svc := NewIdentityService(userRepo, sessionRepo, &identityPermissionLogRepoStub{}, identityTxRunner{})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() error = %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}
	designTeam, ok := findDepartmentTeam(options, string(domain.DepartmentDesignRD))
	if !ok {
		t.Fatalf("missing design-rd team in options: %+v", options.Departments)
	}

	opsUser, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "ops_member_resettable",
		EmployeeNo:  intPtr(1028),
		DisplayName: "Ops Member Resettable",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800001028",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(ops) error = %+v", appErr)
	}
	designUser, appErr := svc.CreateManagedUser(context.Background(), CreateManagedUserParams{
		Username:    "design_member_protected",
		EmployeeNo:  intPtr(1029),
		DisplayName: "Design Member Protected",
		Department:  domain.DepartmentDesignRD,
		Team:        designTeam,
		Mobile:      "13800001029",
		Password:    "Init1234",
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(design) error = %+v", appErr)
	}
	opsDepartmentID := int64(101)
	designDepartmentID := int64(102)
	userRepo.users[opsUser.ID].DepartmentID = &opsDepartmentID
	userRepo.users[designUser.ID].DepartmentID = &designDepartmentID
	opsUser.DepartmentID = &opsDepartmentID
	designUser.DepartmentID = &designDepartmentID
	deptAdminCtx := identityDepartmentAccessManageContext(3, opsDepartmentID)

	if _, appErr := svc.ResetUserPassword(deptAdminCtx, ResetUserPasswordParams{
		UserID:      opsUser.ID,
		NewPassword: "Reset1234",
	}); appErr != nil {
		t.Fatalf("ResetUserPassword(own department) error = %+v", appErr)
	}
	if _, appErr := svc.ResetUserPassword(deptAdminCtx, ResetUserPasswordParams{
		UserID:      designUser.ID,
		NewPassword: "Reset1234",
	}); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("ResetUserPassword(other department) appErr = %+v", appErr)
	}

	if _, appErr := svc.UpdateUser(deptAdminCtx, UpdateUserParams{
		UserID:     designUser.ID,
		Department: ptrDepartment(domain.DepartmentOperations),
		Team:       &opsTeam,
	}); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("UpdateUser(move other department) appErr = %+v", appErr)
	}
}

func ptrDepartment(department domain.Department) *domain.Department {
	return &department
}

type identityTx struct{}

func (identityTx) IsTx() {}

type identityTxRunner struct{}

func (identityTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(identityTx{})
}

type identityAccessAssignmentCall struct {
	userID    int64
	roleCode  string
	scopeMode domain.AccessScopeMode
}

type identityAccessAssignmentWriterStub struct {
	calls []identityAccessAssignmentCall
	err   error
}

func (s *identityAccessAssignmentWriterStub) EnsureExplicitRoleAssignment(_ context.Context, _ repo.Tx, userID int64, roleCode string, scopeMode domain.AccessScopeMode) error {
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, identityAccessAssignmentCall{userID: userID, roleCode: roleCode, scopeMode: scopeMode})
	return nil
}

func (s *identityAccessAssignmentWriterStub) roleCodes() []string {
	out := make([]string, 0, len(s.calls))
	for _, call := range s.calls {
		out = append(out, call.roleCode)
	}
	return out
}

type identityUserRepoStub struct {
	users                   map[int64]*domain.User
	byName                  map[string]int64
	byMob                   map[string]int64
	roles                   map[int64][]domain.Role
	rawRoles                map[int64][]string
	nextID                  int64
	listFilters             []repo.UserListFilter
	listRolesCalls          int
	listRolesRawCalls       int
	listRolesByUserIDsCalls int
}

func newIdentityUserRepo() *identityUserRepoStub {
	return &identityUserRepoStub{
		users:    map[int64]*domain.User{},
		byName:   map[string]int64{},
		byMob:    map[string]int64{},
		roles:    map[int64][]domain.Role{},
		rawRoles: map[int64][]string{},
		nextID:   1,
	}
}

// setRawRolesForTest lets regression tests simulate raw DB rows that contain
// unknown role strings or that are entirely empty, so ResolveRequestActor's
// telemetry paths can be exercised. When set, these strings take precedence
// over the normalized roles slice on reads.
func (r *identityUserRepoStub) setRawRolesForTest(userID int64, rawRoles []string) {
	if r.rawRoles == nil {
		r.rawRoles = map[int64][]string{}
	}
	r.rawRoles[userID] = append([]string(nil), rawRoles...)
	r.roles[userID] = append([]domain.Role(nil), domain.NormalizeRoles(rawRoles)...)
}

func (r *identityUserRepoStub) Count(_ context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *identityUserRepoStub) CountByRole(_ context.Context, role domain.Role) (int64, error) {
	var total int64
	for _, roles := range r.roles {
		for _, current := range roles {
			if current == role {
				total++
				break
			}
		}
	}
	return total, nil
}

func (r *identityUserRepoStub) CountByDepartment(_ context.Context, department string) (int64, error) {
	var total int64
	for _, user := range r.users {
		if user != nil && string(user.Department) == strings.TrimSpace(department) {
			total++
		}
	}
	return total, nil
}

func (r *identityUserRepoStub) CountByTeam(_ context.Context, team string) (int64, error) {
	var total int64
	for _, user := range r.users {
		if user != nil && user.Team == strings.TrimSpace(team) {
			total++
		}
	}
	return total, nil
}

func (r *identityUserRepoStub) Create(_ context.Context, _ repo.Tx, user *domain.User) (int64, error) {
	id := r.nextID
	r.nextID++
	copyUser := *user
	copyUser.ID = id
	r.users[id] = &copyUser
	r.byName[strings.ToLower(copyUser.Username)] = id
	r.byMob[copyUser.Mobile] = id
	return id, nil
}

func (r *identityUserRepoStub) GetByID(_ context.Context, id int64) (*domain.User, error) {
	user := r.users[id]
	if user == nil {
		return nil, nil
	}
	copyUser := *user
	copyUser.Roles = append([]domain.Role{}, r.roles[id]...)
	return &copyUser, nil
}

func (r *identityUserRepoStub) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	id, ok := r.byName[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return nil, nil
	}
	return r.GetByID(context.Background(), id)
}

func (r *identityUserRepoStub) GetByMobile(_ context.Context, mobile string) (*domain.User, error) {
	id, ok := r.byMob[strings.TrimSpace(mobile)]
	if !ok {
		return nil, nil
	}
	return r.GetByID(context.Background(), id)
}

func (r *identityUserRepoStub) GetByEmployeeNo(_ context.Context, employeeNo int) (*domain.User, error) {
	for _, user := range r.users {
		if user == nil || user.EmployeeNo == nil || *user.EmployeeNo != employeeNo {
			continue
		}
		return r.GetByID(context.Background(), user.ID)
	}
	return nil, nil
}

func (r *identityUserRepoStub) GetByJstUID(_ context.Context, jstUID int64) (*domain.User, error) {
	for _, user := range r.users {
		if user.JstUID != nil && *user.JstUID == jstUID {
			copyUser := *user
			copyUser.Roles = append([]domain.Role{}, r.roles[user.ID]...)
			return &copyUser, nil
		}
	}
	return nil, nil
}

func (r *identityUserRepoStub) List(_ context.Context, filter repo.UserListFilter) ([]*domain.User, int64, error) {
	r.listFilters = append(r.listFilters, filter)
	return r.listWithFilter(filter)
}

func (r *identityUserRepoStub) listWithFilter(filter repo.UserListFilter) ([]*domain.User, int64, error) {
	page, pageSize := normalizeIdentityTestPage(filter.Page, filter.PageSize)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	users := make([]*domain.User, 0, len(r.users))
	for id := range r.users {
		user, _ := r.GetByID(context.Background(), id)
		if user == nil {
			continue
		}
		if filter.ScopeRestricted && !filter.ScopeGlobal {
			inScope := slices.Contains(filter.ScopeUserIDs, user.ID)
			if !inScope && user.DepartmentID != nil {
				inScope = slices.Contains(filter.ScopeDepartmentIDs, *user.DepartmentID)
			}
			if !inScope && user.TeamID != nil {
				inScope = slices.Contains(filter.ScopeTeamIDs, *user.TeamID)
			}
			if !inScope {
				continue
			}
		}
		if filter.Status != nil && user.Status != *filter.Status {
			continue
		}
		if filter.Role != nil && !containsRoleValue(r.roles[id], *filter.Role) {
			continue
		}
		if filter.Department != nil && user.Department != *filter.Department {
			continue
		}
		if team := strings.TrimSpace(filter.Team); team != "" && user.Team != team {
			continue
		}
		if keyword != "" {
			employeeNo := ""
			if user.EmployeeNo != nil {
				employeeNo = strconv.Itoa(*user.EmployeeNo)
			}
			nameMatch := strings.Contains(strings.ToLower(user.Username), keyword) ||
				strings.Contains(strings.ToLower(user.DisplayName), keyword) ||
				strings.Contains(employeeNo, keyword)
			if !nameMatch {
				continue
			}
		}
		users = append(users, user)
	}
	// 与 MySQL 仓储的 ORDER BY id DESC 对齐:map 迭代无序,不排序会导致
	// 跨页读取时同一用户重复/漏计(collectOrgMemberCounts 等分页消费方依赖稳定顺序)。
	slices.SortFunc(users, func(a, b *domain.User) int {
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		default:
			return 0
		}
	})
	total := int64(len(users))
	start := (page - 1) * pageSize
	if start >= len(users) {
		return []*domain.User{}, total, nil
	}
	end := start + pageSize
	if end > len(users) {
		end = len(users)
	}
	return users[start:end], total, nil
}

func normalizeIdentityTestPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func (r *identityUserRepoStub) ListConfigManagedAdmins(_ context.Context) ([]*domain.User, error) {
	users := make([]*domain.User, 0)
	for id, user := range r.users {
		if !user.IsConfigSuperAdmin {
			continue
		}
		copyUser := *user
		copyUser.Roles = append([]domain.Role{}, r.roles[id]...)
		users = append(users, &copyUser)
	}
	return users, nil
}

func (r *identityUserRepoStub) Update(_ context.Context, _ repo.Tx, user *domain.User) error {
	copyUser := *user
	r.users[user.ID] = &copyUser
	r.byName[strings.ToLower(copyUser.Username)] = user.ID
	r.byMob[copyUser.Mobile] = user.ID
	return nil
}

func (r *identityUserRepoStub) UpdateJstFields(_ context.Context, _ repo.Tx, userID int64, displayName, status, department, team string, managedDepartments, managedTeams []string, jstRawSnapshot string, jstUID *int64, lastLoginAt *time.Time) error {
	if user := r.users[userID]; user != nil {
		user.DisplayName = displayName
		user.Status = domain.UserStatus(status)
		user.Department = domain.Department(department)
		user.Team = team
		user.ManagedDepartments = append([]string{}, managedDepartments...)
		user.ManagedTeams = append([]string{}, managedTeams...)
		user.JstRawSnapshotJSON = jstRawSnapshot
		user.JstUID = jstUID
		user.LastLoginAt = lastLoginAt
	}
	return nil
}

func (r *identityUserRepoStub) UpdatePassword(_ context.Context, _ repo.Tx, userID int64, passwordHash string, updatedAt time.Time) error {
	if user := r.users[userID]; user != nil {
		user.PasswordHash = passwordHash
		user.UpdatedAt = updatedAt
	}
	return nil
}

func (r *identityUserRepoStub) UpdateLastLogin(_ context.Context, _ repo.Tx, userID int64, at time.Time) error {
	if user := r.users[userID]; user != nil {
		user.LastLoginAt = &at
	}
	return nil
}

func (r *identityUserRepoStub) ReplaceRoles(_ context.Context, _ repo.Tx, userID int64, roles []domain.Role) error {
	r.roles[userID] = append([]domain.Role{}, domain.NormalizeRoleValues(roles)...)
	return nil
}

func (r *identityUserRepoStub) ListRoles(_ context.Context, userID int64) ([]domain.Role, error) {
	r.listRolesCalls++
	return append([]domain.Role{}, r.roles[userID]...), nil
}

// ListRolesRaw satisfies the optional userRoleRawReader interface consumed by
// ResolveRequestActor. When test callers installed rawRoles via
// setRawRolesForTest we return those strings verbatim; otherwise we fall
// back to the normalized slice cast to strings to match production
// semantics on unknown-role-free inputs.
func (r *identityUserRepoStub) ListRolesRaw(_ context.Context, userID int64) ([]string, error) {
	r.listRolesRawCalls++
	if raw, ok := r.rawRoles[userID]; ok {
		return append([]string(nil), raw...), nil
	}
	out := make([]string, 0, len(r.roles[userID]))
	for _, role := range r.roles[userID] {
		out = append(out, string(role))
	}
	return out, nil
}

func (r *identityUserRepoStub) ListRolesByUserIDs(_ context.Context, userIDs []int64) (map[int64][]domain.Role, error) {
	r.listRolesByUserIDsCalls++
	out := make(map[int64][]domain.Role, len(userIDs))
	for _, userID := range userIDs {
		out[userID] = append([]domain.Role{}, r.roles[userID]...)
	}
	return out, nil
}

type identitySessionRepoStub struct {
	sessions map[string]*domain.UserSession
}

func (r *identitySessionRepoStub) Create(_ context.Context, _ repo.Tx, session *domain.UserSession) (*domain.UserSession, error) {
	if r.sessions == nil {
		r.sessions = map[string]*domain.UserSession{}
	}
	copySession := *session
	r.sessions[copySession.TokenHash] = &copySession
	return &copySession, nil
}

func (r *identitySessionRepoStub) GetByTokenHash(_ context.Context, tokenHash string) (*domain.UserSession, error) {
	if session := r.sessions[tokenHash]; session != nil {
		copySession := *session
		return &copySession, nil
	}
	return nil, nil
}

func (r *identitySessionRepoStub) Touch(_ context.Context, sessionID string, at time.Time, expiresAt time.Time) error {
	for _, session := range r.sessions {
		if session.SessionID == sessionID {
			session.LastSeenAt = &at
			session.ExpiresAt = expiresAt
		}
	}
	return nil
}

type identityPermissionLogRepoStub struct {
	logs []*domain.PermissionLog
}

func (r *identityPermissionLogRepoStub) Create(_ context.Context, entry *domain.PermissionLog) error {
	copyEntry := *entry
	r.logs = append(r.logs, &copyEntry)
	return nil
}

func (r *identityPermissionLogRepoStub) List(_ context.Context, _ repo.PermissionLogListFilter) ([]*domain.PermissionLog, int64, error) {
	return r.logs, int64(len(r.logs)), nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsRoleValue(values []domain.Role, target domain.Role) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasPermissionLog(logs []*domain.PermissionLog, actionType string, targetUserID int64, targetRoles []domain.Role) bool {
	for _, entry := range logs {
		if entry.ActionType != actionType {
			continue
		}
		if entry.TargetUserID == nil || *entry.TargetUserID != targetUserID {
			continue
		}
		if len(domain.NormalizeRoleValues(entry.TargetRoles)) != len(domain.NormalizeRoleValues(targetRoles)) {
			continue
		}
		allMatch := true
		for _, role := range domain.NormalizeRoleValues(targetRoles) {
			if !containsRoleValue(entry.TargetRoles, role) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}
	return false
}

func hasPermissionAction(logs []*domain.PermissionLog, actionType string, targetUserID int64) bool {
	for _, entry := range logs {
		if entry.ActionType != actionType {
			continue
		}
		if entry.TargetUserID != nil && *entry.TargetUserID == targetUserID {
			return true
		}
	}
	return false
}

func containsDepartmentOption(options []domain.DepartmentOption, department, team string) bool {
	for _, option := range options {
		if option.Name != department {
			continue
		}
		for _, current := range option.Teams {
			if current == team {
				return true
			}
		}
	}
	return false
}

func findDepartmentTeam(options *domain.OrgOptions, department string) (string, bool) {
	if options == nil {
		return "", false
	}
	for _, option := range options.Departments {
		if option.Name != department {
			continue
		}
		for _, team := range option.Teams {
			if strings.TrimSpace(team) != "" {
				return team, true
			}
		}
	}
	return "", false
}

func pickOrgDepartmentWithTeams(options *domain.OrgOptions, excludedDepartment string, minTeams int) (string, []string, bool) {
	if options == nil {
		return "", nil, false
	}
	for _, option := range options.Departments {
		if option.Name == excludedDepartment {
			continue
		}
		teams := make([]string, 0, len(option.Teams))
		for _, team := range option.Teams {
			trimmed := strings.TrimSpace(team)
			if trimmed == "" {
				continue
			}
			teams = append(teams, trimmed)
		}
		if len(teams) >= minTeams {
			return option.Name, teams, true
		}
	}
	return "", nil, false
}
