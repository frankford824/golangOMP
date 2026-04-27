//go:build integration

package report_l1

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

const (
	sadReportUserMin = int64(50000)
	sadReportUserMax = int64(60000)
	sadReportTaskMin = int64(50000)
	sadReportTaskMax = int64(60000)
)

func sadReportDBSvc(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	wrapped := mysqlrepo.New(db)
	return db, NewService(mysqlrepo.NewReportL1Repo(wrapped), WithPermissionLogRepo(mysqlrepo.NewPermissionLogRepo(wrapped)))
}

func sadReportActor(id int64, role domain.Role) domain.RequestActor {
	return domain.RequestActor{ID: id, Username: fmt.Sprintf("sad_report_user_%d", id), Roles: []domain.Role{role}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
}

func sadReportCleanup(t *testing.T, db *sql.DB, taskIDs []int64, userIDs []int64) {
	t.Helper()
	for _, id := range taskIDs {
		if id < sadReportTaskMin || id >= sadReportTaskMax {
			t.Fatalf("SA-D report task id %d outside range", id)
		}
		r35.CleanupTaskIDs(t, db, id)
	}
	for _, id := range userIDs {
		if id < sadReportUserMin || id >= sadReportUserMax {
			t.Fatalf("SA-D report user id %d outside range", id)
		}
		_, _ = db.Exec(`DELETE FROM permission_logs WHERE actor_id=? OR target_user_id=?`, id, id)
		_, _ = db.Exec(`DELETE FROM user_roles WHERE user_id=?`, id)
		_, _ = db.Exec(`DELETE FROM users WHERE id=?`, id)
	}
}

func sadReportSeedUser(t *testing.T, db *sql.DB, id int64, role domain.Role) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, mobile, email, password_hash,
			 status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, '运营部', '淘系一组', ?, ?, '$2y$10$placeholder', 'active', 'full_time', 0, NOW(6), NOW(6))`,
		id, fmt.Sprintf("sad_report_user_%d", id), fmt.Sprintf("sad_report_user_%d", id), fmt.Sprintf("138%08d", id), fmt.Sprintf("sad_report_user_%d@example.test", id))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, id, role); err != nil {
		t.Fatalf("insert role: %v", err)
	}
}

func sadReportSeedTask(t *testing.T, db *sql.DB, taskID int64) {
	t.Helper()
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: "task_detail", State: string(domain.ModuleStateActive)},
		{Key: "design", State: string(domain.ModuleStateActive)},
		{Key: "audit", State: string(domain.ModuleStateActive)},
		{Key: "customization", State: string(domain.ModuleStateActive)},
		{Key: "warehouse", State: string(domain.ModuleStateActive)},
	})
}

func sadReportInsertEvent(t *testing.T, db *sql.DB, taskID int64, moduleKey, eventType string, at time.Time) {
	t.Helper()
	moduleID := r35.MustModuleID(t, db, taskID, moduleKey)
	_, err := db.Exec(`
		INSERT INTO task_module_events
			(task_module_id, event_type, from_state, to_state, actor_id, actor_snapshot, payload, created_at)
		VALUES (?, ?, NULL, NULL, 50006, JSON_OBJECT(), JSON_OBJECT(), ?)`, moduleID, eventType, at.UTC())
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
}

func sadReportCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 15*time.Second)
}
