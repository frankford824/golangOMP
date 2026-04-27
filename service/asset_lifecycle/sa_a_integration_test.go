//go:build integration

package asset_lifecycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	assetcenter "workflow/service/asset_center"
	"workflow/testsupport/r35"
)

func TestSA_A_I1_SearchDefaultStatesActiveOrClosedRetained(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	center, _, _ := newIntegrationServices(db)
	res, appErr := center.Search(context.Background(), domain.AssetSearchQuery{IsArchived: domain.AssetArchiveFilterFalse, Page: 1, Size: 100})
	if appErr != nil {
		t.Fatalf("search: %v", appErr)
	}
	for _, item := range res.Items {
		if item.LifecycleState != domain.AssetLifecycleStateActive && item.LifecycleState != domain.AssetLifecycleStateClosedRetained {
			t.Fatalf("lifecycle_state = %s, want active/closed_retained", item.LifecycleState)
		}
	}
}

func TestSA_A_I2_SearchAllSeesArchivedAsset(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20002, "design", fixtureOptions{Archived: true})
	defer cleanupSAATask(t, db, fixture.TaskID)
	center, _, _ := newIntegrationServices(db)
	res, appErr := center.Search(context.Background(), domain.AssetSearchQuery{IsArchived: domain.AssetArchiveFilterAll, Page: 1, Size: 100})
	if appErr != nil {
		t.Fatalf("search all: %v", appErr)
	}
	for _, item := range res.Items {
		if item.ID == fixture.AssetID && item.LifecycleState == domain.AssetLifecycleStateArchived {
			return
		}
	}
	t.Fatalf("archived asset %d not found in all search", fixture.AssetID)
}

func TestSA_A_I3_ArchiveRoleAndEvent(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20003, "design", fixtureOptions{})
	defer cleanupSAATask(t, db, fixture.TaskID)
	_, lifecycle, _ := newIntegrationServices(db)
	member := domain.RequestActor{ID: 3001, Roles: []domain.Role{domain.RoleMember}}
	if appErr := lifecycle.Archive(context.Background(), member, fixture.AssetID, "nope"); appErr == nil || appErr.Code != domain.DenyModuleActionRoleDenied {
		t.Fatalf("member archive error = %#v, want module_action_role_denied", appErr)
	}
	admin := domain.RequestActor{ID: 3002, Roles: []domain.Role{domain.RoleSuperAdmin}}
	if appErr := lifecycle.Archive(context.Background(), admin, fixture.AssetID, "archive integration"); appErr != nil {
		t.Fatalf("super archive: %v", appErr)
	}
	assertEventCount(t, db, fixture.ModuleID, "asset_archived_by_admin", 1)
}

func TestSA_A_I4_RestoreClearsArchiveAndWritesEvent(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20004, "design", fixtureOptions{Archived: true})
	defer cleanupSAATask(t, db, fixture.TaskID)
	_, lifecycle, _ := newIntegrationServices(db)
	admin := domain.RequestActor{ID: 3003, Roles: []domain.Role{domain.RoleSuperAdmin}}
	if appErr := lifecycle.Restore(context.Background(), admin, fixture.AssetID); appErr != nil {
		t.Fatalf("restore: %v", appErr)
	}
	var archived int
	var archivedAt sql.NullTime
	if err := db.QueryRow(`SELECT is_archived, archived_at FROM task_assets WHERE id=?`, fixture.VersionID).Scan(&archived, &archivedAt); err != nil {
		t.Fatalf("select archive state: %v", err)
	}
	if archived != 0 || archivedAt.Valid {
		t.Fatalf("archive state = %d/%v, want 0/null", archived, archivedAt.Valid)
	}
	assertEventCount(t, db, fixture.ModuleID, "asset_unarchived_by_admin", 1)
}

func TestSA_A_I5_DeleteSoftDeletesAndWritesEvent(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20005, "design", fixtureOptions{})
	defer cleanupSAATask(t, db, fixture.TaskID)
	_, lifecycle, _ := newIntegrationServices(db)
	admin := domain.RequestActor{ID: 3004, Roles: []domain.Role{domain.RoleSuperAdmin}}
	if appErr := lifecycle.Delete(context.Background(), admin, fixture.AssetID, "delete integration"); appErr != nil {
		t.Fatalf("delete: %v", appErr)
	}
	var deletedAt sql.NullTime
	var storageKey sql.NullString
	if err := db.QueryRow(`SELECT deleted_at, storage_key FROM task_assets WHERE id=?`, fixture.VersionID).Scan(&deletedAt, &storageKey); err != nil {
		t.Fatalf("select delete state: %v", err)
	}
	if !deletedAt.Valid || storageKey.Valid {
		t.Fatalf("delete state deleted_at=%v storage_key_valid=%v, want deleted/null", deletedAt.Valid, storageKey.Valid)
	}
	assertEventCount(t, db, fixture.ModuleID, "asset_deleted_by_admin", 1)
}

func TestSA_A_I6_DownloadAutoCleanedGoneDeletedNotFound(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	cleaned := insertAssetFixture(t, db, 20006, "design", fixtureOptions{Cleaned: true})
	deleted := insertAssetFixture(t, db, 20007, "design", fixtureOptions{Deleted: true})
	defer cleanupSAATask(t, db, cleaned.TaskID)
	defer cleanupSAATask(t, db, deleted.TaskID)
	center, _, _ := newIntegrationServices(db)
	if _, appErr := center.DownloadLatest(context.Background(), cleaned.AssetID); appErr == nil || appErr.Code != assetcenter.ErrCodeAssetGone {
		t.Fatalf("cleaned download error = %#v, want gone", appErr)
	}
	if _, appErr := center.DownloadLatest(context.Background(), deleted.AssetID); appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("deleted download error = %#v, want not found", appErr)
	}
}

func TestSA_A_I7_VersionGoneButAuditSnapshotReadable(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20008, "design", fixtureOptions{Cleaned: true})
	defer cleanupSAATask(t, db, fixture.TaskID)
	snapshot := map[string]interface{}{"asset_versions_snapshot": []map[string]interface{}{{"asset_id": fixture.AssetID, "version_id": fixture.VersionID, "version_no": 1, "storage_key": fixture.StorageKey}}}
	raw, _ := json.Marshal(snapshot)
	if _, err := db.Exec(`INSERT INTO task_module_events (task_module_id, event_type, payload) VALUES (?, 'audit_snapshot_test', CAST(? AS JSON))`, fixture.ModuleID, string(raw)); err != nil {
		t.Fatalf("insert snapshot event: %v", err)
	}
	center, _, _ := newIntegrationServices(db)
	if _, appErr := center.DownloadVersion(context.Background(), fixture.AssetID, fixture.VersionID); appErr == nil || appErr.Code != assetcenter.ErrCodeAssetGone {
		t.Fatalf("version download error = %#v, want gone", appErr)
	}
	var got string
	if err := db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(payload, '$.asset_versions_snapshot[0].storage_key')) FROM task_module_events WHERE task_module_id=? AND event_type='audit_snapshot_test' ORDER BY id DESC LIMIT 1`, fixture.ModuleID).Scan(&got); err != nil {
		t.Fatalf("read snapshot payload: %v", err)
	}
	if got != fixture.StorageKey {
		t.Fatalf("snapshot storage_key = %q, want %q", got, fixture.StorageKey)
	}
}

func TestSA_A_I8_CleanupJobDryRunRealRunIdempotent(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()
	fixture := insertAssetFixture(t, db, 20009, "design", fixtureOptions{})
	defer cleanupSAATask(t, db, fixture.TaskID)
	if _, err := db.Exec(`UPDATE tasks SET task_status='Completed', updated_at = NOW(6) - INTERVAL 400 DAY WHERE id=?`, fixture.TaskID); err != nil {
		t.Fatalf("mark task terminal old: %v", err)
	}
	_, _, cleanup := newIntegrationServices(db)
	dry, appErr := cleanup.Run(context.Background(), CleanupOptions{DryRun: true, Limit: 10})
	if appErr != nil {
		t.Fatalf("cleanup dry-run: %v", appErr)
	}
	if dry.Scanned == 0 || dry.Cleaned != 0 {
		t.Fatalf("dry-run scanned/cleaned = %d/%d, want >0/0", dry.Scanned, dry.Cleaned)
	}
	assertStoragePresent(t, db, fixture.VersionID, true)
	real, appErr := cleanup.Run(context.Background(), CleanupOptions{Limit: 10})
	if appErr != nil {
		t.Fatalf("cleanup real: %v", appErr)
	}
	if real.Cleaned == 0 {
		t.Fatalf("real cleanup cleaned = 0")
	}
	assertStoragePresent(t, db, fixture.VersionID, false)
	assertEventCount(t, db, fixture.ModuleID, "asset_auto_cleaned", 1)
	again, appErr := cleanup.Run(context.Background(), CleanupOptions{Limit: 10})
	if appErr != nil {
		t.Fatalf("cleanup again: %v", appErr)
	}
	if again.Cleaned != 0 {
		t.Fatalf("cleanup idempotent cleaned = %d, want 0", again.Cleaned)
	}
}

type fixtureOptions struct {
	Archived bool
	Cleaned  bool
	Deleted  bool
}

type assetFixture struct {
	TaskID     int64
	AssetID    int64
	VersionID  int64
	ModuleID   int64
	StorageKey string
}

func newIntegrationServices(db *sql.DB) (*assetcenter.Service, *Service, *CleanupJob) {
	mysqlDB := mysqlrepo.New(db)
	searchRepo := mysqlrepo.NewTaskAssetSearchRepo(mysqlDB)
	lifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mysqlDB)
	return assetcenter.NewService(searchRepo, nil, nil),
		NewService(searchRepo, lifecycleRepo, mysqlDB, nil),
		NewCleanupJob(lifecycleRepo, mysqlDB, nil, nil)
}

func insertAssetFixture(t *testing.T, db *sql.DB, taskID int64, moduleKey string, opts fixtureOptions) assetFixture {
	t.Helper()
	cleanupSAATask(t, db, taskID)
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: moduleKey, State: string(domain.ModuleStateActive), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
	})
	moduleID := r35.MustModuleID(t, db, taskID, moduleKey)
	res, err := db.Exec(`INSERT INTO design_assets (task_id, asset_no, asset_type, created_by) VALUES (?, ?, 'source', 3000)`, taskID, fmt.Sprintf("SAA-%d", taskID))
	if err != nil {
		t.Fatalf("insert design_asset: %v", err)
	}
	assetID, _ := res.LastInsertId()
	storageKey := fmt.Sprintf("tasks/saa/%d/source.psd", taskID)
	var archivedAt interface{}
	var archivedBy interface{}
	isArchived := 0
	if opts.Archived || opts.Cleaned {
		isArchived = 1
		archivedAt = time.Now().UTC()
		archivedBy = int64(3000)
	}
	var cleanedAt interface{}
	if opts.Cleaned {
		cleanedAt = time.Now().UTC()
	}
	var deletedAt interface{}
	if opts.Deleted {
		deletedAt = time.Now().UTC()
	}
	storedKey := interface{}(storageKey)
	if opts.Cleaned || opts.Deleted {
		storedKey = nil
	}
	res, err = db.Exec(`
		INSERT INTO task_assets
			(task_id, asset_id, asset_type, version_no, asset_version_no, upload_mode, file_name, original_filename,
			 mime_type, file_size, file_path, storage_key, upload_status, preview_status, uploaded_by, uploaded_at, remark,
			 source_module_key, source_task_module_id, is_archived, archived_at, archived_by, cleaned_at, deleted_at)
		VALUES (?, ?, 'source', 1, 1, 'multipart', 'source.psd', 'source.psd',
			'application/octet-stream', 123, ?, ?, 'uploaded', 'pending', 3000, NOW(6), 'saa',
			?, ?, ?, ?, ?, ?, ?)`,
		taskID, assetID, storageKey, storedKey, moduleKey, moduleID, isArchived, archivedAt, archivedBy, cleanedAt, deletedAt)
	if err != nil {
		t.Fatalf("insert task_asset: %v", err)
	}
	versionID, _ := res.LastInsertId()
	if _, err := db.Exec(`UPDATE design_assets SET current_version_id=? WHERE id=?`, versionID, assetID); err != nil {
		t.Fatalf("update current_version_id: %v", err)
	}
	return assetFixture{TaskID: taskID, AssetID: assetID, VersionID: versionID, ModuleID: moduleID, StorageKey: storageKey}
}

func cleanupSAATask(t *testing.T, db *sql.DB, taskID int64) {
	t.Helper()
	_, _ = db.Exec(`DELETE e FROM task_module_events e JOIN task_modules m ON m.id=e.task_module_id WHERE m.task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM task_assets WHERE task_id=?`, taskID)
	_, _ = db.Exec(`DELETE FROM design_assets WHERE task_id=?`, taskID)
	r35.CleanupTaskIDs(t, db, taskID)
}

func assertEventCount(t *testing.T, db *sql.DB, moduleID int64, eventType string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_module_events WHERE task_module_id=? AND event_type=?`, moduleID, eventType).Scan(&got); err != nil {
		t.Fatalf("count event %s: %v", eventType, err)
	}
	if got != want {
		t.Fatalf("event %s count = %d, want %d", eventType, got, want)
	}
}

func assertStoragePresent(t *testing.T, db *sql.DB, versionID int64, wantPresent bool) {
	t.Helper()
	var storageKey sql.NullString
	var cleanedAt sql.NullTime
	if err := db.QueryRow(`SELECT storage_key, cleaned_at FROM task_assets WHERE id=?`, versionID).Scan(&storageKey, &cleanedAt); err != nil {
		t.Fatalf("select storage state: %v", err)
	}
	if storageKey.Valid != wantPresent {
		t.Fatalf("storage present = %t, want %t", storageKey.Valid, wantPresent)
	}
	if !wantPresent && !cleanedAt.Valid {
		t.Fatalf("cleaned_at is null after cleanup")
	}
}
