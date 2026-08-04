package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type utcTimeArgument struct{}

func (utcTimeArgument) Match(value driver.Value) bool {
	stamp, ok := value.(time.Time)
	return ok && stamp.Location() == time.UTC
}

func TestTaskEventAppendPersistsUTC(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	sqlTx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx := &MySQLTx{tx: sqlTx}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO task_event_sequences (task_id, last_sequence)
		VALUES (?, 0)
		ON DUPLICATE KEY UPDATE task_id = VALUES(task_id)`)).
		WithArgs(int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT last_sequence FROM task_event_sequences WHERE task_id = ? FOR UPDATE`)).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"last_sequence"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(sequence), 0)
		FROM task_event_logs
		WHERE task_id = ?
		FOR UPDATE`)).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE task_event_sequences SET last_sequence = ? WHERE task_id = ?`)).
		WithArgs(int64(1), int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO task_event_logs (id, task_id, sequence, event_type, operator_id, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)).
		WithArgs(sqlmock.AnyArg(), int64(77), int64(1), "task.test", nil, sqlmock.AnyArg(), utcTimeArgument{}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	event, err := NewTaskEventRepo(New(db)).Append(
		context.Background(), tx, 77, "task.test", nil, map[string]any{"ok": true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if event.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", event.CreatedAt.Location())
	}

	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
