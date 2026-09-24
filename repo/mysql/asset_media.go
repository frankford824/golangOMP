package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"workflow/domain"
	"workflow/repo"
)

type assetMediaJobRepo struct{ db *DB }

func NewAssetMediaJobRepo(db *DB) repo.AssetMediaJobRepo { return &assetMediaJobRepo{db: db} }

const mediaJobColumns = `job_id, dedupe_key, kind, pool, resource_id, source_version, recipe,
 state, phase, priority, attempts, next_run_at, lease_owner, lease_epoch, lease_expires_at,
 processed_bytes, total_bytes, payload, COALESCE(checkpoint,JSON_OBJECT()), COALESCE(result, JSON_OBJECT()), error_code,
 retryable, created_at, updated_at`

func scanMediaJob(row interface{ Scan(...interface{}) error }) (*domain.AssetMediaJob, error) {
	j := &domain.AssetMediaJob{}
	err := row.Scan(&j.ID, &j.DedupeKey, &j.Kind, &j.Pool, &j.ResourceID, &j.SourceVersion, &j.Recipe,
		&j.State, &j.Phase, &j.Priority, &j.Attempts, &j.NextRunAt, &j.LeaseOwner, &j.LeaseEpoch,
		&j.LeaseExpiresAt, &j.ProcessedBytes, &j.TotalBytes, &j.Payload, &j.Checkpoint, &j.Result, &j.ErrorCode,
		&j.Retryable, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err == nil {
		var input domain.AssetMediaJobInput
		if json.Unmarshal(j.Payload, &input) == nil {
			j.Filename = input.Filename
		}
	}
	return j, err
}

func (r *assetMediaJobRepo) Enqueue(ctx context.Context, j *domain.AssetMediaJob, actorID int64) (*domain.AssetMediaJob, string, error) {
	if j == nil || j.ResourceID == "" || j.SourceVersion == "" || (j.Pool != "ecs" && j.Pool != "nas" && j.Pool != "scan") || !json.Valid(j.Payload) {
		return nil, "", fmt.Errorf("invalid media job identity or payload")
	}
	j.SetIdentity()
	refreshMissing := j.RefreshMissing
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	if j.Kind == "external_selection_zip" {
		_, err = tx.ExecContext(ctx, `UPDATE asset_media_jobs SET dedupe_key=SHA2(CONCAT(dedupe_key,':expired:',job_id),256)
 WHERE dedupe_key=? AND kind='external_selection_zip' AND state IN ('succeeded','failed','stale')
 AND updated_at<DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 24 HOUR)`, j.DedupeKey)
		if err != nil {
			return nil, "", err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO asset_media_jobs
 (job_id,dedupe_key,kind,pool,resource_id,source_version,recipe,priority,payload,total_bytes)
 VALUES (?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE priority=LEAST(priority,VALUES(priority))`,
		j.ID, j.DedupeKey, j.Kind, j.Pool, j.ResourceID, j.SourceVersion, j.Recipe, j.Priority, []byte(j.Payload), j.TotalBytes)
	if err != nil {
		return nil, "", err
	}
	j, err = scanMediaJob(tx.QueryRowContext(ctx, `SELECT `+mediaJobColumns+` FROM asset_media_jobs WHERE dedupe_key=?`, j.DedupeKey))
	if err != nil || j == nil {
		return nil, "", fmt.Errorf("read enqueued media job: %w", err)
	}
	if refreshMissing && j.State == "succeeded" {
		_, err = tx.ExecContext(ctx, `UPDATE asset_media_jobs SET state='queued',phase='queued',attempts=0,
 next_run_at=UTC_TIMESTAMP(6),result=NULL,error_code='',retryable=0 WHERE job_id=? AND state='succeeded'`, j.ID)
		if err != nil {
			return nil, "", err
		}
		j.State = "queued"
		j.Phase = "queued"
		j.Attempts = 0
		j.Result = nil
	}
	requestID := ""
	if actorID > 0 {
		requestID = uuid.NewString()
		access := domain.AssetMediaOptionsFromContext(ctx).AccessReference
		var accessJSON any
		if access != nil {
			copy := *access
			copy.ExpectedSourceVersion = j.SourceVersion
			if copy.ResourceKind == "package" && strings.HasPrefix(copy.ResourceID, "selection:") {
				copy.ResourceID = j.ID
			}
			raw, _ := json.Marshal(copy)
			accessJSON = raw
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO asset_media_requests (request_id,job_id,actor_id,access_reference)
 VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE cancelled=0,access_reference=COALESCE(VALUES(access_reference),access_reference),updated_at=UTC_TIMESTAMP(6)`, requestID, j.ID, actorID, accessJSON)
		if err != nil {
			return nil, "", err
		}
		err = tx.QueryRowContext(ctx, `SELECT request_id FROM asset_media_requests WHERE job_id=? AND actor_id=?`, j.ID, actorID).Scan(&requestID)
		if err != nil {
			return nil, "", err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	return j, requestID, nil
}

func (r *assetMediaJobRepo) Get(ctx context.Context, id string) (*domain.AssetMediaJob, error) {
	return scanMediaJob(r.db.db.QueryRowContext(ctx, `SELECT `+mediaJobColumns+` FROM asset_media_jobs WHERE job_id=?`, id))
}

func (r *assetMediaJobRepo) AssertLease(ctx context.Context, tx repo.Tx, id, worker string, epoch int64) error {
	var found string
	err := Unwrap(tx).QueryRowContext(ctx, `SELECT job_id FROM asset_media_jobs WHERE job_id=? AND state='processing'
 AND lease_owner=? AND lease_epoch=? AND lease_expires_at>UTC_TIMESTAMP(6) FOR UPDATE`, id, worker, epoch).Scan(&found)
	if err != nil {
		return fmt.Errorf("media job lease rejected: %w", err)
	}
	return nil
}

func (r *assetMediaJobRepo) SaveCheckpoint(ctx context.Context, id, worker string, epoch int64, raw json.RawMessage) error {
	if !json.Valid(raw) || len(raw) > 4<<20 {
		return fmt.Errorf("invalid media checkpoint")
	}
	return requireMediaLease(r.db.db.ExecContext(ctx, `UPDATE asset_media_jobs SET checkpoint=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND lease_owner=? AND lease_epoch=? AND state='processing' AND lease_expires_at>UTC_TIMESTAMP(6)`, []byte(raw), id, worker, epoch))
}

func (r *assetMediaJobRepo) Claim(ctx context.Context, pool, worker string, limit int, kinds ...string) ([]*domain.AssetMediaJob, error) {
	if pool != "ecs" && pool != "nas" && pool != "scan" {
		return nil, fmt.Errorf("invalid media worker pool")
	}
	if strings.TrimSpace(worker) == "" || len(worker) > 128 {
		return nil, fmt.Errorf("invalid worker identity")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 2 {
		limit = 2
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE asset_media_jobs SET state='failed',phase='failed',retryable=1,
 error_code='worker_lease_exhausted',lease_owner='',lease_expires_at=NULL
 WHERE pool=? AND state='processing' AND lease_expires_at<=UTC_TIMESTAMP(6) AND attempts>=4`, pool)
	if err != nil {
		return nil, err
	}
	kindFilter := ""
	args := []any{pool}
	if len(kinds) > 0 {
		marks := []string{}
		for _, kind := range kinds {
			marks = append(marks, "?")
			args = append(args, kind)
		}
		kindFilter = " AND kind IN (" + strings.Join(marks, ",") + ")"
	}
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, `SELECT job_id FROM asset_media_jobs WHERE pool=?`+kindFilter+` AND attempts<4 AND
 (((state='queued' OR state='retry_wait') AND next_run_at<=UTC_TIMESTAMP(6)) OR
 (state='processing' AND lease_expires_at<=UTC_TIMESTAMP(6)))
 ORDER BY GREATEST(IF(priority>=20,11,0),priority-FLOOR(TIMESTAMPDIFF(MINUTE,created_at,UTC_TIMESTAMP())/15)),created_at
 LIMIT ? FOR UPDATE SKIP LOCKED`, args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	jobs := make([]*domain.AssetMediaJob, 0, len(ids))
	for _, id := range ids {
		_, err = tx.ExecContext(ctx, `UPDATE asset_media_jobs SET state='processing',phase='starting',
 attempts=attempts+1,lease_epoch=lease_epoch+1,lease_owner=?,lease_expires_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 120 SECOND),
 error_code='',processed_bytes=0 WHERE job_id=?`, worker, id)
		if err != nil {
			return nil, err
		}
		j, e := scanMediaJob(tx.QueryRowContext(ctx, `SELECT `+mediaJobColumns+` FROM asset_media_jobs WHERE job_id=?`, id))
		if e != nil {
			return nil, e
		}
		jobs = append(jobs, j)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func requireMediaLease(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("media job lease lost or expired")
	}
	return nil
}

func (r *assetMediaJobRepo) Heartbeat(ctx context.Context, id, worker string, epoch int64, phase string, done, total int64) error {
	if len(phase) > 32 || done < 0 || total < 0 || (total > 0 && done > total) {
		return fmt.Errorf("invalid media progress")
	}
	return requireMediaLease(r.db.db.ExecContext(ctx, `UPDATE asset_media_jobs SET
 processed_bytes=IF(phase=?,GREATEST(processed_bytes,?),?),total_bytes=?,phase=?,
 lease_expires_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 120 SECOND)
 WHERE job_id=? AND state='processing' AND lease_owner=? AND lease_epoch=? AND lease_expires_at>UTC_TIMESTAMP(6)`, phase, done, done, total, phase, id, worker, epoch))
}

func (r *assetMediaJobRepo) Finish(ctx context.Context, id, worker string, epoch int64, state string, result json.RawMessage, code string, retryable bool) error {
	if state != "succeeded" && state != "failed" && state != "stale" {
		return fmt.Errorf("invalid terminal state")
	}
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	if !json.Valid(result) {
		return fmt.Errorf("invalid media result")
	}
	if len(code) > 128 {
		code = code[:128]
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT attempts FROM asset_media_jobs WHERE job_id=? AND state='processing'
 AND lease_owner=? AND lease_epoch=? AND lease_expires_at>UTC_TIMESTAMP(6) FOR UPDATE`, id, worker, epoch).Scan(&attempts)
	if err != nil {
		return fmt.Errorf("media completion rejected: %w", err)
	}
	delay := 0
	if state == "failed" && retryable && attempts < 4 {
		state = "retry_wait"
		delays := []int{60, 300, 1800}
		if attempts < 1 {
			attempts = 1
		}
		delay = delays[attempts-1]
	}
	_, err = tx.ExecContext(ctx, `UPDATE asset_media_jobs SET state=?,phase=?,result=?,error_code=?,retryable=?,
 next_run_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL ? SECOND),lease_owner='',lease_expires_at=NULL
 WHERE job_id=? AND lease_epoch=?`, state, state, []byte(result), code, retryable, delay, id, epoch)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *assetMediaJobRepo) HasRequest(ctx context.Context, id string, actor int64) (bool, error) {
	var n int
	err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_media_requests WHERE job_id=? AND actor_id=? AND cancelled=0`, id, actor).Scan(&n)
	return n > 0, err
}

func (r *assetMediaJobRepo) GetRequestAccess(ctx context.Context, id string, actor int64) (*domain.AssetMediaAccessReference, error) {
	var raw []byte
	err := r.db.db.QueryRowContext(ctx, `SELECT access_reference FROM asset_media_requests WHERE job_id=? AND actor_id=? AND cancelled=0`, id, actor).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || len(raw) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var access domain.AssetMediaAccessReference
	if err = json.Unmarshal(raw, &access); err != nil {
		return nil, err
	}
	return &access, nil
}

func (r *assetMediaJobRepo) ListRequests(ctx context.Context, actor int64, since time.Time) ([]domain.AssetMediaRequest, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT r.request_id,r.cancelled,r.created_at,r.updated_at,r.job_id,r.access_reference
 FROM asset_media_requests r JOIN asset_media_jobs j ON j.job_id=r.job_id
 WHERE r.actor_id=? AND (r.updated_at>=? OR j.state IN ('queued','processing','retry_wait'))
 AND JSON_UNQUOTE(JSON_EXTRACT(r.access_reference,'$.purpose'))='download'
 ORDER BY (j.state IN ('queued','processing','retry_wait')) DESC,r.updated_at DESC LIMIT 100`, actor, since.UTC())
	if err != nil {
		return nil, err
	}
	requests := []domain.AssetMediaRequest{}
	ids := []string{}
	for rows.Next() {
		var request domain.AssetMediaRequest
		var id string
		var accessJSON []byte
		request.ActorID = actor
		if err = rows.Scan(&request.ID, &request.Cancelled, &request.CreatedAt, &request.UpdatedAt, &id, &accessJSON); err != nil {
			rows.Close()
			return nil, err
		}
		if len(accessJSON) > 0 {
			var access domain.AssetMediaAccessReference
			if json.Unmarshal(accessJSON, &access) == nil {
				request.AccessReference = &access
			}
		}
		requests = append(requests, request)
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		requests[i].Job, err = r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	return requests, nil
}

func (r *assetMediaJobRepo) CancelRequest(ctx context.Context, id string, actor int64) error {
	_, err := r.db.db.ExecContext(ctx, `UPDATE asset_media_requests SET cancelled=1 WHERE request_id=? AND actor_id=?`, id, actor)
	return err
}

func (r *assetMediaJobRepo) Retry(ctx context.Context, id string, actor int64) error {
	ok, err := r.HasRequest(ctx, id, actor)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("media request not found")
	}
	res, err := r.db.db.ExecContext(ctx, `UPDATE asset_media_jobs SET state='queued',phase='queued',attempts=0,
 next_run_at=UTC_TIMESTAMP(6),manual_retry_at=UTC_TIMESTAMP(6)
 WHERE job_id=? AND state='failed' AND retryable=1
 AND (manual_retry_at IS NULL OR manual_retry_at<DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 60 SECOND))`, id)
	return requireMediaLease(res, err)
}
