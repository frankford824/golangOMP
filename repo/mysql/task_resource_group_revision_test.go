package mysqlrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestListResourceGroupRevisionsPagesNewestFirstAndHydratesOnlyPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_asset_group_revisions`).WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT id[\s\S]+FROM task_asset_group_revisions[\s\S]+ORDER BY revision_no DESC, id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(8), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12).AddRow(11))
	mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).
		WithArgs(int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "revision_no", "status", "mode", "source_task_asset_id", "source_stage", "created_by", "created_by_name", "reason", "submitted_at", "finalized_at", "created_at"}).
			AddRow(11, 8, 1, "superseded", "single", 111, "design", 7, "设计师", "first", now, nil, now).
			AddRow(12, 8, 2, "finalized", "set", 112, "audit", 9, "审核员", "approved", now, now, now))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "created_at"}))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111), int64(112)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key"}).
			AddRow(112, "source-v2.psd", "application/octet-stream", 20, "tasks/8/source-v2.psd"))

	items, total, err := repository.ListResourceGroupRevisions(context.Background(), 8, 1, 20)
	if err != nil {
		t.Fatalf("ListResourceGroupRevisions() error = %v", err)
	}
	if total != 2 || len(items) != 2 || items[0].ID != 12 || items[1].ID != 11 {
		t.Fatalf("items/total = %+v/%d", items, total)
	}
	if items[0].SourceFile == nil || items[0].SourceFile.TaskAssetID != 112 {
		t.Fatalf("newest source hydration = %+v", items[0].SourceFile)
	}
	if items[0].CreatedByName != "审核员" || items[1].CreatedByName != "设计师" {
		t.Fatalf("revision actor names = %q/%q", items[0].CreatedByName, items[1].CreatedByName)
	}
	if items[1].ID != 11 || items[1].RevisionNo != 1 || items[1].SourceTaskAssetID == nil || items[1].SourceFile != nil {
		t.Fatalf("historical revision metadata/file = %+v/%+v", items[1], items[1].SourceFile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentRevisionHydrationRejectsMissingOrInactiveFiles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	now := time.Now().UTC()
	revisionID := int64(21)
	groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

	mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "revision_no", "status", "mode", "source_task_asset_id", "source_stage", "created_by", "created_by_name", "reason", "submitted_at", "finalized_at", "created_at"}).
			AddRow(revisionID, 8, 1, "submitted", "single", 111, "design", 7, "设计师", "submitted", now, nil, now))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "created_at"}))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key"}))

	err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
	if err == nil || !strings.Contains(err.Error(), "expected 1 active bound files, got 0") {
		t.Fatalf("hydrateResourceGroupRevisions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
