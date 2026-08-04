package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskResourceGroupRepoEnsureGroupShellsQualifiesNoopTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	sqlDB := New(db)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_sku_items`).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectExec(`ON DUPLICATE KEY UPDATE updated_at = task_asset_groups.updated_at`).
		WithArgs(int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = sqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		return NewTaskResourceGroupRepo(sqlDB).EnsureGroupShells(
			context.Background(),
			tx,
			77,
			domain.TaskTypeNewProductDevelopment,
		)
	})
	if err != nil {
		t.Fatalf("EnsureGroupShells() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}
