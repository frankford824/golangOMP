package search

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

const (
	CodeInvalidQuery = "invalid_query"
)

type Service struct {
	repo repo.SearchRepo
}

func NewService(repo repo.SearchRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Search(ctx context.Context, actor domain.RequestActor, q string, scope string, limit int) (*domain.SearchResultGroup, *domain.AppError) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, domain.NewAppError(CodeInvalidQuery, "q is required", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "all"
	}
	result := &domain.SearchResultGroup{
		Tasks:    []domain.SearchTask{},
		Assets:   []domain.SearchAsset{},
		Products: []domain.SearchProduct{},
		Users:    []domain.SearchUser{},
	}

	var err error
	switch scope {
	case "all":
		if result.Tasks, err = s.repo.SearchTasks(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
		if result.Assets, err = s.repo.SearchAssets(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
		if result.Products, err = s.repo.SearchProducts(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
		result.Users, err = s.searchUsers(ctx, actor, q, limit)
		if err != nil {
			return nil, internalErr(err)
		}
	case "tasks":
		if result.Tasks, err = s.repo.SearchTasks(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "assets":
		if result.Assets, err = s.repo.SearchAssets(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "products":
		if result.Products, err = s.repo.SearchProducts(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "users":
		result.Users, err = s.searchUsers(ctx, actor, q, limit)
		if err != nil {
			return nil, internalErr(err)
		}
	default:
		return nil, domain.NewAppError(CodeInvalidQuery, "invalid scope", nil)
	}
	normalizeNilSlices(result)
	return result, nil
}

func (s *Service) searchUsers(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.SearchUser, error) {
	if !hasRole(actor, domain.RoleSuperAdmin, domain.RoleHRAdmin) {
		return []domain.SearchUser{}, nil
	}
	return s.repo.SearchUsers(ctx, q, limit)
}

func hasRole(actor domain.RequestActor, roles ...domain.Role) bool {
	for _, actorRole := range actor.Roles {
		for _, role := range roles {
			if actorRole == role {
				return true
			}
		}
	}
	return false
}

func normalizeNilSlices(result *domain.SearchResultGroup) {
	if result.Tasks == nil {
		result.Tasks = []domain.SearchTask{}
	}
	if result.Assets == nil {
		result.Assets = []domain.SearchAsset{}
	}
	if result.Products == nil {
		result.Products = []domain.SearchProduct{}
	}
	if result.Users == nil {
		result.Users = []domain.SearchUser{}
	}
}

func internalErr(err error) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
}
