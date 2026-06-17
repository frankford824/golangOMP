package mysqlrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
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
