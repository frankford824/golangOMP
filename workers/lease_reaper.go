package workers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

// LeaseReaper periodically reclaims expired leases to eliminate zombie jobs (spec §4.1, §1.1).
// A Running job whose lease has expired is marked Stale and becomes eligible for retry.
type LeaseReaper struct {
	db       *sql.DB
	logger   *zap.Logger
	interval time.Duration
}

func NewLeaseReaper(db *sql.DB, logger *zap.Logger) *LeaseReaper {
	return &LeaseReaper{
		db:       db,
		logger:   logger,
		interval: 30 * time.Second,
	}
}

func (w *LeaseReaper) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("LeaseReaper started", zap.Duration("interval", w.interval))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("LeaseReaper stopped")
			return
		case <-ticker.C:
			if err := w.reap(ctx); err != nil {
				w.logger.Error("LeaseReaper reap error", zap.Error(err))
			}
		}
	}
}

func (w *LeaseReaper) ReapOnce(ctx context.Context) error {
	return w.reap(ctx)
}

func (w *LeaseReaper) reap(ctx context.Context) error {
	type expiredJob struct {
		ID    int64
		SKUID int64
	}

	rows, err := w.db.QueryContext(ctx, `
		SELECT dj.id, dj.sku_id
		FROM distribution_jobs dj
		JOIN job_attempts ja ON ja.id = dj.current_attempt_id
		WHERE dj.status = ? AND ja.lease_expires_at < NOW()`,
		domain.JobStatusRunning,
	)
	if err != nil {
		return fmt.Errorf("query expired leases: %w", err)
	}
	defer rows.Close()

	var jobs []expiredJob
	for rows.Next() {
		var j expiredJob
		if err = rows.Scan(&j.ID, &j.SKUID); err != nil {
			return fmt.Errorf("scan expired lease row: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate expired lease rows: %w", err)
	}

	for _, job := range jobs {
		tx, beginErr := w.db.BeginTx(ctx, nil)
		if beginErr != nil {
			w.logger.Error("LeaseReaper begin tx failed", zap.Int64("job_id", job.ID), zap.Error(beginErr))
			continue
		}

		committed := false
		func() {
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()

			if _, err = tx.ExecContext(ctx, `
				UPDATE distribution_jobs
				SET status = ?, current_attempt_id = NULL, updated_at = NOW()
				WHERE id = ?`,
				domain.JobStatusStale,
				job.ID,
			); err != nil {
				w.logger.Error("LeaseReaper mark stale failed", zap.Int64("job_id", job.ID), zap.Error(err))
				return
			}

			if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventJobStale, map[string]interface{}{
				"job_id": job.ID,
			}); err != nil {
				w.logger.Error("LeaseReaper append event failed", zap.Int64("job_id", job.ID), zap.Error(err))
				return
			}

			if err = tx.Commit(); err != nil {
				w.logger.Error("LeaseReaper commit failed", zap.Int64("job_id", job.ID), zap.Error(err))
				return
			}
			committed = true
			w.logger.Info("LeaseReaper reclaimed expired lease", zap.Int64("job_id", job.ID), zap.Int64("sku_id", job.SKUID))
		}()
	}
	return nil
}
