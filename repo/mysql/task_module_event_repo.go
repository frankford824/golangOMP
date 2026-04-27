package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type taskModuleEventRepo struct{ db *DB }

func NewTaskModuleEventRepo(db *DB) repo.TaskModuleEventRepo { return &taskModuleEventRepo{db: db} }

func (r *taskModuleEventRepo) Insert(ctx context.Context, tx repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_module_events
			(task_module_id, event_type, from_state, to_state, actor_id, actor_snapshot, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.TaskModuleID, string(event.EventType), moduleStateNull(event.FromState), moduleStateNull(event.ToState),
		toNullInt64(event.ActorID), jsonOrObject(event.ActorSnapshot), jsonOrObject(event.Payload))
	if err != nil {
		return 0, fmt.Errorf("insert task_module_event: %w", err)
	}
	return res.LastInsertId()
}

func (r *taskModuleEventRepo) ListByTaskModule(ctx context.Context, taskModuleID int64, limit int) ([]*domain.TaskModuleEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, taskModuleEventSelectSQL()+` WHERE task_module_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, taskModuleID, limit)
	if err != nil {
		return nil, fmt.Errorf("list task_module_events by module: %w", err)
	}
	defer rows.Close()
	return scanTaskModuleEvents(rows)
}

func (r *taskModuleEventRepo) ListRecentByTask(ctx context.Context, taskID int64, limit int) ([]*domain.TaskModuleEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, taskModuleEventSelectSQL()+`
		JOIN task_modules tm ON tm.id = task_module_events.task_module_id
		WHERE tm.task_id = ?
		ORDER BY task_module_events.created_at DESC, task_module_events.id DESC
		LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent task_module_events: %w", err)
	}
	defer rows.Close()
	return scanTaskModuleEvents(rows)
}

func taskModuleEventSelectSQL() string {
	return `SELECT task_module_events.id, task_module_events.task_module_id, task_module_events.event_type,
		       task_module_events.from_state, task_module_events.to_state, task_module_events.actor_id,
		       task_module_events.actor_snapshot, task_module_events.payload, task_module_events.created_at
		FROM task_module_events`
}

func scanTaskModuleEvents(rows *sql.Rows) ([]*domain.TaskModuleEvent, error) {
	var out []*domain.TaskModuleEvent
	for rows.Next() {
		var e domain.TaskModuleEvent
		var from, to sql.NullString
		var actorID sql.NullInt64
		var actorSnapshot, payload []byte
		if err := rows.Scan(&e.ID, &e.TaskModuleID, &e.EventType, &from, &to, &actorID, &actorSnapshot, &payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task_module_event: %w", err)
		}
		if from.Valid {
			state := domain.ModuleState(from.String)
			e.FromState = &state
		}
		if to.Valid {
			state := domain.ModuleState(to.String)
			e.ToState = &state
		}
		e.ActorID = fromNullInt64(actorID)
		e.ActorSnapshot = cloneJSON(actorSnapshot)
		e.Payload = cloneJSON(payload)
		out = append(out, &e)
	}
	return out, rows.Err()
}

func moduleStateNull(state *domain.ModuleState) sql.NullString {
	if state == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*state), Valid: true}
}
