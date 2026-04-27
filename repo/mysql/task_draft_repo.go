package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskDraftRepo struct{ db *DB }

func NewTaskDraftRepo(db *DB) repo.TaskDraftRepo { return &taskDraftRepo{db: db} }

func (r *taskDraftRepo) Create(ctx context.Context, tx repo.Tx, draft *domain.TaskDraft) (*domain.TaskDraft, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO task_drafts (owner_user_id, task_type, payload, expires_at)
		VALUES (?, ?, ?, ?)`,
		draft.OwnerUserID, draft.TaskType, jsonOrObject(draft.Payload), draft.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert task_draft: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("task_draft last insert id: %w", err)
	}
	return r.GetForUpdate(ctx, tx, id)
}

func (r *taskDraftRepo) Update(ctx context.Context, tx repo.Tx, draft *domain.TaskDraft) (*domain.TaskDraft, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_drafts
		   SET task_type = ?, payload = ?, expires_at = ?, updated_at = NOW()
		 WHERE id = ? AND owner_user_id = ?`,
		draft.TaskType, jsonOrObject(draft.Payload), draft.ExpiresAt, draft.ID, draft.OwnerUserID)
	if err != nil {
		return nil, fmt.Errorf("update task_draft: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update task_draft affected rows: %w", err)
	}
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	return r.GetForUpdate(ctx, tx, draft.ID)
}

func (r *taskDraftRepo) Get(ctx context.Context, draftID int64) (*domain.TaskDraft, error) {
	row := r.db.db.QueryRowContext(ctx, taskDraftSelectSQL()+` WHERE id = ?`, draftID)
	return scanTaskDraft(row)
}

func (r *taskDraftRepo) GetForUpdate(ctx context.Context, tx repo.Tx, draftID int64) (*domain.TaskDraft, error) {
	row := Unwrap(tx).QueryRowContext(ctx, taskDraftSelectSQL()+` WHERE id = ? FOR UPDATE`, draftID)
	return scanTaskDraft(row)
}

func (r *taskDraftRepo) List(ctx context.Context, filter repo.TaskDraftListFilter) ([]domain.TaskDraftListItem, error) {
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	where := []string{`owner_user_id = ?`}
	args := []interface{}{filter.OwnerUserID}
	if strings.TrimSpace(filter.TaskType) != "" {
		where = append(where, `task_type = ?`)
		args = append(args, strings.TrimSpace(filter.TaskType))
	}
	if filter.BeforeTime != nil && filter.BeforeID > 0 {
		where = append(where, `(updated_at < ? OR (updated_at = ? AND id < ?))`)
		args = append(args, *filter.BeforeTime, *filter.BeforeTime, filter.BeforeID)
	}
	args = append(args, filter.Limit)
	rows, err := r.db.db.QueryContext(ctx, taskDraftSelectSQL()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list task_drafts: %w", err)
	}
	defer rows.Close()
	var out []domain.TaskDraftListItem
	for rows.Next() {
		d, err := scanTaskDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (r *taskDraftRepo) Delete(ctx context.Context, tx repo.Tx, draftID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM task_drafts WHERE id = ?`, draftID)
	if err != nil {
		return fmt.Errorf("delete task_draft: %w", err)
	}
	return nil
}

func (r *taskDraftRepo) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.db.ExecContext(ctx, `DELETE FROM task_drafts WHERE expires_at < ? AND expires_at IS NOT NULL`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired task_drafts: %w", err)
	}
	return res.RowsAffected()
}

func (r *taskDraftRepo) CountByOwnerAndType(ctx context.Context, tx repo.Tx, ownerUserID int64, taskType string) (int, error) {
	var count int
	err := Unwrap(tx).QueryRowContext(ctx, `SELECT COUNT(*) FROM task_drafts WHERE owner_user_id = ? AND task_type = ?`, ownerUserID, taskType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count task_drafts by owner/type: %w", err)
	}
	return count, nil
}

func (r *taskDraftRepo) DeleteOldestByOwnerAndType(ctx context.Context, tx repo.Tx, ownerUserID int64, taskType string) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		DELETE FROM task_drafts
		 WHERE owner_user_id = ? AND task_type = ?
		 ORDER BY created_at ASC, id ASC
		 LIMIT 1`, ownerUserID, taskType)
	if err != nil {
		return fmt.Errorf("delete oldest task_draft: %w", err)
	}
	return nil
}

func taskDraftSelectSQL() string {
	return `SELECT id, owner_user_id, task_type, payload, expires_at, created_at, updated_at FROM task_drafts`
}

func scanTaskDraft(scanner interface{ Scan(...interface{}) error }) (*domain.TaskDraft, error) {
	var d domain.TaskDraft
	var payload []byte
	if err := scanner.Scan(&d.ID, &d.OwnerUserID, &d.TaskType, &payload, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	if !json.Valid(payload) {
		payload = []byte(`{}`)
	}
	d.Payload = cloneJSON(payload)
	return &d, nil
}
