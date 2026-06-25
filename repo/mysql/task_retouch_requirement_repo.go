package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type taskRetouchRequirementRepo struct{ db *DB }

func NewTaskRetouchRequirementRepo(db *DB) repo.TaskRetouchRequirementRepo {
	return &taskRetouchRequirementRepo{db: db}
}

func (r *taskRetouchRequirementRepo) CreateBatch(ctx context.Context, tx repo.Tx, taskID int64, createdBy int64, items []domain.CreateRetouchRequirementItem) error {
	if len(items) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	for _, item := range items {
		description := strings.TrimSpace(item.Description)
		if description == "" {
			continue
		}
		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = 1
		}
		skuCode := strings.TrimSpace(item.SKUCode)
		spec := strings.TrimSpace(item.Spec)
		remark := strings.TrimSpace(item.Remark)
		createdByPtr := int64PtrForInsert(createdBy)
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_retouch_requirements
			  (task_id, description, sku_code, spec, remark, sort_order, created_by, updated_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			taskID,
			description,
			nullIfEmpty(skuCode),
			nullIfEmpty(spec),
			nullIfEmpty(remark),
			sortOrder,
			toNullInt64(createdByPtr),
			toNullInt64(createdByPtr),
		)
		if err != nil {
			return fmt.Errorf("insert task_retouch_requirement: %w", err)
		}
	}
	return nil
}

func (r *taskRetouchRequirementRepo) GetByID(ctx context.Context, id int64) (*domain.TaskRetouchRequirement, error) {
	if id <= 0 {
		return nil, nil
	}
	var req domain.TaskRetouchRequirement
	var skuCode, spec, remark sql.NullString
	var createdBy, updatedBy sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, description, sku_code, spec, remark, sort_order,
		       created_by, updated_by, created_at, updated_at
		FROM task_retouch_requirements
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(
		&req.ID,
		&req.TaskID,
		&req.Description,
		&skuCode,
		&spec,
		&remark,
		&req.SortOrder,
		&createdBy,
		&updatedBy,
		&req.CreatedAt,
		&req.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task_retouch_requirement: %w", err)
	}
	if skuCode.Valid {
		req.SKUCode = skuCode.String
	}
	if spec.Valid {
		req.Spec = spec.String
	}
	if remark.Valid {
		req.Remark = remark.String
	}
	req.CreatedBy = fromNullInt64(createdBy)
	req.UpdatedBy = fromNullInt64(updatedBy)
	return &req, nil
}

func (r *taskRetouchRequirementRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskRetouchRequirement, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, description, sku_code, spec, remark, sort_order,
		       created_by, updated_by, created_at, updated_at
		FROM task_retouch_requirements
		WHERE task_id = ? AND deleted_at IS NULL
		ORDER BY sort_order ASC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_retouch_requirements: %w", err)
	}
	defer rows.Close()

	var out []*domain.TaskRetouchRequirement
	for rows.Next() {
		var req domain.TaskRetouchRequirement
		var skuCode, spec, remark sql.NullString
		var createdBy, updatedBy sql.NullInt64
		if err := rows.Scan(
			&req.ID,
			&req.TaskID,
			&req.Description,
			&skuCode,
			&spec,
			&remark,
			&req.SortOrder,
			&createdBy,
			&updatedBy,
			&req.CreatedAt,
			&req.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task_retouch_requirement: %w", err)
		}
		if skuCode.Valid {
			req.SKUCode = skuCode.String
		}
		if spec.Valid {
			req.Spec = spec.String
		}
		if remark.Valid {
			req.Remark = remark.String
		}
		req.CreatedBy = fromNullInt64(createdBy)
		req.UpdatedBy = fromNullInt64(updatedBy)
		out = append(out, &req)
	}
	return out, rows.Err()
}

func int64PtrForInsert(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}
