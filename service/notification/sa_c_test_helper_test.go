//go:build integration

package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

const (
	saCNotificationUserIDMin = int64(40000)
	saCNotificationUserIDMax = int64(50000)
)

func sacNotificationOpenServiceAndDB(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	wrapped := mysqlrepo.New(db)
	notificationRepo := mysqlrepo.NewNotificationRepo(wrapped)
	logRepo := mysqlrepo.NewPermissionLogRepo(wrapped)
	return db, NewService(notificationRepo, logRepo, nil, zap.NewNop())
}

func sacNotificationInsertUser(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	if id < saCNotificationUserIDMin || id >= saCNotificationUserIDMax {
		t.Fatalf("SA-C fixture user id %d outside [%d, %d)", id, saCNotificationUserIDMin, saCNotificationUserIDMax)
	}
	username := fmt.Sprintf("sac_notification_user_%d", id)
	_, err := db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, mobile, email, password_hash,
			 status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, '运营部', '淘系一组', ?, ?, '$2y$10$placeholder',
			'active', 'full_time', 0, NOW(6), NOW(6))`,
		id, username, username, fmt.Sprintf("138%08d", id), username+"@example.test")
	if err != nil {
		t.Fatalf("insert SA-C notification user %d: %v", id, err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, id, domain.RoleMember); err != nil {
		t.Fatalf("insert SA-C notification role for user %d: %v", id, err)
	}
}

func sacNotificationActor(id int64) domain.RequestActor {
	return domain.RequestActor{
		ID:       id,
		Username: fmt.Sprintf("sac_notification_user_%d", id),
		Roles:    []domain.Role{domain.RoleMember},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
}

func sacNotificationCleanupSAC(t *testing.T, db *sql.DB, userIDs ...int64) {
	t.Helper()
	if len(userIDs) == 0 {
		return
	}
	args := make([]interface{}, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id < saCNotificationUserIDMin || id >= saCNotificationUserIDMax {
			t.Fatalf("SA-C cleanup user id %d outside [%d, %d)", id, saCNotificationUserIDMin, saCNotificationUserIDMax)
		}
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}
	in := strings.Join(placeholders, ",")
	_, _ = db.Exec(`DELETE FROM task_drafts WHERE owner_user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM notifications WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM permission_logs WHERE actor_id IN (`+in+`) OR target_user_id IN (`+in+`)`, append(append([]interface{}{}, args...), args...)...)
	_, _ = db.Exec(`DELETE FROM user_sessions WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM user_roles WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM users WHERE id IN (`+in+`)`, args...)
}

func sacSeedNotification(t *testing.T, db *sql.DB, userID int64, ntype domain.NotificationType, isRead bool, ageSeconds int) int64 {
	t.Helper()
	if userID < saCNotificationUserIDMin || userID >= saCNotificationUserIDMax {
		t.Fatalf("SA-C notification user id %d outside [%d, %d)", userID, saCNotificationUserIDMin, saCNotificationUserIDMax)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"task_id":           900000 + userID,
		"notification_type": string(ntype),
	})
	if err != nil {
		t.Fatalf("marshal notification payload: %v", err)
	}
	readFlag := 0
	var readAt interface{}
	if isRead {
		readFlag = 1
		readAt = "NOW(6)"
	}
	query := `
		INSERT INTO notifications (user_id, notification_type, payload, is_read, read_at, created_at)
		VALUES (?, ?, CAST(? AS JSON), ?, %s, DATE_SUB(NOW(6), INTERVAL ? SECOND))`
	res, err := db.Exec(fmt.Sprintf(query, readAtSQL(readAt)), userID, string(ntype), string(payload), readFlag, ageSeconds)
	if err != nil {
		t.Fatalf("seed notification: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed notification last id: %v", err)
	}
	return id
}

func readAtSQL(value interface{}) string {
	if value == nil {
		return "NULL"
	}
	return "NOW(6)"
}

func sacWithNotificationTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}
