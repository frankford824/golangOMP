package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type taskReferenceAssetBindingRepo struct{ db *DB }

func NewTaskReferenceAssetBindingRepo(db *DB) repo.TaskReferenceAssetBindingRepo {
	return &taskReferenceAssetBindingRepo{db: db}
}

func (r *taskReferenceAssetBindingRepo) Create(ctx context.Context, tx repo.Tx, binding *domain.TaskReferenceAssetBinding) (*domain.TaskReferenceAssetBinding, error) {
	if binding == nil {
		return nil, fmt.Errorf("create task reference asset binding: binding is nil")
	}
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_reference_asset_bindings
		  (task_id, ref_id, design_asset_id, task_asset_id)
		VALUES (?, ?, ?, ?)`,
		binding.TaskID,
		strings.TrimSpace(binding.RefID),
		binding.DesignAssetID,
		binding.TaskAssetID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task_reference_asset_binding: %w", err)
	}
	if _, err := res.LastInsertId(); err != nil {
		return nil, fmt.Errorf("task_reference_asset_binding last insert id: %w", err)
	}
	return r.GetByTaskAndRefID(ctx, binding.TaskID, binding.RefID)
}

func (r *taskReferenceAssetBindingRepo) GetByTaskAndRefID(ctx context.Context, taskID int64, refID string) (*domain.TaskReferenceAssetBinding, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, ref_id, design_asset_id, task_asset_id, created_at
		FROM task_reference_asset_bindings
		WHERE task_id = ? AND ref_id = ?`,
		taskID,
		strings.TrimSpace(refID),
	)
	return scanTaskReferenceAssetBinding(row)
}

func (r *taskReferenceAssetBindingRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskReferenceAssetBinding, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, ref_id, design_asset_id, task_asset_id, created_at
		FROM task_reference_asset_bindings
		WHERE task_id = ?
		ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_reference_asset_bindings: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.TaskReferenceAssetBinding, 0)
	for rows.Next() {
		item, scanErr := scanTaskReferenceAssetBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanTaskReferenceAssetBinding(scanner interface{ Scan(...interface{}) error }) (*domain.TaskReferenceAssetBinding, error) {
	item := &domain.TaskReferenceAssetBinding{}
	if err := scanner.Scan(
		&item.ID,
		&item.TaskID,
		&item.RefID,
		&item.DesignAssetID,
		&item.TaskAssetID,
		&item.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan task_reference_asset_binding: %w", err)
	}
	item.RefID = strings.TrimSpace(item.RefID)
	return item, nil
}
