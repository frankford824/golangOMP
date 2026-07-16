package mysqlrepo

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

func TestAsyncProjectionOutboxClaimsTaskERPWithLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mysqlDB := New(db)
	outbox := NewAsyncProjectionOutboxRepo(mysqlDB)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, task_id, task_sku_item_id, job_type, generation, payload_json, attempt[\s\S]+FROM task_erp_outbox`).
		WithArgs(now, now, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "task_sku_item_id", "job_type", "generation", "payload_json", "attempt"}).
			AddRow(int64(1), int64(42), nil, "task_filing", 1, []byte(`{}`), 2))
	mock.ExpectExec(`UPDATE task_erp_outbox[\s\S]+status='processing'`).
		WithArgs("lease", leaseUntil, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var items []repo.TaskERPOutboxItem
	err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		items, err = outbox.ClaimTaskERPOutbox(context.Background(), tx, "lease", now, leaseUntil, 50)
		return err
	})
	if err != nil {
		t.Fatalf("ClaimTaskERPOutbox() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 || items[0].Attempt != 3 {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAsyncProjectionOutboxClaimsSearchAndRejectsUnknownEntity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mysqlDB := New(db)
	outbox := NewAsyncProjectionOutboxRepo(mysqlDB)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, entity_type, entity_id, attempt[\s\S]+FROM search_reindex_outbox`).
		WithArgs(now, now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "entity_type", "entity_id", "attempt"}).
			AddRow(int64(2), "task", int64(42), 0))
	mock.ExpectExec(`UPDATE search_reindex_outbox[\s\S]+status='processing'`).
		WithArgs("lease", leaseUntil, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var items []repo.SearchReindexOutboxItem
	err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		items, err = outbox.ClaimSearchReindexOutbox(context.Background(), tx, "lease", now, leaseUntil, 100)
		return err
	})
	if err != nil {
		t.Fatalf("ClaimSearchReindexOutbox() error = %v", err)
	}
	if len(items) != 1 || items[0].Attempt != 1 {
		t.Fatalf("items = %+v", items)
	}

	mock.ExpectBegin()
	mock.ExpectRollback()
	err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		return outbox.ApplySearchReindex(context.Background(), tx, repo.SearchReindexOutboxItem{EntityType: "unknown", EntityID: 42})
	})
	if err == nil {
		t.Fatal("ApplySearchReindex(unknown) error = nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
