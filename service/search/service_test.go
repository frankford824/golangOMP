package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"workflow/domain"
)

type stubSearchRepo struct {
	tasksCalls, assetsCalls, productsCalls, usersCalls int
	limitSeen                                          int
	publishedOnly                                      bool
	accessSeen                                         domain.ResourceGroupAccessFilter
	taskAccessSeen                                     domain.ResourceGroupAccessFilter
}

func (s *stubSearchRepo) SearchTasks(context.Context, string, int) ([]domain.SearchTask, error) {
	s.tasksCalls++
	return []domain.SearchTask{{ID: 1, TaskNo: "T1"}}, nil
}
func (s *stubSearchRepo) SearchTasksScoped(_ context.Context, _ string, _ int, access domain.ResourceGroupAccessFilter) ([]domain.SearchTask, error) {
	s.tasksCalls++
	s.taskAccessSeen = access
	return []domain.SearchTask{{ID: 1, TaskNo: "T1"}}, nil
}
func (s *stubSearchRepo) SearchAssets(context.Context, string, int) ([]domain.SearchAsset, error) {
	s.assetsCalls++
	return []domain.SearchAsset{{AssetID: 1, FileName: "a.psd"}}, nil
}
func (s *stubSearchRepo) SearchResourceGroups(_ context.Context, _ string, _ int, publishedOnly bool, access domain.ResourceGroupAccessFilter) ([]domain.SearchAsset, error) {
	s.assetsCalls++
	s.publishedOnly = publishedOnly
	s.accessSeen = access
	return []domain.SearchAsset{{AssetID: 1, ResourceGroupID: 1, SourceType: "task_resource_group", FileName: "final.png"}}, nil
}
func (s *stubSearchRepo) SearchProducts(context.Context, string, int) ([]domain.SearchProduct, error) {
	s.productsCalls++
	return []domain.SearchProduct{{ERPCode: "SKU1", ProductName: "p"}}, nil
}

func TestSearchResourceGroupsUsesEffectiveScope(t *testing.T) {
	departmentID, teamID := int64(9), int64(12)
	tests := []struct {
		name       string
		assignment domain.AccessAssignment
		want       domain.ResourceGroupAccessFilter
	}{
		{name: "self", assignment: domain.AccessAssignment{RoleID: 5, ScopeMode: domain.AccessScopeSelf}, want: domain.ResourceGroupAccessFilter{ActorID: 41, Self: true}},
		{name: "own department", assignment: domain.AccessAssignment{RoleID: 5, ScopeMode: domain.AccessScopeOwnDepartment}, want: domain.ResourceGroupAccessFilter{ActorID: 41, DepartmentIDs: []int64{9}}},
		{name: "own team", assignment: domain.AccessAssignment{RoleID: 5, ScopeMode: domain.AccessScopeOwnTeam}, want: domain.ResourceGroupAccessFilter{ActorID: 41, TeamIDs: []int64{12}}},
		{name: "selected org", assignment: domain.AccessAssignment{RoleID: 5, ScopeMode: domain.AccessScopeSelectedOrg, Subjects: []domain.AccessScopeSubject{{SubjectType: domain.AccessSubjectDepartment, SubjectID: 3}, {SubjectType: domain.AccessSubjectTeam, SubjectID: 7}}}, want: domain.ResourceGroupAccessFilter{ActorID: 41, DepartmentIDs: []int64{3}, TeamIDs: []int64{7}}},
		{name: "global", assignment: domain.AccessAssignment{RoleID: 5, ScopeMode: domain.AccessScopeGlobal}, want: domain.ResourceGroupAccessFilter{ActorID: 41, Global: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repository := &stubSearchRepo{}
			actor := domain.RequestActor{
				ID: 41, DepartmentID: &departmentID, TeamID: &teamID,
				Permissions: []domain.PermissionCode{domain.PermissionAssetView},
				EffectiveAccess: &domain.EffectiveAccess{
					Permissions: []domain.PermissionCode{domain.PermissionAssetView},
					Assignments: []domain.AccessAssignment{tc.assignment},
					Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAssetView, RoleID: 5}},
				},
			}
			got, appErr := NewService(repository).Search(context.Background(), actor, "needle", "assets", 20)
			if appErr != nil || len(got.Assets) != 1 {
				t.Fatalf("Search() result=%+v error=%+v", got, appErr)
			}
			if !reflect.DeepEqual(repository.accessSeen, tc.want) {
				t.Fatalf("access=%+v want %+v", repository.accessSeen, tc.want)
			}
			if repository.publishedOnly {
				t.Fatal("internal actor unexpectedly used published-only search")
			}
		})
	}
}

func TestSearchResourceGroupsFailsClosedWithoutAssetScopeAndPublishesForSubmitter(t *testing.T) {
	repository := &stubSearchRepo{}
	got, appErr := NewService(repository).Search(context.Background(), domain.RequestActor{ID: 4}, "needle", "assets", 20)
	if appErr != nil || len(got.Assets) != 0 || repository.assetsCalls != 0 {
		t.Fatalf("unscoped result=%+v calls=%d error=%+v", got, repository.assetsCalls, appErr)
	}
	repository = &stubSearchRepo{}
	publishedActor := domain.RequestActor{
		ID:          8,
		Permissions: []domain.PermissionCode{domain.PermissionAssetView},
		EffectiveAccess: &domain.EffectiveAccess{
			Permissions: []domain.PermissionCode{domain.PermissionAssetView},
			Assignments: []domain.AccessAssignment{{RoleID: 14, RoleCode: "asset_submitter", ScopeMode: domain.AccessScopeSelf}},
			Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAssetView, RoleID: 14, RoleCode: "asset_submitter", SourceType: "direct", ScopeMode: domain.AccessScopeSelf}},
		},
	}
	got, appErr = NewService(repository).Search(context.Background(), publishedActor, "needle", "assets", 20)
	if appErr != nil || len(got.Assets) != 1 || !repository.publishedOnly {
		t.Fatalf("submitter result=%+v published=%v error=%+v", got, repository.publishedOnly, appErr)
	}
	repository = &stubSearchRepo{}
	got, appErr = NewService(repository).Search(context.Background(), actor(domain.RoleAssetSubmitter), "needle", "assets", 20)
	if appErr != nil || len(got.Assets) != 0 || repository.assetsCalls != 0 {
		t.Fatalf("legacy-only submitter leaked results=%+v calls=%d error=%+v", got, repository.assetsCalls, appErr)
	}
}

func TestSearchBranchesFailClosedByCapability(t *testing.T) {
	external := &countingExternalAssetSearch{}
	repository := &stubSearchRepo{}
	service := NewService(repository)
	service.SetExternalAssetSearchProvider(external)
	actor := domain.RequestActor{
		ID:              7,
		Permissions:     []domain.PermissionCode{domain.PermissionAccountUse},
		EffectiveAccess: &domain.EffectiveAccess{Assignments: []domain.AccessAssignment{}, Sources: []domain.EffectiveAccessNote{}},
	}
	got, appErr := service.Search(context.Background(), actor, "needle", "all", 20)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if repository.tasksCalls != 0 || repository.assetsCalls != 0 || repository.productsCalls != 0 || external.calls != 0 {
		t.Fatalf("unauthorized branches called: repo=%+v external=%d", repository, external.calls)
	}
	if len(got.Tasks) != 0 || len(got.Assets) != 0 || len(got.Products) != 0 {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestSearchBranchesUseTheirOwnCapabilities(t *testing.T) {
	external := &countingExternalAssetSearch{}
	repository := &stubSearchRepo{}
	service := NewService(repository)
	service.SetExternalAssetSearchProvider(external)
	actor := fullyScopedActor(18)
	got, appErr := service.Search(context.Background(), actor, "needle", "all", 20)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if repository.tasksCalls != 1 || repository.assetsCalls != 1 || repository.productsCalls != 1 || external.calls != 1 {
		t.Fatalf("authorized branches not called: repo=%+v external=%d", repository, external.calls)
	}
	if !repository.taskAccessSeen.Self || !repository.accessSeen.Self || len(got.Tasks) != 1 || len(got.Assets) != 2 || len(got.Products) != 1 {
		t.Fatalf("scoped results=%+v task_scope=%+v asset_scope=%+v", got, repository.taskAccessSeen, repository.accessSeen)
	}
}

type countingExternalAssetSearch struct{ calls int }

func (s *countingExternalAssetSearch) SearchGlobal(context.Context, string, int) ([]domain.SearchAsset, error) {
	s.calls++
	return []domain.SearchAsset{{AssetID: 99, ResourceID: "ext-99", SourceType: "external_asset", FileName: "external.png"}}, nil
}

func fullyScopedActor(id int64) domain.RequestActor {
	permissions := []domain.PermissionCode{domain.PermissionAccountUse, domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionCatalogView}
	assignments := []domain.AccessAssignment{{RoleID: 1, ScopeMode: domain.AccessScopeSelf}}
	sources := []domain.EffectiveAccessNote{
		{Permission: domain.PermissionTaskView, RoleID: 1},
		{Permission: domain.PermissionAssetView, RoleID: 1},
		{Permission: domain.PermissionCatalogView, RoleID: 1},
	}
	return domain.RequestActor{ID: id, Permissions: permissions, EffectiveAccess: &domain.EffectiveAccess{Permissions: permissions, Assignments: assignments, Sources: sources}}
}
func (s *stubSearchRepo) SearchUsers(_ context.Context, _ string, limit int) ([]domain.SearchUser, error) {
	s.usersCalls++
	s.limitSeen = limit
	return []domain.SearchUser{{UserID: 1, Username: "u"}}, nil
}

func TestSearchService(t *testing.T) {
	t.Run("empty q", func(t *testing.T) {
		_, appErr := NewService(&stubSearchRepo{}).Search(context.Background(), domain.RequestActor{}, "", "all", 20)
		if appErr == nil || appErr.Code != CodeInvalidQuery {
			t.Fatalf("appErr=%+v want %s", appErr, CodeInvalidQuery)
		}
	})
	t.Run("scope routing all", func(t *testing.T) {
		repo := &stubSearchRepo{}
		_, appErr := NewService(repo).Search(context.Background(), actor(domain.RoleSuperAdmin), "x", "all", 3)
		if appErr != nil {
			t.Fatal(appErr)
		}
		if repo.tasksCalls != 1 || repo.assetsCalls != 1 || repo.productsCalls != 1 || repo.usersCalls != 1 || repo.limitSeen != 3 {
			t.Fatalf("calls tasks=%d assets=%d products=%d users=%d limit=%d", repo.tasksCalls, repo.assetsCalls, repo.productsCalls, repo.usersCalls, repo.limitSeen)
		}
	})
	t.Run("specific scopes", func(t *testing.T) {
		for _, tc := range []struct {
			scope string
			want  string
		}{
			{"tasks", "tasks"}, {"assets", "assets"}, {"products", "products"}, {"users", "users"},
		} {
			repo := &stubSearchRepo{}
			_, appErr := NewService(repo).Search(context.Background(), actor(domain.RoleSuperAdmin), "x", tc.scope, 20)
			if appErr != nil {
				t.Fatalf("%s appErr=%+v", tc.scope, appErr)
			}
			if (tc.want == "tasks") != (repo.tasksCalls == 1) || (tc.want == "assets") != (repo.assetsCalls == 1) || (tc.want == "products") != (repo.productsCalls == 1) || (tc.want == "users") != (repo.usersCalls == 1) {
				t.Fatalf("%s calls=%+v", tc.scope, repo)
			}
		}
	})
	t.Run("low privilege users empty", func(t *testing.T) {
		repo := &stubSearchRepo{}
		got, appErr := NewService(repo).Search(context.Background(), actor(domain.RoleMember), "x", "users", 20)
		if appErr != nil {
			t.Fatal(appErr)
		}
		if repo.usersCalls != 0 || len(got.Users) != 0 {
			t.Fatalf("usersCalls=%d users=%v", repo.usersCalls, got.Users)
		}
	})
	t.Run("super and hr query users", func(t *testing.T) {
		for _, role := range []domain.Role{domain.RoleSuperAdmin, domain.RoleHRAdmin} {
			repo := &stubSearchRepo{}
			got, appErr := NewService(repo).Search(context.Background(), actor(role), "x", "users", 20)
			if appErr != nil || repo.usersCalls != 1 || len(got.Users) != 1 {
				t.Fatalf("role=%s calls=%d got=%+v err=%+v", role, repo.usersCalls, got, appErr)
			}
		}
	})
}

func TestSearchServiceRepoError(t *testing.T) {
	bad := &errorSearchRepo{}
	_, appErr := NewService(bad).Search(context.Background(), actor(domain.RoleSuperAdmin), "x", "tasks", 20)
	if appErr == nil || appErr.Code != domain.ErrCodeInternalError {
		t.Fatalf("appErr=%+v", appErr)
	}
}

func TestSearchServiceExternalAssetErrorDoesNotFailSearch(t *testing.T) {
	for _, scope := range []string{"all", "assets"} {
		repo := &stubSearchRepo{}
		svc := NewService(repo)
		svc.SetExternalAssetSearchProvider(errorExternalAssetSearch{})
		got, appErr := svc.Search(context.Background(), actor(domain.RoleSuperAdmin), "x", scope, 20)
		if appErr != nil {
			t.Fatalf("scope=%s appErr=%+v", scope, appErr)
		}
		if len(got.Assets) != 1 || got.Assets[0].SourceType == string(domain.AssetResourceSourceExternal) {
			t.Fatalf("scope=%s assets=%+v, want system asset only", scope, got.Assets)
		}
	}
}

type errorSearchRepo struct{ stubSearchRepo }

func (e *errorSearchRepo) SearchTasks(context.Context, string, int) ([]domain.SearchTask, error) {
	return nil, errors.New("boom")
}
func (e *errorSearchRepo) SearchTasksScoped(context.Context, string, int, domain.ResourceGroupAccessFilter) ([]domain.SearchTask, error) {
	return nil, errors.New("boom")
}

type errorExternalAssetSearch struct{}

func (errorExternalAssetSearch) SearchGlobal(context.Context, string, int) ([]domain.SearchAsset, error) {
	return nil, errors.New("external down")
}

func actor(role domain.Role) domain.RequestActor {
	result := domain.RequestActor{ID: 1, Roles: []domain.Role{role}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
	if role == domain.RoleSuperAdmin {
		result.Permissions = []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionCatalogView}
		result.EffectiveAccess = &domain.EffectiveAccess{
			Permissions: []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionCatalogView},
			Assignments: []domain.AccessAssignment{{RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
			Sources: []domain.EffectiveAccessNote{
				{Permission: domain.PermissionTaskView, RoleID: 1},
				{Permission: domain.PermissionAssetView, RoleID: 1},
				{Permission: domain.PermissionCatalogView, RoleID: 1},
			},
		}
	}
	return result
}
