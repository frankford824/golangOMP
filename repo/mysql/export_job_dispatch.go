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

type exportJobDispatchRepo struct{ db *DB }

func NewExportJobDispatchRepo(db *DB) repo.ExportJobDispatchRepo {
	return &exportJobDispatchRepo{db: db}
}

const exportJobDispatchSelectCols = `
	dispatch_id, export_job_id, dispatch_no, trigger_source, execution_mode, adapter_key,
	status, submitted_at, received_at, finished_at, expires_at, status_reason, adapter_note, created_at, updated_at`

const exportJobDispatchSelectColsAliased = `
	d.dispatch_id, d.export_job_id, d.dispatch_no, d.trigger_source, d.execution_mode, d.adapter_key,
	d.status, d.submitted_at, d.received_at, d.finished_at, d.expires_at, d.status_reason, d.adapter_note, d.created_at, d.updated_at`

func (r *exportJobDispatchRepo) Create(ctx context.Context, tx repo.Tx, dispatch *domain.ExportJobDispatch) (*domain.ExportJobDispatch, error) {
	if dispatch == nil {
		return nil, fmt.Errorf("create export job dispatch: dispatch is nil")
	}
	sqlTx := Unwrap(tx)

	dispatchID := strings.TrimSpace(dispatch.DispatchID)
	if dispatchID == "" {
		dispatchID = uuid.NewString()
	}
	dispatchNo := dispatch.DispatchNo
	if dispatchNo <= 0 {
		nextNo, err := nextExportJobDispatchNo(ctx, sqlTx, dispatch.ExportJobID)
		if err != nil {
			return nil, err
		}
		dispatchNo = nextNo
	}

	submittedAt := dispatch.SubmittedAt.UTC()
	createdAt := dispatch.CreatedAt
	updatedAt := dispatch.UpdatedAt
	if createdAt.IsZero() {
		createdAt = submittedAt
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	createdAt = createdAt.UTC()
	updatedAt = updatedAt.UTC()

	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO export_job_dispatches (
			dispatch_id, export_job_id, dispatch_no, trigger_source, execution_mode, adapter_key,
			status, submitted_at, received_at, finished_at, expires_at, status_reason, adapter_note, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dispatchID,
		dispatch.ExportJobID,
		dispatchNo,
		strings.TrimSpace(dispatch.TriggerSource),
		string(dispatch.ExecutionMode),
		string(dispatch.AdapterKey),
		string(dispatch.Status),
		submittedAt,
		toNullTime(dispatch.ReceivedAt),
		toNullTime(dispatch.FinishedAt),
		toNullTime(dispatch.ExpiresAt),
		strings.TrimSpace(dispatch.StatusReason),
		strings.TrimSpace(dispatch.AdapterNote),
		createdAt,
		updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert export job dispatch: %w", err)
	}

	copyDispatch := *dispatch
	copyDispatch.DispatchID = dispatchID
	copyDispatch.DispatchNo = dispatchNo
	copyDispatch.SubmittedAt = submittedAt
	copyDispatch.CreatedAt = createdAt
	copyDispatch.UpdatedAt = updatedAt
	domain.HydrateExportJobDispatchDerived(&copyDispatch)
	return &copyDispatch, nil
}

func (r *exportJobDispatchRepo) GetByDispatchID(ctx context.Context, dispatchID string) (*domain.ExportJobDispatch, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+exportJobDispatchSelectCols+`
		FROM export_job_dispatches
		WHERE dispatch_id = ?`, strings.TrimSpace(dispatchID))
	dispatch, err := scanExportJobDispatch(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get export job dispatch: %w", err)
	}
	return dispatch, nil
}

func (r *exportJobDispatchRepo) GetLatestByExportJobID(ctx context.Context, exportJobID int64) (*domain.ExportJobDispatch, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+exportJobDispatchSelectCols+`
		FROM export_job_dispatches
		WHERE export_job_id = ?
		ORDER BY dispatch_no DESC
		LIMIT 1`, exportJobID)
	dispatch, err := scanExportJobDispatch(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest export job dispatch: %w", err)
	}
	return dispatch, nil
}

func (r *exportJobDispatchRepo) ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobDispatch, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+exportJobDispatchSelectCols+`
		FROM export_job_dispatches
		WHERE export_job_id = ?
		ORDER BY dispatch_no DESC`, exportJobID)
	if err != nil {
		return nil, fmt.Errorf("list export job dispatches: %w", err)
	}
	defer rows.Close()

	dispatches := make([]*domain.ExportJobDispatch, 0)
	for rows.Next() {
		dispatch, err := scanExportJobDispatch(rows)
		if err != nil {
			return nil, fmt.Errorf("scan export job dispatch: %w", err)
		}
		dispatches = append(dispatches, dispatch)
	}
	return dispatches, rows.Err()
}

func (r *exportJobDispatchRepo) Update(ctx context.Context, tx repo.Tx, update repo.ExportJobDispatchUpdate) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE export_job_dispatches
		SET status = ?, received_at = ?, finished_at = ?, expires_at = ?, status_reason = ?, adapter_note = ?, updated_at = ?
		WHERE dispatch_id = ?`,
		string(update.Status),
		toNullTime(update.ReceivedAt),
		toNullTime(update.FinishedAt),
		toNullTime(update.ExpiresAt),
		strings.TrimSpace(update.StatusReason),
		strings.TrimSpace(update.AdapterNote),
		time.Now().UTC(),
		strings.TrimSpace(update.DispatchID),
	)
	if err != nil {
		return fmt.Errorf("update export job dispatch: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update export job dispatch rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *exportJobDispatchRepo) SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobDispatchAggregate, error) {
	out := make(map[int64]repo.ExportJobDispatchAggregate, len(exportJobIDs))
	if len(exportJobIDs) == 0 {
		return out, nil
	}
	for _, exportJobID := range exportJobIDs {
		out[exportJobID] = repo.ExportJobDispatchAggregate{}
	}

	inClause, args := buildInt64InClause("export_job_id", exportJobIDs)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT export_job_id, COUNT(*)
		FROM export_job_dispatches
		WHERE `+inClause+`
		GROUP BY export_job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("count export job dispatches: %w", err)
	}
	for rows.Next() {
		var exportJobID int64
		var count int64
		if err := rows.Scan(&exportJobID, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan export job dispatch count: %w", err)
		}
		aggregate := out[exportJobID]
		aggregate.DispatchCount = count
		out[exportJobID] = aggregate
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate export job dispatch counts: %w", err)
	}
	rows.Close()

	latestRows, err := r.db.db.QueryContext(ctx, `
		SELECT `+exportJobDispatchSelectColsAliased+`
		FROM export_job_dispatches d
		INNER JOIN (
			SELECT export_job_id, MAX(dispatch_no) AS latest_dispatch_no
			FROM export_job_dispatches
			WHERE `+inClause+`
			GROUP BY export_job_id
		) latest
			ON latest.export_job_id = d.export_job_id AND latest.latest_dispatch_no = d.dispatch_no
		ORDER BY d.export_job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest export job dispatches: %w", err)
	}
	defer latestRows.Close()
	for latestRows.Next() {
		dispatch, err := scanExportJobDispatch(latestRows)
		if err != nil {
			return nil, fmt.Errorf("scan latest export job dispatch: %w", err)
		}
		aggregate := out[dispatch.ExportJobID]
		aggregate.LatestDispatch = dispatch
		out[dispatch.ExportJobID] = aggregate
	}
	if err := latestRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest export job dispatches: %w", err)
	}
	return out, nil
}

func scanExportJobDispatch(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExportJobDispatch, error) {
	dispatch := &domain.ExportJobDispatch{}
	var executionMode string
	var adapterKey string
	var status string
	var receivedAt sql.NullTime
	var finishedAt sql.NullTime
	var expiresAt sql.NullTime
	if err := scanner.Scan(
		&dispatch.DispatchID,
		&dispatch.ExportJobID,
		&dispatch.DispatchNo,
		&dispatch.TriggerSource,
		&executionMode,
		&adapterKey,
		&status,
		&dispatch.SubmittedAt,
		&receivedAt,
		&finishedAt,
		&expiresAt,
		&dispatch.StatusReason,
		&dispatch.AdapterNote,
		&dispatch.CreatedAt,
		&dispatch.UpdatedAt,
	); err != nil {
		return nil, err
	}
	dispatch.ExecutionMode = domain.ExportJobExecutionMode(executionMode)
	dispatch.AdapterKey = domain.ExportJobRunnerAdapterKey(adapterKey)
	dispatch.Status = domain.ExportJobDispatchStatus(status)
	dispatch.ReceivedAt = fromNullTime(receivedAt)
	dispatch.FinishedAt = fromNullTime(finishedAt)
	dispatch.ExpiresAt = fromNullTime(expiresAt)
	domain.HydrateExportJobDispatchDerived(dispatch)
	return dispatch, nil
}

func nextExportJobDispatchNo(ctx context.Context, sqlTx *sql.Tx, exportJobID int64) (int, error) {
	var lastDispatchNo int
	err := sqlTx.QueryRowContext(ctx, `
		SELECT dispatch_no
		FROM export_job_dispatches
		WHERE export_job_id = ?
		ORDER BY dispatch_no DESC
		LIMIT 1
		FOR UPDATE`, exportJobID).Scan(&lastDispatchNo)
	if err == sql.ErrNoRows {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("next export job dispatch no: %w", err)
	}
	return lastDispatchNo + 1, nil
}
