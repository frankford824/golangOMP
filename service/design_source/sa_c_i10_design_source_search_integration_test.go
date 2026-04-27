//go:build integration

package design_source

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

// SA-C-I10 — GET /v1/design-sources/search
// Asserts: when design_sources is absent in r3_test, search falls back to
// task_assets rows scoped to source_module_key='design'.
func TestSACI10_DesignSourceSearch_FallbackToTaskAssetsStub(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var tableName string
	err := db.QueryRowContext(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'design_sources'`).Scan(&tableName)
	if err != sql.ErrNoRows {
		t.Fatalf("design_sources table presence err=%v table=%q; SA-C-I10 expects r3_test fallback path", err, tableName)
	}

	taskID := int64(40410)
	userID := int64(40018)
	r35.CleanupTaskIDs(t, db, taskID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM task_assets WHERE task_id = ?`, taskID)
		r35.CleanupTaskIDs(t, db, taskID)
	})
	r35.InsertTaskWithModules(t, db, taskID, "customization", "normal", []r35.ModuleFixture{{Key: "design", State: "done", PoolTeamCode: r35.StrPtr("淘系一组")}})
	moduleID := r35.MustModuleID(t, db, taskID, "design")
	fileName := fmt.Sprintf("sac-i10-design-source-%d.psd", taskID)
	res, err := db.ExecContext(ctx, `
		INSERT INTO task_assets
			(task_id, asset_type, version_no, asset_version_no, upload_mode,
			 file_name, original_filename, mime_type, file_size, file_path, storage_key,
			 upload_status, preview_status, uploaded_by, uploaded_at, remark,
			 source_module_key, source_task_module_id, is_archived, cleaned_at, deleted_at)
		VALUES (?, 'source', 1, 1, 'multipart',
			?, ?, 'application/octet-stream', 123, ?, ?,
			'uploaded', 'pending', ?, NOW(6), 'sac-i10',
			'design', ?, 0, NULL, NULL)`,
		taskID, fileName, fileName, "tasks/sac-i10/source.psd", "tasks/sac-i10/source.psd", userID, moduleID)
	if err != nil {
		t.Fatalf("insert design fallback task_asset: %v", err)
	}
	assetID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("task_asset last id: %v", err)
	}

	svc := NewService(mysqlrepo.NewDesignSourceRepo(mysqlrepo.New(db)))
	items, total, appErr := svc.Search(ctx, domain.RequestActor{ID: userID}, "sac-i10-design-source", 1, 20)
	if appErr != nil {
		t.Fatalf("Search appErr=%+v", appErr)
	}
	if total < 1 || len(items) < 1 {
		t.Fatalf("Search total=%d len=%d want fallback result", total, len(items))
	}
	if items[0].ID != assetID || items[0].FileName != fileName || items[0].OriginTaskID == nil || *items[0].OriginTaskID != taskID {
		t.Fatalf("fallback item=%+v want id=%d file=%s task=%d", items[0], assetID, fileName, taskID)
	}
}
