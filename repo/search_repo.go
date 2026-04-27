package repo

import (
	"context"

	"workflow/domain"
)

type SearchRepo interface {
	SearchTasks(ctx context.Context, q string, limit int) ([]domain.SearchTask, error)
	SearchAssets(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error)
	SearchProducts(ctx context.Context, q string, limit int) ([]domain.SearchProduct, error)
	SearchUsers(ctx context.Context, q string, limit int) ([]domain.SearchUser, error)
}
