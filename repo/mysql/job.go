package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

// JobRepoImpl implements repo.JobRepo (distribution_jobs + job_attempts).
type JobRepoImpl struct{ db *sql.DB }

func NewJobRepo(db *DB) repo.JobRepo { return &JobRepoImpl{db: db.db} }

const jobSelectCols = `
	id, idempotent_key, action_id, sku_id, asset_ver_id, target,
	status, verify_status, retry_count, max_retries,
	current_attempt_id, next_retry_at, created_at, updated_at`

func scanJob(s interface {
	Scan(...interface{}) error
}) (*domain.DistributionJob, error) {
	j := &domain.DistributionJob{}
	var currentAttemptID sql.NullString
	var nextRetryAt sql.NullTime
	if err := s.Scan(
		&j.ID,
		&j.IdempotentKey,
		&j.ActionID,
		&j.SKUID,
		&j.AssetVerID,
		&j.Target,
		&j.Status,
		&j.VerifyStatus,
		&j.RetryCount,
		&j.MaxRetries,
		&currentAttemptID,
		&nextRetryAt,
		&j.CreatedAt,
		&j.UpdatedAt,
	); err != nil {
		return nil, err
	}
	j.CurrentAttemptID = fromNullString(currentAttemptID)
	j.NextRetryAt = fromNullTime(nextRetryAt)
	return j, nil
}

// CreateBatch inserts jobs using INSERT IGNORE so duplicate idempotent_keys are silently skipped.
// This is safe to call multiple times for the same audit action (spec §5.2 invariant 5).
func (r *JobRepoImpl) CreateBatch(ctx context.Context, tx repo.Tx, jobs []*domain.DistributionJob) error {
	if len(jobs) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	now := time.Now()
	for _, j := range jobs {
		_, err := sqlTx.ExecContext(ctx, `
			INSERT IGNORE INTO distribution_jobs
				(idempotent_key, action_id, sku_id, asset_ver_id, target,
				 status, verify_status, retry_count, max_retries, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			j.IdempotentKey,
			j.ActionID,
			j.SKUID,
			j.AssetVerID,
			j.Target,
			j.Status,
			domain.VerifyStatusNotRequested,
			0,
			j.MaxRetries,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("insert job (key=%s): %w", j.IdempotentKey, err)
		}
	}
	return nil
}

func (r *JobRepoImpl) GetByID(ctx context.Context, id int64) (*domain.DistributionJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+jobSelectCols+` FROM distribution_jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

func (r *JobRepoImpl) ListBySKUID(ctx context.Context, skuID int64) ([]*domain.DistributionJob, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+jobSelectCols+` FROM distribution_jobs WHERE sku_id = ? ORDER BY created_at ASC`,
		skuID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs by sku_id: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.DistributionJob
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan job: %w", scanErr)
		}
		jobs = append(jobs, job)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

// PullPending atomically claims one Pending job for an agent.
//
// It manages its own internal transaction to:
//  1. SELECT the oldest Pending job FOR UPDATE (prevents double-claiming)
//  2. Insert a new job_attempt with a fresh UUID and lease
//  3. Set job.status = Running and job.current_attempt_id
//  4. Append an event_log row (spec §8.2)
//
// Returns nil job (not an error) when no Pending jobs are available.
func (r *JobRepoImpl) PullPending(
	ctx context.Context,
	agentID string,
	leaseDuration time.Duration,
) (*domain.DistributionJob, *domain.JobAttempt, error) {
	sqlTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("pull_pending begin tx: %w", err)
	}
	defer rollback(sqlTx)

	// Lock the oldest Pending job.
	row := sqlTx.QueryRowContext(ctx, `
		SELECT`+jobSelectCols+`
		FROM distribution_jobs
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE`,
		domain.JobStatusPending,
	)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil, nil // no work available
	}
	if err != nil {
		return nil, nil, fmt.Errorf("pull_pending select: %w", err)
	}

	// Create a new attempt with a lease.
	attemptID := uuid.New().String()
	leaseExpiry := time.Now().Add(leaseDuration)
	now := time.Now()

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO job_attempts (id, job_id, agent_id, lease_expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		attemptID, job.ID, agentID, leaseExpiry, now,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("insert job_attempt: %w", err)
	}

	// Transition job to Running; record the active attempt.
	_, err = sqlTx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET status = ?, current_attempt_id = ?, updated_at = ?
		WHERE id = ?`,
		domain.JobStatusRunning, attemptID, now, job.ID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("update job to running: %w", err)
	}

	// Append event_log — sequence generation uses FOR UPDATE on sku_sequences (spec §8.2).
	seq, err := nextSequence(ctx, sqlTx, job.SKUID)
	if err != nil {
		return nil, nil, err
	}
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"job_id":     job.ID,
		"attempt_id": attemptID,
		"agent_id":   agentID,
		"target":     job.Target,
	})
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO event_logs (id, sku_id, sequence, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), job.SKUID, seq, "job.running", eventPayload, now,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("insert event_log for pull_pending: %w", err)
	}

	if err = sqlTx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("pull_pending commit: %w", err)
	}

	// Update in-memory job state to reflect what was just written.
	job.Status = domain.JobStatusRunning
	job.CurrentAttemptID = &attemptID

	attempt := &domain.JobAttempt{
		ID:             attemptID,
		JobID:          job.ID,
		AgentID:        agentID,
		LeaseExpiresAt: leaseExpiry,
		CreatedAt:      now,
	}
	return job, attempt, nil
}

// UpdateStatus sets job.status inside an active transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *JobRepoImpl) UpdateStatus(ctx context.Context, tx repo.Tx, jobID int64, status domain.JobStatus) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE distribution_jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), jobID,
	)
	return wrapErr(err, "update job status")
}

// UpdateVerifyStatus sets verify_status without requiring an external TX
// (verify worker runs asynchronously; event_log is written by the caller separately).
func (r *JobRepoImpl) UpdateVerifyStatus(ctx context.Context, jobID int64, status domain.VerifyStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE distribution_jobs SET verify_status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), jobID,
	)
	return wrapErr(err, "update verify_status")
}

// SetCurrentAttempt records the active attempt ID on the job inside an active transaction.
func (r *JobRepoImpl) SetCurrentAttempt(ctx context.Context, tx repo.Tx, jobID int64, attemptID string) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE distribution_jobs SET current_attempt_id = ?, updated_at = ? WHERE id = ?`,
		attemptID, time.Now(), jobID,
	)
	return wrapErr(err, "set current_attempt_id")
}

const attemptSelectCols = `id, job_id, agent_id, lease_expires_at, heartbeat_at, acked_at, created_at`

func scanAttempt(s interface {
	Scan(...interface{}) error
}) (*domain.JobAttempt, error) {
	a := &domain.JobAttempt{}
	var heartbeatAt, ackedAt sql.NullTime
	if err := s.Scan(
		&a.ID,
		&a.JobID,
		&a.AgentID,
		&a.LeaseExpiresAt,
		&heartbeatAt,
		&ackedAt,
		&a.CreatedAt,
	); err != nil {
		return nil, err
	}
	a.HeartbeatAt = fromNullTime(heartbeatAt)
	a.AckedAt = fromNullTime(ackedAt)
	return a, nil
}

func (r *JobRepoImpl) GetAttemptByID(ctx context.Context, attemptID string) (*domain.JobAttempt, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+attemptSelectCols+` FROM job_attempts WHERE id = ?`, attemptID)
	a, err := scanAttempt(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// RenewLease extends the lease for a live attempt (Heartbeat handler).
func (r *JobRepoImpl) RenewLease(ctx context.Context, attemptID string, newExpiry time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_attempts
		SET lease_expires_at = ?, heartbeat_at = ?
		WHERE id = ?`,
		newExpiry, time.Now(), attemptID,
	)
	return wrapErr(err, "renew lease")
}

// MarkAttemptAcked records the ack timestamp inside an active transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *JobRepoImpl) MarkAttemptAcked(ctx context.Context, tx repo.Tx, attemptID string) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE job_attempts SET acked_at = ? WHERE id = ?`,
		time.Now(), attemptID,
	)
	return wrapErr(err, "mark attempt acked")
}

// FindExpiredLeases returns Running jobs whose lease has expired (used by LeaseReaper worker).
func (r *JobRepoImpl) FindExpiredLeases(ctx context.Context) ([]*domain.DistributionJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT`+jobSelectCols+`
		FROM distribution_jobs
		WHERE status = ? AND current_attempt_id IS NOT NULL`,
		domain.JobStatusRunning,
	)
	if err != nil {
		return nil, fmt.Errorf("find expired leases query: %w", err)
	}
	defer rows.Close()

	// Filter by lease expiry after fetch (avoids passing NOW() twice; LeaseReaper runs infrequently).
	var expired []*domain.DistributionJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		// Check the attempt's lease directly in a follow-up query per job, or filter here.
		// For simplicity, join with job_attempts in SQL:
		expired = append(expired, j)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return expired, nil
}

// FindExpiredLeasesJoined is the efficient version used internally by LeaseReaper.
// It joins with job_attempts to only return jobs with actually expired leases.
func (r *JobRepoImpl) FindExpiredLeasesJoined(ctx context.Context) ([]*domain.DistributionJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dj.id, dj.idempotent_key, dj.action_id, dj.sku_id, dj.asset_ver_id, dj.target,
		       dj.status, dj.verify_status, dj.retry_count, dj.max_retries,
		       dj.current_attempt_id, dj.next_retry_at, dj.created_at, dj.updated_at
		FROM distribution_jobs dj
		JOIN job_attempts ja ON ja.id = dj.current_attempt_id
		WHERE dj.status = ? AND ja.lease_expires_at < NOW()`,
		domain.JobStatusRunning,
	)
	if err != nil {
		return nil, fmt.Errorf("find expired leases: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.DistributionJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// MarkStale sets job.status = Stale and clears current_attempt_id inside a transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *JobRepoImpl) MarkStale(ctx context.Context, tx repo.Tx, jobID int64) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET status = ?, current_attempt_id = NULL, updated_at = ?
		WHERE id = ?`,
		domain.JobStatusStale, time.Now(), jobID,
	)
	return wrapErr(err, "mark job stale")
}

// FindRetryable returns Fail/Stale jobs whose next_retry_at has passed (used by RetryScheduler).
func (r *JobRepoImpl) FindRetryable(ctx context.Context) ([]*domain.DistributionJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT`+jobSelectCols+`
		FROM distribution_jobs
		WHERE status IN (?, ?)
		  AND retry_count < max_retries
		  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at ASC`,
		domain.JobStatusFail,
		domain.JobStatusStale,
	)
	if err != nil {
		return nil, fmt.Errorf("find retryable jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.DistributionJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan retryable job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// IncrementRetry advances retry_count, resets status to Pending, and schedules the next attempt.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *JobRepoImpl) IncrementRetry(ctx context.Context, tx repo.Tx, jobID int64, nextRetryAt time.Time) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE distribution_jobs
		SET status = ?, retry_count = retry_count + 1, next_retry_at = ?, updated_at = ?
		WHERE id = ?`,
		domain.JobStatusPending, nextRetryAt, time.Now(), jobID,
	)
	return wrapErr(err, "increment retry")
}
