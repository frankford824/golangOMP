package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPurgeSupersededResourceObjectsDryRunDoesNotMutateAssets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`DROP TEMPORARY TABLE IF EXISTS purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TEMPORARY TABLE purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(asset\.file_size\), 0\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count", "bytes"}).AddRow(2, 300))
	mock.ExpectExec(`DROP TEMPORARY TABLE purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	summary, err := PurgeSupersededResourceObjects(context.Background(), db, false)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DryRun || summary.SelectedTaskAssets != 2 || summary.SelectedBytes != 300 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.RevokedTaskAssets != 0 || summary.QueuedObjectDeletions != 0 {
		t.Fatalf("dry run mutated summary = %+v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeSupersededResourceObjectsApplyLocksGroupsAndQueuesExactObjects(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`DROP TEMPORARY TABLE IF EXISTS purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TEMPORARY TABLE purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT id FROM task_asset_groups ORDER BY id FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10).AddRow(11))
	mock.ExpectExec(`INSERT INTO purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(asset\.file_size\), 0\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count", "bytes"}).AddRow(2, 300))
	mock.ExpectExec(`UPDATE task_assets asset[\s\S]+access_revoked_reason = \?`).
		WithArgs(supersededResourceCleanupReason).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT task_asset_id FROM purge_superseded_task_assets ORDER BY task_asset_id`).
		WillReturnRows(sqlmock.NewRows([]string{"task_asset_id"}).AddRow(101).AddRow(102))
	mock.ExpectExec(`INSERT INTO asset_object_deletion_outbox[\s\S]+storage_ref_id, storage_adapter, storage_is_placeholder`).
		WithArgs(int64(101), int64(102), int64(101), int64(102)).
		WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectQuery(`SELECT COUNT\(\*\)[\s\S]+asset_object_deletion_outbox`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	mock.ExpectExec(`DROP TEMPORARY TABLE purge_superseded_task_assets`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	summary, err := PurgeSupersededResourceObjects(context.Background(), db, true)
	if err != nil {
		t.Fatal(err)
	}
	if summary.DryRun || summary.SelectedTaskAssets != 2 || summary.SelectedBytes != 300 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.RevokedTaskAssets != 2 || summary.QueuedObjectDeletions != 4 {
		t.Fatalf("apply summary = %+v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
