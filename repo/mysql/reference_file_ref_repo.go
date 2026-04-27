package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type referenceFileRefFlatRepo struct{ db *DB }

func NewReferenceFileRefFlatRepo(db *DB) repo.ReferenceFileRefFlatRepo {
	return &referenceFileRefFlatRepo{db: db}
}

func (r *referenceFileRefFlatRepo) InsertFlat(ctx context.Context, tx repo.Tx, ref *domain.ReferenceFileRefFlat) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO reference_file_refs (task_id, sku_item_id, ref_id, owner_module_key, context)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE owner_module_key = VALUES(owner_module_key), context = VALUES(context)`,
		ref.TaskID, toNullInt64(ref.SKUItemID), ref.RefID, ref.OwnerModuleKey, toNullString(ref.Context))
	if err != nil {
		return 0, fmt.Errorf("insert reference_file_ref flat: %w", err)
	}
	return res.LastInsertId()
}

func (r *referenceFileRefFlatRepo) ListByTask(ctx context.Context, taskID int64) ([]*domain.ReferenceFileRefFlat, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, sku_item_id, ref_id, owner_module_key, context, attached_at
		FROM reference_file_refs
		WHERE task_id = ?
		ORDER BY owner_module_key, attached_at ASC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list reference_file_refs flat: %w", err)
	}
	defer rows.Close()
	var out []*domain.ReferenceFileRefFlat
	for rows.Next() {
		var ref domain.ReferenceFileRefFlat
		var skuID sql.NullInt64
		var contextValue sql.NullString
		if err := rows.Scan(&ref.ID, &ref.TaskID, &skuID, &ref.RefID, &ref.OwnerModuleKey, &contextValue, &ref.AttachedAt); err != nil {
			return nil, fmt.Errorf("scan reference_file_ref flat: %w", err)
		}
		ref.SKUItemID = fromNullInt64(skuID)
		ref.Context = fromNullString(contextValue)
		out = append(out, &ref)
	}
	return out, rows.Err()
}

func (r *referenceFileRefFlatRepo) DeleteByTaskAndRef(ctx context.Context, tx repo.Tx, taskID int64, refID string) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `DELETE FROM reference_file_refs WHERE task_id = ? AND ref_id = ?`, taskID, refID)
	if err != nil {
		return fmt.Errorf("delete reference_file_ref flat: %w", err)
	}
	return nil
}
