package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

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
