package repo

import (
	"context"

	"workflow/domain"
)

type DesignSourceSearchFilter struct {
	Keyword string
	Page    int
	Size    int
}

type DesignSourceRepo interface {
	Search(ctx context.Context, filter DesignSourceSearchFilter) ([]domain.DesignSourceEntry, int, string, error)
}
