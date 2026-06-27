package mysqlrepo

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestLockPriceMatrixDimensionLocksParentDimensionBeforeExistingRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO asset_workbench_price_matrix_dimensions (
			worker_type, job_grade, difficulty_class
		) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`)).
		WithArgs("parttime", "J1", "A").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id
		FROM asset_workbench_price_matrix_dimensions
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		FOR UPDATE`)).
		WithArgs("parttime", "J1", "A").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchPriceMatrixSelect()+`
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		ORDER BY effective_from ASC, id ASC
		FOR UPDATE`)).
		WithArgs("parttime", "J1", "A").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "worker_type", "job_grade", "difficulty_class", "unit_price", "effective_from", "effective_to", "enabled",
			"revision_no", "created_by", "remark", "created_at", "updated_at",
		}))
	mock.ExpectCommit()

	mysqlDB := New(db)
	workbenchRepo := NewAssetWorkbenchRepo(mysqlDB)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		items, err := workbenchRepo.LockPriceMatrixDimension(context.Background(), tx, "parttime", "J1", "A")
		if err != nil {
			return err
		}
		if len(items) != 0 {
			t.Fatalf("items = %+v, want empty", items)
		}
		return nil
	}); err != nil {
		t.Fatalf("LockPriceMatrixDimension() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
