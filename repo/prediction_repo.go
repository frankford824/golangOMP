package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type PredictionRepo interface {
	SearchSuggestions(ctx context.Context, actor domain.RequestActor, q, scope string, limit int) ([]domain.PredictionSuggestion, error)
	TaskCreateSuggestions(ctx context.Context, actor domain.RequestActor, keyword, taskType string, limit int) ([]domain.PredictionSuggestion, error)
	TaskNextActionSuggestions(ctx context.Context, actor domain.RequestActor, taskID int64, limit int) ([]domain.PredictionSuggestion, error)
	AssetSuggestions(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.PredictionSuggestion, error)
	ManagementSuggestions(ctx context.Context, actor domain.RequestActor, from, to time.Time, limit int) ([]domain.PredictionSuggestion, error)
}
