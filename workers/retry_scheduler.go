package workers

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

// RetryScheduler promotes eligible Fail/Stale jobs back to Pending using
// exponential backoff (spec §4.1).
type RetryScheduler struct {
	db       *sql.DB
	logger   *zap.Logger
	interval time.Duration
}

func NewRetryScheduler(db *sql.DB, logger *zap.Logger) *RetryScheduler {
	return &RetryScheduler{
		db:       db,
		logger:   logger,
		interval: 15 * time.Second,
	}
}

func (w *RetryScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("RetryScheduler started", zap.Duration("interval", w.interval))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("RetryScheduler stopped")
			return
		case <-ticker.C:
			if err := w.schedule(ctx); err != nil {
				w.logger.Error("RetryScheduler error", zap.Error(err))
			}
		}
	}
}

func (w *RetryScheduler) ScheduleOnce(ctx context.Context) error {
	return w.schedule(ctx)
}

func (w *RetryScheduler) schedule(ctx context.Context) error {
	type retryJob struct {
		ID         int64
		SKUID      int64
		RetryCount int
		MaxRetries int
	}

	rows, err := w.db.QueryContext(ctx, `
		SELECT id, sku_id, retry_count, max_retries
		FROM distribution_jobs
		WHERE status IN (?, ?)
		  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at ASC
		LIMIT 50`,
		domain.JobStatusFail,
		domain.JobStatusStale,
	)
	if err != nil {
		return fmt.Errorf("query retryable jobs: %w", err)
	}
	defer rows.Close()

	var jobs []retryJob
	for rows.Next() {
		var j retryJob
		if err = rows.Scan(&j.ID, &j.SKUID, &j.RetryCount, &j.MaxRetries); err != nil {
			return fmt.Errorf("scan retry job: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate retry jobs: %w", err)
	}

	for _, job := range jobs {
		if job.RetryCount < job.MaxRetries {
			if err = w.markRetrying(ctx, job); err != nil {
				w.logger.Error("RetryScheduler retry path failed", zap.Int64("job_id", job.ID), zap.Error(err))
			}
			continue
		}
		if err = w.markExceeded(ctx, job); err != nil {
			w.logger.Error("RetryScheduler exceeded path failed", zap.Int64("job_id", job.ID), zap.Error(err))
		}
	}
	return nil
}

func (w *RetryScheduler) markRetrying(ctx context.Context, job struct {
	ID         int64
	SKUID      int64
	RetryCount int
	MaxRetries int
}) error {
	baseBackoff := 30 * time.Second
	maxBackoff := 1 * time.Hour
	backoff := baseBackoff * time.Duration(math.Pow(2, float64(job.RetryCount)))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	nextRetryAt := time.Now().Add(backoff)

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx retrying: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET status = ?, retry_count = retry_count + 1, next_retry_at = ?, updated_at = NOW()
		WHERE id = ?`,
		domain.JobStatusPending,
		nextRetryAt,
		job.ID,
	); err != nil {
		return fmt.Errorf("update retrying job: %w", err)
	}

	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventJobRetrying, map[string]interface{}{
		"job_id":        job.ID,
		"retry_count":   job.RetryCount + 1,
		"next_retry_at": nextRetryAt,
	}); err != nil {
		return fmt.Errorf("append job.retrying: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit retrying tx: %w", err)
	}
	return nil
}

func (w *RetryScheduler) markExceeded(ctx context.Context, job struct {
	ID         int64
	SKUID      int64
	RetryCount int
	MaxRetries int
}) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx exceeded: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET status = ?, updated_at = NOW()
		WHERE id = ?`,
		domain.JobStatusExceededRetries,
		job.ID,
	); err != nil {
		return fmt.Errorf("update exceeded job: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO incidents (sku_id, job_id, status, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())`,
		job.SKUID,
		job.ID,
		domain.IncidentStatusOpen,
		"job exceeded max retries",
	)
	if err != nil {
		return fmt.Errorf("create incident for exceeded retries: %w", err)
	}
	incidentID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("incident last insert id: %w", err)
	}

	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventJobExceededRetries, map[string]interface{}{
		"job_id":      job.ID,
		"retry_count": job.RetryCount,
		"max_retries": job.MaxRetries,
	}); err != nil {
		return fmt.Errorf("append job.exceeded_retries: %w", err)
	}
	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventIncidentCreated, map[string]interface{}{
		"incident_id": incidentID,
		"job_id":      job.ID,
		"reason":      "job exceeded max retries",
	}); err != nil {
		return fmt.Errorf("append incident.created: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit exceeded tx: %w", err)
	}
	return nil
}
