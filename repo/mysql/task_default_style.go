package mysqlrepo

import (
	"context"
	"database/sql"

	"workflow/domain"
	"workflow/repo"
)

const defaultStyleFileJoin = `
 JOIN task_assets ta ON ta.id = ds.task_asset_id
   AND ta.task_id = g.task_id AND ta.asset_type = 'default_style'
   AND ta.binding_state = 'bound' AND ta.bound_group_id = g.id AND ta.bound_role = 'default_style'
   AND COALESCE(ta.is_archived, 0) = 0
   AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL
   AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
   AND ta.upload_status = 'uploaded' AND COALESCE(ta.storage_key, '') <> ''
 JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
   AND asr.is_placeholder = 0 AND asr.status NOT IN ('archived', 'historical_unavailable')`

const defaultStyleReadColumns = `
 ds.id, ds.group_id, ds.task_asset_id, ds.created_by, ds.reason, ds.created_at,
 COALESCE(ta.file_name, ''), COALESCE(ta.mime_type, ''), ta.file_size,
 CASE WHEN ta.deleted_at IS NULL AND ta.cleaned_at IS NULL AND ta.access_revoked_at IS NULL
  AND ta.object_deleted_at IS NULL AND COALESCE(ta.is_archived, 0) = 0 AND ta.upload_status = 'uploaded'
  AND asr.is_placeholder = 0 AND asr.status NOT IN ('archived', 'historical_unavailable')
  THEN COALESCE(ta.storage_key, '') ELSE '' END`

func scanDefaultStyle(rows *sql.Rows, pending *bool) (domain.DefaultStyleRevision, error) {
	var style domain.DefaultStyleRevision
	var assetID, size sql.NullInt64
	var file domain.TaskResourceFile
	columns := []interface{}{&style.ID, &style.GroupID, &assetID, &style.CreatedBy, &style.Reason, &style.CreatedAt,
		&file.FileName, &file.MimeType, &size, &file.StorageKey}
	if pending != nil {
		columns = append(columns, pending)
	}
	if err := rows.Scan(columns...); err != nil {
		return style, err
	}
	style.Removed = !assetID.Valid
	if assetID.Valid {
		file.TaskAssetID, file.FileSize = assetID.Int64, fromNullInt64(size)
		style.File = &file
	}
	return style, nil
}

func (r *TaskResourceGroupRepo) GetDefaultStyleGroupTaskID(ctx context.Context, groupID int64) (int64, error) {
	var taskID int64
	err := r.db.db.QueryRowContext(ctx, `SELECT task_id FROM task_asset_groups WHERE id = ?`, groupID).Scan(&taskID)
	if err == sql.ErrNoRows {
		return 0, repo.ErrNotFound
	}
	return taskID, err
}

func (r *TaskResourceGroupRepo) LoadDefaultStyles(ctx context.Context, groups []domain.TaskAssetGroup, includePending bool) error {
	if len(groups) == 0 {
		return nil
	}
	byID := make(map[int64]*domain.TaskAssetGroup, len(groups))
	ids := make([]int64, 0, len(groups))
	for i := range groups {
		byID[groups[i].ID] = &groups[i]
		ids = append(ids, groups[i].ID)
	}
	marks, args := int64MutationArgs(ids)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+defaultStyleReadColumns+`, COALESCE(ds.id = g.pending_default_style_id, 0)
  FROM task_asset_groups g
  JOIN task_default_style_revisions ds ON ds.group_id = g.id
    AND (ds.id = g.current_default_style_id OR (? AND ds.id = g.pending_default_style_id))
  LEFT JOIN task_assets ta ON ta.id = ds.task_asset_id AND ta.task_id = g.task_id AND ta.bound_group_id = g.id AND ta.bound_role = 'default_style'
  LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
  WHERE g.id IN (`+marks+`)`, append([]interface{}{includePending}, args...)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pending bool
		style, err := scanDefaultStyle(rows, &pending)
		if err != nil {
			return err
		}
		group := byID[style.GroupID]
		if pending {
			group.PendingDefaultStyle = &style
		} else if !style.Removed {
			group.DefaultStyle = &style
		}
	}
	return rows.Err()
}

func (r *TaskResourceGroupRepo) ListDefaultStyleHistory(ctx context.Context, groupID, beforeID int64) ([]domain.DefaultStyleRevision, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+defaultStyleReadColumns+`
  FROM task_default_style_revisions ds
  LEFT JOIN task_assets ta ON ta.id = ds.task_asset_id AND ta.bound_group_id = ds.group_id AND ta.bound_role = 'default_style'
  LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
  WHERE ds.group_id = ? AND (? = 0 OR ds.id < ?) ORDER BY ds.id DESC LIMIT 50`, groupID, beforeID, beforeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.DefaultStyleRevision{}
	for rows.Next() {
		style, err := scanDefaultStyle(rows, nil)
		if err != nil {
			return nil, err
		}
		items = append(items, style)
	}
	return items, rows.Err()
}

func (r *TaskResourceGroupRepo) CreateDefaultStyleRevision(ctx context.Context, tx repo.Tx, group domain.TaskAssetGroup, request domain.DefaultStyleUpdate, actorID int64, publish bool) (int64, error) {
	sqlTx := Unwrap(tx)
	if request.TaskAssetID != nil {
		// Null-safe equality distinguishes SKU, task and retouch scope without
		// falling back to the task's primary SKU or a filename.
		result, err := sqlTx.ExecContext(ctx, `
   UPDATE task_assets ta JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
   SET ta.binding_state = 'bound', ta.bound_group_id = ?, ta.bound_role = 'default_style', ta.staged_expires_at = NULL
   WHERE ta.id = ? AND ta.task_id = ? AND ta.asset_type = 'default_style'
    AND ta.binding_state = 'staged' AND ta.staged_role = 'default_style' AND ta.staged_by = ?
    AND ta.staged_task_sku_item_id <=> ? AND ta.staged_retouch_requirement_id <=> ?
    AND ta.upload_status = 'uploaded' AND COALESCE(ta.storage_key, '') <> ''
    AND ta.staged_expires_at > UTC_TIMESTAMP()
    AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL AND ta.access_revoked_at IS NULL
    AND ta.object_deleted_at IS NULL AND COALESCE(ta.is_archived, 0) = 0
    AND asr.is_placeholder = 0 AND asr.status NOT IN ('archived', 'historical_unavailable')`,
			group.ID, *request.TaskAssetID, group.TaskID, actorID, toNullInt64(group.TaskSKUItemID), toNullInt64(group.RetouchRequirementID))
		if err != nil {
			return 0, err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			if err != nil {
				return 0, err
			}
			return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "default style file is not staged for this resource group and auditor", nil)
		}
	}
	result, err := sqlTx.ExecContext(ctx, `INSERT INTO task_default_style_revisions (group_id, task_asset_id, created_by, reason)
   VALUES (?, ?, ?, ?)`, group.ID, toNullInt64(request.TaskAssetID), actorID, request.Reason)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	result, err = sqlTx.ExecContext(ctx, `UPDATE task_asset_groups SET lock_version = lock_version + 1,
   current_default_style_id = IF(?, ?, current_default_style_id),
   pending_default_style_id = IF(?, NULL, ?)
   WHERE id = ? AND lock_version = ?`, publish, id, publish, id, group.ID, group.LockVersion)
	if err != nil {
		return 0, err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		if err != nil {
			return 0, err
		}
		return 0, repo.ErrConflict
	}
	return id, nil
}
