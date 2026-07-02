package search

import (
	"context"
	"strings"
	"sync"

	"workflow/domain"
	"workflow/repo"
)

const (
	CodeInvalidQuery = "invalid_query"
)

type Service struct {
	repo     repo.SearchRepo
	external ExternalAssetSearchProvider
}

func NewService(repo repo.SearchRepo) *Service {
	return &Service{repo: repo}
}

type ExternalAssetSearchProvider interface {
	SearchGlobal(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error)
}

func (s *Service) SetExternalAssetSearchProvider(provider ExternalAssetSearchProvider) {
	s.external = provider
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
		var externalAssets []domain.SearchAsset
		if err = runSearchJobs(
			func() error {
				rows, err := s.repo.SearchTasks(ctx, q, limit)
				result.Tasks = rows
				return err
			},
			func() error {
				rows, err := s.repo.SearchAssets(ctx, q, limit)
				result.Assets = rows
				return err
			},
			func() error {
				rows, err := s.searchExternalAssets(ctx, q, limit)
				externalAssets = rows
				return err
			},
			func() error {
				rows, err := s.repo.SearchProducts(ctx, q, limit)
				result.Products = rows
				return err
			},
			func() error {
				rows, err := s.searchUsers(ctx, actor, q, limit)
				result.Users = rows
				return err
			},
		); err != nil {
			return nil, internalErr(err)
		}
		result.Assets = append(result.Assets, externalAssets...)
	case "tasks":
		if result.Tasks, err = s.repo.SearchTasks(ctx, q, limit); err != nil {
			return nil, internalErr(err)
		}
	case "assets":
		var systemAssets []domain.SearchAsset
		var externalAssets []domain.SearchAsset
		if err = runSearchJobs(
			func() error {
				rows, err := s.repo.SearchAssets(ctx, q, limit)
				systemAssets = rows
				return err
			},
			func() error {
				rows, err := s.searchExternalAssets(ctx, q, limit)
				externalAssets = rows
				return err
			},
		); err != nil {
			return nil, internalErr(err)
		}
		result.Assets = append(systemAssets, externalAssets...)
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

func (s *Service) searchExternalAssets(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
	if s.external == nil {
		return []domain.SearchAsset{}, nil
	}
	items, err := s.external.SearchGlobal(ctx, q, limit)
	if err != nil {
		return []domain.SearchAsset{}, nil
	}
	return items, nil
}

func runSearchJobs(jobs ...func() error) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := job(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
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
