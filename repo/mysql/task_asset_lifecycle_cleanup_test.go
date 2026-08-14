package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestResolveOrCreateLifecycleEventModuleReturnsResolvedModuleID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mysqlDB := New(db)
	lifecycleRepo := NewTaskAssetLifecycleRepo(mysqlDB).(*taskAssetLifecycleRepo)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO task_modules`).
		WithArgs(int64(539), "design").
		WillReturnResult(sqlmock.NewResult(8711, 1))
	mock.ExpectCommit()

	var moduleID int64
	err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		moduleID, err = lifecycleRepo.ResolveOrCreateLifecycleEventModule(context.Background(), tx, 539, "design")
		return err
	})
	if err != nil {
		t.Fatalf("ResolveOrCreateLifecycleEventModule() error = %v", err)
	}
	if moduleID != 8711 {
		t.Fatalf("moduleID = %d, want 8711", moduleID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestTaskAssetLifecycleRepoCleanupUsesAssetAgeAndExcludesCurrentVersion(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "cleanup-candidate-asset-age-current-guard" {
			return fmt.Errorf("unexpected expected SQL marker %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, required := range []string{
			"COALESCE(ta.uploaded_at, ta.created_at)",
			"COALESCE(ta.uploaded_at, ta.created_at) < ?",
			"ta.id <> COALESCE(( SELECT da.current_version_id FROM design_assets da WHERE da.id = ta.asset_id ), 0)",
			"ORDER BY COALESCE(ta.uploaded_at, ta.created_at) ASC",
		} {
			if !strings.Contains(normalized, required) {
				return fmt.Errorf("cleanup query missing %q: %s", required, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	assetTimestamp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("cleanup-candidate-asset-age-current-guard").
		WithArgs("Completed", "Cancelled", "Archived", sqlmock.AnyArg(), 25).
		WillReturnRows(sqlmock.NewRows([]string{
			"asset_id", "version_id", "task_id", "source_task_module_id", "storage_key", "source_module_key", "asset_timestamp",
		}).AddRow(int64(31), int64(77), int64(21), int64(91), "tasks/21/old.psd", "design", assetTimestamp))

	candidates, err := NewTaskAssetLifecycleRepo(New(db)).ListEligibleForCleanup(t.Context(), time.Now().UTC(), 25)
	if err != nil {
		t.Fatalf("ListEligibleForCleanup() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].VersionID != 77 || !candidates[0].TaskUpdatedAt.Equal(assetTimestamp) {
		t.Fatalf("candidates = %+v", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestLockCleanupObjectIDsLocksRootAndDerivedVersions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskAssetLifecycleRepo(New(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id\\s+FROM task_assets").WithArgs(int64(101), int64(101)).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(int64(101)).AddRow(int64(102)),
	)
	mock.ExpectQuery(`SELECT EXISTS[\s\S]+task_asset_group_revisions`).
		WithArgs(int64(101), int64(102), int64(101), int64(102), int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows([]string{"referenced"}).AddRow(false))
	mock.ExpectCommit()
	var ids []int64
	err = New(db).RunInTx(context.Background(), func(tx repo.Tx) error {
		var lockErr error
		ids, lockErr = repository.LockCleanupObjectIDs(context.Background(), tx, 101)
		return lockErr
	})
	if err != nil {
		t.Fatalf("LockCleanupObjectIDs() error = %v", err)
	}
	if len(ids) != 2 || ids[0] != 101 || ids[1] != 102 {
		t.Fatalf("locked ids = %v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLockCleanupObjectIDsRejectsRevisionReferencedAssets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskAssetLifecycleRepo(New(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id\\s+FROM task_assets").WithArgs(int64(201), int64(201)).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(int64(201)).AddRow(int64(202)),
	)
	mock.ExpectQuery(`SELECT EXISTS[\s\S]+task_asset_group_revisions`).
		WithArgs(int64(201), int64(202), int64(201), int64(202), int64(201), int64(202)).
		WillReturnRows(sqlmock.NewRows([]string{"referenced"}).AddRow(true))
	mock.ExpectRollback()

	err = New(db).RunInTx(context.Background(), func(tx repo.Tx) error {
		_, lockErr := repository.LockCleanupObjectIDs(context.Background(), tx, 201)
		return lockErr
	})
	if err != repo.ErrConflict {
		t.Fatalf("LockCleanupObjectIDs() error = %v, want conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnqueueObjectDeletionsSnapshotsResourceAndDerivedObjectsInTransaction(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "enqueue-resource-and-derived-object-deletions" {
			return fmt.Errorf("unexpected expected SQL marker %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		for _, required := range []string{
			"task_asset_id, storage_ref_id, storage_adapter, storage_is_placeholder, storage_key, dedupe_key",
			"CONCAT('asset-delete:task-asset:', object_keys.task_asset_id, ':', SHA2(object_keys.storage_key, 256))",
			"TRIM(ta.storage_key) AS storage_key",
			"LEFT JOIN asset_storage_refs storage_ref ON storage_ref.ref_id = ta.storage_ref_id",
			"JOIN asset_storage_refs storage_ref ON storage_ref.ref_id = ta.storage_ref_id",
			"COALESCE(NULLIF(TRIM(storage_ref.storage_adapter), ''), 'unknown') AS storage_adapter",
			"COALESCE(storage_ref.is_placeholder, 0) AS storage_is_placeholder",
			"ta.id IN (?,?,?)",
			"ta.deleted_at IS NULL",
		} {
			if !strings.Contains(normalized, required) {
				return fmt.Errorf("outbox query missing %q: %s", required, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("enqueue-resource-and-derived-object-deletions").
		WithArgs(int64(901), int64(902), int64(903), int64(901), int64(902), int64(903)).
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectCommit()

	mysqlDB := New(db)
	lifecycleRepo := NewTaskAssetLifecycleRepo(mysqlDB)
	err = mysqlDB.RunInTx(t.Context(), func(tx repo.Tx) error {
		return lifecycleRepo.EnqueueObjectDeletions(t.Context(), tx, []int64{901, 902, 903})
	})
	if err != nil {
		t.Fatalf("EnqueueObjectDeletions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestLockGenericDeleteGuardLocksHistoricalRevisionAndPublicationReferences(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id[\s\S]+FROM design_assets[\s\S]+source_asset_id`).
		WithArgs(int64(12402), int64(12402)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(int64(12402)).
			AddRow(int64(12403)))
	mock.ExpectQuery(`SELECT ta\.id, ta\.binding_state, ta\.bound_group_id, ta\.bound_role`).
		WithArgs(int64(12402), int64(12403)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "binding_state", "bound_group_id", "bound_role"}).
			AddRow(int64(901), "staged", nil, nil).
			AddRow(int64(902), "staged", nil, nil))
	mock.ExpectQuery(`SELECT id FROM task_asset_group_revisions WHERE source_task_asset_id IN`).
		WithArgs(int64(901), int64(902)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(701)))
	mock.ExpectQuery(`SELECT revision_id FROM task_asset_group_revision_items WHERE task_asset_id IN`).
		WithArgs(int64(901), int64(902)).
		WillReturnRows(sqlmock.NewRows([]string{"revision_id"}))
	mock.ExpectQuery(`SELECT revision_id FROM task_asset_group_revision_references WHERE formal_task_asset_id IN`).
		WithArgs(int64(901), int64(902)).
		WillReturnRows(sqlmock.NewRows([]string{"revision_id"}))
	mock.ExpectQuery(`SELECT id[\s\S]+FROM asset_workbench_client_materials`).
		WithArgs(int64(12402), int64(12403)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(801)))
	mock.ExpectCommit()

	mysqlDB := New(db)
	lifecycleRepo := NewTaskAssetLifecycleRepo(mysqlDB)
	var guard *repo.TaskAssetDeleteGuard
	err = mysqlDB.RunInTx(t.Context(), func(tx repo.Tx) error {
		var err error
		guard, err = lifecycleRepo.LockGenericDeleteGuard(t.Context(), tx, 12402)
		return err
	})
	if err != nil {
		t.Fatalf("LockGenericDeleteGuard() error = %v", err)
	}
	if !guard.AllStagedUnbound || fmt.Sprint(guard.DesignAssetIDs) != "[12402 12403]" || fmt.Sprint(guard.TaskAssetIDs) != "[901 902]" || fmt.Sprint(guard.RevisionReferenceIDs) != "[701]" || fmt.Sprint(guard.PublicationPinIDs) != "[801]" {
		t.Fatalf("guard = %+v", guard)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestObjectDeletionOutboxClaimReclaimsExpiredLeaseAndIncrementsAttempt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT deletion.id,[\s\S]+retain_physical_object[\s\S]+FOR UPDATE SKIP LOCKED`).
		WithArgs(now, now, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_asset_id", "storage_ref_id", "storage_adapter", "storage_is_placeholder", "storage_key", "attempt", "retain_physical_object"}).
			AddRow(int64(1), int64(101), "ref-101", "oss_upload_service", false, "tasks/1/source.psd", 0, true).
			AddRow(int64(2), int64(102), nil, "placeholder_storage", true, "tasks/1/preview.webp", 2, false))
	mock.ExpectExec(`UPDATE asset_object_deletion_outbox[\s\S]+attempt = attempt \+ 1`).
		WithArgs("lease-1", leaseUntil, int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	mysqlDB := New(db)
	repository := NewTaskAssetLifecycleRepo(mysqlDB).(repo.AssetObjectDeletionOutboxRepo)
	var items []repo.AssetObjectDeletionOutboxItem
	err = mysqlDB.RunInTx(t.Context(), func(tx repo.Tx) error {
		var err error
		items, err = repository.ClaimObjectDeletions(t.Context(), tx, "lease-1", now, leaseUntil, 50)
		return err
	})
	if err != nil {
		t.Fatalf("ClaimObjectDeletions() error = %v", err)
	}
	if len(items) != 2 || items[0].Attempt != 1 || items[1].Attempt != 3 || items[0].StorageAdapter != "oss_upload_service" || items[0].StorageRefID == nil || *items[0].StorageRefID != "ref-101" || !items[0].RetainPhysicalObject || items[1].RetainPhysicalObject || !items[1].StorageIsPlaceholder {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestObjectDeletionOutboxSuccessMarksTaskAssetObjectDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	taskAssetID := int64(101)
	item := repo.AssetObjectDeletionOutboxItem{ID: 1, TaskAssetID: &taskAssetID, StorageKey: "tasks/1/source.psd", Attempt: 1}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE asset_object_deletion_outbox[\s\S]+status = 'succeeded'`).
		WithArgs(item.ID, "lease-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE task_assets[\s\S]+object_deleted_at = COALESCE`).
		WithArgs(now, taskAssetID, taskAssetID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mysqlDB := New(db)
	repository := NewTaskAssetLifecycleRepo(mysqlDB).(repo.AssetObjectDeletionOutboxRepo)
	err = mysqlDB.RunInTx(t.Context(), func(tx repo.Tx) error {
		return repository.MarkObjectDeletionSucceeded(t.Context(), tx, item, "lease-1", now)
	})
	if err != nil {
		t.Fatalf("MarkObjectDeletionSucceeded() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
