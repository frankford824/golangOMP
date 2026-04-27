package workers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/domain"
)

// VerifyWorker asynchronously verifies job evidence against the remote target
// (e.g., checking remote file size) when verify_on_done policy is enabled (spec §4.1, §2.1 module 5).
type VerifyWorker struct {
	db       *sql.DB
	rdb      *redis.Client
	logger   *zap.Logger
	interval time.Duration
}

func NewVerifyWorker(db *sql.DB, rdb *redis.Client, logger *zap.Logger) *VerifyWorker {
	return &VerifyWorker{
		db:       db,
		rdb:      rdb,
		logger:   logger,
		interval: 10 * time.Second,
	}
}

func (w *VerifyWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("VerifyWorker started", zap.Duration("interval", w.interval))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("VerifyWorker stopped")
			return
		case <-ticker.C:
			if err := w.verify(ctx); err != nil {
				w.logger.Error("VerifyWorker error", zap.Error(err))
			}
		}
	}
}

func (w *VerifyWorker) VerifyOnce(ctx context.Context) error {
	return w.verify(ctx)
}

func (w *VerifyWorker) verify(ctx context.Context) error {
	type verifyJob struct {
		ID         int64
		SKUID      int64
		AssetVerID int64
		Target     string
	}

	rows, err := w.db.QueryContext(ctx, `
		SELECT id, sku_id, asset_ver_id, target
		FROM distribution_jobs
		WHERE verify_status = ?
		LIMIT 20`,
		domain.VerifyStatusVerifying,
	)
	if err != nil {
		return fmt.Errorf("query verify jobs: %w", err)
	}
	defer rows.Close()

	var jobs []verifyJob
	for rows.Next() {
		var j verifyJob
		if err = rows.Scan(&j.ID, &j.SKUID, &j.AssetVerID, &j.Target); err != nil {
			return fmt.Errorf("scan verify job: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate verify jobs: %w", err)
	}

	for _, job := range jobs {
		passed, verifyErr := w.verifyTarget(ctx, job)
		if verifyErr != nil {
			w.logger.Error("VerifyWorker verifyTarget error", zap.Int64("job_id", job.ID), zap.Error(verifyErr))
			continue
		}
		if passed {
			if err = w.markVerifyPassed(ctx, job); err != nil {
				w.logger.Error("VerifyWorker mark passed error", zap.Int64("job_id", job.ID), zap.Error(err))
			}
			continue
		}
		if err = w.markVerifyFailed(ctx, job); err != nil {
			w.logger.Error("VerifyWorker mark failed error", zap.Int64("job_id", job.ID), zap.Error(err))
		}
	}
	return nil
}

func (w *VerifyWorker) markVerifyPassed(ctx context.Context, job struct {
	ID         int64
	SKUID      int64
	AssetVerID int64
	Target     string
}) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx verify passed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET verify_status = ?, updated_at = NOW()
		WHERE id = ?`,
		domain.VerifyStatusVerified,
		job.ID,
	); err != nil {
		return fmt.Errorf("update verify_status verified: %w", err)
	}

	complete, checkErr := w.allSiblingJobsDoneAndVerified(ctx, tx, job.SKUID)
	if checkErr != nil {
		return checkErr
	}
	if complete {
		res, updateErr := tx.ExecContext(ctx, `
			UPDATE skus
			SET workflow_status = ?, updated_at = NOW()
			WHERE id = ? AND workflow_status = ?`,
			domain.WorkflowCompleted,
			job.SKUID,
			domain.WorkflowDistributionRunning,
		)
		if updateErr != nil {
			return fmt.Errorf("update sku completed: %w", updateErr)
		}
		affected, rowsErr := res.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("sku completed rows affected: %w", rowsErr)
		}
		if affected == 1 {
			if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventSKUStatusChanged, map[string]interface{}{
				"from":         string(domain.WorkflowDistributionRunning),
				"to":           string(domain.WorkflowCompleted),
				"triggered_by": "verify_worker",
			}); err != nil {
				return fmt.Errorf("append sku.status_changed completed: %w", err)
			}
		}
	}

	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventVerifyPassed, map[string]interface{}{
		"job_id":       job.ID,
		"asset_ver_id": job.AssetVerID,
		"target":       job.Target,
	}); err != nil {
		return fmt.Errorf("append verify.passed: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit verify passed tx: %w", err)
	}
	return nil
}

func (w *VerifyWorker) markVerifyFailed(ctx context.Context, job struct {
	ID         int64
	SKUID      int64
	AssetVerID int64
	Target     string
}) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx verify failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET verify_status = ?, updated_at = NOW()
		WHERE id = ?`,
		domain.VerifyStatusVerifyFailed,
		job.ID,
	); err != nil {
		return fmt.Errorf("update verify_status failed: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO incidents (sku_id, job_id, status, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())`,
		job.SKUID,
		job.ID,
		domain.IncidentStatusOpen,
		"verify failed",
	)
	if err != nil {
		return fmt.Errorf("insert verify failed incident: %w", err)
	}
	incidentID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("incident last insert id: %w", err)
	}

	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventVerifyFailed, map[string]interface{}{
		"job_id":       job.ID,
		"asset_ver_id": job.AssetVerID,
		"target":       job.Target,
	}); err != nil {
		return fmt.Errorf("append verify.failed: %w", err)
	}
	if err = workerAppendEvent(ctx, tx, job.SKUID, domain.EventIncidentCreated, map[string]interface{}{
		"incident_id": incidentID,
		"job_id":      job.ID,
		"reason":      "verify failed",
	}); err != nil {
		return fmt.Errorf("append incident.created: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit verify failed tx: %w", err)
	}
	return nil
}

func (w *VerifyWorker) allSiblingJobsDoneAndVerified(ctx context.Context, tx *sql.Tx, skuID int64) (bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT status, verify_status
		FROM distribution_jobs
		WHERE sku_id = ?`,
		skuID,
	)
	if err != nil {
		return false, fmt.Errorf("query sibling jobs: %w", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var status domain.JobStatus
		var verifyStatus domain.VerifyStatus
		if err = rows.Scan(&status, &verifyStatus); err != nil {
			return false, fmt.Errorf("scan sibling job: %w", err)
		}
		found = true
		if status != domain.JobStatusDone {
			return false, nil
		}
		if verifyStatus != domain.VerifyStatusVerified && verifyStatus != domain.VerifyStatusNotRequested {
			return false, nil
		}
	}
	if err = rows.Err(); err != nil {
		return false, fmt.Errorf("iterate sibling jobs: %w", err)
	}
	return found, nil
}

func (w *VerifyWorker) verifyTarget(_ context.Context, _ struct {
	ID         int64
	SKUID      int64
	AssetVerID int64
	Target     string
}) (bool, error) {
	// Stub for now; production implementation uses target adapters (remote size/hash checks).
	return true, nil
}
