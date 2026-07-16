package repo

import (
	"context"
	"encoding/json"
	"time"
)

type TaskERPOutboxItem struct {
	ID            int64
	TaskID        int64
	TaskSKUItemID *int64
	JobType       string
	Generation    int
	Payload       json.RawMessage
	Attempt       int
}

type SearchReindexOutboxItem struct {
	ID         int64
	EntityType string
	EntityID   int64
	Attempt    int
}

type AsyncProjectionOutboxRepo interface {
	ClaimTaskERPOutbox(ctx context.Context, tx Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]TaskERPOutboxItem, error)
	MarkTaskERPOutboxSucceeded(ctx context.Context, tx Tx, id int64, leaseToken string) error
	MarkTaskERPOutboxRetry(ctx context.Context, tx Tx, id int64, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error
	ClaimSearchReindexOutbox(ctx context.Context, tx Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]SearchReindexOutboxItem, error)
	ApplySearchReindex(ctx context.Context, tx Tx, item SearchReindexOutboxItem) error
	MarkSearchReindexOutboxSucceeded(ctx context.Context, tx Tx, id int64, leaseToken string) error
	MarkSearchReindexOutboxRetry(ctx context.Context, tx Tx, id int64, leaseToken, lastError string, nextRetryAt time.Time) error
}
