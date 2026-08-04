package mysqlrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func revisionHydrationColumns() []string {
	return []string{
		"id", "group_id", "revision_no", "status", "mode",
		"source_task_asset_id", "source_stage", "created_by",
		"created_by_name", "reason", "submitted_at", "finalized_at",
		"created_at", "task_id", "scope_kind", "task_sku_item_id",
		"retouch_requirement_id",
	}
}

func revisionReferenceHydrationColumns() []string {
	return []string{
		"id", "revision_id", "reference_file_ref_id",
		"formal_task_asset_id", "sort_order", "ref_id_snapshot",
		"file_name_snapshot", "scope_snapshot", "mime_type", "file_size",
		"storage_key", "storage_ref_status", "formal_storage_ref_status",
		"formal_task_asset_active", "created_at", "reference_task_id",
		"reference_sku_item_id", "reference_retouch_requirement_id",
		"reference_ref_id", "formal_task_id", "formal_binding_state",
		"formal_asset_type", "formal_bound_role", "formal_storage_ref_id",
		"snapshot_storage_ref_id",
	}
}

func revisionFileHydrationColumns() []string {
	return []string{
		"id", "file_name", "mime_type", "file_size", "storage_key",
		"storage_ref_status", "file_active", "task_id", "asset_type",
		"bound_group_id", "bound_role",
	}
}

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
		WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
			AddRow(11, 8, 1, "superseded", "single", 111, "design", 7, "设计师", "first", now, nil, now, 80, "task", nil, nil).
			AddRow(12, 8, 2, "finalized", "set", 112, "audit", 9, "审核员", "approved", now, now, now, 80, "task", nil, nil))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111), int64(112)).
		WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()).
			AddRow(111, "source-v1.psd", "application/octet-stream", 19, "tasks/8/source-v1.psd", "historical_unavailable", false, 80, "source", 8, "source").
			AddRow(112, "source-v2.psd", "application/octet-stream", 20, "tasks/8/source-v2.psd", "recorded", true, 80, "source", 8, "source"))

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
		WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
			AddRow(revisionID, 8, 1, "submitted", "single", 111, "design", 7, "设计师", "submitted", now, nil, now, 80, "task", nil, nil))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111)).
		WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()))

	err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
	if err == nil || !errors.Is(err, repo.ErrDataIntegrity) || !strings.Contains(err.Error(), "expected 1 explicit active bound rows, got 0") {
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
		WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
			AddRow(11, 8, 1, "superseded", "single", nil, "migration", 7, "管理员", "historical", now, now, now, 80, "task", nil, nil))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}).
			AddRow(21, 11, 12323, 0, "lost.psd", now))
	mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
	mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(12323)).
		WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()).
			AddRow(12323, "lost.psd", "application/octet-stream", 17755216, "legacy/lost.psd", "historical_unavailable", false, 80, "delivery", 8, "final"))

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
		{name: "current pointer fails closed", formalStatus: "historical_unavailable", formalActive: false, wantError: true},
		{name: "historical page preserves tombstone metadata", historical: true, formalStatus: "historical_unavailable", formalActive: false},
		{name: "historical page preserves archived metadata", historical: true, formalStatus: "archived", formalActive: false},
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
				WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
					AddRow(revisionID, 8, 1, "finalized", "single", nil, "migration", 7, "管理员", "historical", now, now, now, 80, "task", nil, nil))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()).
					AddRow(41, revisionID, 51, 9901, 0, "ref-snapshot", "lost-reference.png", "task", "image/png", 128, "tasks/8/reference.png", "recorded", test.formalStatus, test.formalActive, now, 80, nil, nil, "ref-snapshot", 80, "bound", "reference", nil, "ref-snapshot", "ref-snapshot"))

			if test.historical {
				err = repository.hydrateHistoricalResourceGroupRevisions(context.Background(), groups)
			} else {
				err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			}
			if test.wantError {
				if err == nil || !errors.Is(err, repo.ErrDataIntegrity) {
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

func TestRevisionHydrationRejectsCrossGroupAssetOwnership(t *testing.T) {
	for _, test := range []struct {
		name        string
		sourceTask  int64
		sourceGroup int64
		sourceRole  string
		finalTask   int64
		finalGroup  int64
		finalRole   string
	}{
		{
			name:       "source belongs to another group",
			sourceTask: 80, sourceGroup: 9, sourceRole: "source",
			finalTask: 80, finalGroup: 8, finalRole: "final",
		},
		{
			name:       "final belongs to another task",
			sourceTask: 80, sourceGroup: 8, sourceRole: "source",
			finalTask: 81, finalGroup: 8, finalRole: "final",
		},
		{
			name:       "final has source role",
			sourceTask: 80, sourceGroup: 8, sourceRole: "source",
			finalTask: 80, finalGroup: 8, finalRole: "source",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			now := time.Now().UTC()
			revisionID := int64(61)
			groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

			mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
					AddRow(revisionID, 8, 1, "finalized", "single", 111, "design", 7, "审核员", "", now, now, now, 80, "task", nil, nil))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}).
					AddRow(71, revisionID, 112, 0, "final.png", now))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
			mock.ExpectQuery(`FROM task_assets ta`).WithArgs(int64(111), int64(112)).
				WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()).
					AddRow(111, "source.psd", "application/octet-stream", 10, "source.psd", "recorded", true, test.sourceTask, "source", test.sourceGroup, test.sourceRole).
					AddRow(112, "final.png", "image/png", 10, "final.png", "recorded", true, test.finalTask, "delivery", test.finalGroup, test.finalRole))

			err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			if !errors.Is(err, repo.ErrDataIntegrity) {
				t.Fatalf("hydration error = %v, want ErrDataIntegrity", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRevisionHydrationRejectsReferenceScopeOrSnapshotMismatch(t *testing.T) {
	for _, test := range []struct {
		name          string
		referenceTask int64
		referenceSKU  interface{}
		actualRefID   string
		scopeSnapshot string
	}{
		{name: "cross task", referenceTask: 81, referenceSKU: int64(801), actualRefID: "ref-snapshot", scopeSnapshot: "sku:801"},
		{name: "cross sku", referenceTask: 80, referenceSKU: int64(802), actualRefID: "ref-snapshot", scopeSnapshot: "sku:801"},
		{name: "snapshot ref drift", referenceTask: 80, referenceSKU: int64(801), actualRefID: "other-ref", scopeSnapshot: "sku:801"},
		{name: "snapshot scope drift", referenceTask: 80, referenceSKU: int64(801), actualRefID: "ref-snapshot", scopeSnapshot: "sku:802"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			now := time.Now().UTC()
			revisionID := int64(81)
			groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

			mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
					AddRow(revisionID, 8, 1, "finalized", "single", nil, "migration", 7, "管理员", "", now, now, now, 80, "sku", 801, nil))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()).
					AddRow(91, revisionID, 92, nil, 0, "ref-snapshot", "reference.png", test.scopeSnapshot, "image/png", 128, "reference.png", "recorded", "", true, now, test.referenceTask, test.referenceSKU, nil, test.actualRefID, nil, nil, nil, nil, nil, "ref-snapshot"))

			err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			if !errors.Is(err, repo.ErrDataIntegrity) {
				t.Fatalf("hydration error = %v, want ErrDataIntegrity", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRevisionHydrationRejectsMissingReferenceFileRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	now := time.Now().UTC()
	revisionID := int64(95)
	groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

	mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
			AddRow(revisionID, 8, 1, "finalized", "single", nil, "migration", 7, "管理员", "", now, now, now, 80, "task", nil, nil))
	mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
	mock.ExpectQuery(`FROM task_asset_group_revision_references rr[\s\S]+LEFT JOIN reference_file_refs f`).
		WithArgs(revisionID).
		WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()).
			AddRow(
				96, revisionID, 97, nil, 0,
				"missing-ref", "missing.png", "task", "image/png", 128,
				"tasks/80/missing.png", "recorded", "", true, now,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, "missing-ref",
			))

	err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
	if !errors.Is(err, repo.ErrDataIntegrity) {
		t.Fatalf("hydration error = %v, want ErrDataIntegrity", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalRevisionHydrationRetainsRevokedOrArchivedFileMetadata(t *testing.T) {
	for _, test := range []struct {
		name          string
		historical    bool
		storageStatus string
		fileActive    bool
		wantError     bool
	}{
		{name: "current revoked source fails closed", storageStatus: "recorded", fileActive: false, wantError: true},
		{name: "historical revoked source is metadata only", historical: true, storageStatus: "recorded", fileActive: false},
		{name: "current archived source fails closed", storageStatus: "archived", fileActive: false, wantError: true},
		{name: "historical archived source is metadata only", historical: true, storageStatus: "archived", fileActive: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			now := time.Now().UTC()
			revisionID := int64(101)
			groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

			mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
					AddRow(revisionID, 8, 1, "superseded", "single", 111, "audit", 7, "审核员", "", now, now, now, 80, "task", nil, nil))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
			mock.ExpectQuery(`CASE[\s\S]+ta\.is_archived[\s\S]+ta\.access_revoked_at[\s\S]+FROM task_assets ta`).WithArgs(int64(111)).
				WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()).
					AddRow(111, "superseded-source.psd", "application/octet-stream", 32, "tasks/80/superseded-source.psd", test.storageStatus, test.fileActive, 80, "source", 8, "source"))

			if test.historical {
				err = repository.hydrateHistoricalResourceGroupRevisions(context.Background(), groups)
			} else {
				err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			}
			if test.wantError {
				if !errors.Is(err, repo.ErrDataIntegrity) {
					t.Fatalf("hydration error = %v, want ErrDataIntegrity", err)
				}
			} else {
				if err != nil {
					t.Fatalf("historical hydration error = %v", err)
				}
				file := groups[0].WorkingRevision.SourceFile
				if file == nil ||
					file.FileName != "superseded-source.psd" ||
					file.Availability != domain.TaskResourceFileHistoricalUnavailable ||
					file.StorageKey != "" {
					t.Fatalf("historical metadata-only file = %+v", file)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRevisionHydrationRejectsInvalidFormalReferenceOwnership(t *testing.T) {
	for _, test := range []struct {
		name             string
		formalTaskID     int64
		formalBinding    string
		formalAssetType  string
		formalBoundRole  interface{}
		formalStorageRef string
	}{
		{name: "cross task", formalTaskID: 81, formalBinding: "bound", formalAssetType: "reference", formalStorageRef: "ref-snapshot"},
		{name: "not bound", formalTaskID: 80, formalBinding: "staged", formalAssetType: "reference", formalStorageRef: "ref-snapshot"},
		{name: "wrong asset type", formalTaskID: 80, formalBinding: "bound", formalAssetType: "delivery", formalStorageRef: "ref-snapshot"},
		{name: "resource role assigned", formalTaskID: 80, formalBinding: "bound", formalAssetType: "reference", formalBoundRole: "final", formalStorageRef: "ref-snapshot"},
		{name: "storage lineage differs", formalTaskID: 80, formalBinding: "bound", formalAssetType: "reference", formalStorageRef: "other-ref"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewTaskResourceGroupRepo(New(db))
			now := time.Now().UTC()
			revisionID := int64(121)
			groups := []domain.TaskAssetGroup{{ID: 8, WorkingRevisionID: &revisionID}}

			mock.ExpectQuery(`SELECT r.id, r.group_id, r.revision_no, r.status, r.mode`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
					AddRow(revisionID, 8, 1, "finalized", "single", nil, "migration", 7, "管理员", "", now, now, now, 80, "task", nil, nil))
			mock.ExpectQuery(`FROM task_asset_group_revision_items`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}))
			mock.ExpectQuery(`FROM task_asset_group_revision_references`).WithArgs(revisionID).
				WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()).
					AddRow(131, revisionID, 141, 9901, 0, "ref-snapshot", "reference.png", "task", "image/png", 128, "reference.png", "recorded", "recorded", true, now, 80, nil, nil, "ref-snapshot", test.formalTaskID, test.formalBinding, test.formalAssetType, test.formalBoundRole, test.formalStorageRef, "ref-snapshot"))

			err = repository.hydrateResourceGroupRevisions(context.Background(), groups)
			if !errors.Is(err, repo.ErrDataIntegrity) {
				t.Fatalf("hydration error = %v, want ErrDataIntegrity", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
