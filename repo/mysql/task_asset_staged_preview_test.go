package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestTaskAssetCreateQueuesSearchRefreshWithoutSynchronousProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	database := New(db)
	repository := NewTaskAssetRepo(database).(*taskAssetRepo)
	mock.ExpectBegin()
	tx, sqlTx, err := database.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO task_assets").
		WillReturnResult(sqlmock.NewResult(69379, 1))
	mock.ExpectExec(`INSERT IGNORE INTO search_reindex_outbox[\s\S]+VALUES \('task', \?, \?\)`).
		WithArgs(int64(5375), "task:5375:asset:69379").
		WillReturnResult(sqlmock.NewResult(0, 1))

	id, err := repository.Create(context.Background(), tx, &domain.TaskAsset{
		TaskID:          5375,
		AssetType:       domain.TaskAssetTypeDelivery,
		VersionNo:       322,
		FileName:        "CGN000088-final.jpg",
		UploadedBy:      242,
		SourceModuleKey: "design",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id != 69379 {
		t.Fatalf("Create() id = %d, want 69379", id)
	}
	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetStagedPreviewAccessByDesignAssetIDReadsOnlyActiveStagedBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskAssetRepo(New(db)).(*taskAssetRepo)
	mock.ExpectQuery("SELECT ta.id, ta.task_id, COALESCE\\(ta.staged_by, ta.uploaded_by\\)").
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "staged_by"}).AddRow(int64(901), int64(44), int64(12)))

	item, err := repository.GetStagedPreviewAccessByDesignAssetID(context.Background(), 77)
	if err != nil {
		t.Fatalf("GetStagedPreviewAccessByDesignAssetID() error = %v", err)
	}
	if item == nil || item.TaskAssetID != 901 || item.TaskID != 44 || item.StagedBy != 12 {
		t.Fatalf("item = %+v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
