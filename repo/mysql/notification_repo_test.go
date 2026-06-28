package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestNotificationRepoCreateWritesApplicationCreatedAt(t *testing.T) {
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

	payload := json.RawMessage(`{"task_id":1560}`)
	before := time.Now()
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO notifications (user_id, notification_type, payload, is_read, read_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`)).
		WithArgs(int64(228), string(domain.NotificationTypeTaskAssignedToMe), string(payload), false, sqlmock.AnyArg(), timeArgBetween{start: before, end: before.Add(5 * time.Second)}).
		WillReturnResult(sqlmock.NewResult(8164, 1))
	mock.ExpectQuery(regexp.QuoteMeta(notificationSelectSQL() + ` WHERE id = ?`)).
		WithArgs(int64(8164)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "notification_type", "payload", "is_read", "read_at", "created_at"}).
			AddRow(int64(8164), int64(228), string(domain.NotificationTypeTaskAssignedToMe), []byte(payload), false, nil, before))
	mock.ExpectRollback()

	repo := NewNotificationRepo(New(db))
	got, err := repo.Create(context.Background(), &MySQLTx{tx: sqlTx}, &domain.Notification{
		UserID:           228,
		NotificationType: domain.NotificationTypeTaskAssignedToMe,
		Payload:          payload,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got == nil || got.ID != 8164 {
		t.Fatalf("Create() = %+v, want id 8164", got)
	}
	if err := sqlTx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestNotificationRepoClaimWebPushDeliveriesSkipsAndDeadsInactiveSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "dead-inactive-web-push-deliveries":
			for _, fragment := range []string{
				"UPDATE notification_delivery_outbox d LEFT JOIN web_push_subscriptions s ON s.id = d.subscription_id",
				"SET d.status = 'dead'",
				"AND (s.id IS NULL OR s.status <> 'active')",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("inactive delivery cleanup SQL missing %q: %s", fragment, normalized)
				}
			}
		case "claim-active-web-push-deliveries":
			for _, fragment := range []string{
				"UPDATE notification_delivery_outbox SET status = 'sending'",
				"JOIN web_push_subscriptions s ON s.id = d.subscription_id AND s.status = 'active'",
				"ORDER BY d.id LIMIT ?",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("claim SQL missing %q: %s", fragment, normalized)
				}
			}
		case "select-claimed-web-push-deliveries":
			for _, fragment := range []string{
				"FROM notification_delivery_outbox d JOIN web_push_subscriptions s ON s.id = d.subscription_id",
				"AND d.claim_token = ? AND s.status = 'active'",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("select claimed SQL missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected expected SQL marker %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	mock.ExpectExec("dead-inactive-web-push-deliveries").
		WithArgs(now, now).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("claim-active-web-push-deliveries").
		WithArgs("claim-1", leaseUntil, now, now, now, 10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("select-claimed-web-push-deliveries").
		WithArgs("claim-1").
		WillReturnRows(sqlmock.NewRows(notificationDeliveryOutboxColumns()))

	repo := NewNotificationRepo(New(db))
	items, err := repo.ClaimWebPushDeliveries(context.Background(), 10, "claim-1", leaseUntil, now)
	if err != nil {
		t.Fatalf("ClaimWebPushDeliveries() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ClaimWebPushDeliveries() returned %d items, want 0", len(items))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestNotificationRepoListRecentTaskFilingFailuresBatchesSKUItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "list-recent-task-filing-failures":
			for _, fragment := range []string{
				"FROM tasks t JOIN task_details td ON td.task_id = t.id",
				"td.filing_status = 'filing_failed'",
				"COALESCE(td.erp_sync_required, 0) = 1",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("recent filing failure SQL missing %q: %s", fragment, normalized)
				}
			}
		case "list-failed-sku-items-batch":
			for _, fragment := range []string{
				"FROM task_sku_items",
				"task_id IN (?,?)",
				"filing_status = 'filing_failed'",
				"COALESCE(erp_sync_required, 0) = 1",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("batch item SQL missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected expected SQL marker %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("list-recent-task-filing-failures").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_no", "sku_code", "product_name_snapshot", "erp_sync_version", "filing_error_message"}).
			AddRow(int64(101), "RW-101", "TASK-SKU-1", "Task Product 1", int64(3), "任务失败1").
			AddRow(int64(102), "RW-102", "TASK-SKU-2", "Task Product 2", int64(4), "任务失败2"))
	mock.ExpectQuery("list-failed-sku-items-batch").
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "id", "sku_code", "product_name_snapshot", "filing_error_message"}).
			AddRow(int64(101), int64(1001), "SKU-A", "A", "ERP频控").
			AddRow(int64(101), int64(1002), "SKU-B", "B", "ERP超时").
			AddRow(int64(102), int64(2001), "SKU-C", "C", "ERP拒绝"))

	repo := NewNotificationRepo(New(db))
	got, err := repo.ListRecentTaskFilingFailures(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListRecentTaskFilingFailures() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if len(got[0].FailureItems) != 2 || got[0].FailureItems[0].SKUCode != "SKU-A" {
		t.Fatalf("task 101 items = %#v, want batched SKU items", got[0].FailureItems)
	}
	if len(got[1].FailureItems) != 1 || got[1].FailureItems[0].SKUCode != "SKU-C" {
		t.Fatalf("task 102 items = %#v, want batched SKU items", got[1].FailureItems)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func notificationDeliveryOutboxColumns() []string {
	return []string{
		"id", "notification_id", "subscription_id", "user_id", "channel", "payload",
		"status", "attempt_count", "next_attempt_at", "lease_until", "claim_token",
		"last_error", "provider_status_code", "sent_at", "created_at", "updated_at",
		"endpoint", "p256dh", "auth",
	}
}

type timeArgBetween struct {
	start time.Time
	end   time.Time
}

func (m timeArgBetween) Match(value driver.Value) bool {
	t, ok := value.(time.Time)
	if !ok {
		return false
	}
	return !t.Before(m.start) && !t.After(m.end)
}
