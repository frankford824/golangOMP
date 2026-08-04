package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestAIChatConversationReadsHideDeletedAndExpiredRows(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		actual = strings.Join(strings.Fields(actual), " ")
		for _, fragment := range strings.Split(expected, "|") {
			if !strings.Contains(actual, strings.Join(strings.Fields(fragment), " ")) {
				return fmt.Errorf("query missing %q: %s", fragment, actual)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewAIChatRepo(New(db))
	now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	columns := []string{"id", "owner", "owner_name", "title", "status", "lock", "expires", "deleted", "created", "updated"}
	mock.ExpectQuery("SELECT COUNT(*) FROM ai_conversations c|c.status <> 'deleted'|c.expires_at > UTC_TIMESTAMP()|c.owner_user_id = ?").
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM ai_conversations c|c.status <> 'deleted'|c.expires_at > UTC_TIMESTAMP()|c.owner_user_id = ?").
		WithArgs(int64(7), 20, 0).WillReturnRows(sqlmock.NewRows(columns).AddRow("c1", 7, "用户", "标题", "active", 0, now.Add(time.Hour), nil, now, now))
	owner := int64(7)
	items, total, err := repository.ListConversations(context.Background(), &owner, domain.AIAdminConversationFilter{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	mock.ExpectQuery("FROM ai_conversations c|WHERE c.id = ?|c.status <> 'deleted'|c.expires_at > UTC_TIMESTAMP()").
		WithArgs("c1").WillReturnRows(sqlmock.NewRows(columns).AddRow("c1", 7, "用户", "标题", "active", 0, now.Add(time.Hour), nil, now, now))
	if _, err := repository.GetConversation(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAIRetrievalOutboxReclaimsExpiredProcessingLease(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		actual = strings.Join(strings.Fields(actual), " ")
		for _, fragment := range strings.Split(expected, "|") {
			if !strings.Contains(actual, strings.Join(strings.Fields(fragment), " ")) {
				return fmt.Errorf("query missing %q: %s", fragment, actual)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 20, 4, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery("status IN ('pending','retry')|status='processing'|lease_until IS NOT NULL|lease_until <= ?|FOR UPDATE SKIP LOCKED").
		WithArgs(now, now, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "document_id", "operation", "content_hash", "embedding_version", "attempt"}).
			AddRow(int64(91), "doc-91", "upsert", "hash", "embed:v1", 2))
	mock.ExpectExec("UPDATE ai_retrieval_outbox|status='processing'|attempt=attempt+1").
		WithArgs("lease-new", leaseUntil, now, int64(91)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mdb := New(db)
	repository := NewAIRetrievalRepo(mdb)
	var items []domain.AIRetrievalOutboxItem
	if err := mdb.RunInTx(context.Background(), func(tx repo.Tx) error {
		var claimErr error
		items, claimErr = repository.ClaimRetrievalOutbox(context.Background(), tx, "lease-new", now, leaseUntil, 10)
		return claimErr
	}); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 91 || items[0].Attempt != 3 {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
