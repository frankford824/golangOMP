package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTaskDetailBundleResolvesFormalizedReferenceMetadata(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected != "task-detail-reference-binding" {
			return fmt.Errorf("unexpected expectation %q", expected)
		}
		for _, token := range []string{
			"task_reference_asset_bindings ref_binding",
			"CONVERT(ref_binding.ref_id USING utf8mb4) COLLATE utf8mb4_unicode_ci",
			"CONVERT(rfr.ref_id USING utf8mb4) COLLATE utf8mb4_unicode_ci",
			"bound_asset.id = ref_binding.task_asset_id",
			"bound_storage.ref_id = bound_asset.storage_ref_id",
			"NULLIF(bound_asset.original_filename, '')",
			"bound_asset.upload_status = 'uploaded'",
		} {
			if !strings.Contains(actual, token) {
				return fmt.Errorf("task detail query missing %q", token)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repository := &taskRepo{db: New(db)}
	mock.ExpectQuery("task-detail-reference-binding").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	bundle, err := repository.GetTaskDetailReadBundle(context.Background(), 3772, 50)
	if err != nil {
		t.Fatalf("GetTaskDetailReadBundle() error = %v", err)
	}
	if bundle != nil {
		t.Fatalf("bundle = %+v, want nil for empty task fixture", bundle)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
