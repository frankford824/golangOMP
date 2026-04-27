//go:build integration

package search

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
	sadUserMin = int64(50000)
	sadUserMax = int64(60000)
	sadTaskMin = int64(50000)
	sadTaskMax = int64(60000)
)

func sadSearchDBSvc(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return db, NewService(mysqlrepo.NewSearchRepo(mysqlrepo.New(db)))
}

func sadActor(id int64, role domain.Role) domain.RequestActor {
	return domain.RequestActor{ID: id, Username: fmt.Sprintf("sad_user_%d", id), Roles: []domain.Role{role}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
}

func sadCleanup(t *testing.T, db *sql.DB, taskIDs []int64, userIDs []int64) {
	t.Helper()
	for _, id := range taskIDs {
		if id < sadTaskMin || id >= sadTaskMax {
			t.Fatalf("SA-D task id %d outside range", id)
		}
		_, _ = db.Exec(`DELETE FROM task_assets WHERE task_id=?`, id)
		r35.CleanupTaskIDs(t, db, id)
	}
	for _, id := range userIDs {
		if id < sadUserMin || id >= sadUserMax {
			t.Fatalf("SA-D user id %d outside range", id)
		}
		_, _ = db.Exec(`DELETE FROM permission_logs WHERE actor_id=? OR target_user_id=?`, id, id)
		_, _ = db.Exec(`DELETE FROM user_sessions WHERE user_id=?`, id)
		_, _ = db.Exec(`DELETE FROM user_roles WHERE user_id=?`, id)
		_, _ = db.Exec(`DELETE FROM users WHERE id=?`, id)
	}
}

func sadInsertUser(t *testing.T, db *sql.DB, id int64, role domain.Role, username string) {
	t.Helper()
	if id < sadUserMin || id >= sadUserMax {
		t.Fatalf("SA-D user id %d outside range", id)
	}
	if username == "" {
		username = fmt.Sprintf("sad_user_%d", id)
	}
	_, err := db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, mobile, email, password_hash,
			 status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, '运营部', '淘系一组', ?, ?, '$2y$10$placeholder', 'active', 'full_time', 0, NOW(6), NOW(6))`,
		id, username, username, fmt.Sprintf("139%08d", id), username+"@example.test")
	if err != nil {
		t.Fatalf("insert SA-D user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, id, role); err != nil {
		t.Fatalf("insert SA-D role: %v", err)
	}
}

func sadInsertTaskAsset(t *testing.T, db *sql.DB, taskID int64, taskNo, sku, fileName string) {
	t.Helper()
	if taskID < sadTaskMin || taskID >= sadTaskMax {
		t.Fatalf("SA-D task id %d outside range", taskID)
	}
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: "task_detail", State: string(domain.ModuleStateActive)},
		{Key: "design", State: string(domain.ModuleStatePendingClaim)},
	})
	_, err := db.Exec(`UPDATE tasks SET task_no=?, sku_code=?, primary_sku_code=?, product_name_snapshot=? WHERE id=?`, taskNo, sku, sku, "SA-D Product "+sku, taskID)
	if err != nil {
		t.Fatalf("update SA-D task: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO task_assets
			(task_id, asset_id, asset_type, version_no, asset_version_no, upload_mode, file_name, uploaded_by, remark, source_module_key, created_at)
		VALUES (?, ?, 'delivery', 1, 1, 'small', ?, 50001, 'sa-d', 'design', NOW(6))`,
		taskID, taskID, fileName)
	if err != nil {
		t.Fatalf("insert SA-D asset: %v", err)
	}
}

func sadCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 15*time.Second)
}
