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
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "storage_ref_status", "formal_storage_ref_status", "formal_task_asset_active", "created_at"}))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111), int64(112)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key", "storage_ref_status"}).
			AddRow(111, "source-v1.psd", "application/octet-stream", 19, "tasks/8/source-v1.psd", "historical_unavailable").
			AddRow(112, "source-v2.psd", "application/octet-stream", 20, "tasks/8/source-v2.psd", "recorded"))

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
	if items[1].ID != 11 ||
		items[1].RevisionNo != 1 ||
		items[1].SourceTaskAssetID == nil ||
		items[1].SourceFile == nil ||
		items[1].SourceFile.Availability != domain.TaskResourceFileHistoricalUnavailable ||
		items[1].SourceFile.StorageKey != "" {
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
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "storage_ref_status", "formal_storage_ref_status", "formal_task_asset_active", "created_at"}))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key", "storage_ref_status"}))

	err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
	if err == nil || !strings.Contains(err.Error(), "expected 1 explicit active bound rows, got 0") {
		t.Fatalf("hydrateResourceGroupRevisions() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalRevisionHydrationRetainsUnavailableFileMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_asset_group_revisions`).WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id[\s\S]+FROM task_asset_group_revisions[\s\S]+ORDER BY revision_no DESC, id DESC LIMIT \? OFFSET \?`).
		WithArgs(int64(8), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "revision_no", "status", "mode", "source_task_asset_id", "source_stage", "created_by", "created_by_name", "reason", "submitted_at", "finalized_at", "created_at"}).
			AddRow(11, 8, 1, "superseded", "single", nil, "migration", 7, "管理员", "historical", now, now, now))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}).
			AddRow(21, 11, 12323, 0, "lost.psd", now))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "storage_ref_status", "formal_storage_ref_status", "formal_task_asset_active", "created_at"}))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(12323)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key", "storage_ref_status"}).
			AddRow(12323, "lost.psd", "application/octet-stream", 17755216, "legacy/lost.psd", "historical_unavailable"))

	items, _, err := repository.ListResourceGroupRevisions(context.Background(), 8, 1, 20)
	if err != nil {
		t.Fatalf("ListResourceGroupRevisions() error = %v", err)
	}
	file := items[0].Items[0].File
	if file == nil || file.Availability != domain.TaskResourceFileHistoricalUnavailable ||
		file.StorageKey != "" || file.UnavailableReason == "" {
		t.Fatalf("historical unavailable file = %+v", file)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevisionReferenceHydrationHonorsFormalTaskAssetTombstone(t *testing.T) {
	for _, test := range []struct {
		name         string
		historical   bool
		formalStatus string
		formalActive bool
		wantError    bool
	}{
		{name: "current pointer fails closed", formalStatus: "historical_unavailable", formalActive: true, wantError: true},
		{name: "historical page preserves tombstone metadata", historical: true, formalStatus: "historical_unavailable", formalActive: true},
		{name: "historical page fails closed for inactive formal alias", historical: true, formalStatus: "recorded", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			now := time.Now().UTC()
			revisionID := int64(31)
			groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

			mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "revision_no", "status", "mode", "source_task_asset_id", "source_stage", "created_by", "created_by_name", "reason", "submitted_at", "finalized_at", "created_at"}).
					AddRow(revisionID, 8, 1, "finalized", "single", nil, "migration", 7, "管理员", "historical", now, now, now))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "mime_type", "file_size", "storage_key", "storage_ref_status", "formal_storage_ref_status", "formal_task_asset_active", "created_at"}).
					AddRow(41, revisionID, 51, 9901, 0, "ref-snapshot", "lost-reference.png", "task", "image/png", 128, "tasks/8/reference.png", "recorded", test.formalStatus, test.formalActive, now))

			if test.historical {
				err = repository.hydrateHistoricalResourceGroupRevisions(context.Background(), groups)
			} else {
				err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			}
			if test.wantError {
				if err == nil {
					t.Fatal("hydration error = nil")
				}
			} else {
				if err != nil {
					t.Fatalf("historical hydration error = %v", err)
				}
				reference := groups[0].WorkingRevision.References[0]
				if reference.Availability != domain.TaskResourceFileHistoricalUnavailable ||
					reference.UnavailableReason == "" || reference.StorageKey != "" {
					t.Fatalf("historical reference tombstone = %+v", reference)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
