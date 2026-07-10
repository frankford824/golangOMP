package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestBroadcastToAllActiveUsers(t *testing.T) {
	notifications := &broadcastNotificationRepo{}
	users := &broadcastUserRepo{users: map[int64]*domain.User{
		1: {ID: 1, Status: domain.UserStatusActive},
		2: {ID: 2, Status: domain.UserStatusActive},
		3: {ID: 3, Status: domain.UserStatusDisabled},
	}}
	svc := NewService(notifications, nil, nil, nil, WithUserRepo(users), WithTxRunner(broadcastTxRunner{}))

	got, appErr := svc.Broadcast(context.Background(), domain.RequestActor{
		ID:       99,
		Username: "admin",
		Roles:    []domain.Role{domain.RoleAdmin},
	}, BroadcastParams{Audience: BroadcastAudienceAll, Title: "系统通知", Content: "今天 18:00 前完成处理"})
	if appErr != nil {
		t.Fatalf("Broadcast returned error: %v", appErr)
	}
	if got.NotificationCount != 2 || got.RecipientCount != 2 {
		t.Fatalf("unexpected result: %#v", got)
	}
	if len(notifications.created) != 2 {
		t.Fatalf("created %d notifications, want 2", len(notifications.created))
	}
	var broadcastID string
	for _, n := range notifications.created {
		if n.NotificationType != domain.NotificationTypeSystemBroadcast {
			t.Fatalf("notification type = %q, want system_broadcast", n.NotificationType)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(n.Payload, &payload); err != nil {
			t.Fatalf("payload json: %v", err)
		}
		if payload["title"] != "系统通知" || payload["content"] != "今天 18:00 前完成处理" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		currentBroadcastID := strings.TrimSpace(fmt.Sprint(payload["broadcast_id"]))
		if currentBroadcastID == "" {
			t.Fatalf("payload missing broadcast_id: %#v", payload)
		}
		if broadcastID == "" {
			broadcastID = currentBroadcastID
		} else if currentBroadcastID != broadcastID {
			t.Fatalf("broadcast_id = %q, want %q", currentBroadcastID, broadcastID)
		}
	}
}

func TestBroadcastSelectedUsersRejectsInactiveRecipient(t *testing.T) {
	svc := NewService(&broadcastNotificationRepo{}, nil, nil, nil,
		WithUserRepo(&broadcastUserRepo{users: map[int64]*domain.User{
			1: {ID: 1, Status: domain.UserStatusActive},
			2: {ID: 2, Status: domain.UserStatusDisabled},
		}}),
		WithTxRunner(broadcastTxRunner{}),
	)

	_, appErr := svc.Broadcast(context.Background(), domain.RequestActor{
		ID:    99,
		Roles: []domain.Role{domain.RoleDeptAdmin},
	}, BroadcastParams{Audience: BroadcastAudienceUsers, UserIDs: []int64{1, 2}, Title: "通知", Content: "内容"})
	if appErr == nil {
		t.Fatal("expected error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

func TestBroadcastAllRejectsDepartmentAdmin(t *testing.T) {
	svc := NewService(&broadcastNotificationRepo{}, nil, nil, nil,
		WithUserRepo(&broadcastUserRepo{}),
		WithTxRunner(broadcastTxRunner{}),
	)

	_, appErr := svc.Broadcast(context.Background(), domain.RequestActor{
		ID:    99,
		Roles: []domain.Role{domain.RoleDeptAdmin},
	}, BroadcastParams{Audience: BroadcastAudienceAll, Title: "通知", Content: "内容"})
	if appErr == nil {
		t.Fatal("expected error")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
	}
}

type broadcastTx struct{}

func (broadcastTx) IsTx() {}

type broadcastTxRunner struct{}

func (broadcastTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(broadcastTx{})
}

type broadcastRollbackTxRunner struct {
	repo *broadcastNotificationRepo
}

func (r broadcastRollbackTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	snapshot := r.repo.snapshot()
	err := fn(broadcastTx{})
	if err != nil {
		r.repo.restore(snapshot)
	}
	return err
}

type broadcastNotificationRepo struct {
	nextID       int64
	created      []*domain.Notification
	pref         domain.NotificationPreference
	subs         []domain.WebPushSubscription
	deliveries   []domain.NotificationDeliveryOutbox
	staleUserID  int64
	staleKeyHash string
	failCreate   bool
	dedupeClaims map[string]domain.NotificationDedupeClaim
}

type broadcastNotificationRepoSnapshot struct {
	nextID       int64
	created      []*domain.Notification
	deliveries   []domain.NotificationDeliveryOutbox
	dedupeClaims map[string]domain.NotificationDedupeClaim
}

func (r *broadcastNotificationRepo) snapshot() broadcastNotificationRepoSnapshot {
	return broadcastNotificationRepoSnapshot{
		nextID:       r.nextID,
		created:      append([]*domain.Notification(nil), r.created...),
		deliveries:   append([]domain.NotificationDeliveryOutbox(nil), r.deliveries...),
		dedupeClaims: cloneBroadcastDedupeClaims(r.dedupeClaims),
	}
}

func (r *broadcastNotificationRepo) restore(snapshot broadcastNotificationRepoSnapshot) {
	r.nextID = snapshot.nextID
	r.created = snapshot.created
	r.deliveries = snapshot.deliveries
	r.dedupeClaims = snapshot.dedupeClaims
}

func cloneBroadcastDedupeClaims(in map[string]domain.NotificationDedupeClaim) map[string]domain.NotificationDedupeClaim {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.NotificationDedupeClaim, len(in))
	for key, claim := range in {
		out[key] = claim
	}
	return out
}

func (r *broadcastNotificationRepo) Create(ctx context.Context, tx repo.Tx, notification *domain.Notification) (*domain.Notification, error) {
	if notification == nil {
		return nil, errors.New("notification is nil")
	}
	if r.failCreate {
		return nil, errors.New("create failed")
	}
	r.nextID++
	cp := *notification
	cp.ID = r.nextID
	cp.CreatedAt = time.Now().UTC()
	r.created = append(r.created, &cp)
	return &cp, nil
}

func (r *broadcastNotificationRepo) Get(ctx context.Context, id int64) (*domain.Notification, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastNotificationRepo) List(ctx context.Context, filter repo.NotificationListFilter) ([]domain.Notification, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastNotificationRepo) MarkRead(ctx context.Context, id, userID int64, at time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *broadcastNotificationRepo) MarkReadScoped(ctx context.Context, id, userID int64, at time.Time, scope repo.NotificationScope) (int64, error) {
	return r.MarkRead(ctx, id, userID, at)
}

func (r *broadcastNotificationRepo) MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *broadcastNotificationRepo) MarkAllReadScoped(ctx context.Context, userID int64, at time.Time, scope repo.NotificationScope) (int64, error) {
	return r.MarkAllRead(ctx, userID, at)
}

func (r *broadcastNotificationRepo) UnreadCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (r *broadcastNotificationRepo) UnreadCountScoped(ctx context.Context, userID int64, scope repo.NotificationScope) (int, error) {
	return r.UnreadCount(ctx, userID)
}

func (r *broadcastNotificationRepo) UpsertWebPushSubscription(ctx context.Context, tx repo.Tx, sub *domain.WebPushSubscription) (*domain.WebPushSubscription, error) {
	return sub, nil
}

func (r *broadcastNotificationRepo) DisableWebPushSubscriptionByEndpointHash(ctx context.Context, userID int64, endpointHash string, at time.Time, reason string) (int64, error) {
	return 0, nil
}

func (r *broadcastNotificationRepo) DisableWebPushSubscriptionByID(ctx context.Context, id int64, at time.Time, reason string) error {
	return nil
}

func (r *broadcastNotificationRepo) MarkStaleWebPushSubscriptionsByUserExceptKeyHash(ctx context.Context, userID int64, vapidKeyHash string, at time.Time) (int64, error) {
	r.staleUserID = userID
	r.staleKeyHash = vapidKeyHash
	return 0, nil
}

func (r *broadcastNotificationRepo) ListActiveWebPushSubscriptionsForUser(ctx context.Context, tx repo.Tx, userID int64, vapidKeyHash string) ([]domain.WebPushSubscription, error) {
	out := make([]domain.WebPushSubscription, 0, len(r.subs))
	for _, sub := range r.subs {
		if sub.UserID == userID && sub.Status == domain.WebPushSubscriptionActive && sub.VAPIDKeyHash == vapidKeyHash {
			out = append(out, sub)
		}
	}
	return out, nil
}

func (r *broadcastNotificationRepo) GetNotificationPreference(ctx context.Context, userID int64) (*domain.NotificationPreference, error) {
	if r.pref.UserID == userID {
		cp := r.pref
		return &cp, nil
	}
	return &domain.NotificationPreference{UserID: userID}, nil
}

func (r *broadcastNotificationRepo) UpsertNotificationPreference(ctx context.Context, tx repo.Tx, pref *domain.NotificationPreference) error {
	return nil
}

func (r *broadcastNotificationRepo) EnqueueNotificationDelivery(ctx context.Context, tx repo.Tx, delivery *domain.NotificationDeliveryOutbox) error {
	if delivery != nil {
		r.deliveries = append(r.deliveries, *delivery)
	}
	return nil
}

func (r *broadcastNotificationRepo) ClaimWebPushDeliveries(ctx context.Context, limit int, claimToken string, leaseUntil time.Time, now time.Time) ([]domain.NotificationDeliveryOutbox, error) {
	return nil, nil
}

func (r *broadcastNotificationRepo) MarkWebPushDeliverySent(ctx context.Context, id int64, claimToken string, at time.Time) error {
	return nil
}

func (r *broadcastNotificationRepo) MarkWebPushDeliveryRetry(ctx context.Context, id int64, claimToken string, nextAttemptAt time.Time, lastError string, providerStatusCode *int, at time.Time) error {
	return nil
}

func (r *broadcastNotificationRepo) MarkWebPushDeliveryDead(ctx context.Context, id int64, claimToken string, lastError string, providerStatusCode *int, at time.Time) error {
	return nil
}

func (r *broadcastNotificationRepo) TryCreateNotificationDedupeClaim(ctx context.Context, tx repo.Tx, claim domain.NotificationDedupeClaim) (bool, error) {
	if r.dedupeClaims == nil {
		r.dedupeClaims = map[string]domain.NotificationDedupeClaim{}
	}
	key := broadcastDedupeClaimKey(claim)
	if _, ok := r.dedupeClaims[key]; ok {
		return false, nil
	}
	r.dedupeClaims[key] = claim
	return true, nil
}

func (r *broadcastNotificationRepo) UpdateNotificationDedupeClaimNotificationID(ctx context.Context, tx repo.Tx, claim domain.NotificationDedupeClaim, notificationID int64) error {
	if r.dedupeClaims == nil {
		r.dedupeClaims = map[string]domain.NotificationDedupeClaim{}
	}
	key := broadcastDedupeClaimKey(claim)
	claim.NotificationID = &notificationID
	r.dedupeClaims[key] = claim
	return nil
}

func (r *broadcastNotificationRepo) ClearNotificationDedupeScope(ctx context.Context, scope string) error {
	return nil
}

func (r *broadcastNotificationRepo) ListRecentTaskFilingFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error) {
	return nil, nil
}

func (r *broadcastNotificationRepo) ListRecentProductBaseSyncFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error) {
	return nil, nil
}

func broadcastDedupeClaimKey(claim domain.NotificationDedupeClaim) string {
	return fmt.Sprintf("%d:%s:%s", claim.UserID, claim.NotificationType, claim.DedupeKey)
}

type broadcastUserRepo struct {
	users map[int64]*domain.User
}

func (r *broadcastUserRepo) Count(ctx context.Context) (int64, error) { return 0, nil }

func (r *broadcastUserRepo) CountByRole(ctx context.Context, role domain.Role) (int64, error) {
	return 0, nil
}

func (r *broadcastUserRepo) CountByDepartment(ctx context.Context, department string) (int64, error) {
	return 0, nil
}

func (r *broadcastUserRepo) CountByTeam(ctx context.Context, team string) (int64, error) {
	return 0, nil
}

func (r *broadcastUserRepo) Create(ctx context.Context, tx repo.Tx, user *domain.User) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *broadcastUserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	user := r.users[id]
	if user == nil {
		return nil, nil
	}
	cp := *user
	return &cp, nil
}

func (r *broadcastUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) GetByMobile(ctx context.Context, mobile string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) GetByEmployeeNo(ctx context.Context, employeeNo int) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) GetByJstUID(ctx context.Context, jstUID int64) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) List(ctx context.Context, filter repo.UserListFilter) ([]*domain.User, int64, error) {
	active := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		if user == nil {
			continue
		}
		if filter.Status != nil && user.Status != *filter.Status {
			continue
		}
		cp := *user
		active = append(active, &cp)
	}
	return active, int64(len(active)), nil
}

func (r *broadcastUserRepo) ListActiveByRole(ctx context.Context, role domain.Role) ([]*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) ListConfigManagedAdmins(ctx context.Context) ([]*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *broadcastUserRepo) Update(ctx context.Context, tx repo.Tx, user *domain.User) error {
	return errors.New("not implemented")
}

func (r *broadcastUserRepo) UpdateJstFields(ctx context.Context, tx repo.Tx, userID int64, displayName, status, department, team string, managedDepartments, managedTeams []string, jstRawSnapshot string, jstUID *int64, lastLoginAt *time.Time) error {
	return errors.New("not implemented")
}

func (r *broadcastUserRepo) UpdatePassword(ctx context.Context, tx repo.Tx, userID int64, passwordHash string, updatedAt time.Time) error {
	return errors.New("not implemented")
}

func (r *broadcastUserRepo) UpdateLastLogin(ctx context.Context, tx repo.Tx, userID int64, at time.Time) error {
	return errors.New("not implemented")
}

func (r *broadcastUserRepo) ReplaceRoles(ctx context.Context, tx repo.Tx, userID int64, roles []domain.Role) error {
	return errors.New("not implemented")
}

func (r *broadcastUserRepo) ListRoles(ctx context.Context, userID int64) ([]domain.Role, error) {
	return nil, errors.New("not implemented")
}
