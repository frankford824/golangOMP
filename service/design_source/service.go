package design_source

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type Service struct {
	repo repo.DesignSourceRepo
}

func NewService(repo repo.DesignSourceRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Search(ctx context.Context, actor domain.RequestActor, keyword string, page, size int) ([]domain.DesignSourceEntry, int, *domain.AppError) {
	_ = actor
	if page <= 0 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be >= 1", nil)
	}
	if size <= 0 || size > 100 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "size must be between 1 and 100", nil)
	}
	items, total, _, err := s.repo.Search(ctx, repo.DesignSourceSearchFilter{
		Keyword: strings.TrimSpace(keyword),
		Page:    page,
		Size:    size,
	})
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return items, total, nil
}
