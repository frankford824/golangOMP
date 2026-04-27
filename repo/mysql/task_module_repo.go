package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type taskModuleRepo struct{ db *DB }

func NewTaskModuleRepo(db *DB) repo.TaskModuleRepo { return &taskModuleRepo{db: db} }

func (r *taskModuleRepo) GetByTaskAndKey(ctx context.Context, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	row := r.db.db.QueryRowContext(ctx, taskModuleSelectSQL()+` WHERE task_id = ? AND module_key = ?`, taskID, moduleKey)
	m, err := scanTaskModule(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *taskModuleRepo) getByTaskAndKeyTx(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	sqlTx := Unwrap(tx)
	row := sqlTx.QueryRowContext(ctx, taskModuleSelectSQL()+` WHERE task_id = ? AND module_key = ?`, taskID, moduleKey)
	m, err := scanTaskModule(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *taskModuleRepo) ListByTask(ctx context.Context, taskID int64) ([]*domain.TaskModule, error) {
	rows, err := r.db.db.QueryContext(ctx, taskModuleSelectSQL()+`
		WHERE task_id = ?
		  AND COALESCE(JSON_EXTRACT(data, '$.backfill_placeholder'), CAST('false' AS JSON)) != CAST('true' AS JSON)
		ORDER BY FIELD(module_key, 'basic_info', 'customization', 'design', 'retouch', 'procurement', 'audit', 'warehouse'), id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_modules: %w", err)
	}
	defer rows.Close()
	return scanTaskModules(rows)
}

func (r *taskModuleRepo) ClaimCAS(ctx context.Context, tx repo.Tx, taskID int64, moduleKey, poolTeamCode string, actorID int64, claimedTeamCode string, actorSnapshot json.RawMessage) (bool, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_modules
		   SET state = 'in_progress',
		       claimed_by = ?,
		       claimed_team_code = ?,
		       claimed_at = NOW(),
		       actor_org_snapshot = ?,
		       updated_at = NOW()
		 WHERE task_id = ?
		   AND module_key = ?
		   AND state = 'pending_claim'
		   AND pool_team_code = ?`,
		actorID, claimedTeamCode, jsonOrObject(actorSnapshot), taskID, moduleKey, poolTeamCode)
	if err != nil {
		return false, fmt.Errorf("claim task_module cas: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim task_module affected rows: %w", err)
	}
	return n == 1, nil
}

func (r *taskModuleRepo) Enter(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, poolTeamCode *string, data json.RawMessage) (*domain.TaskModule, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_modules (task_id, module_key, state, pool_team_code, data)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			state = VALUES(state),
			pool_team_code = VALUES(pool_team_code),
			claimed_by = NULL,
			claimed_team_code = NULL,
			claimed_at = NULL,
			terminal_at = NULL,
			data = VALUES(data),
			updated_at = NOW()`,
		taskID, moduleKey, string(state), toNullString(poolTeamCode), jsonOrObject(data))
	if err != nil {
		return nil, fmt.Errorf("enter task_module: %w", err)
	}
	_, _ = res.LastInsertId()
	m, err := r.getByTaskAndKeyTx(ctx, tx, taskID, moduleKey)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("enter task_module: inserted module not readable in tx task_id=%d module_key=%s", taskID, moduleKey)
	}
	return m, nil
}

func (r *taskModuleRepo) UpdateState(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, terminal bool, data json.RawMessage) error {
	sqlTx := Unwrap(tx)
	terminalSQL := "terminal_at = terminal_at"
	if terminal {
		terminalSQL = "terminal_at = COALESCE(terminal_at, NOW())"
	}
	query := fmt.Sprintf(`UPDATE task_modules SET state = ?, %s, data = CASE WHEN ? IS NULL THEN data ELSE ? END, updated_at = NOW() WHERE task_id = ? AND module_key = ?`, terminalSQL)
	_, err := sqlTx.ExecContext(ctx, query, string(state), nullJSONString(data), nullJSONString(data), taskID, moduleKey)
	if err != nil {
		return fmt.Errorf("update task_module state: %w", err)
	}
	return nil
}

func (r *taskModuleRepo) Reassign(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, actorID int64, claimedTeamCode string, actorSnapshot json.RawMessage) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE task_modules
		   SET state = 'in_progress', claimed_by = ?, claimed_team_code = ?, claimed_at = NOW(),
		       actor_org_snapshot = ?, updated_at = NOW()
		 WHERE task_id = ? AND module_key = ? AND state IN ('pending_claim', 'in_progress')`,
		actorID, claimedTeamCode, jsonOrObject(actorSnapshot), taskID, moduleKey)
	if err != nil {
		return fmt.Errorf("reassign task_module: %w", err)
	}
	return nil
}

func (r *taskModuleRepo) PoolReassign(ctx context.Context, tx repo.Tx, taskID int64, moduleKey, poolTeamCode string) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE task_modules
		   SET state = 'pending_claim', pool_team_code = ?, claimed_by = NULL, claimed_team_code = NULL,
		       claimed_at = NULL, actor_org_snapshot = NULL, updated_at = NOW()
		 WHERE task_id = ? AND module_key = ?`, poolTeamCode, taskID, moduleKey)
	if err != nil {
		return fmt.Errorf("pool reassign task_module: %w", err)
	}
	return nil
}

func (r *taskModuleRepo) CloseOpenModules(ctx context.Context, tx repo.Tx, taskID int64, state domain.ModuleState) ([]*domain.TaskModule, error) {
	before, err := r.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sqlTx := Unwrap(tx)
	_, err = sqlTx.ExecContext(ctx, `
		UPDATE task_modules
		   SET state = ?, terminal_at = COALESCE(terminal_at, NOW()), updated_at = NOW()
		 WHERE task_id = ?
		   AND state NOT IN ('closed', 'forcibly_closed', 'closed_by_admin', 'completed')`, string(state), taskID)
	if err != nil {
		return nil, fmt.Errorf("close open task_modules: %w", err)
	}
	return before, nil
}

func (r *taskModuleRepo) InsertPlaceholder(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	return r.Enter(ctx, tx, taskID, moduleKey, domain.ModuleStateClosed, nil, json.RawMessage(`{"backfill_placeholder":true}`))
}

func taskModuleSelectSQL() string {
	return `SELECT id, task_id, module_key, state, pool_team_code, claimed_by, claimed_team_code,
		       claimed_at, actor_org_snapshot, entered_at, terminal_at, data, updated_at FROM task_modules`
}

func scanTaskModule(scanner interface{ Scan(...interface{}) error }) (*domain.TaskModule, error) {
	var m domain.TaskModule
	var pool, claimedTeam sql.NullString
	var claimedBy sql.NullInt64
	var claimedAt, terminalAt sql.NullTime
	var actorSnapshot, data []byte
	if err := scanner.Scan(&m.ID, &m.TaskID, &m.ModuleKey, &m.State, &pool, &claimedBy, &claimedTeam, &claimedAt, &actorSnapshot, &m.EnteredAt, &terminalAt, &data, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.PoolTeamCode = fromNullString(pool)
	m.ClaimedBy = fromNullInt64(claimedBy)
	m.ClaimedTeamCode = fromNullString(claimedTeam)
	m.ClaimedAt = fromNullTime(claimedAt)
	m.TerminalAt = fromNullTime(terminalAt)
	m.ActorOrgSnapshot = cloneJSON(actorSnapshot)
	m.Data = cloneJSON(data)
	if len(m.Data) == 0 {
		m.Data = json.RawMessage(`{}`)
	}
	return &m, nil
}

func scanTaskModules(rows *sql.Rows) ([]*domain.TaskModule, error) {
	var out []*domain.TaskModule
	for rows.Next() {
		m, err := scanTaskModule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task_module: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonOrObject(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func nullJSONString(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}

func cloneJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}
