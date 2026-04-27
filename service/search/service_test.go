package search

import (
	"context"
	"errors"
	"testing"

	"workflow/domain"
)

type stubSearchRepo struct {
	tasksCalls, assetsCalls, productsCalls, usersCalls int
	limitSeen                                          int
}

func (s *stubSearchRepo) SearchTasks(context.Context, string, int) ([]domain.SearchTask, error) {
	s.tasksCalls++
	return []domain.SearchTask{{ID: 1, TaskNo: "T1"}}, nil
}
func (s *stubSearchRepo) SearchAssets(context.Context, string, int) ([]domain.SearchAsset, error) {
	s.assetsCalls++
	return []domain.SearchAsset{{AssetID: 1, FileName: "a.psd"}}, nil
}
func (s *stubSearchRepo) SearchProducts(context.Context, string, int) ([]domain.SearchProduct, error) {
	s.productsCalls++
	return []domain.SearchProduct{{ERPCode: "SKU1", ProductName: "p"}}, nil
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

type errorSearchRepo struct{ stubSearchRepo }

func (e *errorSearchRepo) SearchTasks(context.Context, string, int) ([]domain.SearchTask, error) {
	return nil, errors.New("boom")
}

func actor(role domain.Role) domain.RequestActor {
	return domain.RequestActor{ID: 1, Roles: []domain.Role{role}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
}
