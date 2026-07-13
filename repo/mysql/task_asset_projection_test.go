package mysqlrepo

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTaskAssetCreatePersistsResolvedSourceTaskModuleID(t *testing.T) {
	normalized := strings.Join(strings.Fields(taskAssetInsertSQL), " ")
	for _, required := range []string{
		"source_module_key, source_task_module_id",
		"COALESCE(?, ( SELECT tm.id FROM task_modules tm WHERE tm.task_id = ? AND tm.module_key = ? LIMIT 1 ))",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("task asset insert must persist the source module link via %q: %s", required, normalized)
		}
	}
}

func TestTaskAssetRepoGetByIDProjectsLifecycleState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	columns := []string{
		"id", "task_id", "asset_id", "scope_sku_code", "retouch_requirement_id", "asset_type", "version_no", "asset_version_no", "upload_mode", "upload_request_id", "storage_ref_id",
		"file_name", "original_filename", "remote_file_id", "mime_type", "file_size", "file_path", "storage_key", "whole_hash", "upload_status", "preview_status", "uploaded_by", "uploaded_at", "remark", "created_at",
		"source_module_key", "source_task_module_id", "is_archived", "archived_at", "archived_by", "cleaned_at", "deleted_at",
		"flow_review_status", "approved_at", "approved_by", "rejected_at", "rejected_by", "superseded_by_version_id", "superseded_at", "cleanup_after_at", "source_asset_version_id",
		"ref_id", "ref_asset_id", "owner_type", "owner_id", "ref_upload_request_id", "storage_adapter", "ref_type", "ref_key", "ref_file_name", "ref_mime_type", "ref_file_size", "is_placeholder", "checksum_hint", "ref_status", "ref_created_at",
	}
	values := []driver.Value{
		int64(77), int64(21), int64(31), "SKU-1", nil, "delivery", int64(2), int64(2), "multipart", "request-77", nil,
		"delivery.psd", "delivery.psd", nil, "application/octet-stream", int64(4096), nil, "tasks/21/delivery.psd", "hash", "uploaded", "not_applicable", int64(501), now, "remark", now,
		"design", int64(91), true, now, int64(502), now.Add(time.Hour), now.Add(2 * time.Hour),
		"superseded", now, int64(503), nil, nil, int64(78), now.Add(3 * time.Hour), now.Add(4 * time.Hour), int64(70),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	}
	if len(values) != len(columns) {
		t.Fatalf("test fixture values=%d columns=%d", len(values), len(columns))
	}
	mock.ExpectQuery(`FROM task_assets ta .* WHERE ta.id = \?`).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))

	asset, err := NewTaskAssetRepo(New(db)).GetByID(t.Context(), 77)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if asset == nil {
		t.Fatal("GetByID() asset = nil")
	}
	if asset.SourceTaskModuleID == nil || *asset.SourceTaskModuleID != 91 {
		t.Fatalf("source_task_module_id = %+v", asset.SourceTaskModuleID)
	}
	if !asset.IsArchived || asset.ArchivedAt == nil || asset.ArchivedBy == nil || *asset.ArchivedBy != 502 {
		t.Fatalf("archive projection = archived:%t at:%+v by:%+v", asset.IsArchived, asset.ArchivedAt, asset.ArchivedBy)
	}
	if asset.CleanedAt == nil || asset.DeletedAt == nil {
		t.Fatalf("cleanup/delete projection = cleaned:%+v deleted:%+v", asset.CleanedAt, asset.DeletedAt)
	}
	if asset.SupersededByVersionID == nil || *asset.SupersededByVersionID != 78 || asset.SupersededAt == nil {
		t.Fatalf("superseded projection = by:%+v at:%+v", asset.SupersededByVersionID, asset.SupersededAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
