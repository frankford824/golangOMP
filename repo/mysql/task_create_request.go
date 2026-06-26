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

type taskCreateRequestRepo struct {
	db *DB
}

func NewTaskCreateRequestRepo(db *DB) repo.TaskCreateRequestRepo {
	return &taskCreateRequestRepo{db: db}
}

func (r *taskCreateRequestRepo) Reserve(ctx context.Context, actorID int64, clientCreateID, payloadHash, requestPayloadJSON string, expiresAt time.Time) (*domain.TaskCreateRequest, string, error) {
	clientCreateID = strings.TrimSpace(clientCreateID)
	payloadHash = strings.TrimSpace(payloadHash)
	if actorID <= 0 || clientCreateID == "" || payloadHash == "" {
		return nil, "", fmt.Errorf("actor_id, client_create_id and payload_hash are required")
	}

	sqlTx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin task create request reserve: %w", err)
	}
	defer rollback(sqlTx)

	res, err := sqlTx.ExecContext(ctx, `
		INSERT IGNORE INTO task_create_requests
		  (client_create_id, actor_id, payload_hash, request_payload_json, status, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		clientCreateID,
		actorID,
		payloadHash,
		requestPayloadJSON,
		domain.TaskCreateRequestStatusInProgress,
		expiresAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("insert task_create_requests: %w", err)
	}
	insertedRows, _ := res.RowsAffected()

	record, err := scanTaskCreateRequest(sqlTx.QueryRowContext(ctx, `
		SELECT id, client_create_id, actor_id, payload_hash, status, task_id,
		       COALESCE(error_message, ''), COALESCE(request_payload_json, ''), expires_at, created_at, updated_at
		FROM task_create_requests
		WHERE actor_id = ? AND client_create_id = ?
		FOR UPDATE`,
		actorID,
		clientCreateID,
	))
	if err != nil {
		return nil, "", fmt.Errorf("select task_create_requests for update: %w", err)
	}

	now := time.Now()
	state := domain.TaskCreateRequestReserveStarted
	switch {
	case insertedRows > 0:
		state = domain.TaskCreateRequestReserveStarted
	case strings.TrimSpace(record.PayloadHash) != payloadHash:
		state = domain.TaskCreateRequestReservePayloadConflict
	case record.TaskID != nil && *record.TaskID > 0 && record.Status == domain.TaskCreateRequestStatusSucceeded:
		state = domain.TaskCreateRequestReserveReplay
	case record.Status == domain.TaskCreateRequestStatusInProgress && record.ExpiresAt != nil && record.ExpiresAt.After(now):
		state = domain.TaskCreateRequestReserveInProgress
	default:
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE task_create_requests
			SET payload_hash = ?,
			    request_payload_json = ?,
			    status = ?,
			    task_id = NULL,
			    error_message = '',
			    expires_at = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			payloadHash,
			requestPayloadJSON,
			domain.TaskCreateRequestStatusInProgress,
			expiresAt,
			record.ID,
		); err != nil {
			return nil, "", fmt.Errorf("reset task_create_requests: %w", err)
		}
		record.PayloadHash = payloadHash
		record.RequestPayload = requestPayloadJSON
		record.Status = domain.TaskCreateRequestStatusInProgress
		record.TaskID = nil
		record.ErrorMessage = ""
		record.ExpiresAt = &expiresAt
		state = domain.TaskCreateRequestReserveStarted
	}

	if err := sqlTx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit task create request reserve: %w", err)
	}
	return record, state, nil
}

func (r *taskCreateRequestRepo) MarkSucceeded(ctx context.Context, tx repo.Tx, actorID int64, clientCreateID, payloadHash string, taskID int64) error {
	if actorID <= 0 || strings.TrimSpace(clientCreateID) == "" || strings.TrimSpace(payloadHash) == "" || taskID <= 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE task_create_requests
		SET status = ?,
		    task_id = ?,
		    error_message = '',
		    expires_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE actor_id = ? AND client_create_id = ? AND payload_hash = ?`,
		domain.TaskCreateRequestStatusSucceeded,
		taskID,
		actorID,
		strings.TrimSpace(clientCreateID),
		strings.TrimSpace(payloadHash),
	)
	if err != nil {
		return fmt.Errorf("mark task_create_requests succeeded: %w", err)
	}
	return nil
}

func (r *taskCreateRequestRepo) FindRecentActiveByActorPayloadHash(ctx context.Context, actorID int64, payloadHash string, since time.Time) (*domain.TaskCreateRequest, error) {
	payloadHash = strings.TrimSpace(payloadHash)
	if actorID <= 0 || payloadHash == "" {
		return nil, nil
	}
	record, err := scanTaskCreateRequest(r.db.db.QueryRowContext(ctx, `
		SELECT id, client_create_id, actor_id, payload_hash, status, task_id,
		       COALESCE(error_message, ''), COALESCE(request_payload_json, ''), expires_at, created_at, updated_at
		FROM task_create_requests
		WHERE actor_id = ?
		  AND payload_hash = ?
		  AND updated_at >= ?
		  AND status IN (?, ?)
		ORDER BY
		  CASE WHEN status = ? AND task_id IS NOT NULL THEN 0 ELSE 1 END,
		  updated_at DESC
		LIMIT 1`,
		actorID,
		payloadHash,
		since,
		domain.TaskCreateRequestStatusSucceeded,
		domain.TaskCreateRequestStatusInProgress,
		domain.TaskCreateRequestStatusSucceeded,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find recent task_create_requests by payload hash: %w", err)
	}
	return record, nil
}

func (r *taskCreateRequestRepo) MarkFailed(ctx context.Context, actorID int64, clientCreateID, payloadHash, errorMessage string) error {
	if actorID <= 0 || strings.TrimSpace(clientCreateID) == "" || strings.TrimSpace(payloadHash) == "" {
		return nil
	}
	if len(errorMessage) > 500 {
		errorMessage = errorMessage[:500]
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE task_create_requests
		SET status = ?,
		    error_message = ?,
		    expires_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE actor_id = ?
		  AND client_create_id = ?
		  AND payload_hash = ?
		  AND task_id IS NULL
		  AND status = ?`,
		domain.TaskCreateRequestStatusFailed,
		strings.TrimSpace(errorMessage),
		actorID,
		strings.TrimSpace(clientCreateID),
		strings.TrimSpace(payloadHash),
		domain.TaskCreateRequestStatusInProgress,
	)
	if err != nil {
		return fmt.Errorf("mark task_create_requests failed: %w", err)
	}
	return nil
}

func scanTaskCreateRequest(row *sql.Row) (*domain.TaskCreateRequest, error) {
	var record domain.TaskCreateRequest
	var taskID sql.NullInt64
	var expiresAt sql.NullTime
	if err := row.Scan(
		&record.ID,
		&record.ClientCreateID,
		&record.ActorID,
		&record.PayloadHash,
		&record.Status,
		&taskID,
		&record.ErrorMessage,
		&record.RequestPayload,
		&expiresAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if taskID.Valid {
		record.TaskID = &taskID.Int64
	}
	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}
	return &record, nil
}
