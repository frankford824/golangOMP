//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/testsupport/r35"
)

type autoArchiveCLIResult struct {
	Subcommand string `json:"subcommand"`
	DryRun     bool   `json:"dry_run"`
	Scanned    int    `json:"scanned"`
	Archived   int    `json:"archived"`
	Cutoff     string `json:"cutoff"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	Error      string `json:"error"`
}

func TestR6A3_AutoArchive_CLI_DryRun(t *testing.T) {
	db := openR6A3CLIDB(t)
	cleanupR6A3CLISegment(t, db)
	t.Cleanup(func() {
		cleanupR6A3CLISegment(t, db)
		assertR6A3CLIAuditClean(t, db)
		_ = db.Close()
	})
	seedR6A3CLIFixture(t, db, 60000)

	res := runAutoArchiveCLI(t, "auto-archive", "--dry-run", "--limit=100")
	if res.Scanned != 8 || res.Archived != 0 {
		t.Fatalf("auto-archive dry-run scanned/archived = %d/%d, want 8/0", res.Scanned, res.Archived)
	}
	assertR6A3CLIStatusCount(t, db, 60000, 12, domain.TaskStatusArchived, 0)
}

func TestR6A3_AutoArchive_CLI_RealRun(t *testing.T) {
	db := openR6A3CLIDB(t)
	cleanupR6A3CLISegment(t, db)
	t.Cleanup(func() {
		cleanupR6A3CLISegment(t, db)
		assertR6A3CLIAuditClean(t, db)
		_ = db.Close()
	})
	seedR6A3CLIFixture(t, db, 60020)

	res := runAutoArchiveCLI(t, "auto-archive", "--limit=100")
	if res.Scanned != 8 || res.Archived != 8 {
		t.Fatalf("auto-archive real-run scanned/archived = %d/%d, want 8/8", res.Scanned, res.Archived)
	}
	assertR6A3CLIStatusCount(t, db, 60020, 12, domain.TaskStatusArchived, 8)
}

func TestR6A3_AS_X_E2E_100(t *testing.T) {
	db := openR6A3CLIDB(t)
	cleanupR6A3CLISegment(t, db)
	t.Cleanup(func() {
		cleanupR6A3CLISegment(t, db)
		assertR6A3CLIAuditClean(t, db)
		_ = db.Close()
	})
	for i := int64(0); i < 100; i++ {
		seedR6A3CLITask(t, db, 60100+i, domain.TaskStatusCompleted, true)
	}

	res := runAutoArchiveCLI(t, "auto-archive", "--limit=200")
	if res.Archived != 100 {
		t.Fatalf("AS-X archived = %d, want 100; result=%+v", res.Archived, res)
	}
	assertR6A3CLIStatusCount(t, db, 60100, 100, domain.TaskStatusArchived, 100)
	t.Logf("AS-X elapsed_ms=%d", res.ElapsedMS)
}

func openR6A3CLIDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("R35_MODE") == "" {
		t.Setenv("R35_MODE", "1")
	}
	return r35.MustOpenTestDB(t)
}

func runAutoArchiveCLI(t *testing.T, args ...string) autoArchiveCLIResult {
	t.Helper()
	cmdArgs := append([]string{"-test.run=TestR6A1_HelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "R6A1_HELPER_PROCESS=1", "OSS_DELETER_DISABLED=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run-cleanup %s failed: %v\nstdout=%s\nstderr=%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	var res autoArchiveCLIResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &res); err != nil {
		t.Fatalf("decode stdout JSON %q: %v\nstderr=%s", stdout.String(), err, stderr.String())
	}
	if res.Error != "" {
		t.Fatalf("cli returned error: %s\nstderr=%s", res.Error, stderr.String())
	}
	return res
}

func seedR6A3CLIFixture(t *testing.T, db *sql.DB, startID int64) {
	t.Helper()
	for i := int64(0); i < 5; i++ {
		seedR6A3CLITask(t, db, startID+i, domain.TaskStatusCompleted, true)
	}
	for i := int64(5); i < 8; i++ {
		seedR6A3CLITask(t, db, startID+i, domain.TaskStatusCancelled, true)
	}
	for i := int64(8); i < 12; i++ {
		seedR6A3CLITask(t, db, startID+i, domain.TaskStatusCompleted, false)
	}
}

func seedR6A3CLITask(t *testing.T, db *sql.DB, taskID int64, status domain.TaskStatus, old bool) {
	t.Helper()
	requireR6A3CLIID(t, taskID, "task_id")
	cleanupR6A3CLITask(t, db, taskID)
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
		VALUES (?, ?, 'existing_product', ?, 'R6A3 CLI Product', ?,
			'R6A3 Team', '运营部', 'operations_r6a3', 10001, 10001,
			?, 'normal', 0, 1, 'single', ?, 'not_applicable', NOW(6), `+updatedExpr+`)`,
		taskID, fmt.Sprintf("R6A3CLI-%d", taskID), fmt.Sprintf("R6A3CLI-SKU-%d", taskID),
		string(domain.TaskTypeOriginalProductDevelopment), string(status), fmt.Sprintf("R6A3CLI-SKU-%d", taskID))
	if err != nil {
		t.Fatalf("insert task %d: %v", taskID, err)
	}
}

func assertR6A3CLIStatusCount(t *testing.T, db *sql.DB, startID int64, count int64, status domain.TaskStatus, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id >= ? AND id < ? AND task_status = ?`, startID, startID+count, string(status)).Scan(&got); err != nil {
		t.Fatalf("count status %s: %v", status, err)
	}
	if got != want {
		t.Fatalf("status %s count = %d, want %d", status, got, want)
	}
}

func cleanupR6A3CLITask(t *testing.T, db *sql.DB, taskID int64) {
	t.Helper()
	requireR6A3CLIID(t, taskID, "task_id")
	_, _ = db.Exec(`DELETE e FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_assets WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_details WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_modules WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM tasks WHERE id=?`, taskID)
}

func cleanupR6A3CLISegment(t *testing.T, db *sql.DB) {
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

func assertR6A3CLIAuditClean(t *testing.T, db *sql.DB) {
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

func requireR6A3CLIID(t *testing.T, id int64, label string) {
	t.Helper()
	if id < 60000 || id >= 70000 {
		t.Fatalf("%s %d outside [60000,70000)", label, id)
	}
}
