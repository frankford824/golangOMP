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

type integrationExecutionRepo struct{ db *DB }

func NewIntegrationExecutionRepo(db *DB) repo.IntegrationExecutionRepo {
	return &integrationExecutionRepo{db: db}
}

const integrationExecutionSelectCols = `
	execution_id, call_log_id, connector_key, execution_no, execution_mode, trigger_source,
	status, status_updated_at, started_at, finished_at, error_message, adapter_note, retryable,
	created_at, updated_at`

const integrationExecutionSelectColsAliased = `
	e.execution_id, e.call_log_id, e.connector_key, e.execution_no, e.execution_mode, e.trigger_source,
	e.status, e.status_updated_at, e.started_at, e.finished_at, e.error_message, e.adapter_note, e.retryable,
	e.created_at, e.updated_at`

func (r *integrationExecutionRepo) Create(ctx context.Context, tx repo.Tx, execution *domain.IntegrationExecution) (*domain.IntegrationExecution, error) {
	if execution == nil {
		return nil, fmt.Errorf("create integration execution: execution is nil")
	}
	sqlTx := Unwrap(tx)

	executionID := strings.TrimSpace(execution.ExecutionID)
	if executionID == "" {
		executionID = uuid.NewString()
	}
	executionNo := execution.ExecutionNo
	if executionNo <= 0 {
		nextNo, err := nextIntegrationExecutionNo(ctx, sqlTx, execution.CallLogID)
		if err != nil {
			return nil, err
		}
		executionNo = nextNo
	}

	startedAt := execution.StartedAt.UTC()
	latestStatusAt := execution.LatestStatusAt.UTC()
	createdAt := execution.CreatedAt
	if createdAt.IsZero() {
		createdAt = startedAt
	}
	updatedAt := execution.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = latestStatusAt
	}
	createdAt = createdAt.UTC()
	updatedAt = updatedAt.UTC()

	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO integration_call_executions (
			execution_id, call_log_id, connector_key, execution_no, execution_mode, trigger_source,
			status, status_updated_at, started_at, finished_at, error_message, adapter_note, retryable,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		executionID,
		execution.CallLogID,
		string(execution.ConnectorKey),
		executionNo,
		string(execution.ExecutionMode),
		strings.TrimSpace(execution.TriggerSource),
		string(execution.Status),
		latestStatusAt,
		startedAt,
		toNullTime(execution.FinishedAt),
		strings.TrimSpace(execution.ErrorMessage),
		strings.TrimSpace(execution.AdapterNote),
		execution.Retryable,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert integration execution: %w", err)
	}

	copyExecution := *execution
	copyExecution.ExecutionID = executionID
	copyExecution.ExecutionNo = executionNo
	copyExecution.StartedAt = startedAt
	copyExecution.LatestStatusAt = latestStatusAt
	copyExecution.CreatedAt = createdAt
	copyExecution.UpdatedAt = updatedAt
	return &copyExecution, nil
}

func (r *integrationExecutionRepo) GetByExecutionID(ctx context.Context, executionID string) (*domain.IntegrationExecution, error) {
	row := r.db.db.QueryRowContext(ctx, `SELECT `+integrationExecutionSelectCols+` FROM integration_call_executions WHERE execution_id = ?`, strings.TrimSpace(executionID))
	execution, err := scanIntegrationExecution(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get integration execution: %w", err)
	}
	return execution, nil
}

func (r *integrationExecutionRepo) GetLatestByCallLogID(ctx context.Context, callLogID int64) (*domain.IntegrationExecution, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+integrationExecutionSelectCols+`
		FROM integration_call_executions
		WHERE call_log_id = ?
		ORDER BY execution_no DESC
		LIMIT 1`, callLogID)
	execution, err := scanIntegrationExecution(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest integration execution: %w", err)
	}
	return execution, nil
}

func (r *integrationExecutionRepo) ListByCallLogID(ctx context.Context, callLogID int64) ([]*domain.IntegrationExecution, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+integrationExecutionSelectCols+`
		FROM integration_call_executions
		WHERE call_log_id = ?
		ORDER BY execution_no DESC`, callLogID)
	if err != nil {
		return nil, fmt.Errorf("list integration executions: %w", err)
	}
	defer rows.Close()

	executions := make([]*domain.IntegrationExecution, 0)
	for rows.Next() {
		execution, err := scanIntegrationExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan integration execution: %w", err)
		}
		executions = append(executions, execution)
	}
	return executions, rows.Err()
}

func (r *integrationExecutionRepo) Update(ctx context.Context, tx repo.Tx, update repo.IntegrationExecutionUpdate) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE integration_call_executions
		SET status = ?, status_updated_at = ?, finished_at = ?, error_message = ?, adapter_note = ?, retryable = ?, updated_at = ?
		WHERE execution_id = ?`,
		string(update.Status),
		update.LatestStatusAt.UTC(),
		toNullTime(update.FinishedAt),
		strings.TrimSpace(update.ErrorMessage),
		strings.TrimSpace(update.AdapterNote),
		update.Retryable,
		time.Now().UTC(),
		strings.TrimSpace(update.ExecutionID),
	)
	if err != nil {
		return fmt.Errorf("update integration execution: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update integration execution rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *integrationExecutionRepo) SummariesByCallLogIDs(ctx context.Context, callLogIDs []int64) (map[int64]repo.IntegrationExecutionAggregate, error) {
	out := make(map[int64]repo.IntegrationExecutionAggregate, len(callLogIDs))
	if len(callLogIDs) == 0 {
		return out, nil
	}
	for _, callLogID := range callLogIDs {
		out[callLogID] = repo.IntegrationExecutionAggregate{}
	}

	inClause, args := buildInt64InClause("call_log_id", callLogIDs)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT call_log_id, COUNT(*)
		FROM integration_call_executions
		WHERE `+inClause+`
		GROUP BY call_log_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("count integration executions: %w", err)
	}
	for rows.Next() {
		var callLogID int64
		var count int64
		if err := rows.Scan(&callLogID, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan integration execution count: %w", err)
		}
		aggregate := out[callLogID]
		aggregate.ExecutionCount = count
		out[callLogID] = aggregate
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate integration execution counts: %w", err)
	}
	rows.Close()

	actionRows, err := r.db.db.QueryContext(ctx, `
		SELECT call_log_id, trigger_source, COUNT(*)
		FROM integration_call_executions
		WHERE `+inClause+` AND trigger_source IN (?, ?)
		GROUP BY call_log_id, trigger_source`, append(args, "manual_retry", "manual_replay")...)
	if err != nil {
		return nil, fmt.Errorf("count integration execution actions: %w", err)
	}
	for actionRows.Next() {
		var callLogID int64
		var triggerSource string
		var count int64
		if err := actionRows.Scan(&callLogID, &triggerSource, &count); err != nil {
			actionRows.Close()
			return nil, fmt.Errorf("scan integration execution action count: %w", err)
		}
		aggregate := out[callLogID]
		switch triggerSource {
		case "manual_retry":
			aggregate.RetryCount = count
		case "manual_replay":
			aggregate.ReplayCount = count
		}
		out[callLogID] = aggregate
	}
	if err := actionRows.Err(); err != nil {
		actionRows.Close()
		return nil, fmt.Errorf("iterate integration execution action counts: %w", err)
	}
	actionRows.Close()

	latestRows, err := r.db.db.QueryContext(ctx, `
		SELECT `+integrationExecutionSelectColsAliased+`
		FROM integration_call_executions e
		INNER JOIN (
			SELECT call_log_id, MAX(execution_no) AS latest_execution_no
			FROM integration_call_executions
			WHERE `+inClause+`
			GROUP BY call_log_id
		) latest
			ON latest.call_log_id = e.call_log_id AND latest.latest_execution_no = e.execution_no
		ORDER BY e.call_log_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest integration executions: %w", err)
	}
	defer latestRows.Close()
	for latestRows.Next() {
		execution, err := scanIntegrationExecution(latestRows)
		if err != nil {
			return nil, fmt.Errorf("scan latest integration execution: %w", err)
		}
		aggregate := out[execution.CallLogID]
		aggregate.LatestExecution = execution
		out[execution.CallLogID] = aggregate
	}
	if err := latestRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest integration executions: %w", err)
	}

	actionLatestRows, err := r.db.db.QueryContext(ctx, `
		SELECT `+integrationExecutionSelectColsAliased+`
		FROM integration_call_executions e
		INNER JOIN (
			SELECT call_log_id, trigger_source, MAX(execution_no) AS latest_execution_no
			FROM integration_call_executions
			WHERE `+inClause+` AND trigger_source IN (?, ?)
			GROUP BY call_log_id, trigger_source
		) latest
			ON latest.call_log_id = e.call_log_id
			AND latest.trigger_source = e.trigger_source
			AND latest.latest_execution_no = e.execution_no
		ORDER BY e.call_log_id, e.execution_no`, append(args, "manual_retry", "manual_replay")...)
	if err != nil {
		return nil, fmt.Errorf("list latest integration execution actions: %w", err)
	}
	defer actionLatestRows.Close()
	for actionLatestRows.Next() {
		execution, err := scanIntegrationExecution(actionLatestRows)
		if err != nil {
			return nil, fmt.Errorf("scan latest integration execution action: %w", err)
		}
		aggregate := out[execution.CallLogID]
		switch strings.TrimSpace(execution.TriggerSource) {
		case "manual_retry":
			aggregate.LatestRetryExecution = execution
		case "manual_replay":
			aggregate.LatestReplayExecution = execution
		}
		out[execution.CallLogID] = aggregate
	}
	if err := actionLatestRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest integration execution actions: %w", err)
	}
	return out, nil
}

func scanIntegrationExecution(scanner interface {
	Scan(...interface{}) error
}) (*domain.IntegrationExecution, error) {
	execution := &domain.IntegrationExecution{}
	var connectorKey string
	var executionMode string
	var status string
	var finishedAt sql.NullTime
	if err := scanner.Scan(
		&execution.ExecutionID,
		&execution.CallLogID,
		&connectorKey,
		&execution.ExecutionNo,
		&executionMode,
		&execution.TriggerSource,
		&status,
		&execution.LatestStatusAt,
		&execution.StartedAt,
		&finishedAt,
		&execution.ErrorMessage,
		&execution.AdapterNote,
		&execution.Retryable,
		&execution.CreatedAt,
		&execution.UpdatedAt,
	); err != nil {
		return nil, err
	}
	execution.ConnectorKey = domain.IntegrationConnectorKey(connectorKey)
	execution.ExecutionMode = domain.IntegrationExecutionMode(executionMode)
	execution.Status = domain.IntegrationExecutionStatus(status)
	execution.FinishedAt = fromNullTime(finishedAt)
	domain.HydrateIntegrationExecutionDerived(execution)
	return execution, nil
}

func nextIntegrationExecutionNo(ctx context.Context, sqlTx *sql.Tx, callLogID int64) (int, error) {
	var lastExecutionNo int
	err := sqlTx.QueryRowContext(ctx, `
		SELECT execution_no
		FROM integration_call_executions
		WHERE call_log_id = ?
		ORDER BY execution_no DESC
		LIMIT 1
		FOR UPDATE`, callLogID).Scan(&lastExecutionNo)
	if err == sql.ErrNoRows {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("next integration execution no: %w", err)
	}
	return lastExecutionNo + 1, nil
}
