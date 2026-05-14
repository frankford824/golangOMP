package notification

import (
	"context"
	"encoding/json"
	"errors"
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

type broadcastNotificationRepo struct {
	nextID  int64
	created []*domain.Notification
}

func (r *broadcastNotificationRepo) Create(ctx context.Context, tx repo.Tx, notification *domain.Notification) (*domain.Notification, error) {
	if notification == nil {
		return nil, errors.New("notification is nil")
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

func (r *broadcastNotificationRepo) MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *broadcastNotificationRepo) UnreadCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
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
