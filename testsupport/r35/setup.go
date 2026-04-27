package r35

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

type ModuleFixture struct {
	Key             string
	State           string
	PoolTeamCode    *string
	ClaimedBy       *int64
	ClaimedTeamCode *string
	Data            string
}

func MustOpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set")
	}
	if os.Getenv("R35_MODE") != "1" {
		t.Fatalf("R35_MODE=1 is required for R3.5 integration tests")
	}
	if err := GuardR35DSN(dsn); err != nil {
		t.Fatalf("R35 guard failed: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(32)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping DB: %v", err)
	}
	return db
}

func GuardR35DSN(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse DSN: %w", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_r3_test") {
		return fmt.Errorf("R3.5 safety violation: DSN database %q must end with '_r3_test'", cfg.DBName)
	}
	return nil
}

func StrPtr(v string) *string { return &v }

func Int64Ptr(v int64) *int64 { return &v }

func CleanupTaskIDs(t *testing.T, db *sql.DB, taskIDs ...int64) {
	t.Helper()
	for _, taskID := range taskIDs {
		_, _ = db.Exec(`DELETE e FROM task_module_events e JOIN task_modules m ON m.id = e.task_module_id WHERE m.task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM reference_file_refs WHERE task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM task_modules WHERE task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM task_customization_orders WHERE task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM task_sku_items WHERE task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM task_details WHERE task_id = ?`, taskID)
		_, _ = db.Exec(`DELETE FROM tasks WHERE id = ?`, taskID)
	}
}

func InsertTaskWithModules(t *testing.T, db *sql.DB, taskID int64, taskType, priority string, modules []ModuleFixture) {
	t.Helper()
	CleanupTaskIDs(t, db, taskID)
	taskNo := fmt.Sprintf("R35-%d", taskID)
	_, err := db.Exec(`
		INSERT INTO tasks
			(id, task_no, source_mode, sku_code, product_name_snapshot, task_type,
			 owner_team, owner_department, owner_org_team, creator_id, requester_id,
			 task_status, priority, is_batch_task, batch_item_count, batch_mode,
			 primary_sku_code, sku_generation_status, created_at, updated_at)
		VALUES (?, ?, 'existing_product', ?, 'R35 Test Product', ?,
			'R35 Team', '运营部', 'operations_r35', 10001, 10001,
			'InProgress', ?, 0, 1, 'single', ?, 'not_applicable', NOW(6), NOW(6))`,
		taskID, taskNo, fmt.Sprintf("R35-SKU-%d", taskID), taskType, priority, fmt.Sprintf("R35-SKU-%d", taskID))
	if err != nil {
		t.Fatalf("insert task %d: %v", taskID, err)
	}
	_, err = db.Exec(`
		INSERT INTO task_details
			(task_id, demand_text, copy_text, remark, note, category, spec_text,
			 change_request, design_requirement, reference_images_json, reference_file_refs_json,
			 reference_link, matched_mapping_rule_json, product_selection_snapshot_json,
			 filing_status, filing_error_message)
		VALUES (?, 'r35 demand', 'r35 copy', 'r35 remark', 'r35 note', 'R35',
			'r35 spec', 'r35 change', 'r35 design', '[]', '[]', '',
			'{}', '{}', 'pending_filing', '')`,
		taskID)
	if err != nil {
		t.Fatalf("insert task_detail %d: %v", taskID, err)
	}
	for _, m := range modules {
		data := strings.TrimSpace(m.Data)
		if data == "" {
			data = "{}"
		}
		_, err = db.Exec(`
			INSERT INTO task_modules
				(task_id, module_key, state, pool_team_code, claimed_by, claimed_team_code,
				 claimed_at, actor_org_snapshot, data, entered_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, IF(? IS NULL, NULL, NOW(6)), '{}', CAST(? AS JSON), NOW(6), NOW(6))`,
			taskID, m.Key, m.State, m.PoolTeamCode, m.ClaimedBy, m.ClaimedTeamCode, m.ClaimedBy, data)
		if err != nil {
			t.Fatalf("insert task_module %s/%d: %v", m.Key, taskID, err)
		}
	}
}

func MustModuleID(t *testing.T, db *sql.DB, taskID int64, moduleKey string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM task_modules WHERE task_id=? AND module_key=?`, taskID, moduleKey).Scan(&id); err != nil {
		t.Fatalf("select module id %d/%s: %v", taskID, moduleKey, err)
	}
	return id
}

func TruncateR2Tables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, _ = db.Exec(`DELETE e FROM task_module_events e JOIN task_modules m ON m.id = e.task_module_id WHERE m.task_id >= 10000`)
	_, _ = db.Exec(`DELETE FROM reference_file_refs WHERE task_id >= 10000`)
	_, _ = db.Exec(`DELETE FROM task_modules WHERE task_id >= 10000`)
	_, _ = db.Exec(`DELETE FROM task_customization_orders WHERE task_id >= 10000`)
}
