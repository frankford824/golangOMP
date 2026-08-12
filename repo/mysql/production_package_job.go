package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type productionPackageJobRepo struct{ db *DB }

func NewProductionPackageJobRepo(db *DB) repo.ProductionPackageJobRepo {
	return &productionPackageJobRepo{db: db}
}

const productionPackageJobSelect = `SELECT id, job_id, status, requested_by,
	request_payload_json, COALESCE(result_payload_json, JSON_OBJECT()), total_count,
	processed_count, failed_count, error_message, lease_owner, lease_expires_at,
	started_at, finished_at, created_at, updated_at
	FROM asset_workbench_batch_jobs`

func (r *productionPackageJobRepo) Create(ctx context.Context, job *domain.ProductionPackageJob) error {
	if job == nil {
		return fmt.Errorf("production package job is required")
	}
	_, err := r.db.db.ExecContext(ctx, `INSERT INTO asset_workbench_batch_jobs
		(job_id, job_type, status, action, selection_scope, requested_by,
		 request_payload_json, total_count)
		VALUES (?, ?, ?, 'build', 'main_ops', ?, ?, ?)`,
		job.JobID, domain.ProductionPackageJobType, string(job.Status), job.RequestedBy,
		job.RequestPayload, job.TotalCount)
	if err != nil {
		return fmt.Errorf("create production package job: %w", err)
	}
	return nil
}

func (r *productionPackageJobRepo) Get(ctx context.Context, jobID string) (*domain.ProductionPackageJob, error) {
	return scanProductionPackageJob(r.db.db.QueryRowContext(ctx,
		productionPackageJobSelect+` WHERE job_type = ? AND job_id = ?`,
		domain.ProductionPackageJobType, strings.TrimSpace(jobID)))
}

func (r *productionPackageJobRepo) Claim(ctx context.Context, workerID string, limit int, leaseUntil time.Time) ([]*domain.ProductionPackageJob, error) {
	if limit <= 0 {
		limit = 1
	}
	if limit > 4 {
		limit = 4
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin production package claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM asset_workbench_batch_jobs
		WHERE job_type = ? AND (status = 'queued' OR
		(status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= UTC_TIMESTAMP()))
		ORDER BY created_at, id LIMIT ? FOR UPDATE`, domain.ProductionPackageJobType, limit)
	if err != nil {
		return nil, fmt.Errorf("select production package claims: %w", err)
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	jobs := make([]*domain.ProductionPackageJob, 0, len(ids))
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE asset_workbench_batch_jobs
			SET status='running', lease_owner=?, lease_expires_at=?,
			started_at=COALESCE(started_at, UTC_TIMESTAMP()) WHERE id=?`, workerID, leaseUntil.UTC(), id); err != nil {
			return nil, fmt.Errorf("claim production package job: %w", err)
		}
		job, err := scanProductionPackageJob(tx.QueryRowContext(ctx, productionPackageJobSelect+` WHERE id=?`, id))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit production package claims: %w", err)
	}
	return jobs, nil
}

func (r *productionPackageJobRepo) UpdateProgress(ctx context.Context, jobID, workerID string, processedCount, failedCount int) error {
	_, err := r.db.db.ExecContext(ctx, `UPDATE asset_workbench_batch_jobs
		SET processed_count=?, failed_count=?, lease_expires_at=DATE_ADD(UTC_TIMESTAMP(), INTERVAL 30 MINUTE)
		WHERE job_type=? AND job_id=? AND status='running' AND lease_owner=?`,
		processedCount, failedCount, domain.ProductionPackageJobType, jobID, workerID)
	return err
}

func (r *productionPackageJobRepo) Complete(ctx context.Context, jobID, workerID string, result []byte, failedCount int, finishedAt time.Time) error {
	res, err := r.db.db.ExecContext(ctx, `UPDATE asset_workbench_batch_jobs
		SET status='succeeded', result_payload_json=?, processed_count=total_count,
		failed_count=?, lease_owner='', lease_expires_at=NULL, finished_at=?
		WHERE job_type=? AND job_id=? AND status='running' AND lease_owner=?`,
		result, failedCount, finishedAt.UTC(), domain.ProductionPackageJobType, jobID, workerID)
	if err != nil {
		return err
	}
	return requireProductionPackageJobUpdate(res)
}

func (r *productionPackageJobRepo) Fail(ctx context.Context, jobID, workerID, message string, finishedAt time.Time) error {
	res, err := r.db.db.ExecContext(ctx, `UPDATE asset_workbench_batch_jobs
		SET status='failed', error_message=?, lease_owner='', lease_expires_at=NULL, finished_at=?
		WHERE job_type=? AND job_id=? AND status='running' AND lease_owner=?`,
		truncateProductionPackageError(message), finishedAt.UTC(), domain.ProductionPackageJobType, jobID, workerID)
	if err != nil {
		return err
	}
	return requireProductionPackageJobUpdate(res)
}

func (r *productionPackageJobRepo) FailWithResult(ctx context.Context, jobID, workerID, message string, result []byte, failedCount int, finishedAt time.Time) error {
	res, err := r.db.db.ExecContext(ctx, `UPDATE asset_workbench_batch_jobs
		SET status='failed', error_message=?, result_payload_json=?, processed_count=total_count,
		failed_count=?, lease_owner='', lease_expires_at=NULL, finished_at=?
		WHERE job_type=? AND job_id=? AND status='running' AND lease_owner=?`,
		truncateProductionPackageError(message), result, failedCount, finishedAt.UTC(),
		domain.ProductionPackageJobType, jobID, workerID)
	if err != nil {
		return err
	}
	return requireProductionPackageJobUpdate(res)
}

type productionPackageJobScanner interface{ Scan(...interface{}) error }

func scanProductionPackageJob(row productionPackageJobScanner) (*domain.ProductionPackageJob, error) {
	job := &domain.ProductionPackageJob{}
	var status string
	if err := row.Scan(&job.ID, &job.JobID, &status, &job.RequestedBy,
		&job.RequestPayload, &job.ResultPayload, &job.TotalCount, &job.ProcessedCount,
		&job.FailedCount, &job.ErrorMessage, &job.LeaseOwner, &job.LeaseExpiresAt,
		&job.StartedAt, &job.FinishedAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return nil, err
	}
	job.Status = domain.ProductionPackageJobStatus(status)
	return job, nil
}

func requireProductionPackageJobUpdate(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func truncateProductionPackageError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1000 {
		value = value[:1000]
	}
	return value
}
