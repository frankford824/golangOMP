package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type TaskDraftListFilter struct {
	OwnerUserID int64
	TaskType    string
	Limit       int
	BeforeTime  *time.Time
	BeforeID    int64
}

type TaskDraftRepo interface {
	Create(ctx context.Context, tx Tx, draft *domain.TaskDraft) (*domain.TaskDraft, error)
	Update(ctx context.Context, tx Tx, draft *domain.TaskDraft) (*domain.TaskDraft, error)
	Get(ctx context.Context, draftID int64) (*domain.TaskDraft, error)
	GetForUpdate(ctx context.Context, tx Tx, draftID int64) (*domain.TaskDraft, error)
	List(ctx context.Context, filter TaskDraftListFilter) ([]domain.TaskDraftListItem, error)
	Delete(ctx context.Context, tx Tx, draftID int64) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
	CountByOwnerAndType(ctx context.Context, tx Tx, ownerUserID int64, taskType string) (int, error)
	DeleteOldestByOwnerAndType(ctx context.Context, tx Tx, ownerUserID int64, taskType string) error
}
