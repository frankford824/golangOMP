package service

import (
	"context"
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

func TestIdentityServiceDisableTeamAndDepartmentRequiresUnassignedUsers(t *testing.T) {
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
	adminCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleAdmin, domain.RoleHRAdmin},
	})
	if _, appErr := svc.CreateManagedUser(adminCtx, CreateManagedUserParams{
		Username:    "brand_user_2",
		DisplayName: "Brand User 2",
		Department:  domain.Department("品牌二部"),
		Team:        "品牌二组",
		Mobile:      "13800009902",
		Password:    "Init12345",
		Roles:       []domain.Role{domain.RoleOps},
	}); appErr != nil {
		t.Fatalf("CreateManagedUser() unexpected error: %+v", appErr)
	}

	disabled := false
	if _, appErr := svc.UpdateTeam(context.Background(), UpdateOrgTeamParams{ID: team.ID, Enabled: &disabled}); appErr == nil || appErr.Message != "team still has assigned users" {
		t.Fatalf("UpdateTeam(disable) appErr = %+v", appErr)
	}
	if _, appErr := svc.UpdateDepartment(context.Background(), UpdateOrgDepartmentParams{ID: department.ID, Enabled: &disabled}); appErr == nil || appErr.Message != "department still has assigned users" {
		t.Fatalf("UpdateDepartment(disable) appErr = %+v", appErr)
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
		current.Enabled = team.Enabled
		current.UpdatedAt = time.Now().UTC()
	}
	return nil
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
