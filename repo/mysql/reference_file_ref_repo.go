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
		INSERT INTO reference_file_refs (task_id, sku_item_id, retouch_requirement_id, ref_id, owner_module_key, context)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE owner_module_key = VALUES(owner_module_key), context = VALUES(context)`,
		ref.TaskID, toNullInt64(ref.SKUItemID), toNullInt64(ref.RetouchRequirementID), ref.RefID, ref.OwnerModuleKey, toNullString(ref.Context))
	if err != nil {
		return 0, fmt.Errorf("insert reference_file_ref flat: %w", err)
	}
	return res.LastInsertId()
}

func (r *referenceFileRefFlatRepo) ListByTask(ctx context.Context, taskID int64) ([]*domain.ReferenceFileRefFlat, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT rfr.id, rfr.task_id, formal_asset.asset_id, rfr.sku_item_id,
		       rfr.retouch_requirement_id, rfr.ref_id,
		       rfr.owner_module_key, rfr.context, rfr.attached_at,
		       COALESCE(asr.ref_key, ''), COALESCE(asr.file_name, ''),
		       COALESCE(asr.mime_type, ''), asr.file_size,
		       COALESCE(asr.status, '')
		FROM reference_file_refs rfr
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = rfr.ref_id
		LEFT JOIN task_assets formal_asset ON formal_asset.id = asr.asset_id
		WHERE rfr.task_id = ?
		ORDER BY rfr.owner_module_key, rfr.attached_at ASC, rfr.id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list reference_file_refs flat: %w", err)
	}
	defer rows.Close()
	var out []*domain.ReferenceFileRefFlat
	for rows.Next() {
		var ref domain.ReferenceFileRefFlat
		var designAssetID, skuID, retouchRequirementID sql.NullInt64
		var contextValue sql.NullString
		var fileSize sql.NullInt64
		if err := rows.Scan(
			&ref.ID, &ref.TaskID, &designAssetID, &skuID, &retouchRequirementID,
			&ref.RefID, &ref.OwnerModuleKey, &contextValue,
			&ref.AttachedAt, &ref.StorageKey, &ref.FileName,
			&ref.MimeType, &fileSize, &ref.StorageStatus,
		); err != nil {
			return nil, fmt.Errorf("scan reference_file_ref flat: %w", err)
		}
		ref.DesignAssetID = fromNullInt64(designAssetID)
		ref.SKUItemID = fromNullInt64(skuID)
		ref.RetouchRequirementID = fromNullInt64(retouchRequirementID)
		ref.Context = fromNullString(contextValue)
		ref.FileSize = fromNullInt64(fileSize)
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

func (r *referenceFileRefFlatRepo) ReplaceTaskLevelReference(ctx context.Context, tx repo.Tx, taskID int64, oldRefID, newRefID string) error {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT ref_id
		FROM reference_file_refs
		WHERE task_id = ?
		  AND sku_item_id IS NULL
		  AND retouch_requirement_id IS NULL
		  AND ref_id IN (?, ?)
		FOR UPDATE`, taskID, oldRefID, newRefID)
	if err != nil {
		return fmt.Errorf("lock task reference replacement rows: %w", err)
	}
	found := map[string]bool{}
	for rows.Next() {
		var refID string
		if scanErr := rows.Scan(&refID); scanErr != nil {
			rows.Close()
			return fmt.Errorf("scan task reference replacement row: %w", scanErr)
		}
		found[refID] = true
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return rowsErr
	}
	if !found[oldRefID] || !found[newRefID] {
		return repo.ErrNotFound
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		DELETE FROM reference_file_refs
		WHERE task_id = ? AND sku_item_id IS NULL AND retouch_requirement_id IS NULL AND ref_id = ?`, taskID, oldRefID); err != nil {
		return fmt.Errorf("replace task reference relation: %w", err)
	}
	return nil
}
