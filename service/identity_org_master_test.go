package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestIdentityServiceOrgMasterBackendizesOptionsUsersAndTaskCatalog(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}

	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "品牌部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	team, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{
		DepartmentID: &department.ID,
		Name:         "品牌一组",
	})
	if appErr != nil {
		t.Fatalf("CreateTeam() unexpected error: %+v", appErr)
	}
	if team.DepartmentID != department.ID || team.Department != "品牌部" {
		t.Fatalf("CreateTeam() team = %+v", team)
	}

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() unexpected error: %+v", appErr)
	}
	if !orgOptionsContainDepartmentTeam(options, "品牌部", "品牌一组") {
		t.Fatalf("GetOrgOptions() = %+v, want 品牌部/品牌一组", options)
	}

	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       1,
		Username: "admin",
		Roles:    []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	user, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "brand_user",
		EmployeeNo:  intPtr(2101),
		DisplayName: "Brand User",
		Department:  domain.Department("品牌部"),
		Team:        "品牌一组",
		Mobile:      "13800009901",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleOps},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() unexpected error: %+v", appErr)
	}
	if user.Department != domain.Department("品牌部") || user.Team != "品牌一组" {
		t.Fatalf("CreateManagedUser() user = %+v", user)
	}

	p := normalizeCreateTaskParams(ownerTeamGuardrailBaseParams("品牌一组"))
	p.OwnerDepartment = "品牌部"
	p.OwnerOrgTeam = "品牌一组"
	if appErr := validateCreateTaskEntry(context.Background(), p); appErr != nil {
		t.Fatalf("validateCreateTaskEntry() unexpected error: %+v", appErr)
	}
	ownership, appErr := resolveTaskCanonicalOrgOwnership(p)
	if appErr != nil {
		t.Fatalf("resolveTaskCanonicalOrgOwnership() unexpected error: %+v", appErr)
	}
	if ownership.OwnerDepartment != "品牌部" || ownership.OwnerOrgTeam != "品牌一组" || ownership.LegacyOwnerTeam != "品牌一组" {
		t.Fatalf("task ownership = %+v", ownership)
	}
}

func TestIdentityServiceRenameTeamRewritesAllPagedUsers(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "分页运营部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	team, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "淘系二组"})
	if appErr != nil {
		t.Fatalf("CreateTeam() unexpected error: %+v", appErr)
	}

	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	const memberTotal = 125
	for i := 0; i < memberTotal; i++ {
		_, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
			Username:    fmt.Sprintf("paged_team_member_%03d", i),
			EmployeeNo:  intPtr(3000 + i),
			DisplayName: fmt.Sprintf("Paged Team Member %03d", i),
			Department:  domain.Department("分页运营部"),
			Team:        "淘系二组",
			Mobile:      fmt.Sprintf("13877%06d", i),
			Password:    "Init12345",
			Roles:       []domain.Role{domain.RoleOps},
		})
		if appErr != nil {
			t.Fatalf("CreateManagedUser(%d) unexpected error: %+v", i, appErr)
		}
	}

	userRepo.listFilters = nil
	nextName := "淘系运营二部"
	updated, appErr := svc.UpdateTeam(context.Background(), UpdateOrgTeamParams{
		ID:   team.ID,
		Name: &nextName,
	})
	if appErr != nil {
		t.Fatalf("UpdateTeam(rename) unexpected error: %+v", appErr)
	}
	if updated.Name != nextName {
		t.Fatalf("UpdateTeam(rename) name = %q, want %q", updated.Name, nextName)
	}

	oldCount := 0
	newCount := 0
	for _, user := range userRepo.users {
		if user == nil || user.Department != domain.Department("分页运营部") {
			continue
		}
		switch user.Team {
		case "淘系二组":
			oldCount++
		case nextName:
			newCount++
		}
	}
	if oldCount != 0 || newCount != memberTotal {
		t.Fatalf("renamed users old=%d new=%d, want old=0 new=%d", oldCount, newCount, memberTotal)
	}
	if len(userRepo.listFilters) < 2 {
		t.Fatalf("rewriteAllUsers list calls = %d, want multiple pages", len(userRepo.listFilters))
	}
}

func TestIdentityServiceRenameTeamReclaimsDisabledEmptyConflict(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "回收运营部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	current, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "淘系二组"})
	if appErr != nil {
		t.Fatalf("CreateTeam(current) unexpected error: %+v", appErr)
	}
	stale, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "淘系运营二部"})
	if appErr != nil {
		t.Fatalf("CreateTeam(stale) unexpected error: %+v", appErr)
	}
	orgRepo.teams[stale.ID].Enabled = false

	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	for i := 0; i < 3; i++ {
		_, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
			Username:    fmt.Sprintf("reclaim_team_member_%d", i),
			EmployeeNo:  intPtr(3300 + i),
			DisplayName: fmt.Sprintf("Reclaim Team Member %d", i),
			Department:  domain.Department("回收运营部"),
			Team:        "淘系二组",
			Mobile:      fmt.Sprintf("13878%06d", i),
			Password:    "Init12345",
			Roles:       []domain.Role{domain.RoleOps},
		})
		if appErr != nil {
			t.Fatalf("CreateManagedUser(%d) unexpected error: %+v", i, appErr)
		}
	}

	nextName := "淘系运营二部"
	updated, appErr := svc.UpdateTeam(context.Background(), UpdateOrgTeamParams{
		ID:   current.ID,
		Name: &nextName,
	})
	if appErr != nil {
		t.Fatalf("UpdateTeam(rename into disabled empty conflict) unexpected error: %+v", appErr)
	}
	if updated.ID != current.ID || updated.Name != nextName {
		t.Fatalf("UpdateTeam(rename) updated = %+v, want id=%d name=%q", updated, current.ID, nextName)
	}
	if _, ok := orgRepo.teams[stale.ID]; ok {
		t.Fatalf("disabled empty conflict team id=%d still exists", stale.ID)
	}
	for _, user := range userRepo.users {
		if user == nil || user.Department != domain.Department("回收运营部") {
			continue
		}
		if user.Team != nextName {
			t.Fatalf("user %d team = %q, want %q", user.ID, user.Team, nextName)
		}
	}
}

func TestIdentityServiceRenameTeamRejectsDisabledConflictWithMembers(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "占用运营部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	current, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "淘系二组"})
	if appErr != nil {
		t.Fatalf("CreateTeam(current) unexpected error: %+v", appErr)
	}
	conflict, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "淘系运营二部"})
	if appErr != nil {
		t.Fatalf("CreateTeam(conflict) unexpected error: %+v", appErr)
	}

	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	_, appErr = svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "occupied_team_member",
		EmployeeNo:  intPtr(3400),
		DisplayName: "Occupied Team Member",
		Department:  domain.Department("占用运营部"),
		Team:        "淘系运营二部",
		Mobile:      "13879000000",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleOps},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(conflict member) unexpected error: %+v", appErr)
	}
	orgRepo.teams[conflict.ID].Enabled = false

	nextName := "淘系运营二部"
	updated, appErr := svc.UpdateTeam(context.Background(), UpdateOrgTeamParams{
		ID:   current.ID,
		Name: &nextName,
	})
	if appErr == nil {
		t.Fatalf("UpdateTeam(rename into occupied disabled conflict) appErr = nil, updated = %+v", updated)
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UpdateTeam(rename) code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("UpdateTeam(rename) details = %#v, want map", appErr.Details)
	}
	if details["deny_code"] != "team_name_conflict" {
		t.Fatalf("UpdateTeam(rename) deny_code = %v, want team_name_conflict", details["deny_code"])
	}
	if got := orgRepo.teams[current.ID].Name; got != "淘系二组" {
		t.Fatalf("current team name = %q, want unchanged", got)
	}
	if _, ok := orgRepo.teams[conflict.ID]; !ok {
		t.Fatalf("occupied disabled conflict team id=%d was deleted", conflict.ID)
	}
}

func TestIdentityServiceDisableTeamAndDepartmentMovesUsersToUnassignedPool(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "品牌二部"})
	team, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "品牌二组"})
	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() unexpected error: %+v", appErr)
	}
	opsTeam, ok := findDepartmentTeam(options, string(domain.DepartmentOperations))
	if !ok {
		t.Fatalf("missing operations team in options: %+v", options.Departments)
	}
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	user, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "brand_user_2",
		EmployeeNo:  intPtr(2102),
		DisplayName: "Brand User 2",
		Department:  domain.Department("品牌二部"),
		Team:        "品牌二组",
		Mobile:      "13800009902",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleOps},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser() unexpected error: %+v", appErr)
	}
	manager, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "brand_scope_manager",
		EmployeeNo:  intPtr(2103),
		DisplayName: "Brand Scope Manager",
		Department:  domain.DepartmentOperations,
		Team:        opsTeam,
		Mobile:      "13800009903",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(manager) unexpected error: %+v", appErr)
	}
	userRepo.users[manager.ID].ManagedDepartments = []string{"品牌二部", string(domain.DepartmentOperations)}
	userRepo.users[manager.ID].ManagedTeams = []string{"品牌二组", opsTeam}

	disabled := false
	if _, appErr := svc.UpdateTeam(context.Background(), UpdateOrgTeamParams{ID: team.ID, Enabled: &disabled}); appErr != nil {
		t.Fatalf("UpdateTeam(disable) appErr = %+v", appErr)
	}
	moved, appErr := svc.GetUser(context.Background(), user.ID)
	if appErr != nil {
		t.Fatalf("GetUser(after team disable) appErr = %+v", appErr)
	}
	if moved.Department != domain.DepartmentUnassigned || moved.Team != "未分配池" {
		t.Fatalf("UpdateTeam(disable) user = %+v, want unassigned pool", moved)
	}
	managerAfterTeamDisable, appErr := svc.GetUser(context.Background(), manager.ID)
	if appErr != nil {
		t.Fatalf("GetUser(manager after team disable) appErr = %+v", appErr)
	}
	if containsString(managerAfterTeamDisable.ManagedTeams, "品牌二组") {
		t.Fatalf("UpdateTeam(disable) manager scopes = %+v, want removed 品牌二组", managerAfterTeamDisable.ManagedTeams)
	}
	if _, appErr := svc.UpdateDepartment(context.Background(), UpdateOrgDepartmentParams{ID: department.ID, Enabled: &disabled}); appErr != nil {
		t.Fatalf("UpdateDepartment(disable) appErr = %+v", appErr)
	}
	managerAfterDepartmentDisable, appErr := svc.GetUser(context.Background(), manager.ID)
	if appErr != nil {
		t.Fatalf("GetUser(manager after department disable) appErr = %+v", appErr)
	}
	if containsString(managerAfterDepartmentDisable.ManagedDepartments, "品牌二部") {
		t.Fatalf("UpdateDepartment(disable) manager scopes = %+v, want removed 品牌二部", managerAfterDepartmentDisable.ManagedDepartments)
	}
}

func TestIdentityServiceRenameDepartmentWithAssignedUsersUsesSnapshot(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "云仓测试部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	team, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "默认组"})
	if appErr != nil {
		t.Fatalf("CreateTeam() unexpected error: %+v", appErr)
	}
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	user, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "warehouse_rename_user",
		EmployeeNo:  intPtr(2110),
		DisplayName: "Warehouse Rename User",
		Department:  domain.Department("云仓测试部"),
		Team:        team.Name,
		Mobile:      "13800009910",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleWarehouse},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(user) unexpected error: %+v", appErr)
	}
	manager, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "warehouse_rename_manager",
		EmployeeNo:  intPtr(2111),
		DisplayName: "Warehouse Rename Manager",
		Department:  domain.Department("云仓测试部"),
		Team:        team.Name,
		Mobile:      "13800009911",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(manager) unexpected error: %+v", appErr)
	}
	userRepo.users[manager.ID].ManagedDepartments = []string{"云仓测试部"}
	userRepo.listFilters = nil

	nextName := "定制中心"
	if _, appErr := svc.UpdateDepartment(context.Background(), UpdateOrgDepartmentParams{ID: department.ID, Name: &nextName}); appErr != nil {
		t.Fatalf("UpdateDepartment(rename) appErr = %+v", appErr)
	}
	renamed, appErr := svc.GetUser(context.Background(), user.ID)
	if appErr != nil {
		t.Fatalf("GetUser(user after rename) appErr = %+v", appErr)
	}
	if renamed.Department != domain.Department("定制中心") || renamed.Team != team.Name {
		t.Fatalf("renamed user = %+v, want department 定制中心 and same team", renamed)
	}
	managerAfterRename, appErr := svc.GetUser(context.Background(), manager.ID)
	if appErr != nil {
		t.Fatalf("GetUser(manager after rename) appErr = %+v", appErr)
	}
	if !containsString(managerAfterRename.ManagedDepartments, "定制中心") || containsString(managerAfterRename.ManagedDepartments, "云仓测试部") {
		t.Fatalf("manager scopes after rename = %+v, want replaced department scope", managerAfterRename.ManagedDepartments)
	}
	if got := countListCallsForDepartment(userRepo.listFilters, "云仓测试部"); got != 0 {
		t.Fatalf("department-scoped user list calls = %d, want 0 (rename runs a single full-table snapshot pass)", got)
	}
}

func TestIdentityServiceMergeDepartmentDoesNotRemoveSameNameManagedTeamOutsideSourceDepartment(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	sourceDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "源部门"})
	targetDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "目标部门"})
	otherDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "其他部门"})
	sourceTeam, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &sourceDept.ID, Name: "默认组"})
	if _, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &targetDept.ID, Name: "接收组"}); appErr != nil {
		t.Fatalf("CreateTeam(target) unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &otherDept.ID, Name: "默认组"}); appErr != nil {
		t.Fatalf("CreateTeam(other) unexpected error: %+v", appErr)
	}
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin}})
	sourceUser, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "merge_dept_source_user",
		EmployeeNo:  intPtr(2120),
		DisplayName: "Merge Dept Source User",
		Department:  domain.Department("源部门"),
		Team:        "默认组",
		Mobile:      "13800009920",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleOps},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(sourceUser) unexpected error: %+v", appErr)
	}
	sourceManager, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "merge_dept_source_manager",
		EmployeeNo:  intPtr(2121),
		DisplayName: "Merge Dept Source Manager",
		Department:  domain.Department("源部门"),
		Team:        "默认组",
		Mobile:      "13800009921",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin, domain.RoleTeamLead},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(sourceManager) unexpected error: %+v", appErr)
	}
	otherManager, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "merge_dept_other_manager",
		EmployeeNo:  intPtr(2122),
		DisplayName: "Merge Dept Other Manager",
		Department:  domain.Department("其他部门"),
		Team:        "默认组",
		Mobile:      "13800009922",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin, domain.RoleTeamLead},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(otherManager) unexpected error: %+v", appErr)
	}
	userRepo.users[sourceManager.ID].ManagedDepartments = []string{"源部门"}
	userRepo.users[sourceManager.ID].ManagedTeams = []string{"默认组"}
	userRepo.users[otherManager.ID].ManagedDepartments = []string{"其他部门"}
	userRepo.users[otherManager.ID].ManagedTeams = []string{"默认组"}

	if _, appErr := svc.MergeDepartment(context.Background(), MergeOrgDepartmentParams{SourceID: sourceDept.ID, TargetID: targetDept.ID}); appErr != nil {
		t.Fatalf("MergeDepartment() unexpected error: %+v", appErr)
	}
	moved, _ := svc.GetUser(context.Background(), sourceUser.ID)
	if moved.Department != domain.Department("目标部门") || moved.Team != "" {
		t.Fatalf("source user after merge = %+v, want target department and empty team", moved)
	}
	sourceManagerAfter, _ := svc.GetUser(context.Background(), sourceManager.ID)
	if !containsString(sourceManagerAfter.ManagedDepartments, "目标部门") || containsString(sourceManagerAfter.ManagedDepartments, "源部门") {
		t.Fatalf("source manager departments = %+v, want source replaced with target", sourceManagerAfter.ManagedDepartments)
	}
	if containsString(sourceManagerAfter.ManagedTeams, "默认组") {
		t.Fatalf("source manager teams = %+v, want source-only team removed", sourceManagerAfter.ManagedTeams)
	}
	otherManagerAfter, _ := svc.GetUser(context.Background(), otherManager.ID)
	if otherManagerAfter.Department != domain.Department("其他部门") || !containsString(otherManagerAfter.ManagedTeams, "默认组") {
		t.Fatalf("other manager after merge = %+v, want unrelated same-name team preserved", otherManagerAfter)
	}
	sourceAfter, _ := orgRepo.GetDepartmentByID(context.Background(), sourceDept.ID)
	if sourceAfter == nil || sourceAfter.Enabled {
		t.Fatalf("source department after merge = %+v, want disabled", sourceAfter)
	}
	sourceTeamAfter, _ := orgRepo.GetTeamByID(context.Background(), sourceTeam.ID)
	if sourceTeamAfter == nil || sourceTeamAfter.Enabled {
		t.Fatalf("source team after merge = %+v, want disabled", sourceTeamAfter)
	}
}

func TestIdentityServiceMergeTeamDoesNotRewriteSameNameManagedTeamOutsideSourceDepartment(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	sourceDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "源小组部门"})
	targetDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "目标小组部门"})
	otherDept, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "其他小组部门"})
	sourceTeam, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &sourceDept.ID, Name: "默认组"})
	targetTeam, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &targetDept.ID, Name: "接收组"})
	if _, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &otherDept.ID, Name: "默认组"}); appErr != nil {
		t.Fatalf("CreateTeam(other) unexpected error: %+v", appErr)
	}
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin}})
	sourceLead, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "merge_team_source_lead",
		EmployeeNo:  intPtr(2130),
		DisplayName: "Merge Team Source Lead",
		Department:  domain.Department("源小组部门"),
		Team:        "默认组",
		Mobile:      "13800009930",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleTeamLead},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(sourceLead) unexpected error: %+v", appErr)
	}
	otherLead, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "merge_team_other_lead",
		EmployeeNo:  intPtr(2131),
		DisplayName: "Merge Team Other Lead",
		Department:  domain.Department("其他小组部门"),
		Team:        "默认组",
		Mobile:      "13800009931",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleTeamLead},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(otherLead) unexpected error: %+v", appErr)
	}
	userRepo.users[sourceLead.ID].ManagedTeams = []string{"默认组"}
	userRepo.users[otherLead.ID].ManagedTeams = []string{"默认组"}

	if _, appErr := svc.MergeTeam(context.Background(), MergeOrgTeamParams{SourceID: sourceTeam.ID, TargetID: targetTeam.ID}); appErr != nil {
		t.Fatalf("MergeTeam() unexpected error: %+v", appErr)
	}
	sourceLeadAfter, _ := svc.GetUser(context.Background(), sourceLead.ID)
	if sourceLeadAfter.Department != domain.Department("目标小组部门") || sourceLeadAfter.Team != "接收组" {
		t.Fatalf("source lead after merge = %+v, want target team", sourceLeadAfter)
	}
	if !containsString(sourceLeadAfter.ManagedTeams, "接收组") || containsString(sourceLeadAfter.ManagedTeams, "默认组") {
		t.Fatalf("source lead managed teams = %+v, want source team replaced", sourceLeadAfter.ManagedTeams)
	}
	otherLeadAfter, _ := svc.GetUser(context.Background(), otherLead.ID)
	if otherLeadAfter.Department != domain.Department("其他小组部门") || !containsString(otherLeadAfter.ManagedTeams, "默认组") {
		t.Fatalf("other lead after merge = %+v, want unrelated same-name team preserved", otherLeadAfter)
	}
	sourceTeamAfter, _ := orgRepo.GetTeamByID(context.Background(), sourceTeam.ID)
	if sourceTeamAfter == nil || sourceTeamAfter.Enabled {
		t.Fatalf("source team after merge = %+v, want disabled", sourceTeamAfter)
	}
}

func TestIdentityServiceDeleteOrgRowsMovesMembersToUnassignedPool(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "待清理部门"})
	team, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "待清理小组"})
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	teamMember, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "cleanup_team_member",
		EmployeeNo:  intPtr(2210),
		DisplayName: "Cleanup Team Member",
		Department:  domain.Department("待清理部门"),
		Team:        "待清理小组",
		Mobile:      "13800002210",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(team member) unexpected error: %+v", appErr)
	}
	userRepo.users[teamMember.ID].ManagedTeams = []string{"待清理小组", "其他小组"}
	if appErr := svc.DeleteTeam(context.Background(), team.ID); appErr != nil {
		t.Fatalf("DeleteTeam(enabled with members) unexpected error: %+v", appErr)
	}
	if got, _ := orgRepo.GetTeamByID(context.Background(), team.ID); got != nil {
		t.Fatalf("team after delete = %+v, want nil", got)
	}
	movedTeamMember, appErr := svc.GetUser(context.Background(), teamMember.ID)
	if appErr != nil {
		t.Fatalf("GetUser(team member after delete) appErr = %+v", appErr)
	}
	if movedTeamMember.Department != domain.DepartmentUnassigned || movedTeamMember.Team != "未分配池" {
		t.Fatalf("DeleteTeam moved user = %+v, want unassigned pool", movedTeamMember)
	}
	if containsString(movedTeamMember.ManagedTeams, "待清理小组") {
		t.Fatalf("DeleteTeam managed teams = %+v, want removed 待清理小组", movedTeamMember.ManagedTeams)
	}

	departmentWithChildren, _ := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "待清理部门二"})
	childTeam, _ := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &departmentWithChildren.ID, Name: "待级联小组"})
	departmentMember, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "cleanup_department_member",
		EmployeeNo:  intPtr(2211),
		DisplayName: "Cleanup Department Member",
		Department:  domain.Department("待清理部门二"),
		Team:        "待级联小组",
		Mobile:      "13800002211",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleDeptAdmin},
	})
	if appErr != nil {
		t.Fatalf("CreateManagedUser(department member) unexpected error: %+v", appErr)
	}
	userRepo.users[departmentMember.ID].ManagedDepartments = []string{"待清理部门二", "其他部门"}
	userRepo.users[departmentMember.ID].ManagedTeams = []string{"待级联小组", "其他小组"}
	if appErr := svc.DeleteDepartment(context.Background(), departmentWithChildren.ID); appErr != nil {
		t.Fatalf("DeleteDepartment(enabled with members) unexpected error: %+v", appErr)
	}
	if got, _ := orgRepo.GetDepartmentByID(context.Background(), departmentWithChildren.ID); got != nil {
		t.Fatalf("department after delete = %+v, want nil", got)
	}
	if got, _ := orgRepo.GetTeamByID(context.Background(), childTeam.ID); got != nil {
		t.Fatalf("child team after department delete = %+v, want nil", got)
	}
	movedDepartmentMember, appErr := svc.GetUser(context.Background(), departmentMember.ID)
	if appErr != nil {
		t.Fatalf("GetUser(department member after delete) appErr = %+v", appErr)
	}
	if movedDepartmentMember.Department != domain.DepartmentUnassigned || movedDepartmentMember.Team != "未分配池" {
		t.Fatalf("DeleteDepartment moved user = %+v, want unassigned pool", movedDepartmentMember)
	}
	if containsString(movedDepartmentMember.ManagedDepartments, "待清理部门二") {
		t.Fatalf("DeleteDepartment managed departments = %+v, want removed 待清理部门二", movedDepartmentMember.ManagedDepartments)
	}
	if containsString(movedDepartmentMember.ManagedTeams, "待级联小组") {
		t.Fatalf("DeleteDepartment managed teams = %+v, want removed 待级联小组", movedDepartmentMember.ManagedTeams)
	}
}

func TestIdentityServiceOrgOptionsExposeMemberCounts(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}
	department, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "品牌计数部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment() unexpected error: %+v", appErr)
	}
	if _, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &department.ID, Name: "品牌计数一组"}); appErr != nil {
		t.Fatalf("CreateTeam() unexpected error: %+v", appErr)
	}

	// 120 人跨越两页(仓储分页上限 100),回归两处历史缺陷:
	// 1. collectOrgMemberCounts 只统计第一页;2. cloneOrgOptions 丢失 MemberCount。
	const memberTotal = 120
	for i := 1; i <= memberTotal; i++ {
		id := int64(9000 + i)
		userRepo.users[id] = &domain.User{
			ID:          id,
			Username:    fmt.Sprintf("count_user_%d", i),
			DisplayName: fmt.Sprintf("计数用户%d", i),
			Department:  domain.Department("品牌计数部"),
			Team:        "品牌计数一组",
			Status:      domain.UserStatusActive,
		}
	}

	options, appErr := svc.GetOrgOptionsIncludingDisabled(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptionsIncludingDisabled() unexpected error: %+v", appErr)
	}
	var deptFound, teamFound bool
	for _, dept := range options.Departments {
		if dept.Name != "品牌计数部" {
			continue
		}
		deptFound = true
		if dept.MemberCount != memberTotal {
			t.Fatalf("department member_count = %d, want %d", dept.MemberCount, memberTotal)
		}
		for _, item := range dept.TeamItems {
			if item.Name != "品牌计数一组" {
				continue
			}
			teamFound = true
			if item.MemberCount != memberTotal {
				t.Fatalf("team member_count = %d, want %d", item.MemberCount, memberTotal)
			}
		}
	}
	if !deptFound || !teamFound {
		t.Fatalf("options missing 品牌计数部/品牌计数一组: %+v", options.Departments)
	}
}

func TestIdentityServiceCreateTeamAllowsSameNameAcrossDepartments(t *testing.T) {
	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	svc := NewIdentityService(userRepo, &identitySessionRepoStub{}, &identityPermissionLogRepoStub{}, identityTxRunner{}, WithOrgRepo(orgRepo))

	firstDepartment, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "一部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment(first) unexpected error: %+v", appErr)
	}
	secondDepartment, appErr := svc.CreateDepartment(context.Background(), CreateOrgDepartmentParams{Name: "二部"})
	if appErr != nil {
		t.Fatalf("CreateDepartment(second) unexpected error: %+v", appErr)
	}

	firstTeam, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &firstDepartment.ID, Name: "默认组"})
	if appErr != nil {
		t.Fatalf("CreateTeam(first) unexpected error: %+v", appErr)
	}
	secondTeam, appErr := svc.CreateTeam(context.Background(), CreateOrgTeamParams{DepartmentID: &secondDepartment.ID, Name: "默认组"})
	if appErr != nil {
		t.Fatalf("CreateTeam(second) unexpected error: %+v", appErr)
	}

	if firstTeam.ID == secondTeam.ID {
		t.Fatalf("expected distinct team rows for duplicate names across departments, got first=%+v second=%+v", firstTeam, secondTeam)
	}
	if firstTeam.DepartmentID != firstDepartment.ID || secondTeam.DepartmentID != secondDepartment.ID {
		t.Fatalf("CreateTeam() department binding mismatch, first=%+v second=%+v", firstTeam, secondTeam)
	}
}

func TestIdentityServiceSyncConfiguredAuthSeedsDuplicateOfficialDefaultTeams(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	userRepo := newIdentityUserRepo()
	orgRepo := newIdentityOrgRepo()
	authSettings := domain.AuthSettings{
		Departments: []domain.Department{
			domain.DepartmentDesignRD,
			domain.DepartmentCustomizationArt,
			domain.DepartmentCloudWarehouse,
			domain.DepartmentUnassigned,
		},
		DepartmentTeams: map[string][]string{
			string(domain.DepartmentDesignRD):         {"默认组"},
			string(domain.DepartmentCustomizationArt): {"默认组"},
			string(domain.DepartmentCloudWarehouse):   {"默认组"},
			string(domain.DepartmentUnassigned):       {"未分配池"},
		},
		PhoneUnique: true,
		SuperAdmins: []domain.ConfiguredSuperAdmin{
			{
				Username:    "admin",
				DisplayName: "系统管理员",
				Department:  domain.DepartmentUnassigned,
				Team:        "未分配池",
				Mobile:      "13900000000",
				Password:    "ChangeMeAdmin123",
			},
		},
		UnassignedPoolEnabled: true,
	}
	svc := NewIdentityService(
		userRepo,
		&identitySessionRepoStub{},
		&identityPermissionLogRepoStub{},
		identityTxRunner{},
		WithOrgRepo(orgRepo),
		WithIdentitySettings(authSettings, defaultFrontendAccessSettings()),
	)

	if appErr := svc.SyncConfiguredAuth(context.Background()); appErr != nil {
		t.Fatalf("SyncConfiguredAuth() unexpected error: %+v", appErr)
	}

	options, appErr := svc.GetOrgOptions(context.Background())
	if appErr != nil {
		t.Fatalf("GetOrgOptions() unexpected error: %+v", appErr)
	}
	for _, department := range []string{
		string(domain.DepartmentDesignRD),
		string(domain.DepartmentCustomizationArt),
		string(domain.DepartmentCloudWarehouse),
	} {
		if !orgOptionsContainDepartmentTeam(options, department, "默认组") {
			t.Fatalf("GetOrgOptions() missing %s/默认组: %+v", department, options)
		}
	}

	teams, err := orgRepo.ListTeams(context.Background(), true)
	if err != nil {
		t.Fatalf("ListTeams() unexpected error: %v", err)
	}
	defaultGroupCount := 0
	for _, team := range teams {
		if team != nil && team.Name == "默认组" {
			defaultGroupCount++
		}
	}
	if defaultGroupCount != 3 {
		t.Fatalf("default team row count = %d, want 3 distinct department-scoped rows", defaultGroupCount)
	}
}

type identityOrgRepo struct {
	nextDepartmentID int64
	nextTeamID       int64
	departments      map[int64]*domain.OrgDepartment
	teams            map[int64]*domain.OrgTeam
}

func newIdentityOrgRepo() *identityOrgRepo {
	return &identityOrgRepo{
		nextDepartmentID: 1,
		nextTeamID:       1,
		departments:      map[int64]*domain.OrgDepartment{},
		teams:            map[int64]*domain.OrgTeam{},
	}
}

func (r *identityOrgRepo) ListDepartments(_ context.Context, includeDisabled bool) ([]*domain.OrgDepartment, error) {
	out := make([]*domain.OrgDepartment, 0, len(r.departments))
	for _, item := range r.departments {
		if item == nil || (!includeDisabled && !item.Enabled) {
			continue
		}
		copyItem := *item
		out = append(out, &copyItem)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *identityOrgRepo) ListTeams(_ context.Context, includeDisabled bool) ([]*domain.OrgTeam, error) {
	out := make([]*domain.OrgTeam, 0, len(r.teams))
	for _, item := range r.teams {
		if item == nil || (!includeDisabled && !item.Enabled) {
			continue
		}
		department := r.departments[item.DepartmentID]
		if department == nil || (!includeDisabled && !department.Enabled) {
			continue
		}
		copyItem := *item
		copyItem.Department = department.Name
		out = append(out, &copyItem)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *identityOrgRepo) GetDepartmentByID(_ context.Context, id int64) (*domain.OrgDepartment, error) {
	item := r.departments[id]
	if item == nil {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *identityOrgRepo) GetDepartmentByName(_ context.Context, name string) (*domain.OrgDepartment, error) {
	name = strings.TrimSpace(name)
	for _, item := range r.departments {
		if item != nil && item.Name == name {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, nil
}

func (r *identityOrgRepo) GetTeamByID(_ context.Context, id int64) (*domain.OrgTeam, error) {
	item := r.teams[id]
	if item == nil {
		return nil, nil
	}
	copyItem := *item
	if department := r.departments[item.DepartmentID]; department != nil {
		copyItem.Department = department.Name
	}
	return &copyItem, nil
}

func (r *identityOrgRepo) GetTeamByName(_ context.Context, name string) (*domain.OrgTeam, error) {
	name = strings.TrimSpace(name)
	for _, item := range r.teams {
		if item != nil && item.Name == name {
			copyItem := *item
			if department := r.departments[item.DepartmentID]; department != nil {
				copyItem.Department = department.Name
			}
			return &copyItem, nil
		}
	}
	return nil, nil
}

func (r *identityOrgRepo) GetTeamByDepartmentAndName(_ context.Context, departmentID int64, name string) (*domain.OrgTeam, error) {
	name = strings.TrimSpace(name)
	for _, item := range r.teams {
		if item != nil && item.DepartmentID == departmentID && item.Name == name {
			copyItem := *item
			if department := r.departments[item.DepartmentID]; department != nil {
				copyItem.Department = department.Name
			}
			return &copyItem, nil
		}
	}
	return nil, nil
}

func (r *identityOrgRepo) CreateDepartment(_ context.Context, _ repo.Tx, department *domain.OrgDepartment) (int64, error) {
	id := r.nextDepartmentID
	r.nextDepartmentID++
	now := time.Now().UTC()
	copyItem := &domain.OrgDepartment{
		ID:        id,
		Name:      strings.TrimSpace(department.Name),
		Enabled:   department.Enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.departments[id] = copyItem
	return id, nil
}

func (r *identityOrgRepo) UpdateDepartment(_ context.Context, _ repo.Tx, department *domain.OrgDepartment) error {
	if current := r.departments[department.ID]; current != nil {
		current.Name = strings.TrimSpace(department.Name)
		current.Enabled = department.Enabled
		current.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (r *identityOrgRepo) CreateTeam(_ context.Context, _ repo.Tx, team *domain.OrgTeam) (int64, error) {
	id := r.nextTeamID
	r.nextTeamID++
	now := time.Now().UTC()
	copyItem := &domain.OrgTeam{
		ID:           id,
		DepartmentID: team.DepartmentID,
		Name:         strings.TrimSpace(team.Name),
		Enabled:      team.Enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if department := r.departments[team.DepartmentID]; department != nil {
		copyItem.Department = department.Name
	}
	r.teams[id] = copyItem
	return id, nil
}

func (r *identityOrgRepo) UpdateTeam(_ context.Context, _ repo.Tx, team *domain.OrgTeam) error {
	if current := r.teams[team.ID]; current != nil {
		current.Name = strings.TrimSpace(team.Name)
		current.Enabled = team.Enabled
		current.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (r *identityOrgRepo) DeleteDepartment(_ context.Context, _ repo.Tx, id int64) error {
	delete(r.departments, id)
	return nil
}

func (r *identityOrgRepo) DeleteTeam(_ context.Context, _ repo.Tx, id int64) error {
	delete(r.teams, id)
	return nil
}

func (r *identityOrgRepo) DeleteTeamsByDepartment(_ context.Context, _ repo.Tx, departmentID int64) error {
	for id, team := range r.teams {
		if team != nil && team.DepartmentID == departmentID {
			delete(r.teams, id)
		}
	}
	return nil
}

func countListCallsForDepartment(filters []repo.UserListFilter, department string) int {
	count := 0
	for _, filter := range filters {
		if filter.Department == nil || string(*filter.Department) != department {
			continue
		}
		count++
	}
	return count
}

func orgOptionsContainDepartmentTeam(options *domain.OrgOptions, department, team string) bool {
	if options == nil {
		return false
	}
	for _, item := range options.Departments {
		if item.Name != department {
			continue
		}
		for _, candidate := range item.Teams {
			if candidate == team {
				return true
			}
		}
	}
	return false
}
