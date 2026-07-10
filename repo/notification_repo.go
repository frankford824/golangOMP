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
	Scope      NotificationScope
}

type NotificationScope struct {
	TypePrefix string
	Exclude    bool
}

type NotificationRepo interface {
	Create(ctx context.Context, tx Tx, notification *domain.Notification) (*domain.Notification, error)
	Get(ctx context.Context, id int64) (*domain.Notification, error)
	List(ctx context.Context, filter NotificationListFilter) ([]domain.Notification, error)
	MarkRead(ctx context.Context, id, userID int64, at time.Time) (int64, error)
	MarkReadScoped(ctx context.Context, id, userID int64, at time.Time, scope NotificationScope) (int64, error)
	MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error)
	MarkAllReadScoped(ctx context.Context, userID int64, at time.Time, scope NotificationScope) (int64, error)
	UnreadCount(ctx context.Context, userID int64) (int, error)
	UnreadCountScoped(ctx context.Context, userID int64, scope NotificationScope) (int, error)

	UpsertWebPushSubscription(ctx context.Context, tx Tx, sub *domain.WebPushSubscription) (*domain.WebPushSubscription, error)
	DisableWebPushSubscriptionByEndpointHash(ctx context.Context, userID int64, endpointHash string, at time.Time, reason string) (int64, error)
	DisableWebPushSubscriptionByID(ctx context.Context, id int64, at time.Time, reason string) error
	MarkStaleWebPushSubscriptionsByUserExceptKeyHash(ctx context.Context, userID int64, vapidKeyHash string, at time.Time) (int64, error)
	ListActiveWebPushSubscriptionsForUser(ctx context.Context, tx Tx, userID int64, vapidKeyHash string) ([]domain.WebPushSubscription, error)
	GetNotificationPreference(ctx context.Context, userID int64) (*domain.NotificationPreference, error)
	UpsertNotificationPreference(ctx context.Context, tx Tx, pref *domain.NotificationPreference) error
	EnqueueNotificationDelivery(ctx context.Context, tx Tx, delivery *domain.NotificationDeliveryOutbox) error
	ClaimWebPushDeliveries(ctx context.Context, limit int, claimToken string, leaseUntil time.Time, now time.Time) ([]domain.NotificationDeliveryOutbox, error)
	MarkWebPushDeliverySent(ctx context.Context, id int64, claimToken string, at time.Time) error
	MarkWebPushDeliveryRetry(ctx context.Context, id int64, claimToken string, nextAttemptAt time.Time, lastError string, providerStatusCode *int, at time.Time) error
	MarkWebPushDeliveryDead(ctx context.Context, id int64, claimToken string, lastError string, providerStatusCode *int, at time.Time) error
	TryCreateNotificationDedupeClaim(ctx context.Context, tx Tx, claim domain.NotificationDedupeClaim) (bool, error)
	UpdateNotificationDedupeClaimNotificationID(ctx context.Context, tx Tx, claim domain.NotificationDedupeClaim, notificationID int64) error
	ClearNotificationDedupeScope(ctx context.Context, scope string) error
	ListRecentTaskFilingFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error)
	ListRecentProductBaseSyncFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error)
}
