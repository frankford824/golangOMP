package mysqlrepo

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNextTaskEventSequenceRepairsLaggingCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO task_event_sequences (task_id, last_sequence)
		VALUES (?, 0)
		ON DUPLICATE KEY UPDATE task_id = VALUES(task_id)`)).
		WithArgs(int64(2254)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT last_sequence FROM task_event_sequences WHERE task_id = ? FOR UPDATE`)).
		WithArgs(int64(2254)).
		WillReturnRows(sqlmock.NewRows([]string{"last_sequence"}).AddRow(int64(19)))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT COALESCE(MAX(sequence), 0)
		FROM task_event_logs
		WHERE task_id = ?
		FOR UPDATE`)).
		WithArgs(int64(2254)).
		WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(int64(20)))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE task_event_sequences SET last_sequence = ? WHERE task_id = ?`)).
		WithArgs(int64(21), int64(2254)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	next, err := nextTaskEventSequence(context.Background(), tx, 2254)
	if err != nil {
		t.Fatalf("nextTaskEventSequence() error = %v", err)
	}
	if next != 21 {
		t.Fatalf("nextTaskEventSequence() = %d, want 21", next)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestNextTaskEventSequenceKeepsAheadCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	mock.ExpectExec("INSERT INTO task_event_sequences").
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT last_sequence FROM task_event_sequences").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"last_sequence"}).AddRow(int64(8)))
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(sequence\\), 0\\)").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(int64(6)))
	mock.ExpectExec("UPDATE task_event_sequences SET last_sequence").
		WithArgs(int64(9), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	next, err := nextTaskEventSequence(context.Background(), tx, 9)
	if err != nil {
		t.Fatalf("nextTaskEventSequence() error = %v", err)
	}
	if next != 9 {
		t.Fatalf("nextTaskEventSequence() = %d, want 9", next)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestNextTaskEventSequenceStartsAfterExistingEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	mock.ExpectExec("INSERT INTO task_event_sequences").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT last_sequence FROM task_event_sequences").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"last_sequence"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(sequence\\), 0\\)").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(int64(3)))
	mock.ExpectExec("UPDATE task_event_sequences SET last_sequence").
		WithArgs(int64(4), int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	next, err := nextTaskEventSequence(context.Background(), tx, 10)
	if err != nil {
		t.Fatalf("nextTaskEventSequence() error = %v", err)
	}
	if next != 4 {
		t.Fatalf("nextTaskEventSequence() = %d, want 4", next)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
