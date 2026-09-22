package repo

import (
	"context"
	"workflow/domain"
)

type CostBoundSKUReader interface {
	ListBoundSKUs(context.Context, CostRuleBindingListFilter) ([]*domain.CostBoundSKU, int64, error)
}
