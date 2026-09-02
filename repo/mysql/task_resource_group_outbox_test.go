package mysqlrepo

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAssetObjectDeletionOutboxProducersUseAdapterSnapshot(t *testing.T) {
	const outboxInsert = "insert into asset_object_deletion_outbox"
	const adapterColumns = "task_asset_id, storage_ref_id, storage_adapter, storage_is_placeholder, storage_key, dedupe_key"
	var producers []string
	err := filepath.WalkDir("../..", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == "tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		normalized := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
		if !strings.Contains(normalized, outboxInsert) {
			return nil
		}
		producers = append(producers, path)
		if !strings.Contains(normalized, adapterColumns) {
			t.Errorf("legacy asset deletion outbox producer without adapter snapshot: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(producers) != 1 || !strings.HasSuffix(filepath.ToSlash(producers[0]), "repo/mysql/task_asset_lifecycle_repo.go") {
		t.Fatalf("asset deletion outbox producers = %v, want only unified lifecycle helper", producers)
	}
}

func TestSupersededResourceQueryMaterializesReachableAssetsWithoutCartesianRevisionJoin(t *testing.T) {
	raw, err := os.ReadFile("task_resource_group.go")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.Join(strings.Fields(string(raw)), " ")
	if !strings.Contains(normalized, "current_revision_ids AS") || !strings.Contains(normalized, "reachable_assets AS") {
		t.Fatalf("superseded resource query must materialize current revision and reachable asset sets")
	}
	if strings.Contains(normalized, "current_revision.id = current_group.working_revision_id OR current_revision.id = current_group.finalized_revision_id") {
		t.Fatalf("superseded resource query reintroduced the cartesian OR revision join")
	}
}

func TestFinalizeGroupQueuesAllSupersededResourceObjectsWithAdapterSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	database := New(db)
	repository := NewTaskResourceGroupRepo(database)

	const (
		groupID         = int64(10)
		previousRevID   = int64(19)
		nextRevisionID  = int64(20)
		previousSource  = int64(101)
		nextSource      = int64(102)
		expectedVersion = int64(3)
	)
	mock.ExpectBegin()
	tx, sqlTx, err := database.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT g\.finalized_revision_id,[\s\S]+previous_source_task_asset_id,[\s\S]+next_revision\.source_task_asset_id`).
		WithArgs(nextRevisionID, groupID, expectedVersion).
		WillReturnRows(sqlmock.NewRows([]string{"finalized_revision_id", "previous_source_id", "next_source_id"}).
			AddRow(previousRevID, previousSource, nextSource))
	mock.ExpectExec(`UPDATE task_asset_group_revisions SET status = 'finalized'`).
		WithArgs(sqlmock.AnyArg(), nextRevisionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE task_asset_group_revisions SET status = 'superseded'`).
		WithArgs(previousRevID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE task_asset_groups[\s\S]+finalized_revision_id`).
		WithArgs(nextRevisionID, nextRevisionID, groupID, expectedVersion).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT DISTINCT candidate\.task_asset_id[\s\S]+resource_revision_superseded|SELECT DISTINCT candidate\.task_asset_id`).
		WithArgs(groupID, groupID, groupID).
		WillReturnRows(sqlmock.NewRows([]string{"task_asset_id"}).AddRow(previousSource).AddRow(int64(103)).AddRow(int64(104)))
	mock.ExpectExec(`UPDATE task_assets[\s\S]+resource_revision_superseded`).
		WithArgs(sqlmock.AnyArg(), previousSource, int64(103), int64(104)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`INSERT INTO asset_object_deletion_outbox[\s\S]+storage_ref_id, storage_adapter, storage_is_placeholder`).
		WithArgs(previousSource, int64(103), int64(104), previousSource, int64(103), int64(104)).
		WillReturnResult(sqlmock.NewResult(0, 6))
	mock.ExpectExec(`INSERT INTO task_asset_group_search_documents`).
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO search_reindex_outbox`).
		WithArgs(groupID, fmt.Sprintf("task_resource_group:%d:%d", groupID, nextRevisionID)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repository.FinalizeGroup(context.Background(), tx, groupID, nextRevisionID, expectedVersion, 10001); err != nil {
		_ = sqlTx.Rollback()
		t.Fatalf("FinalizeGroup() error = %v", err)
	}
	mock.ExpectCommit()
	if err := sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeGroupFirstApprovalQueuesReplacedSubmittedDesignSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	database := New(db)
	repository := NewTaskResourceGroupRepo(database)

	const (
		groupID         = int64(10)
		nextRevisionID  = int64(20)
		designSource    = int64(101)
		auditSource     = int64(102)
		expectedVersion = int64(3)
	)
	mock.ExpectBegin()
	tx, sqlTx, err := database.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT g\.finalized_revision_id,[\s\S]+previous_source_task_asset_id,[\s\S]+next_revision\.source_task_asset_id`).
		WithArgs(nextRevisionID, groupID, expectedVersion).
		WillReturnRows(sqlmock.NewRows([]string{"finalized_revision_id", "previous_source_id", "next_source_id"}).
			AddRow(nil, designSource, auditSource))
	mock.ExpectExec(`UPDATE task_asset_group_revisions SET status = 'finalized'`).
		WithArgs(sqlmock.AnyArg(), nextRevisionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE task_asset_groups[\s\S]+finalized_revision_id`).
		WithArgs(nextRevisionID, nextRevisionID, groupID, expectedVersion).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT DISTINCT candidate\.task_asset_id`).
		WithArgs(groupID, groupID, groupID).
		WillReturnRows(sqlmock.NewRows([]string{"task_asset_id"}).AddRow(designSource))
	mock.ExpectExec(`UPDATE task_assets[\s\S]+resource_revision_superseded`).
		WithArgs(sqlmock.AnyArg(), designSource).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO asset_object_deletion_outbox[\s\S]+storage_ref_id, storage_adapter, storage_is_placeholder`).
		WithArgs(designSource, designSource).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO task_asset_group_search_documents`).
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO search_reindex_outbox`).
		WithArgs(groupID, fmt.Sprintf("task_resource_group:%d:%d", groupID, nextRevisionID)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repository.FinalizeGroup(context.Background(), tx, groupID, nextRevisionID, expectedVersion, 10001); err != nil {
		_ = sqlTx.Rollback()
		t.Fatalf("FinalizeGroup() error = %v", err)
	}
	mock.ExpectCommit()
	if err := sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeGroupFirstApprovalReusingSubmittedDesignSourceSkipsSupersededScan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	database := New(db)
	repository := NewTaskResourceGroupRepo(database)

	const (
		groupID         = int64(10)
		nextRevisionID  = int64(20)
		designSource    = int64(101)
		expectedVersion = int64(3)
	)
	mock.ExpectBegin()
	tx, sqlTx, err := database.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT g\.finalized_revision_id,[\s\S]+previous_source_task_asset_id,[\s\S]+next_revision\.source_task_asset_id`).
		WithArgs(nextRevisionID, groupID, expectedVersion).
		WillReturnRows(sqlmock.NewRows([]string{"finalized_revision_id", "previous_source_id", "next_source_id"}).
			AddRow(nil, designSource, designSource))
	mock.ExpectExec(`UPDATE task_asset_group_revisions SET status = 'finalized'`).
		WithArgs(sqlmock.AnyArg(), nextRevisionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE task_asset_groups[\s\S]+finalized_revision_id`).
		WithArgs(nextRevisionID, nextRevisionID, groupID, expectedVersion).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// No superseded-object SELECT is expected. sqlmock fails the test if that
	// historical reachability scan is accidentally reintroduced here.
	mock.ExpectExec(`INSERT INTO task_asset_group_search_documents`).
		WithArgs(groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO search_reindex_outbox`).
		WithArgs(groupID, fmt.Sprintf("task_resource_group:%d:%d", groupID, nextRevisionID)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repository.FinalizeGroup(context.Background(), tx, groupID, nextRevisionID, expectedVersion, 10001); err != nil {
		_ = sqlTx.Rollback()
		t.Fatalf("FinalizeGroup() error = %v", err)
	}
	mock.ExpectCommit()
	if err := sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
