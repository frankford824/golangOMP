package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type moduleNotificationRepo struct{ db *DB }

func NewModuleNotificationRepo(db *DB) repo.ModuleNotificationRepo {
	return &moduleNotificationRepo{db: db}
}

func (r *moduleNotificationRepo) GetTaskModuleByID(ctx context.Context, tx repo.Tx, taskModuleID int64) (*domain.TaskModule, error) {
	exec := interface {
		QueryRowContext(context.Context, string, ...interface{}) *sql.Row
	}(r.db.db)
	if tx != nil {
		exec = Unwrap(tx)
	}
	row := exec.QueryRowContext(ctx, taskModuleSelectSQL()+` WHERE id = ?`, taskModuleID)
	m, err := scanTaskModule(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *moduleNotificationRepo) ListActiveUserIDsByTeam(ctx context.Context, tx repo.Tx, teamCode string, excludeUserID *int64) ([]int64, error) {
	query := `SELECT id FROM users WHERE team = ? AND status = 'active'`
	args := []interface{}{teamCode}
	if excludeUserID != nil {
		query += ` AND id <> ?`
		args = append(args, *excludeUserID)
	}
	return queryInt64s(ctx, r.db, tx, query, args...)
}

func (r *moduleNotificationRepo) ListClaimedUserIDsByTask(ctx context.Context, tx repo.Tx, taskID int64, excludeUserID *int64) ([]int64, error) {
	query := `SELECT DISTINCT claimed_by FROM task_modules WHERE task_id = ? AND claimed_by IS NOT NULL`
	args := []interface{}{taskID}
	if excludeUserID != nil {
		query += ` AND claimed_by <> ?`
		args = append(args, *excludeUserID)
	}
	return queryInt64s(ctx, r.db, tx, query, args...)
}

func queryInt64s(ctx context.Context, db *DB, tx repo.Tx, query string, args ...interface{}) ([]int64, error) {
	exec := interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	}(db.db)
	if tx != nil {
		exec = Unwrap(tx)
	}
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query int64s: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
