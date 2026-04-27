package notification

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const CodeNotificationNotOwner = "notification_not_owner"

type Broadcaster interface {
	BroadcastToUser(userID int64, event domain.WebSocketEvent)
}

type Service struct {
	notifications repo.NotificationRepo
	logs          repo.PermissionLogRepo
	hub           Broadcaster
	now           func() time.Time
	logger        *zap.Logger
}

type ListFilter struct {
	IsRead *bool
	Limit  int
	Cursor string
}

func NewService(notifications repo.NotificationRepo, logs repo.PermissionLogRepo, hub Broadcaster, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{notifications: notifications, logs: logs, hub: hub, now: time.Now, logger: logger}
}

func (s *Service) List(ctx context.Context, actor domain.RequestActor, filter ListFilter) ([]domain.Notification, string, *domain.AppError) {
	beforeTime, beforeID, appErr := decodeCursor(filter.Cursor)
	if appErr != nil {
		return nil, "", appErr
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, err := s.notifications.List(ctx, repo.NotificationListFilter{
		UserID:     actor.ID,
		IsRead:     filter.IsRead,
		Limit:      limit + 1,
		BeforeTime: beforeTime,
		BeforeID:   beforeID,
	})
	if err != nil {
		return nil, "", domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(last.CreatedAt, last.ID)
		items = items[:limit]
	}
	return items, next, nil
}

func (s *Service) MarkRead(ctx context.Context, actor domain.RequestActor, id int64) *domain.AppError {
	affected, err := s.notifications.MarkRead(ctx, id, actor.ID, s.now().UTC())
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if affected > 0 {
		return nil
	}
	n, err := s.notifications.Get(ctx, id)
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if n.UserID != actor.ID {
		s.recordDenied(ctx, actor, id)
		return domain.NewAppError(CodeNotificationNotOwner, "not the notification owner", nil)
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, actor domain.RequestActor) *domain.AppError {
	if _, err := s.notifications.MarkAllRead(ctx, actor.ID, s.now().UTC()); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return nil
}

func (s *Service) UnreadCount(ctx context.Context, actor domain.RequestActor) (int, *domain.AppError) {
	count, err := s.notifications.UnreadCount(ctx, actor.ID)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return count, nil
}

func (s *Service) CreateNotification(ctx context.Context, tx repo.Tx, userID int64, ntype domain.NotificationType, payload json.RawMessage) (*domain.Notification, error) {
	if !ntype.Valid() {
		s.logger.Warn("skip invalid notification type", zap.String("notification_type", string(ntype)))
		return nil, nil
	}
	if !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	n, err := s.notifications.Create(ctx, tx, &domain.Notification{UserID: userID, NotificationType: ntype, Payload: payload})
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		registerAfterCommit(tx, func() {
			unread, _ := s.notifications.UnreadCount(context.Background(), userID)
			s.hub.BroadcastToUser(userID, domain.NewWebSocketEvent(domain.WebSocketEventNotificationArrived, map[string]interface{}{
				"notification_id":   n.ID,
				"notification_type": string(n.NotificationType),
				"unread_count":      unread,
			}))
		})
	}
	return n, nil
}

func (s *Service) recordDenied(ctx context.Context, actor domain.RequestActor, id int64) {
	if s.logs == nil {
		return
	}
	now := s.now().UTC()
	_ = s.logs.Create(ctx, &domain.PermissionLog{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorSource:   actor.Source,
		AuthMode:      actor.AuthMode,
		Readiness:     domain.APIReadinessReadyForFrontend,
		ActionType:    "notification_access_denied",
		ActorRoles:    actor.Roles,
		Method:        "POST",
		RoutePath:     "/v1/me/notifications/{id}/read",
		Granted:       false,
		Reason:        fmt.Sprintf(`{"actor":%d,"notification_id":%d,"reason":"not_owner"}`, actor.ID, id),
		CreatedAt:     now,
	})
}

type afterCommitter interface{ AfterCommit(func()) }

func registerAfterCommit(tx repo.Tx, fn func()) {
	if c, ok := tx.(afterCommitter); ok {
		c.AfterCommit(fn)
	}
}

func encodeCursor(t time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%d", t.UnixMilli(), id)))
}

func decodeCursor(raw string) (*time.Time, int64, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	ms, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || id <= 0 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	t := time.UnixMilli(ms).UTC()
	return &t, id, nil
}
