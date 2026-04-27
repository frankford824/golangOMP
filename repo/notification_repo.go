package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type NotificationListFilter struct {
	UserID     int64
	IsRead     *bool
	Limit      int
	BeforeTime *time.Time
	BeforeID   int64
}

type NotificationRepo interface {
	Create(ctx context.Context, tx Tx, notification *domain.Notification) (*domain.Notification, error)
	Get(ctx context.Context, id int64) (*domain.Notification, error)
	List(ctx context.Context, filter NotificationListFilter) ([]domain.Notification, error)
	MarkRead(ctx context.Context, id, userID int64, at time.Time) (int64, error)
	MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error)
	UnreadCount(ctx context.Context, userID int64) (int, error)
}
