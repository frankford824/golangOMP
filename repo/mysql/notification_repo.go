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
	createdAt := n.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO notifications (user_id, notification_type, payload, is_read, read_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		n.UserID, string(n.NotificationType), jsonOrObject(n.Payload), n.IsRead, toNullTime(n.ReadAt), createdAt)
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
	where, args = appendNotificationScope(where, args, filter.Scope)
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
	return r.MarkReadScoped(ctx, id, userID, at, repo.NotificationScope{})
}

func (r *notificationRepo) MarkReadScoped(ctx context.Context, id, userID int64, at time.Time, scope repo.NotificationScope) (int64, error) {
	where := []string{`id = ?`, `user_id = ?`, `is_read = 0`}
	args := []interface{}{id, userID}
	where, args = appendNotificationScope(where, args, scope)
	args = append([]interface{}{at}, args...)
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE notifications
		   SET is_read = 1, read_at = COALESCE(read_at, ?)
		 WHERE `+strings.Join(where, ` AND `), args...)
	if err != nil {
		return 0, fmt.Errorf("mark notification read: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, userID int64, at time.Time) (int64, error) {
	return r.MarkAllReadScoped(ctx, userID, at, repo.NotificationScope{})
}

func (r *notificationRepo) MarkAllReadScoped(ctx context.Context, userID int64, at time.Time, scope repo.NotificationScope) (int64, error) {
	where := []string{`user_id = ?`, `is_read = 0`}
	args := []interface{}{userID}
	where, args = appendNotificationScope(where, args, scope)
	args = append([]interface{}{at}, args...)
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE notifications
		   SET is_read = 1, read_at = COALESCE(read_at, ?)
		 WHERE `+strings.Join(where, ` AND `), args...)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) UnreadCount(ctx context.Context, userID int64) (int, error) {
	return r.UnreadCountScoped(ctx, userID, repo.NotificationScope{})
}

func (r *notificationRepo) UnreadCountScoped(ctx context.Context, userID int64, scope repo.NotificationScope) (int, error) {
	where := []string{`user_id = ?`, `is_read = 0`}
	args := []interface{}{userID}
	where, args = appendNotificationScope(where, args, scope)
	var count int
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE `+strings.Join(where, ` AND `), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func appendNotificationScope(where []string, args []interface{}, scope repo.NotificationScope) ([]string, []interface{}) {
	prefix := strings.TrimSpace(scope.TypePrefix)
	if prefix == "" {
		return where, args
	}
	operator := "="
	if scope.Exclude {
		operator = "<>"
	}
	where = append(where, `LEFT(notification_type, ?) `+operator+` ?`)
	args = append(args, len(prefix), prefix)
	return where, args
}

func (r *notificationRepo) UpsertWebPushSubscription(ctx context.Context, tx repo.Tx, sub *domain.WebPushSubscription) (*domain.WebPushSubscription, error) {
	if sub == nil {
		return nil, fmt.Errorf("web push subscription is nil")
	}
	now := sub.LastSeenAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO web_push_subscriptions (
		  user_id, endpoint_hash, endpoint, p256dh, auth, user_agent, platform,
		  status, vapid_key_hash, last_seen_at, disabled_at, disabled_reason, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, NULL, '', ?, ?)
		ON DUPLICATE KEY UPDATE
		  id = LAST_INSERT_ID(id),
		  user_id = VALUES(user_id),
		  endpoint = VALUES(endpoint),
		  p256dh = VALUES(p256dh),
		  auth = VALUES(auth),
		  user_agent = VALUES(user_agent),
		  platform = VALUES(platform),
		  status = 'active',
		  vapid_key_hash = VALUES(vapid_key_hash),
		  last_seen_at = VALUES(last_seen_at),
		  disabled_at = NULL,
		  disabled_reason = '',
		  updated_at = VALUES(updated_at)`,
		sub.UserID, sub.EndpointHash, sub.Endpoint, sub.P256DH, sub.Auth, sub.UserAgent, sub.Platform,
		sub.VAPIDKeyHash, now, now, now)
	if err != nil {
		return nil, fmt.Errorf("upsert web push subscription: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("web push subscription last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, webPushSubscriptionSelectSQL()+` WHERE id = ?`, id)
	return scanWebPushSubscription(row)
}

func (r *notificationRepo) DisableWebPushSubscriptionByEndpointHash(ctx context.Context, userID int64, endpointHash string, at time.Time, reason string) (int64, error) {
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE web_push_subscriptions
		   SET status = 'disabled', disabled_at = ?, disabled_reason = ?, updated_at = ?
		 WHERE user_id = ? AND endpoint_hash = ?`,
		at, trimForDB(reason, 255), at, userID, strings.TrimSpace(endpointHash))
	if err != nil {
		return 0, fmt.Errorf("disable web push subscription: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) DisableWebPushSubscriptionByID(ctx context.Context, id int64, at time.Time, reason string) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE web_push_subscriptions
		   SET status = 'disabled', disabled_at = ?, disabled_reason = ?, updated_at = ?
		 WHERE id = ?`,
		at, trimForDB(reason, 255), at, id)
	if err != nil {
		return fmt.Errorf("disable web push subscription by id: %w", err)
	}
	return nil
}

func (r *notificationRepo) MarkStaleWebPushSubscriptionsByUserExceptKeyHash(ctx context.Context, userID int64, vapidKeyHash string, at time.Time) (int64, error) {
	res, err := r.db.db.ExecContext(ctx, `
		UPDATE web_push_subscriptions
		   SET status = 'stale', disabled_at = ?, disabled_reason = 'vapid_key_rotated', updated_at = ?
		 WHERE user_id = ? AND status = 'active' AND vapid_key_hash <> ?`,
		at, at, userID, strings.TrimSpace(vapidKeyHash))
	if err != nil {
		return 0, fmt.Errorf("mark stale web push subscriptions: %w", err)
	}
	return res.RowsAffected()
}

func (r *notificationRepo) ListActiveWebPushSubscriptionsForUser(ctx context.Context, tx repo.Tx, userID int64, vapidKeyHash string) ([]domain.WebPushSubscription, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, webPushSubscriptionSelectSQL()+`
		WHERE user_id = ? AND status = 'active' AND vapid_key_hash = ?
		ORDER BY last_seen_at DESC, id DESC`, userID, strings.TrimSpace(vapidKeyHash))
	if err != nil {
		return nil, fmt.Errorf("list active web push subscriptions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.WebPushSubscription, 0)
	for rows.Next() {
		sub, err := scanWebPushSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sub)
	}
	return out, rows.Err()
}

func (r *notificationRepo) GetNotificationPreference(ctx context.Context, userID int64) (*domain.NotificationPreference, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT user_id, web_push_enabled, last_test_sent_at, vapid_key_hash, created_at, updated_at
		  FROM notification_preferences
		 WHERE user_id = ?`, userID)
	pref, err := scanNotificationPreference(row)
	if err == sql.ErrNoRows {
		return &domain.NotificationPreference{UserID: userID}, nil
	}
	return pref, err
}

func (r *notificationRepo) UpsertNotificationPreference(ctx context.Context, tx repo.Tx, pref *domain.NotificationPreference) error {
	if pref == nil {
		return fmt.Errorf("notification preference is nil")
	}
	now := time.Now().UTC()
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO notification_preferences (user_id, web_push_enabled, last_test_sent_at, vapid_key_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  web_push_enabled = VALUES(web_push_enabled),
		  last_test_sent_at = COALESCE(VALUES(last_test_sent_at), last_test_sent_at),
		  vapid_key_hash = VALUES(vapid_key_hash),
		  updated_at = VALUES(updated_at)`,
		pref.UserID, pref.WebPushEnabled, toNullTime(pref.LastTestSentAt), strings.TrimSpace(pref.VAPIDKeyHash), now, now)
	if err != nil {
		return fmt.Errorf("upsert notification preference: %w", err)
	}
	return nil
}

func (r *notificationRepo) EnqueueNotificationDelivery(ctx context.Context, tx repo.Tx, delivery *domain.NotificationDeliveryOutbox) error {
	if delivery == nil {
		return fmt.Errorf("notification delivery is nil")
	}
	nextAttempt := delivery.NextAttemptAt
	if nextAttempt.IsZero() {
		nextAttempt = time.Now().UTC()
	}
	payload := delivery.Payload
	if !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT IGNORE INTO notification_delivery_outbox (
		  notification_id, subscription_id, user_id, channel, payload, status, attempt_count, next_attempt_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, 'pending', 0, ?, ?, ?)`,
		delivery.NotificationID, delivery.SubscriptionID, delivery.UserID, firstNonEmpty(delivery.Channel, "web_push"), payload, nextAttempt, nextAttempt, nextAttempt)
	if err != nil {
		return fmt.Errorf("enqueue notification delivery: %w", err)
	}
	return nil
}

func (r *notificationRepo) ClaimWebPushDeliveries(ctx context.Context, limit int, claimToken string, leaseUntil time.Time, now time.Time) ([]domain.NotificationDeliveryOutbox, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	claimToken = strings.TrimSpace(claimToken)
	if claimToken == "" {
		return nil, fmt.Errorf("claim token is required")
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE notification_delivery_outbox d
		  LEFT JOIN web_push_subscriptions s ON s.id = d.subscription_id
		   SET d.status = 'dead', d.lease_until = NULL, d.claim_token = '',
		       d.last_error = CASE
		           WHEN s.id IS NULL THEN 'web push subscription missing'
		           ELSE CONCAT('web push subscription ', s.status)
		       END,
		       d.provider_status_code = NULL, d.updated_at = ?
		 WHERE d.channel = 'web_push'
		   AND (
		        d.status IN ('pending', 'retry')
		        OR (d.status = 'sending' AND (d.lease_until IS NULL OR d.lease_until <= ?))
		   )
		   AND (s.id IS NULL OR s.status <> 'active')`, now, now)
	if err != nil {
		return nil, fmt.Errorf("mark inactive web push deliveries dead: %w", err)
	}
	_, err = r.db.db.ExecContext(ctx, `
		UPDATE notification_delivery_outbox
		   SET status = 'sending', claim_token = ?, lease_until = ?, updated_at = ?
		 WHERE id IN (
		       SELECT id FROM (
		           SELECT d.id
		             FROM notification_delivery_outbox d
		             JOIN web_push_subscriptions s ON s.id = d.subscription_id AND s.status = 'active'
		            WHERE d.channel = 'web_push'
		              AND (
		                   d.status IN ('pending', 'retry')
		                   OR (d.status = 'sending' AND (d.lease_until IS NULL OR d.lease_until <= ?))
		              )
		              AND d.next_attempt_at <= ?
		            ORDER BY d.id
		            LIMIT ?
		       ) AS claimable_web_push_deliveries
		   )`,
		claimToken, leaseUntil, now, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim web push deliveries: %w", err)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT d.id, d.notification_id, d.subscription_id, d.user_id, d.channel, d.payload,
		       d.status, d.attempt_count, d.next_attempt_at, d.lease_until, d.claim_token,
		       d.last_error, d.provider_status_code, d.sent_at, d.created_at, d.updated_at,
		       s.endpoint, s.p256dh, s.auth
		  FROM notification_delivery_outbox d
		  JOIN web_push_subscriptions s ON s.id = d.subscription_id
		 WHERE d.channel = 'web_push'
		   AND d.status = 'sending'
		   AND d.claim_token = ?
		   AND s.status = 'active'
		 ORDER BY d.id`, claimToken)
	if err != nil {
		return nil, fmt.Errorf("select claimed web push deliveries: %w", err)
	}
	defer rows.Close()
	out := make([]domain.NotificationDeliveryOutbox, 0)
	for rows.Next() {
		item, err := scanNotificationDeliveryOutbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *notificationRepo) MarkWebPushDeliverySent(ctx context.Context, id int64, claimToken string, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE notification_delivery_outbox
		   SET status = 'sent', sent_at = ?, lease_until = NULL, claim_token = '', last_error = NULL,
		       provider_status_code = NULL, updated_at = ?
		 WHERE id = ? AND claim_token = ?`, at, at, id, claimToken)
	if err != nil {
		return fmt.Errorf("mark web push delivery sent: %w", err)
	}
	return nil
}

func (r *notificationRepo) MarkWebPushDeliveryRetry(ctx context.Context, id int64, claimToken string, nextAttemptAt time.Time, lastError string, providerStatusCode *int, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE notification_delivery_outbox
		   SET status = 'retry', attempt_count = attempt_count + 1, next_attempt_at = ?,
		       lease_until = NULL, claim_token = '', last_error = ?, provider_status_code = ?, updated_at = ?
		 WHERE id = ? AND claim_token = ?`,
		nextAttemptAt, trimForDB(lastError, 2048), toNullInt(providerStatusCode), at, id, claimToken)
	if err != nil {
		return fmt.Errorf("mark web push delivery retry: %w", err)
	}
	return nil
}

func (r *notificationRepo) MarkWebPushDeliveryDead(ctx context.Context, id int64, claimToken string, lastError string, providerStatusCode *int, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE notification_delivery_outbox
		   SET status = 'dead', attempt_count = attempt_count + 1,
		       lease_until = NULL, claim_token = '', last_error = ?, provider_status_code = ?, updated_at = ?
		 WHERE id = ? AND claim_token = ?`,
		trimForDB(lastError, 2048), toNullInt(providerStatusCode), at, id, claimToken)
	if err != nil {
		return fmt.Errorf("mark web push delivery dead: %w", err)
	}
	return nil
}

func (r *notificationRepo) TryCreateNotificationDedupeClaim(ctx context.Context, tx repo.Tx, claim domain.NotificationDedupeClaim) (bool, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT IGNORE INTO notification_dedupe_claims (user_id, notification_type, dedupe_scope, dedupe_key)
		VALUES (?, ?, ?, ?)`,
		claim.UserID, string(claim.NotificationType), strings.TrimSpace(claim.DedupeScope), strings.TrimSpace(claim.DedupeKey))
	if err != nil {
		return false, fmt.Errorf("insert notification dedupe claim: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("notification dedupe rows affected: %w", err)
	}
	return affected > 0, nil
}

func (r *notificationRepo) UpdateNotificationDedupeClaimNotificationID(ctx context.Context, tx repo.Tx, claim domain.NotificationDedupeClaim, notificationID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE notification_dedupe_claims
		   SET notification_id = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE user_id = ? AND notification_type = ? AND dedupe_key = ?`,
		notificationID, claim.UserID, string(claim.NotificationType), strings.TrimSpace(claim.DedupeKey))
	if err != nil {
		return fmt.Errorf("update notification dedupe claim notification id: %w", err)
	}
	return nil
}

func (r *notificationRepo) ClearNotificationDedupeScope(ctx context.Context, scope string) error {
	_, err := r.db.db.ExecContext(ctx, `DELETE FROM notification_dedupe_claims WHERE dedupe_scope = ?`, strings.TrimSpace(scope))
	if err != nil {
		return fmt.Errorf("clear notification dedupe scope: %w", err)
	}
	return nil
}

func (r *notificationRepo) ListRecentTaskFilingFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id, t.task_no, t.sku_code, t.product_name_snapshot,
		       td.erp_sync_version, COALESCE(td.filing_error_message, '')
		  FROM tasks t
		  JOIN task_details td ON td.task_id = t.id
		 WHERE td.filing_status = 'filing_failed'
		   AND COALESCE(td.erp_sync_required, 0) = 1
		 ORDER BY td.updated_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent task filing failures: %w", err)
	}
	defer rows.Close()
	out := make([]domain.SKUSyncFailureNotificationRequest, 0)
	for rows.Next() {
		var req domain.SKUSyncFailureNotificationRequest
		var skuCode, productName, message string
		if err := rows.Scan(&req.TaskID, &req.TaskNo, &skuCode, &productName, &req.ERPSyncVersion, &message); err != nil {
			return nil, err
		}
		req.Source = domain.SKUSyncFailureSourceTaskFiling
		req.Summary = message
		req.FailureItems = []domain.SKUSyncFailureItem{{SKUCode: skuCode, ProductName: productName, Error: message}}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	taskIDs := make([]int64, 0, len(out))
	for _, req := range out {
		taskIDs = append(taskIDs, req.TaskID)
	}
	if itemsByTask, err := r.failedSKUItemsForTasks(ctx, taskIDs); err == nil {
		for i := range out {
			if items := itemsByTask[out[i].TaskID]; len(items) > 0 {
				out[i].FailureItems = items
			}
		}
	}
	return out, nil
}

func (r *notificationRepo) ListRecentProductBaseSyncFailures(ctx context.Context, limit int) ([]domain.SKUSyncFailureNotificationRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, task_no, COALESCE(task_sku_item_id, 0), sku_code, product_name, COALESCE(base_sync_error, last_sync_error)
		  FROM erp_product_sync_records
		 WHERE base_sync_status = 'failed'
		   AND COALESCE(base_sync_error, last_sync_error, '') <> ''
		 ORDER BY updated_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent product base sync failures: %w", err)
	}
	defer rows.Close()
	out := make([]domain.SKUSyncFailureNotificationRequest, 0)
	for rows.Next() {
		var req domain.SKUSyncFailureNotificationRequest
		var item domain.SKUSyncFailureItem
		var message string
		if err := rows.Scan(&req.RecordID, &req.TaskID, &req.TaskNo, &item.SKUItemID, &item.SKUCode, &item.ProductName, &message); err != nil {
			return nil, err
		}
		req.Source = domain.SKUSyncFailureSourceProductBaseSync
		req.Summary = message
		item.Error = message
		req.FailureItems = []domain.SKUSyncFailureItem{item}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *notificationRepo) failedSKUItemsForTasks(ctx context.Context, taskIDs []int64) (map[int64][]domain.SKUSyncFailureItem, error) {
	out := map[int64][]domain.SKUSyncFailureItem{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(taskIDs))
	args := make([]interface{}, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID <= 0 {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, taskID)
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT task_id, id, sku_code, product_name_snapshot, COALESCE(filing_error_message, '')
		  FROM task_sku_items
		 WHERE task_id IN (`+strings.Join(placeholders, ",")+`)
		   AND filing_status = 'filing_failed'
		   AND COALESCE(erp_sync_required, 0) = 1
		 ORDER BY task_id ASC, sku_code ASC, id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		var item domain.SKUSyncFailureItem
		if err := rows.Scan(&taskID, &item.SKUItemID, &item.SKUCode, &item.ProductName, &item.Error); err != nil {
			return nil, err
		}
		out[taskID] = append(out[taskID], item)
	}
	return out, rows.Err()
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

func webPushSubscriptionSelectSQL() string {
	return `SELECT id, user_id, endpoint_hash, endpoint, p256dh, auth, user_agent, platform, status, vapid_key_hash, last_seen_at, disabled_at, disabled_reason, created_at, updated_at FROM web_push_subscriptions`
}

func scanWebPushSubscription(scanner interface{ Scan(...interface{}) error }) (*domain.WebPushSubscription, error) {
	var sub domain.WebPushSubscription
	var disabledAt sql.NullTime
	if err := scanner.Scan(&sub.ID, &sub.UserID, &sub.EndpointHash, &sub.Endpoint, &sub.P256DH, &sub.Auth, &sub.UserAgent, &sub.Platform, &sub.Status, &sub.VAPIDKeyHash, &sub.LastSeenAt, &disabledAt, &sub.DisabledReason, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
		return nil, err
	}
	if disabledAt.Valid {
		sub.DisabledAt = &disabledAt.Time
	}
	return &sub, nil
}

func scanNotificationPreference(scanner interface{ Scan(...interface{}) error }) (*domain.NotificationPreference, error) {
	var pref domain.NotificationPreference
	var lastTest sql.NullTime
	if err := scanner.Scan(&pref.UserID, &pref.WebPushEnabled, &lastTest, &pref.VAPIDKeyHash, &pref.CreatedAt, &pref.UpdatedAt); err != nil {
		return nil, err
	}
	if lastTest.Valid {
		pref.LastTestSentAt = &lastTest.Time
	}
	return &pref, nil
}

func scanNotificationDeliveryOutbox(scanner interface{ Scan(...interface{}) error }) (*domain.NotificationDeliveryOutbox, error) {
	var d domain.NotificationDeliveryOutbox
	var payload []byte
	var leaseUntil, sentAt sql.NullTime
	var claimToken, lastError sql.NullString
	var providerStatus sql.NullInt64
	if err := scanner.Scan(
		&d.ID, &d.NotificationID, &d.SubscriptionID, &d.UserID, &d.Channel, &payload,
		&d.Status, &d.AttemptCount, &d.NextAttemptAt, &leaseUntil, &claimToken,
		&lastError, &providerStatus, &sentAt, &d.CreatedAt, &d.UpdatedAt,
		&d.SubscriptionEndpoint, &d.SubscriptionP256DH, &d.SubscriptionAuth,
	); err != nil {
		return nil, err
	}
	if !json.Valid(payload) {
		payload = []byte(`{}`)
	}
	d.Payload = cloneJSON(payload)
	if leaseUntil.Valid {
		d.LeaseUntil = &leaseUntil.Time
	}
	if claimToken.Valid {
		d.ClaimToken = claimToken.String
	}
	if lastError.Valid {
		d.LastError = lastError.String
	}
	if providerStatus.Valid {
		code := int(providerStatus.Int64)
		d.ProviderStatusCode = &code
	}
	if sentAt.Valid {
		d.SentAt = &sentAt.Time
	}
	return &d, nil
}

func trimForDB(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
