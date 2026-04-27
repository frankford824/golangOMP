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

const (
	r6MinID = int64(50000)
	r6MaxID = int64(60000)
)

type cliResult struct {
	Subcommand string `json:"subcommand"`
	DryRun     bool   `json:"dry_run"`
	Scanned    int    `json:"scanned"`
	Cleaned    int    `json:"cleaned"`
	Deleted    int    `json:"deleted"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	Error      string `json:"error"`
}

func TestR6A1_HelperProcess(t *testing.T) {
	if os.Getenv("R6A1_HELPER_PROCESS") != "1" {
		return
	}
	idx := -1
	for i, arg := range os.Args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(os.Args) {
		os.Exit(2)
	}
	os.Exit(run(os.Args[idx+1:], os.Getenv, os.Stdout, os.Stderr))
}

func TestR6A1_OSS365_DryRun(t *testing.T) {
	db := openR6DB(t)
	cleanupR6Segment(t, db)
	t.Cleanup(func() {
		cleanupR6Segment(t, db)
		assertR6AuditClean(t, db)
		_ = db.Close()
	})
	oldTaskID := int64(50100)
	freshTaskID := int64(50101)
	seedCleanupTask(t, db, oldTaskID, 50100, true)
	seedCleanupTask(t, db, freshTaskID, 50101, false)
	seedAssets(t, db, oldTaskID, 50100, 50100, 5)
	seedAssets(t, db, freshTaskID, 50110, 50110, 5)

	res := runCLI(t, "oss-365", "--dry-run", "--limit=100")
	if res.Scanned != 5 || res.Cleaned != 0 {
		t.Fatalf("oss dry-run scanned/cleaned = %d/%d, want 5/0", res.Scanned, res.Cleaned)
	}
	assertCleanedCount(t, db, oldTaskID, 0)
	assertAutoCleanedEvents(t, db, oldTaskID, 0)
}

func TestR6A1_OSS365_RealRun(t *testing.T) {
	db := openR6DB(t)
	cleanupR6Segment(t, db)
	t.Cleanup(func() {
		cleanupR6Segment(t, db)
		assertR6AuditClean(t, db)
		_ = db.Close()
	})
	oldTaskID := int64(50200)
	freshTaskID := int64(50201)
	seedCleanupTask(t, db, oldTaskID, 50200, true)
	seedCleanupTask(t, db, freshTaskID, 50201, false)
	seedAssets(t, db, oldTaskID, 50200, 50200, 5)
	seedAssets(t, db, freshTaskID, 50210, 50210, 5)

	res := runCLI(t, "oss-365", "--limit=100")
	if res.Scanned != 5 || res.Cleaned != 5 {
		t.Fatalf("oss real scanned/cleaned = %d/%d, want 5/5", res.Scanned, res.Cleaned)
	}
	assertCleanedCount(t, db, oldTaskID, 5)
	assertAutoCleanedEvents(t, db, oldTaskID, 5)
}

func TestR6A1_Drafts7d(t *testing.T) {
	db := openR6DB(t)
	cleanupR6Segment(t, db)
	t.Cleanup(func() {
		cleanupR6Segment(t, db)
		assertR6AuditClean(t, db)
		_ = db.Close()
	})
	seedDrafts(t, db, 50010, 3, true)
	seedDrafts(t, db, 50020, 3, false)

	res := runCLI(t, "drafts-7d")
	if res.Deleted != 3 {
		t.Fatalf("drafts deleted = %d, want 3", res.Deleted)
	}
	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_drafts WHERE owner_user_id >= 50000 AND owner_user_id < 51000`).Scan(&remaining); err != nil {
		t.Fatalf("count remaining drafts: %v", err)
	}
	if remaining != 3 {
		t.Fatalf("remaining drafts = %d, want 3", remaining)
	}
}

func TestR6A1_AS_A5_E2E(t *testing.T) {
	db := openR6DB(t)
	cleanupR6Segment(t, db)
	t.Cleanup(func() {
		cleanupR6Segment(t, db)
		assertR6AuditClean(t, db)
		_ = db.Close()
	})
	taskID := int64(55500)
	seedCleanupTask(t, db, taskID, 55500, true)
	seedAssets(t, db, taskID, 55000, 55000, 1000)

	res := runCLI(t, "oss-365", "--limit=1000")
	if res.Cleaned != 1000 {
		t.Fatalf("a5 cleaned = %d, want 1000; result=%+v", res.Cleaned, res)
	}
	assertCleanedCount(t, db, taskID, 1000)
	assertAutoCleanedEvents(t, db, taskID, 1000)
	t.Logf("AS-A5 elapsed_ms=%d", res.ElapsedMS)
}

func openR6DB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("R35_MODE") == "" {
		t.Setenv("R35_MODE", "1")
	}
	return r35.MustOpenTestDB(t)
}

func runCLI(t *testing.T, args ...string) cliResult {
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
	var res cliResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &res); err != nil {
		t.Fatalf("decode stdout JSON %q: %v\nstderr=%s", stdout.String(), err, stderr.String())
	}
	if res.Error != "" {
		t.Fatalf("cli returned error: %s\nstderr=%s", res.Error, stderr.String())
	}
	return res
}

func seedCleanupTask(t *testing.T, db *sql.DB, taskID, moduleID int64, old bool) {
	t.Helper()
	requireR6ID(t, taskID, "task_id")
	requireR6ID(t, moduleID, "module_id")
	updatedExpr := "NOW(6) - INTERVAL 100 DAY"
	if old {
		updatedExpr = "NOW(6) - INTERVAL 366 DAY"
	}
	r35.CleanupTaskIDs(t, db, taskID)
	_, err := db.Exec(`
		INSERT INTO tasks
			(id, task_no, source_mode, sku_code, product_name_snapshot, task_type,
			 owner_team, owner_department, owner_org_team, creator_id, requester_id,
			 task_status, priority, is_batch_task, batch_item_count, batch_mode,
			 primary_sku_code, sku_generation_status, created_at, updated_at)
		VALUES (?, ?, 'existing_product', ?, 'R6 Test Product', ?,
			'R6 Team', '运营部', 'operations_r6', 10001, 10001,
			'Completed', 'normal', 0, 1, 'single', ?, 'not_applicable', NOW(6), `+updatedExpr+`)`,
		taskID, fmt.Sprintf("R6-%d", taskID), fmt.Sprintf("R6-SKU-%d", taskID), string(domain.TaskTypeOriginalProductDevelopment), fmt.Sprintf("R6-SKU-%d", taskID))
	if err != nil {
		t.Fatalf("insert task %d: %v", taskID, err)
	}
	_, err = db.Exec(`
		INSERT INTO task_details
			(task_id, demand_text, copy_text, remark, note, category, spec_text,
			 change_request, design_requirement, reference_images_json, reference_file_refs_json,
			 reference_link, matched_mapping_rule_json, product_selection_snapshot_json,
			 filing_status, filing_error_message)
		VALUES (?, 'r6 demand', 'r6 copy', 'r6 remark', 'r6 note', 'R6',
			'r6 spec', 'r6 change', 'r6 design', '[]', '[]', '',
			'{}', '{}', 'pending_filing', '')`,
		taskID)
	if err != nil {
		t.Fatalf("insert task_detail %d: %v", taskID, err)
	}
	_, err = db.Exec(`
		INSERT INTO task_modules
			(id, task_id, module_key, state, pool_team_code, actor_org_snapshot, data, entered_at, updated_at)
		VALUES (?, ?, 'design', 'completed', 'design_standard', '{}', '{}', NOW(6), NOW(6))`,
		moduleID, taskID)
	if err != nil {
		t.Fatalf("insert task_module %d/%d: %v", moduleID, taskID, err)
	}
}

func seedAssets(t *testing.T, db *sql.DB, taskID, assetIDStart, versionIDStart int64, count int) {
	t.Helper()
	moduleID := mustR6ModuleID(t, db, taskID)
	for i := 0; i < count; i++ {
		assetID := assetIDStart + int64(i)
		versionID := versionIDStart + int64(i)
		requireR6ID(t, assetID, "asset_id")
		requireR6ID(t, versionID, "version_id")
		assetNo := fmt.Sprintf("R6-A-%d-%d", taskID, i)
		if _, err := db.Exec(`
			INSERT INTO design_assets (id, task_id, asset_no, asset_type, created_by)
			VALUES (?, ?, ?, 'source', 10001)`,
			assetID, taskID, assetNo); err != nil {
			t.Fatalf("insert design_asset %d: %v", assetID, err)
		}
		storageKey := fmt.Sprintf("tasks/r6/%d/%d/source.psd", taskID, versionID)
		_, err := db.Exec(`
			INSERT INTO task_assets
				(id, task_id, asset_id, asset_type, version_no, asset_version_no, upload_mode, file_name, original_filename,
				 mime_type, file_size, file_path, storage_key, upload_status, preview_status, uploaded_by, uploaded_at, remark,
				 source_module_key, source_task_module_id, is_archived)
			VALUES (?, ?, ?, 'source', ?, 1, 'multipart', 'source.psd', 'source.psd',
				'application/octet-stream', 123, ?, ?, 'uploaded', 'pending', 10001, NOW(6), 'r6',
				'design', ?, 0)`,
			versionID, taskID, assetID, i+1, storageKey, storageKey, moduleID)
		if err != nil {
			t.Fatalf("insert task_asset %d: %v", versionID, err)
		}
		if _, err := db.Exec(`UPDATE design_assets SET current_version_id=? WHERE id=?`, versionID, assetID); err != nil {
			t.Fatalf("update design current version: %v", err)
		}
	}
}

func seedDrafts(t *testing.T, db *sql.DB, ownerStart int64, count int, expired bool) {
	t.Helper()
	for i := 0; i < count; i++ {
		ownerID := ownerStart + int64(i)
		requireR6ID(t, ownerID, "owner_user_id")
		expiresExpr := "DATE_ADD(NOW(6), INTERVAL 7 DAY)"
		if expired {
			expiresExpr = "DATE_SUB(NOW(6), INTERVAL 1 DAY)"
		}
		_, err := db.Exec(`
			INSERT INTO task_drafts (owner_user_id, task_type, payload, expires_at)
			VALUES (?, 'r6', CAST(? AS JSON), `+expiresExpr+`)`,
			ownerID, fmt.Sprintf(`{"owner":%d}`, ownerID))
		if err != nil {
			t.Fatalf("insert draft owner %d: %v", ownerID, err)
		}
	}
}

func mustR6ModuleID(t *testing.T, db *sql.DB, taskID int64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM task_modules WHERE task_id=? AND module_key='design'`, taskID).Scan(&id); err != nil {
		t.Fatalf("select module id for task %d: %v", taskID, err)
	}
	return id
}

func assertCleanedCount(t *testing.T, db *sql.DB, taskID int64, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_assets WHERE task_id=? AND cleaned_at IS NOT NULL AND storage_key IS NULL`, taskID).Scan(&got); err != nil {
		t.Fatalf("count cleaned task assets: %v", err)
	}
	if got != want {
		t.Fatalf("cleaned task_assets = %d, want %d", got, want)
	}
}

func assertAutoCleanedEvents(t *testing.T, db *sql.DB, taskID int64, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		  FROM task_module_events e
		  JOIN task_modules m ON m.id=e.task_module_id
		 WHERE m.task_id=? AND e.event_type='asset_auto_cleaned'`, taskID).Scan(&got); err != nil {
		t.Fatalf("count auto-cleaned events: %v", err)
	}
	if got != want {
		t.Fatalf("asset_auto_cleaned events = %d, want %d", got, want)
	}
}

func cleanupR6Segment(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	execs := []string{
		`DELETE e FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id >= 50000 AND m.task_id < 60000`,
		`DELETE FROM task_assets WHERE task_id >= 50000 AND task_id < 60000 OR id >= 50000 AND id < 60000 OR asset_id >= 50000 AND asset_id < 60000`,
		`UPDATE design_assets SET current_version_id=NULL WHERE id >= 50000 AND id < 60000 OR task_id >= 50000 AND task_id < 60000`,
		`DELETE FROM design_assets WHERE id >= 50000 AND id < 60000 OR task_id >= 50000 AND task_id < 60000`,
		`DELETE FROM task_drafts WHERE owner_user_id >= 50000 AND owner_user_id < 60000 OR id >= 50000 AND id < 60000`,
		`DELETE FROM notifications WHERE user_id >= 50000 AND user_id < 60000 OR id >= 50000 AND id < 60000`,
		`DELETE FROM permission_logs WHERE actor_id >= 50000 AND actor_id < 60000 OR target_user_id >= 50000 AND target_user_id < 60000 OR id >= 50000 AND id < 60000`,
		`DELETE FROM task_modules WHERE task_id >= 50000 AND task_id < 60000 OR id >= 50000 AND id < 60000`,
		`DELETE FROM task_details WHERE task_id >= 50000 AND task_id < 60000`,
		`DELETE FROM tasks WHERE id >= 50000 AND id < 60000`,
		`DELETE FROM users WHERE id >= 50000 AND id < 60000`,
	}
	for _, q := range execs {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("cleanup query %q: %v", q, err)
		}
	}
}

func assertR6AuditClean(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := map[string]string{
		"users":              `id >= 50000 AND id < 60000`,
		"tasks":              `id >= 50000 AND id < 60000`,
		"task_modules":       `id >= 50000 AND id < 60000 OR task_id >= 50000 AND task_id < 60000`,
		"task_module_events": `id >= 50000 AND id < 60000`,
		"task_assets":        `id >= 50000 AND id < 60000 OR task_id >= 50000 AND task_id < 60000 OR asset_id >= 50000 AND asset_id < 60000`,
		"task_drafts":        `id >= 50000 AND id < 60000 OR owner_user_id >= 50000 AND owner_user_id < 60000`,
		"notifications":      `id >= 50000 AND id < 60000 OR user_id >= 50000 AND user_id < 60000`,
		"permission_logs":    `id >= 50000 AND id < 60000 OR actor_id >= 50000 AND actor_id < 60000 OR target_user_id >= 50000 AND target_user_id < 60000`,
	}
	for table, where := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE ` + where).Scan(&n); err != nil {
			t.Fatalf("audit count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("audit table %s has %d rows in [50000,60000)", table, n)
		}
	}
	if tableExists(t, db, "task_asset_versions") {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM task_asset_versions WHERE id >= 50000 AND id < 60000`).Scan(&n); err != nil {
			t.Fatalf("audit count task_asset_versions: %v", err)
		}
		if n != 0 {
			t.Fatalf("audit table task_asset_versions has %d rows in [50000,60000)", n)
		}
	}
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&n); err != nil {
		t.Fatalf("check table %s exists: %v", table, err)
	}
	return n > 0
}

func requireR6ID(t *testing.T, id int64, label string) {
	t.Helper()
	if id < r6MinID || id >= r6MaxID {
		t.Fatalf("%s %d outside [%d,%d)", label, id, r6MinID, r6MaxID)
	}
}
