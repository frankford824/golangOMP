package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskAutoArchiveRepo struct {
	db *DB
}

func NewTaskAutoArchiveRepo(db *DB) repo.TaskAutoArchiveRepo {
	return &taskAutoArchiveRepo{db: db}
}

func (r *taskAutoArchiveRepo) ListEligibleForArchive(ctx context.Context, cutoff time.Time, limit int) ([]int64, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id
		  FROM tasks
		 WHERE task_status IN (?, ?)
		   AND updated_at < ?
		 ORDER BY updated_at ASC, id ASC
		 LIMIT ?`,
		string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list eligible tasks for archive: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan task id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *taskAutoArchiveRepo) ArchiveTasks(ctx context.Context, tx repo.Tx, taskIDs []int64) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(taskIDs)), ",")
	args := make([]interface{}, 0, len(taskIDs)+3)
	args = append(args, string(domain.TaskStatusArchived))
	for _, id := range taskIDs {
		args = append(args, id)
	}
	args = append(args, string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled))

	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE tasks
		   SET task_status = ?
		 WHERE id IN (`+placeholders+`)
		   AND task_status IN (?, ?)`,
		args...)
	if err != nil {
		return 0, fmt.Errorf("archive tasks: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("archive tasks rows affected: %w", err)
	}
	return int(n), nil
}

var _ repo.TaskAutoArchiveRepo = (*taskAutoArchiveRepo)(nil)
