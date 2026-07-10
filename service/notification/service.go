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

var (
	mainOpsNotificationScope = repo.NotificationScope{
		TypePrefix: domain.NotificationTypeAssetWorkbenchNotificationPrefix,
		Exclude:    true,
	}
	assetWorkbenchNotificationScope = repo.NotificationScope{
		TypePrefix: domain.NotificationTypeAssetWorkbenchNotificationPrefix,
	}
)

type Broadcaster interface {
	BroadcastToUser(userID int64, event domain.WebSocketEvent)
}

type Service struct {
	notifications repo.NotificationRepo
	users         repo.UserRepo
	tasks         repo.TaskRepo
	txRunner      repo.TxRunner
	logs          repo.PermissionLogRepo
	hub           Broadcaster
	external      ExternalNotifier
	webPush       WebPushConfig
	now           func() time.Time
	logger        *zap.Logger
}

type ExternalNotifier interface {
	Notify(ctx context.Context, n domain.Notification)
}

type ServiceOption func(*Service)

func WithUserRepo(users repo.UserRepo) ServiceOption {
	return func(s *Service) {
		s.users = users
	}
}

func WithTaskRepo(tasks repo.TaskRepo) ServiceOption {
	return func(s *Service) {
		s.tasks = tasks
	}
}

func WithTxRunner(txRunner repo.TxRunner) ServiceOption {
	return func(s *Service) {
		s.txRunner = txRunner
	}
}

func WithExternalNotifier(notifier ExternalNotifier) ServiceOption {
	return func(s *Service) {
		s.external = notifier
	}
}

type WebPushConfig struct {
	Enabled     bool
	PublicKey   string
	PrivateKey  string
	Subject     string
	KeyHash     string
	LeaseTTL    time.Duration
	RetryBase   time.Duration
	MaxAttempts int
}

func WithWebPushConfig(config WebPushConfig) ServiceOption {
	return func(s *Service) {
		if strings.TrimSpace(config.KeyHash) == "" {
			config.KeyHash = PublicKeyHash(config.PublicKey)
		}
		if config.LeaseTTL <= 0 {
			config.LeaseTTL = 2 * time.Minute
		}
		if config.RetryBase <= 0 {
			config.RetryBase = 30 * time.Second
		}
		if config.MaxAttempts <= 0 {
			config.MaxAttempts = 5
		}
		s.webPush = config
	}
}

type ListFilter struct {
	IsRead *bool
	Limit  int
	Cursor string
}

type BroadcastAudience string

const (
	BroadcastAudienceAll   BroadcastAudience = "all"
	BroadcastAudienceUsers BroadcastAudience = "users"
)

type BroadcastParams struct {
	Audience BroadcastAudience
	UserIDs  []int64
	Title    string
	Content  string
}

type BroadcastResult struct {
	NotificationCount int     `json:"notification_count"`
	RecipientCount    int     `json:"recipient_count"`
	RecipientIDs      []int64 `json:"recipient_ids"`
}

func NewService(notifications repo.NotificationRepo, logs repo.PermissionLogRepo, hub Broadcaster, logger *zap.Logger, opts ...ServiceOption) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	svc := &Service{notifications: notifications, logs: logs, hub: hub, now: time.Now, logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *Service) Broadcast(ctx context.Context, actor domain.RequestActor, params BroadcastParams) (*BroadcastResult, *domain.AppError) {
	if !canBroadcastNotifications(actor) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "notification broadcast requires admin role", map[string]interface{}{
			"deny_code": "notification_broadcast_forbidden",
		})
	}
	if s == nil || s.notifications == nil || s.users == nil || s.txRunner == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "notification broadcast service is not configured", nil)
	}
	title := strings.TrimSpace(params.Title)
	content := strings.TrimSpace(params.Content)
	if title == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "title is required", map[string]interface{}{"deny_code": "notification_title_required"})
	}
	if content == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "content is required", map[string]interface{}{"deny_code": "notification_content_required"})
	}
	if len([]rune(title)) > 80 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "title must be 80 characters or fewer", map[string]interface{}{"deny_code": "notification_title_too_long"})
	}
	if len([]rune(content)) > 1000 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "content must be 1000 characters or fewer", map[string]interface{}{"deny_code": "notification_content_too_long"})
	}

	recipients, appErr := s.resolveBroadcastRecipients(ctx, actor, params)
	if appErr != nil {
		return nil, appErr
	}
	if len(recipients) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "no active recipients selected", map[string]interface{}{"deny_code": "notification_recipients_empty"})
	}
	audience := params.Audience
	if audience == "" {
		audience = BroadcastAudienceUsers
	}
	payload := mustRaw(map[string]interface{}{
		"broadcast_id":              fmt.Sprintf("broadcast_%d_%d", actor.ID, s.now().UTC().UnixNano()),
		"title":                     title,
		"content":                   content,
		"broadcast_by":              actor.ID,
		"broadcast_by_name":         strings.TrimSpace(actor.Username),
		"broadcast_audience":        string(audience),
		"broadcast_recipient_count": len(recipients),
	})
	created := 0
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		for _, userID := range recipients {
			if _, err := s.CreateNotification(ctx, tx, userID, domain.NotificationTypeSystemBroadcast, payload); err != nil {
				return err
			}
			created++
		}
		return nil
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return &BroadcastResult{NotificationCount: created, RecipientCount: len(recipients), RecipientIDs: recipients}, nil
}

func (s *Service) resolveBroadcastRecipients(ctx context.Context, actor domain.RequestActor, params BroadcastParams) ([]int64, *domain.AppError) {
	switch params.Audience {
	case BroadcastAudienceAll:
		if !canBroadcastAllNotifications(actor) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "broadcast to all users requires super admin, admin, or HR admin", map[string]interface{}{
				"deny_code": "notification_broadcast_all_forbidden",
			})
		}
		return s.listAllActiveUserIDs(ctx)
	case BroadcastAudienceUsers, "":
		ids := uniquePositiveInt64s(params.UserIDs)
		if len(ids) == 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_ids is required for selected-user broadcast", map[string]interface{}{"deny_code": "notification_user_ids_required"})
		}
		if len(ids) > 200 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_ids cannot exceed 200 recipients", map[string]interface{}{"deny_code": "notification_user_ids_too_many"})
		}
		return s.activeUserIDsByIDs(ctx, ids)
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "audience must be all or users", map[string]interface{}{"deny_code": "notification_audience_invalid"})
	}
}

func (s *Service) listAllActiveUserIDs(ctx context.Context) ([]int64, *domain.AppError) {
	var out []int64
	for page := 1; ; page++ {
		users, total, err := s.users.List(ctx, repo.UserListFilter{Status: userStatusPtr(domain.UserStatusActive), Page: page, PageSize: 100})
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		for _, user := range users {
			if user != nil && user.ID > 0 {
				out = append(out, user.ID)
			}
		}
		if int64(len(out)) >= total || len(users) == 0 {
			return uniquePositiveInt64s(out), nil
		}
	}
}

func (s *Service) activeUserIDsByIDs(ctx context.Context, ids []int64) ([]int64, *domain.AppError) {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		user, err := s.users.GetByID(ctx, id)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
		}
		if user == nil || user.Status != domain.UserStatusActive {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "recipient user is not active or does not exist", map[string]interface{}{
				"deny_code": "notification_recipient_not_active",
				"user_id":   id,
			})
		}
		out = append(out, id)
	}
	return uniquePositiveInt64s(out), nil
}

func (s *Service) List(ctx context.Context, actor domain.RequestActor, filter ListFilter) ([]domain.Notification, string, *domain.AppError) {
	return s.listScoped(ctx, actor, filter, mainOpsNotificationScope)
}

func (s *Service) ListAssetWorkbench(ctx context.Context, actor domain.RequestActor, filter ListFilter) ([]domain.Notification, string, *domain.AppError) {
	return s.listScoped(ctx, actor, filter, assetWorkbenchNotificationScope)
}

func (s *Service) listScoped(ctx context.Context, actor domain.RequestActor, filter ListFilter, scope repo.NotificationScope) ([]domain.Notification, string, *domain.AppError) {
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
		Scope:      scope,
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
	return s.markReadScoped(ctx, actor, id, mainOpsNotificationScope)
}

func (s *Service) MarkAssetWorkbenchRead(ctx context.Context, actor domain.RequestActor, id int64) *domain.AppError {
	return s.markReadScoped(ctx, actor, id, assetWorkbenchNotificationScope)
}

func (s *Service) markReadScoped(ctx context.Context, actor domain.RequestActor, id int64, scope repo.NotificationScope) *domain.AppError {
	affected, err := s.notifications.MarkReadScoped(ctx, id, actor.ID, s.now().UTC(), scope)
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
	if !notificationMatchesScope(n.NotificationType, scope) {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, actor domain.RequestActor) *domain.AppError {
	return s.markAllReadScoped(ctx, actor, mainOpsNotificationScope)
}

func (s *Service) MarkAllAssetWorkbenchRead(ctx context.Context, actor domain.RequestActor) *domain.AppError {
	return s.markAllReadScoped(ctx, actor, assetWorkbenchNotificationScope)
}

func (s *Service) markAllReadScoped(ctx context.Context, actor domain.RequestActor, scope repo.NotificationScope) *domain.AppError {
	if _, err := s.notifications.MarkAllReadScoped(ctx, actor.ID, s.now().UTC(), scope); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return nil
}

func (s *Service) UnreadCount(ctx context.Context, actor domain.RequestActor) (int, *domain.AppError) {
	return s.unreadCountScoped(ctx, actor, mainOpsNotificationScope)
}

func (s *Service) AssetWorkbenchUnreadCount(ctx context.Context, actor domain.RequestActor) (int, *domain.AppError) {
	return s.unreadCountScoped(ctx, actor, assetWorkbenchNotificationScope)
}

func (s *Service) unreadCountScoped(ctx context.Context, actor domain.RequestActor, scope repo.NotificationScope) (int, *domain.AppError) {
	count, err := s.notifications.UnreadCountScoped(ctx, actor.ID, scope)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return count, nil
}

func notificationMatchesScope(ntype domain.NotificationType, scope repo.NotificationScope) bool {
	prefix := strings.TrimSpace(scope.TypePrefix)
	if prefix == "" {
		return true
	}
	matches := strings.HasPrefix(string(ntype), prefix)
	if scope.Exclude {
		return !matches
	}
	return matches
}

func (s *Service) CreateNotification(ctx context.Context, tx repo.Tx, userID int64, ntype domain.NotificationType, payload json.RawMessage) (*domain.Notification, error) {
	if !ntype.Valid() {
		s.logger.Warn("skip invalid notification type", zap.String("notification_type", string(ntype)))
		return nil, nil
	}
	if !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	payload = s.enrichPayload(ctx, userID, ntype, payload)
	n, err := s.notifications.Create(ctx, tx, &domain.Notification{UserID: userID, NotificationType: ntype, Payload: payload})
	if err != nil {
		return nil, err
	}
	if err := s.enqueueWebPushDeliveries(ctx, tx, n); err != nil {
		return nil, err
	}
	if s.hub != nil || s.external != nil {
		registerAfterCommit(tx, func() {
			if s.hub != nil {
				scope := mainOpsNotificationScope
				if n.NotificationType.IsAssetWorkbench() {
					scope = assetWorkbenchNotificationScope
				}
				unread, _ := s.notifications.UnreadCountScoped(context.Background(), userID, scope)
				s.hub.BroadcastToUser(userID, domain.NewWebSocketEvent(domain.WebSocketEventNotificationArrived, map[string]interface{}{
					"notification_id":   n.ID,
					"notification_type": string(n.NotificationType),
					"unread_count":      unread,
					"scope":             map[bool]string{true: "asset_workbench", false: "main_ops"}[n.NotificationType.IsAssetWorkbench()],
				}))
			}
			if s.external != nil {
				s.external.Notify(context.Background(), *n)
			}
		})
	}
	return n, nil
}

func (s *Service) enrichPayload(ctx context.Context, userID int64, ntype domain.NotificationType, payload json.RawMessage) json.RawMessage {
	p := payloadMap(payload)
	taskID := payloadInt64(p, "task_id")
	if taskID > 0 && s.tasks != nil {
		if task, err := s.tasks.GetByID(ctx, taskID); err == nil && task != nil {
			if strings.TrimSpace(payloadString(p, "task_no")) == "" && strings.TrimSpace(task.TaskNo) != "" {
				p["task_no"] = task.TaskNo
			}
			if payloadInt64(p, "creator_id") == 0 && task.CreatorID > 0 {
				p["creator_id"] = task.CreatorID
			}
			if payloadInt64(p, "designer_id") == 0 && task.DesignerID != nil {
				p["designer_id"] = *task.DesignerID
			}
		}
	}
	if ntype == domain.NotificationTypeTaskAssignedToMe && payloadInt64(p, "assigned_to_id") == 0 && userID > 0 {
		p["assigned_to_id"] = userID
	}
	if teamName := payloadString(p, "team_name"); looksTechnical(teamName) {
		if label := businessTeamLabel(payloadString(p, "pool_team_code", "team_code", "target_team_code", "team_name")); label != "" {
			p["team_name"] = label
		}
	}
	if moduleName := payloadString(p, "module_name"); looksTechnical(moduleName) {
		if label := businessModuleLabel(payloadString(p, "module_key", "module_name")); label != "" {
			p["module_name"] = label
		}
	}
	s.enrichUserName(ctx, p, "creator_id", "creator_name")
	s.enrichUserName(ctx, p, "designer_id", "designer_name")
	s.enrichUserName(ctx, p, "assigned_to_id", "assigned_to_name")
	s.enrichUserName(ctx, p, "assigned_by", "assigned_by_name")
	s.enrichUserName(ctx, p, "closed_by", "closed_by_name")
	s.enrichUserName(ctx, p, "cancelled_by", "cancelled_by_name")
	return mustRaw(p)
}

func (s *Service) enrichUserName(ctx context.Context, p map[string]interface{}, idKey, nameKey string) {
	if s.users == nil || strings.TrimSpace(payloadString(p, nameKey)) != "" {
		return
	}
	userID := payloadInt64(p, idKey)
	if userID <= 0 {
		return
	}
	if name, err := s.userDisplayName(ctx, userID); err == nil && name != "" {
		p[nameKey] = name
	}
}

func (s *Service) userDisplayName(ctx context.Context, userID int64) (string, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "", err
	}
	for _, candidate := range []string{user.DisplayName, user.Name, user.RealName, user.Username} {
		if name := strings.TrimSpace(candidate); name != "" {
			return name, nil
		}
	}
	return fmt.Sprintf("用户%d", userID), nil
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

func canBroadcastNotifications(actor domain.RequestActor) bool {
	return hasAnyBroadcastRole(actor, domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin)
}

func canBroadcastAllNotifications(actor domain.RequestActor) bool {
	return hasAnyBroadcastRole(actor, domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleHRAdmin)
}

func hasAnyBroadcastRole(actor domain.RequestActor, allowed ...domain.Role) bool {
	for _, role := range actor.Roles {
		for _, candidate := range allowed {
			if role == candidate {
				return true
			}
		}
	}
	return false
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func userStatusPtr(status domain.UserStatus) *domain.UserStatus {
	return &status
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
