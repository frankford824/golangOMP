package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProductCodeSequenceRepoAllocateRangeBootstrapsFromExistingSKUCodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	sqlTx, err := db.Begin()
	if err != nil {
		t.Fatalf("db.Begin() error = %v", err)
	}
	tx := &MySQLTx{tx: sqlTx}

	mock.ExpectExec("INSERT INTO product_code_sequences").
		WithArgs("NS", "KT").
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectQuery("SELECT LAST_INSERT_ID\\(\\)").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectQuery("SELECT next_value\\s+FROM product_code_sequences").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"next_value"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT MAX\\(CAST\\(SUBSTRING\\(sku_code, \\?, 6\\) AS UNSIGNED\\)\\)").
		WithArgs(5, "NSKT%", 10, 5).
		WillReturnRows(sqlmock.NewRows([]string{"max_seq"}).AddRow(int64(123)))
	mock.ExpectExec("UPDATE product_code_sequences").
		WithArgs(int64(126), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	repo := NewProductCodeSequenceRepo(&DB{db: db})
	start, err := repo.AllocateRange(context.Background(), tx, "NS", "KT", 2)
	if err != nil {
		t.Fatalf("AllocateRange() error = %v", err)
	}
	if start != 124 {
		t.Fatalf("AllocateRange() start = %d, want 124", start)
	}

	if err := sqlTx.Rollback(); err != nil {
		t.Fatalf("sqlTx.Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}

func TestProductCodeSequenceRepoAllocateRangeSkipsBootstrapWhenCounterExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	sqlTx, err := db.Begin()
	if err != nil {
		t.Fatalf("db.Begin() error = %v", err)
	}
	tx := &MySQLTx{tx: sqlTx}

	mock.ExpectExec("INSERT INTO product_code_sequences").
		WithArgs("NS", "KT").
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectQuery("SELECT LAST_INSERT_ID\\(\\)").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))
	mock.ExpectQuery("SELECT next_value\\s+FROM product_code_sequences").
		WithArgs(int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"next_value"}).AddRow(int64(8)))
	mock.ExpectExec("UPDATE product_code_sequences").
		WithArgs(int64(11), int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	repo := NewProductCodeSequenceRepo(&DB{db: db})
	start, err := repo.AllocateRange(context.Background(), tx, "NS", "KT", 3)
	if err != nil {
		t.Fatalf("AllocateRange() error = %v", err)
	}
	if start != 8 {
		t.Fatalf("AllocateRange() start = %d, want 8", start)
	}

	if err := sqlTx.Rollback(); err != nil {
		t.Fatalf("sqlTx.Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}
