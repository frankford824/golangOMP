package prediction

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type Service struct {
	repo repo.PredictionRepo
}

func NewService(repo repo.PredictionRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) SearchSuggestions(ctx context.Context, actor domain.RequestActor, q, scope string, limit int) (*domain.PredictionBundle, *domain.AppError) {
	if s == nil || s.repo == nil {
		return emptyBundle(), nil
	}
	items, err := s.repo.SearchSuggestions(ctx, actor, strings.TrimSpace(q), strings.TrimSpace(scope), normalizeLimit(limit))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return bundle(items), nil
}

func (s *Service) TaskCreateSuggestions(ctx context.Context, actor domain.RequestActor, keyword, taskType string, limit int) (*domain.PredictionBundle, *domain.AppError) {
	if s == nil || s.repo == nil {
		return emptyBundle(), nil
	}
	items, err := s.repo.TaskCreateSuggestions(ctx, actor, strings.TrimSpace(keyword), strings.TrimSpace(taskType), normalizeLimit(limit))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return bundle(items), nil
}

func (s *Service) TaskNextActionSuggestions(ctx context.Context, actor domain.RequestActor, taskID int64, limit int) (*domain.PredictionBundle, *domain.AppError) {
	if s == nil || s.repo == nil {
		return emptyBundle(), nil
	}
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil)
	}
	items, err := s.repo.TaskNextActionSuggestions(ctx, actor, taskID, normalizeLimit(limit))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return bundle(items), nil
}

func (s *Service) AssetSuggestions(ctx context.Context, actor domain.RequestActor, q string, limit int) (*domain.PredictionBundle, *domain.AppError) {
	if s == nil || s.repo == nil {
		return emptyBundle(), nil
	}
	items, err := s.repo.AssetSuggestions(ctx, actor, strings.TrimSpace(q), normalizeLimit(limit))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return bundle(items), nil
}

func (s *Service) ManagementSuggestions(ctx context.Context, actor domain.RequestActor, from, to time.Time, limit int) (*domain.PredictionBundle, *domain.AppError) {
	if s == nil || s.repo == nil {
		return emptyBundle(), nil
	}
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -7)
	}
	if to.IsZero() {
		to = time.Now()
	}
	if from.After(to) {
		return nil, domain.NewAppError("invalid_date_range", "from must be before or equal to to", nil)
	}
	items, err := s.repo.ManagementSuggestions(ctx, actor, from, to, normalizeLimit(limit))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return bundle(items), nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func bundle(items []domain.PredictionSuggestion) *domain.PredictionBundle {
	if items == nil {
		items = []domain.PredictionSuggestion{}
	}
	return &domain.PredictionBundle{
		Suggestions: items,
		GeneratedAt: time.Now().UTC(),
	}
}

func emptyBundle() *domain.PredictionBundle {
	return bundle([]domain.PredictionSuggestion{})
}
