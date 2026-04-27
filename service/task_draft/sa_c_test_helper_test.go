//go:build integration

package task_draft

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

// SA-C.1 fixture user id segment is [40000, 50000) — distinct from SA-A (20000+) and SA-B (30000+).
const (
	saCUserIDMin = int64(40000)
	saCUserIDMax = int64(50000)
)

func sacOpenServiceAndDB(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	wrapped := mysqlrepo.New(db)
	draftRepo := mysqlrepo.NewTaskDraftRepo(wrapped)
	logRepo := mysqlrepo.NewPermissionLogRepo(wrapped)
	return db, NewService(draftRepo, logRepo, wrapped)
}

// sacInsertUser inserts a minimal users row with mandatory NOT NULL columns satisfied.
// Mirrors saBInsertOrgMoveUser semantics; SA-C only needs the row to exist as an FK target.
func sacInsertUser(t *testing.T, db *sql.DB, id int64, department, team string, roles []domain.Role) {
	t.Helper()
	if id < saCUserIDMin || id >= saCUserIDMax {
		t.Fatalf("SA-C fixture user id %d outside [%d, %d)", id, saCUserIDMin, saCUserIDMax)
	}
	if department == "" {
		department = string(domain.DepartmentOperations)
	}
	if team == "" {
		team = "淘系一组"
	}
	username := fmt.Sprintf("sac_user_%d", id)
	_, err := db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, mobile, email, password_hash,
			 status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '$2y$10$placeholder', 'active', 'full_time', 0, NOW(6), NOW(6))`,
		id, username, username, department, team, fmt.Sprintf("139%08d", id), username+"@example.test")
	if err != nil {
		t.Fatalf("insert SA-C user %d: %v", id, err)
	}
	if len(roles) == 0 {
		roles = []domain.Role{domain.RoleMember}
	}
	for _, role := range domain.NormalizeRoleValues(roles) {
		if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, id, role); err != nil {
			t.Fatalf("insert SA-C role %s for user %d: %v", role, id, err)
		}
	}
}

// sacActor builds a request actor for service-level assertions in SA-C.1.
func sacActor(id int64, roles []domain.Role) domain.RequestActor {
	if len(roles) == 0 {
		roles = []domain.Role{domain.RoleMember}
	}
	return domain.RequestActor{
		ID:       id,
		Username: fmt.Sprintf("sac_user_%d", id),
		Roles:    domain.NormalizeRoleValues(roles),
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
}

// sacCleanupSAC purges fixtures created by SA-C.1 tests. Cleanup order:
// task_drafts -> notifications -> permission_logs -> user_roles -> users (FK-safe).
func sacCleanupSAC(t *testing.T, db *sql.DB, userIDs ...int64) {
	t.Helper()
	if len(userIDs) == 0 {
		return
	}
	args := make([]interface{}, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id < saCUserIDMin || id >= saCUserIDMax {
			t.Fatalf("SA-C cleanup user id %d outside [%d, %d)", id, saCUserIDMin, saCUserIDMax)
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

// sacRawJSON returns a JSON RawMessage built from the supplied k/v map.
func sacRawJSON(t *testing.T, payload map[string]interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal SA-C payload: %v", err)
	}
	return raw
}

// sacSeedTaskDraftFor inserts a task_drafts row directly so list/get/delete tests
// do not depend on Service.CreateOrUpdate's quota path.
func sacSeedTaskDraftFor(t *testing.T, db *sql.DB, ownerID int64, taskType string, payload json.RawMessage) int64 {
	t.Helper()
	if ownerID < saCUserIDMin || ownerID >= saCUserIDMax {
		t.Fatalf("SA-C draft owner id %d outside [%d, %d)", ownerID, saCUserIDMin, saCUserIDMax)
	}
	if !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	res, err := db.Exec(`
		INSERT INTO task_drafts (owner_user_id, task_type, payload, expires_at)
		VALUES (?, ?, CAST(? AS JSON), DATE_ADD(NOW(6), INTERVAL 7 DAY))`,
		ownerID, taskType, string(payload))
	if err != nil {
		t.Fatalf("seed task_draft: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed task_draft last id: %v", err)
	}
	return id
}

// sacAppDenyCode mirrors saBAppDenyCode for SA-C error introspection on AppError.Details.
func sacAppDenyCode(appErr *domain.AppError) string {
	if appErr == nil || appErr.Details == nil {
		return ""
	}
	switch d := appErr.Details.(type) {
	case map[string]string:
		return d["deny_code"]
	case map[string]interface{}:
		if v, ok := d["deny_code"].(string); ok {
			return v
		}
	}
	return ""
}

// sacWithTimeout returns a context with a generous timeout for integration calls.
func sacWithTimeout(t *testing.T, parent context.Context) (context.Context, context.CancelFunc) {
	t.Helper()
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 15*time.Second)
}
