//go:build integration

package task_lifecycle

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

const (
	r6a3MinID = int64(60000)
	r6a3MaxID = int64(70000)
)

func TestR6A3_AutoArchive_DryRun(t *testing.T) {
	db, job := openR6A3Job(t)
	cleanupR6A3Segment(t, db)
	t.Cleanup(func() {
		cleanupR6A3Segment(t, db)
		assertR6A3AuditClean(t, db)
		_ = db.Close()
	})
	seedR6A3ArchiveFixture(t, db, 60000)

	result, appErr := job.Run(context.Background(), AutoArchiveOptions{DryRun: true, Limit: 100})
	if appErr != nil {
		t.Fatalf("auto archive dry-run: %v", appErr)
	}
	if result.Scanned != 8 || result.Archived != 0 {
		t.Fatalf("scanned/archived = %d/%d, want 8/0", result.Scanned, result.Archived)
	}
	assertR6A3StatusCount(t, db, 60000, 12, domain.TaskStatusArchived, 0)
}

func TestR6A3_AutoArchive_RealRun(t *testing.T) {
	db, job := openR6A3Job(t)
	cleanupR6A3Segment(t, db)
	t.Cleanup(func() {
		cleanupR6A3Segment(t, db)
		assertR6A3AuditClean(t, db)
		_ = db.Close()
	})
	seedR6A3ArchiveFixture(t, db, 60020)

	result, appErr := job.Run(context.Background(), AutoArchiveOptions{Limit: 100})
	if appErr != nil {
		t.Fatalf("auto archive real-run: %v", appErr)
	}
	if result.Scanned != 8 || result.Archived != 8 {
		t.Fatalf("scanned/archived = %d/%d, want 8/8", result.Scanned, result.Archived)
	}
	assertR6A3StatusCount(t, db, 60020, 12, domain.TaskStatusArchived, 8)
	assertR6A3StatusCount(t, db, 60028, 4, domain.TaskStatusCompleted, 4)
}

func TestR6A3_AutoArchive_Idempotent(t *testing.T) {
	db, job := openR6A3Job(t)
	cleanupR6A3Segment(t, db)
	t.Cleanup(func() {
		cleanupR6A3Segment(t, db)
		assertR6A3AuditClean(t, db)
		_ = db.Close()
	})
	seedR6A3ArchiveFixture(t, db, 60040)

	first, appErr := job.Run(context.Background(), AutoArchiveOptions{Limit: 100})
	if appErr != nil {
		t.Fatalf("auto archive first run: %v", appErr)
	}
	second, appErr := job.Run(context.Background(), AutoArchiveOptions{Limit: 100})
	if appErr != nil {
		t.Fatalf("auto archive second run: %v", appErr)
	}
	if first.Archived != 8 || second.Scanned != 0 || second.Archived != 0 {
		t.Fatalf("first/second = %+v/%+v, want first archived=8 second 0/0", first, second)
	}
}

func TestR6A3_AutoArchive_Limit(t *testing.T) {
	db, job := openR6A3Job(t)
	cleanupR6A3Segment(t, db)
	t.Cleanup(func() {
		cleanupR6A3Segment(t, db)
		assertR6A3AuditClean(t, db)
		_ = db.Close()
	})
	for i := int64(0); i < 12; i++ {
		seedR6A3Task(t, db, 60060+i, domain.TaskStatusCompleted, true)
	}

	result, appErr := job.Run(context.Background(), AutoArchiveOptions{Limit: 5})
	if appErr != nil {
		t.Fatalf("auto archive limit: %v", appErr)
	}
	if result.Scanned != 5 || result.Archived != 5 {
		t.Fatalf("scanned/archived = %d/%d, want 5/5", result.Scanned, result.Archived)
	}
	assertR6A3StatusCount(t, db, 60060, 12, domain.TaskStatusArchived, 5)
}

func openR6A3Job(t *testing.T) (*sql.DB, *AutoArchiveJob) {
	t.Helper()
	if r35Mode := strings.TrimSpace(os.Getenv("R35_MODE")); r35Mode == "" {
		t.Setenv("R35_MODE", "1")
	}
	db := r35.MustOpenTestDB(t)
	mdb := mysqlrepo.New(db)
	return db, NewAutoArchiveJob(mysqlrepo.NewTaskAutoArchiveRepo(mdb), mdb, nil)
}

func seedR6A3ArchiveFixture(t *testing.T, db *sql.DB, startID int64) {
	t.Helper()
	for i := int64(0); i < 5; i++ {
		seedR6A3Task(t, db, startID+i, domain.TaskStatusCompleted, true)
	}
	for i := int64(5); i < 8; i++ {
		seedR6A3Task(t, db, startID+i, domain.TaskStatusCancelled, true)
	}
	for i := int64(8); i < 12; i++ {
		seedR6A3Task(t, db, startID+i, domain.TaskStatusCompleted, false)
	}
}

func seedR6A3Task(t *testing.T, db *sql.DB, taskID int64, status domain.TaskStatus, old bool) {
	t.Helper()
	requireR6A3ID(t, taskID, "task_id")
	cleanupR6A3Task(t, db, taskID)
	updatedExpr := "NOW(6) - INTERVAL 30 DAY"
	if old {
		updatedExpr = "NOW(6) - INTERVAL 91 DAY"
	}
	_, err := db.Exec(`
		INSERT INTO tasks
			(id, task_no, source_mode, sku_code, product_name_snapshot, task_type,
			 owner_team, owner_department, owner_org_team, creator_id, requester_id,
			 task_status, priority, is_batch_task, batch_item_count, batch_mode,
			 primary_sku_code, sku_generation_status, created_at, updated_at)
		VALUES (?, ?, 'existing_product', ?, 'R6A3 Test Product', ?,
			'R6A3 Team', '运营部', 'operations_r6a3', 10001, 10001,
			?, 'normal', 0, 1, 'single', ?, 'not_applicable', NOW(6), `+updatedExpr+`)`,
		taskID, fmt.Sprintf("R6A3-%d", taskID), fmt.Sprintf("R6A3-SKU-%d", taskID),
		string(domain.TaskTypeOriginalProductDevelopment), string(status), fmt.Sprintf("R6A3-SKU-%d", taskID))
	if err != nil {
		t.Fatalf("insert task %d: %v", taskID, err)
	}
}

func assertR6A3StatusCount(t *testing.T, db *sql.DB, startID int64, count int64, status domain.TaskStatus, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id >= ? AND id < ? AND task_status = ?`, startID, startID+count, string(status)).Scan(&got); err != nil {
		t.Fatalf("count status %s: %v", status, err)
	}
	if got != want {
		t.Fatalf("status %s count = %d, want %d", status, got, want)
	}
}

func cleanupR6A3Task(t *testing.T, db *sql.DB, taskID int64) {
	t.Helper()
	requireR6A3ID(t, taskID, "task_id")
	_, _ = db.Exec(`DELETE e FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_assets WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_details WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_modules WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM tasks WHERE id=?`, taskID)
}

func cleanupR6A3Segment(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	execs := []string{
		`DELETE e FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id >= 60000 AND m.task_id < 70000`,
		`DELETE FROM task_assets WHERE task_id >= 60000 AND task_id < 70000 OR id >= 60000 AND id < 70000 OR asset_id >= 60000 AND asset_id < 70000`,
		`DELETE FROM task_drafts WHERE owner_user_id >= 60000 AND owner_user_id < 70000 OR id >= 60000 AND id < 70000`,
		`DELETE FROM notifications WHERE user_id >= 60000 AND user_id < 70000 OR id >= 60000 AND id < 70000`,
		`DELETE FROM permission_logs WHERE actor_id >= 60000 AND actor_id < 70000 OR target_user_id >= 60000 AND target_user_id < 70000 OR id >= 60000 AND id < 70000`,
		`DELETE FROM task_details WHERE task_id >= 60000 AND task_id < 70000`,
		`DELETE FROM task_modules WHERE task_id >= 60000 AND task_id < 70000 OR id >= 60000 AND id < 70000`,
		`DELETE FROM tasks WHERE id >= 60000 AND id < 70000`,
	}
	for _, q := range execs {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("cleanup query %q: %v", q, err)
		}
	}
}

func assertR6A3AuditClean(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := map[string]string{
		"tasks":              `id >= 60000 AND id < 70000`,
		"task_modules":       `id >= 60000 AND id < 70000 OR task_id >= 60000 AND task_id < 70000`,
		"task_module_events": `id >= 60000 AND id < 70000`,
		"task_assets":        `id >= 60000 AND id < 70000 OR task_id >= 60000 AND task_id < 70000 OR asset_id >= 60000 AND asset_id < 70000`,
		"task_drafts":        `id >= 60000 AND id < 70000 OR owner_user_id >= 60000 AND owner_user_id < 70000`,
		"notifications":      `id >= 60000 AND id < 70000 OR user_id >= 60000 AND user_id < 70000`,
		"permission_logs":    `id >= 60000 AND id < 70000 OR actor_id >= 60000 AND actor_id < 70000 OR target_user_id >= 60000 AND target_user_id < 70000`,
	}
	for table, where := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE ` + where).Scan(&n); err != nil {
			t.Fatalf("audit count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("audit table %s has %d rows in [60000,70000)", table, n)
		}
	}
}

func requireR6A3ID(t *testing.T, id int64, label string) {
	t.Helper()
	if id < r6a3MinID || id >= r6a3MaxID {
		t.Fatalf("%s %d outside [%d,%d)", label, id, r6a3MinID, r6a3MaxID)
	}
}
