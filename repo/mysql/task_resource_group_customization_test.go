package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestCustomizationReadyForSubmitGateUsesLockedModuleAndLatestJob(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	mock.ExpectBegin()
	sqlTx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tx := &MySQLTx{tx: sqlTx}
	mock.ExpectQuery("SELECT tm.state, cj.status").WithArgs(int64(42)).WillReturnRows(
		sqlmock.NewRows([]string{"state", "status"}).AddRow(domain.ModuleStateSubmitted, domain.CustomizationJobStatusReadyForSubmit),
	)
	if err := repository.RequireCustomizationReadyForSubmit(context.Background(), tx, 42); err != nil {
		t.Fatalf("RequireCustomizationReadyForSubmit() error = %v", err)
	}
	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomizationReadyForSubmitGateRejectsIncompleteInternalWork(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	mock.ExpectBegin()
	sqlTx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tx := &MySQLTx{tx: sqlTx}
	mock.ExpectQuery("SELECT tm.state, cj.status").WithArgs(int64(42)).WillReturnRows(
		sqlmock.NewRows([]string{"state", "status"}).AddRow(domain.ModuleStateInProgress, domain.CustomizationJobStatusInProgress),
	)
	err = repository.RequireCustomizationReadyForSubmit(context.Background(), tx, 42)
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("gate error = %v", err)
	}
	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetCustomizationReadyForSubmitRequiresBothRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	mock.ExpectBegin()
	sqlTx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tx := &MySQLTx{tx: sqlTx}
	mock.ExpectExec("UPDATE task_modules").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE customization_jobs").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repository.ResetCustomizationReadyForSubmit(context.Background(), tx, 42); err != nil {
		t.Fatalf("ResetCustomizationReadyForSubmit() error = %v", err)
	}
	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
