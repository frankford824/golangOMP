package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/repo"
)

type asyncProjectionOutboxRepo struct{ db *DB }

func NewAsyncProjectionOutboxRepo(db *DB) repo.AsyncProjectionOutboxRepo {
	return &asyncProjectionOutboxRepo{db: db}
}

func (r *asyncProjectionOutboxRepo) ClaimTaskERPOutbox(ctx context.Context, tx repo.Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]repo.TaskERPOutboxItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	sqlTx := Unwrap(tx)
	rows, err := sqlTx.QueryContext(ctx, `
		SELECT id, task_id, task_sku_item_id, job_type, generation, payload_json, attempt
		FROM task_erp_outbox
		WHERE ((status IN ('pending','retry') AND (next_retry_at IS NULL OR next_retry_at <= ?))
		   OR (status = 'processing' AND lease_until IS NOT NULL AND lease_until <= ?))
		ORDER BY id
		LIMIT ?
		FOR UPDATE SKIP LOCKED`, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim task ERP outbox: %w", err)
	}
	items := make([]repo.TaskERPOutboxItem, 0, limit)
	for rows.Next() {
		var item repo.TaskERPOutboxItem
		var skuItemID sql.NullInt64
		var payload []byte
		if err := rows.Scan(&item.ID, &item.TaskID, &skuItemID, &item.JobType, &item.Generation, &payload, &item.Attempt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.TaskSKUItemID = fromNullInt64(skuItemID)
		item.Payload = append(item.Payload[:0], payload...)
		item.Attempt++
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	marks, args := int64MutationArgs(ids)
	updateArgs := []interface{}{strings.TrimSpace(leaseToken), leaseUntil}
	updateArgs = append(updateArgs, args...)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE task_erp_outbox
		SET status='processing', lease_token=?, lease_until=?, attempt=attempt+1, updated_at=CURRENT_TIMESTAMP
		WHERE id IN (`+marks+`)`, updateArgs...)
	if err != nil {
		return nil, fmt.Errorf("lease task ERP outbox: %w", err)
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected != int64(len(items)) {
		return nil, repo.ErrConflict
	}
	return items, nil
}

func (r *asyncProjectionOutboxRepo) MarkTaskERPOutboxSucceeded(ctx context.Context, tx repo.Tx, id int64, leaseToken string) error {
	return requireAsyncOutboxUpdate(Unwrap(tx).ExecContext(ctx, `
		UPDATE task_erp_outbox
		SET status='succeeded', lease_token=NULL, lease_until=NULL, next_retry_at=NULL, last_error=NULL, alert_status='none', updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status='processing' AND lease_token=?`, id, leaseToken))
}

func (r *asyncProjectionOutboxRepo) MarkTaskERPOutboxRetry(ctx context.Context, tx repo.Tx, id int64, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error {
	alertStatus := "none"
	if alert {
		alertStatus = "alerted"
	}
	return requireAsyncOutboxUpdate(Unwrap(tx).ExecContext(ctx, `
		UPDATE task_erp_outbox
		SET status='retry', lease_token=NULL, lease_until=NULL, next_retry_at=?, last_error=?, alert_status=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status='processing' AND lease_token=?`, nextRetryAt, truncateAsyncOutboxError(lastError), alertStatus, id, leaseToken))
}

func (r *asyncProjectionOutboxRepo) ClaimSearchReindexOutbox(ctx context.Context, tx repo.Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]repo.SearchReindexOutboxItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	sqlTx := Unwrap(tx)
	rows, err := sqlTx.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, attempt
		FROM search_reindex_outbox
		WHERE ((status IN ('pending','retry') AND (next_retry_at IS NULL OR next_retry_at <= ?))
		   OR (status = 'processing' AND lease_until IS NOT NULL AND lease_until <= ?))
		ORDER BY id
		LIMIT ?
		FOR UPDATE SKIP LOCKED`, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim search reindex outbox: %w", err)
	}
	items := make([]repo.SearchReindexOutboxItem, 0, limit)
	for rows.Next() {
		var item repo.SearchReindexOutboxItem
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.Attempt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.Attempt++
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	marks, args := int64MutationArgs(ids)
	updateArgs := []interface{}{strings.TrimSpace(leaseToken), leaseUntil}
	updateArgs = append(updateArgs, args...)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE search_reindex_outbox
		SET status='processing', lease_token=?, lease_until=?, attempt=attempt+1, updated_at=CURRENT_TIMESTAMP
		WHERE id IN (`+marks+`)`, updateArgs...)
	if err != nil {
		return nil, fmt.Errorf("lease search reindex outbox: %w", err)
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected != int64(len(items)) {
		return nil, repo.ErrConflict
	}
	return items, nil
}

func (r *asyncProjectionOutboxRepo) ApplySearchReindex(ctx context.Context, tx repo.Tx, item repo.SearchReindexOutboxItem) error {
	switch strings.TrimSpace(item.EntityType) {
	case "task":
		return reindexTaskSearchDocument(ctx, Unwrap(tx), item.EntityID)
	case "task_resource_group":
		return reindexTaskAssetGroupSearchDocument(ctx, Unwrap(tx), item.EntityID)
	default:
		return fmt.Errorf("unsupported search reindex entity type %q", item.EntityType)
	}
}

func (r *asyncProjectionOutboxRepo) MarkSearchReindexOutboxSucceeded(ctx context.Context, tx repo.Tx, id int64, leaseToken string) error {
	return requireAsyncOutboxUpdate(Unwrap(tx).ExecContext(ctx, `
		UPDATE search_reindex_outbox
		SET status='succeeded', lease_token=NULL, lease_until=NULL, next_retry_at=NULL, last_error=NULL, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status='processing' AND lease_token=?`, id, leaseToken))
}

func (r *asyncProjectionOutboxRepo) MarkSearchReindexOutboxRetry(ctx context.Context, tx repo.Tx, id int64, leaseToken, lastError string, nextRetryAt time.Time) error {
	return requireAsyncOutboxUpdate(Unwrap(tx).ExecContext(ctx, `
		UPDATE search_reindex_outbox
		SET status='retry', lease_token=NULL, lease_until=NULL, next_retry_at=?, last_error=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status='processing' AND lease_token=?`, nextRetryAt, truncateAsyncOutboxError(lastError), id, leaseToken))
}

func requireAsyncOutboxUpdate(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return repo.ErrConflict
	}
	return nil
}

func truncateAsyncOutboxError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 4000 {
		return message
	}
	return message[:4000]
}
