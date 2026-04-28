package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type notificationRepo struct{ db *DB }

func NewNotificationRepo(db *DB) repo.NotificationRepo { return &notificationRepo{db: db} }

func (r *notificationRepo) Create(ctx context.Context, tx repo.Tx, n *domain.Notification) (*domain.Notification, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO notifications (user_id, notification_type, payload, is_read, read_at)
		VALUES (?, ?, ?, ?, ?)`,
		n.UserID, string(n.NotificationType), jsonOrObject(n.Payload), n.IsRead, toNullTime(n.ReadAt))
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("notification last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, notificationSelectSQL()+` WHERE id = ?`, id)
	return scanNotification(row)
}

func (r *notificationRepo) Get(ctx context.Context, id int64) (*domain.Notification, error) {
	row := r.db.db.QueryRowContext(ctx, notificationSelectSQL()+` WHERE id = ?`, id)
	return scanNotification(row)
}

func (r *notificationRepo) List(ctx context.Context, filter repo.NotificationListFilter) ([]domain.Notification, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	where := []string{`user_id = ?`}
	args := []interface{}{filter.UserID}
	if filter.IsRead != nil {
		where = append(where, `is_read = ?`)
		args = append(args, *filter.IsRead)
	}
	if filter.BeforeTime != nil && filter.BeforeID > 0 {
		where = append(where, `(created_at < ? OR (created_at = ? AND id < ?))`)
		args = append(args, *filter.BeforeTime, *filter.BeforeTime, filter.BeforeID)
	}
	args = append(args, filter.Limit)
	rows, err := r.db.db.QueryContext(ctx, notificationSelectSQL()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

func (r *notificationRepo) MarkRead(ctx context.Context, id, userID int64, at time.Time) (int64, error) {
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE notifications
		   SET is_read = 1, read_at = COALESCE(read_at, ?)
		 WHERE id = ? AND user_id = ? AND is_read = 0`, at, id, userID)
	if err != nil {
		return 0, fmt.Errorf("mark notification read: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error) {
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE notifications
		   SET is_read = 1, read_at = COALESCE(read_at, ?)
		 WHERE user_id = ? AND is_read = 0`, at, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) UnreadCount(ctx context.Context, userID int64) (int, error) {
	var count int
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func notificationSelectSQL() string {
	return `SELECT id, user_id, notification_type, payload, is_read, read_at, created_at FROM notifications`
}

func scanNotification(scanner interface{ Scan(...interface{}) error }) (*domain.Notification, error) {
	var n domain.Notification
	var payload []byte
	var readAt sql.NullTime
	if err := scanner.Scan(&n.ID, &n.UserID, &n.NotificationType, &payload, &n.IsRead, &readAt, &n.CreatedAt); err != nil {
		return nil, err
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	if !json.Valid(payload) {
		payload = []byte(`{}`)
	}
	n.Payload = cloneJSON(payload)
	return &n, nil
}
