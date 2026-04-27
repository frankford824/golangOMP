package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type taskEventRepo struct{ db *DB }

func NewTaskEventRepo(db *DB) repo.TaskEventRepo { return &taskEventRepo{db: db} }

// Append generates the next per-task sequence number, then inserts a task_event_logs row.
// MUST be called inside an active transaction (same pattern as V6 EventRepo.Append).
func (r *taskEventRepo) Append(
	ctx context.Context,
	tx repo.Tx,
	taskID int64,
	eventType string,
	operatorID *int64,
	payload interface{},
) (*domain.TaskEvent, error) {
	sqlTx := Unwrap(tx)

	seq, err := nextTaskEventSequence(ctx, sqlTx, taskID)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal task event payload: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO task_event_logs (id, task_id, sequence, event_type, operator_id, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, taskID, seq, eventType, toNullInt64(operatorID), raw, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task_event_log: %w", err)
	}

	return &domain.TaskEvent{
		ID:         id,
		TaskID:     taskID,
		Sequence:   seq,
		EventType:  eventType,
		OperatorID: operatorID,
		Payload:    raw,
		CreatedAt:  now,
	}, nil
}

func (r *taskEventRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskEvent, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, sequence, event_type, operator_id, payload, created_at
		FROM task_event_logs WHERE task_id = ? ORDER BY sequence ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_event_logs: %w", err)
	}
	defer rows.Close()

	var events []*domain.TaskEvent
	for rows.Next() {
		var e domain.TaskEvent
		var operatorID sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.TaskID, &e.Sequence, &e.EventType, &operatorID, &e.Payload, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task_event_log: %w", err)
		}
		e.OperatorID = fromNullInt64(operatorID)
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (r *taskEventRepo) ListRecent(ctx context.Context, filter repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if filter.TaskID != nil && *filter.TaskID > 0 {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if eventType := strings.TrimSpace(filter.EventType); eventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, eventType)
	}

	countQuery := `SELECT COUNT(*) FROM task_event_logs WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count task_event_logs: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, sequence, event_type, operator_id, payload, created_at
		FROM task_event_logs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, sequence DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list recent task_event_logs: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.TaskEvent, 0)
	for rows.Next() {
		var e domain.TaskEvent
		var operatorID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Sequence, &e.EventType, &operatorID, &e.Payload, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan recent task_event_log: %w", err)
		}
		e.OperatorID = fromNullInt64(operatorID)
		events = append(events, &e)
	}
	return events, total, rows.Err()
}

// nextTaskEventSequence atomically returns the next sequence number for a task.
// Uses task_event_sequences counter table with SELECT FOR UPDATE.
// MUST be called inside an active transaction.
func nextTaskEventSequence(ctx context.Context, sqlTx *sql.Tx, taskID int64) (int64, error) {
	var current int64
	err := sqlTx.QueryRowContext(ctx,
		`SELECT last_sequence FROM task_event_sequences WHERE task_id = ? FOR UPDATE`,
		taskID,
	).Scan(&current)

	if err == sql.ErrNoRows {
		if _, err = sqlTx.ExecContext(ctx,
			`INSERT INTO task_event_sequences (task_id, last_sequence) VALUES (?, 1)`,
			taskID,
		); err != nil {
			return 0, fmt.Errorf("task_event nextSequence insert: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("task_event nextSequence select: %w", err)
	}

	next := current + 1
	if _, err = sqlTx.ExecContext(ctx,
		`UPDATE task_event_sequences SET last_sequence = ? WHERE task_id = ?`,
		next, taskID,
	); err != nil {
		return 0, fmt.Errorf("task_event nextSequence update: %w", err)
	}
	return next, nil
}
