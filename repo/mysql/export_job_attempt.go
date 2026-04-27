package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type exportJobAttemptRepo struct{ db *DB }

func NewExportJobAttemptRepo(db *DB) repo.ExportJobAttemptRepo {
	return &exportJobAttemptRepo{db: db}
}

const exportJobAttemptSelectCols = `
	attempt_id, export_job_id, dispatch_id, attempt_no, trigger_source, execution_mode, adapter_key,
	status, started_at, finished_at, error_message, adapter_note, created_at, updated_at`

const exportJobAttemptSelectColsAliased = `
	a.attempt_id, a.export_job_id, a.dispatch_id, a.attempt_no, a.trigger_source, a.execution_mode, a.adapter_key,
	a.status, a.started_at, a.finished_at, a.error_message, a.adapter_note, a.created_at, a.updated_at`

func (r *exportJobAttemptRepo) Create(ctx context.Context, tx repo.Tx, attempt *domain.ExportJobAttempt) (*domain.ExportJobAttempt, error) {
	if attempt == nil {
		return nil, fmt.Errorf("create export job attempt: attempt is nil")
	}
	sqlTx := Unwrap(tx)

	attemptID := strings.TrimSpace(attempt.AttemptID)
	if attemptID == "" {
		attemptID = uuid.NewString()
	}
	attemptNo := attempt.AttemptNo
	if attemptNo <= 0 {
		nextNo, err := nextExportJobAttemptNo(ctx, sqlTx, attempt.ExportJobID)
		if err != nil {
			return nil, err
		}
		attemptNo = nextNo
	}

	startedAt := attempt.StartedAt.UTC()
	createdAt := attempt.CreatedAt
	updatedAt := attempt.UpdatedAt
	if createdAt.IsZero() {
		createdAt = startedAt
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	createdAt = createdAt.UTC()
	updatedAt = updatedAt.UTC()
	finishedAt := toNullTime(attempt.FinishedAt)

	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO export_job_attempts (
			attempt_id, export_job_id, dispatch_id, attempt_no, trigger_source, execution_mode, adapter_key,
			status, started_at, finished_at, error_message, adapter_note, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attemptID,
		attempt.ExportJobID,
		nullIfEmpty(attempt.DispatchID),
		attemptNo,
		strings.TrimSpace(attempt.TriggerSource),
		string(attempt.ExecutionMode),
		string(attempt.AdapterKey),
		string(attempt.Status),
		startedAt,
		finishedAt,
		strings.TrimSpace(attempt.ErrorMessage),
		strings.TrimSpace(attempt.AdapterNote),
		createdAt,
		updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert export job attempt: %w", err)
	}

	copyAttempt := *attempt
	copyAttempt.AttemptID = attemptID
	copyAttempt.AttemptNo = attemptNo
	copyAttempt.StartedAt = startedAt
	copyAttempt.CreatedAt = createdAt
	copyAttempt.UpdatedAt = updatedAt
	domain.HydrateExportJobAttemptDerived(&copyAttempt)
	return &copyAttempt, nil
}

func (r *exportJobAttemptRepo) GetLatestByExportJobID(ctx context.Context, exportJobID int64) (*domain.ExportJobAttempt, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+exportJobAttemptSelectCols+`
		FROM export_job_attempts
		WHERE export_job_id = ?
		ORDER BY attempt_no DESC
		LIMIT 1`, exportJobID)
	attempt, err := scanExportJobAttempt(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest export job attempt: %w", err)
	}
	return attempt, nil
}

func (r *exportJobAttemptRepo) ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobAttempt, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+exportJobAttemptSelectCols+`
		FROM export_job_attempts
		WHERE export_job_id = ?
		ORDER BY attempt_no DESC`, exportJobID)
	if err != nil {
		return nil, fmt.Errorf("list export job attempts: %w", err)
	}
	defer rows.Close()

	attempts := make([]*domain.ExportJobAttempt, 0)
	for rows.Next() {
		attempt, err := scanExportJobAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("scan export job attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func (r *exportJobAttemptRepo) Update(ctx context.Context, tx repo.Tx, update repo.ExportJobAttemptUpdate) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE export_job_attempts
		SET status = ?, finished_at = ?, error_message = ?, adapter_note = ?, updated_at = ?
		WHERE attempt_id = ?`,
		string(update.Status),
		toNullTime(update.FinishedAt),
		strings.TrimSpace(update.ErrorMessage),
		strings.TrimSpace(update.AdapterNote),
		time.Now().UTC(),
		strings.TrimSpace(update.AttemptID),
	)
	if err != nil {
		return fmt.Errorf("update export job attempt: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update export job attempt rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *exportJobAttemptRepo) SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobAttemptAggregate, error) {
	out := make(map[int64]repo.ExportJobAttemptAggregate, len(exportJobIDs))
	if len(exportJobIDs) == 0 {
		return out, nil
	}
	for _, exportJobID := range exportJobIDs {
		out[exportJobID] = repo.ExportJobAttemptAggregate{}
	}

	inClause, args := buildInt64InClause("export_job_id", exportJobIDs)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT export_job_id, COUNT(*)
		FROM export_job_attempts
		WHERE `+inClause+`
		GROUP BY export_job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("count export job attempts: %w", err)
	}
	for rows.Next() {
		var exportJobID int64
		var count int64
		if err := rows.Scan(&exportJobID, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan export job attempt count: %w", err)
		}
		aggregate := out[exportJobID]
		aggregate.AttemptCount = count
		out[exportJobID] = aggregate
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate export job attempt counts: %w", err)
	}
	rows.Close()

	latestRows, err := r.db.db.QueryContext(ctx, `
		SELECT `+exportJobAttemptSelectColsAliased+`
		FROM export_job_attempts a
		INNER JOIN (
			SELECT export_job_id, MAX(attempt_no) AS latest_attempt_no
			FROM export_job_attempts
			WHERE `+inClause+`
			GROUP BY export_job_id
		) latest
			ON latest.export_job_id = a.export_job_id AND latest.latest_attempt_no = a.attempt_no
		ORDER BY a.export_job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest export job attempts: %w", err)
	}
	defer latestRows.Close()
	for latestRows.Next() {
		attempt, err := scanExportJobAttempt(latestRows)
		if err != nil {
			return nil, fmt.Errorf("scan latest export job attempt: %w", err)
		}
		aggregate := out[attempt.ExportJobID]
		aggregate.LatestAttempt = attempt
		out[attempt.ExportJobID] = aggregate
	}
	if err := latestRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest export job attempts: %w", err)
	}
	return out, nil
}

func scanExportJobAttempt(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExportJobAttempt, error) {
	attempt := &domain.ExportJobAttempt{}
	var executionMode string
	var adapterKey string
	var status string
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&attempt.AttemptID,
		&attempt.ExportJobID,
		&attempt.DispatchID,
		&attempt.AttemptNo,
		&attempt.TriggerSource,
		&executionMode,
		&adapterKey,
		&status,
		&attempt.StartedAt,
		&finishedAt,
		&attempt.ErrorMessage,
		&attempt.AdapterNote,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	); err != nil {
		return nil, err
	}
	attempt.ExecutionMode = domain.ExportJobExecutionMode(executionMode)
	attempt.AdapterKey = domain.ExportJobRunnerAdapterKey(adapterKey)
	attempt.Status = domain.ExportJobAttemptStatus(status)
	attempt.FinishedAt = fromNullTime(finishedAt)
	domain.HydrateExportJobAttemptDerived(attempt)
	return attempt, nil
}

func nextExportJobAttemptNo(ctx context.Context, sqlTx *sql.Tx, exportJobID int64) (int, error) {
	var lastAttemptNo int
	err := sqlTx.QueryRowContext(ctx, `
		SELECT attempt_no
		FROM export_job_attempts
		WHERE export_job_id = ?
		ORDER BY attempt_no DESC
		LIMIT 1
		FOR UPDATE`, exportJobID).Scan(&lastAttemptNo)
	if err == sql.ErrNoRows {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("next export job attempt no: %w", err)
	}
	return lastAttemptNo + 1, nil
}

func nullIfEmpty(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
