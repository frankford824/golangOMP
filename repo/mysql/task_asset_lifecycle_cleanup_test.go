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

func TestListResourceDeletionStorageKeysIncludesDerivedObjects(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT DISTINCT COALESCE\(ta\.storage_key, ''\)`).
		WithArgs(int64(12401), int64(12401)).
		WillReturnRows(sqlmock.NewRows([]string{"storage_key"}).
			AddRow("tasks/RW-1/delivery-B.psd").
			AddRow("tasks/RW-1/previews/delivery-B-preview.webp").
			AddRow("tasks/RW-1/previews/delivery-B-thumb.webp"))

	keys, err := NewTaskAssetLifecycleRepo(New(db)).(*taskAssetLifecycleRepo).
		ListResourceDeletionStorageKeys(t.Context(), 12401)
	if err != nil {
		t.Fatalf("ListResourceDeletionStorageKeys() error = %v", err)
	}
	want := []string{
		"tasks/RW-1/delivery-B.psd",
		"tasks/RW-1/previews/delivery-B-preview.webp",
		"tasks/RW-1/previews/delivery-B-thumb.webp",
	}
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
