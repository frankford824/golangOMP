package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type OrgMoveRequestFilter struct {
	State              *domain.OrgMoveRequestState
	UserID             *int64
	SourceDepartmentID *int64
	SourceDepartments  []string
	Page               int
	PageSize           int
}

type OrgMoveRequestRepo interface {
	Create(ctx context.Context, tx Tx, request *domain.OrgMoveRequest) (int64, error)
	Get(ctx context.Context, id int64) (*domain.OrgMoveRequest, error)
	List(ctx context.Context, filter OrgMoveRequestFilter) ([]*domain.OrgMoveRequest, int64, error)
	UpdateState(ctx context.Context, tx Tx, id int64, from, to domain.OrgMoveRequestState, decidedBy int64, reason string, decidedAt time.Time) (bool, error)
}
