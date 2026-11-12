package mysqlrepo

import (
	"context"
	"database/sql"

	"workflow/repo"
)

func (r *productionPackageRepo) ListFinalizedSyncV2(ctx context.Context) (finals, waived, defaults []repo.ProductionPackageAsset, err error) {
	tx, err := r.db.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback()
	finals, err = queryProductionPackageAssets(ctx, tx, "1 = 1")
	if err != nil {
		return nil, nil, nil, err
	}
	waived, err = listWaivedFinalizedGroups(ctx, tx)
	if err != nil {
		return nil, nil, nil, err
	}
	defaults, err = listCurrentDefaultStyleAssets(ctx, tx, "1 = 1")
	if err != nil {
		return nil, nil, nil, err
	}
	err = tx.Commit()
	return
}

func (r *productionPackageRepo) ListCurrentDefaultStyleAssetsByIDs(ctx context.Context, ids []int64) ([]repo.ProductionPackageAsset, error) {
	marks, args := int64MutationArgs(ids)
	if marks == "" {
		return nil, nil
	}
	return listCurrentDefaultStyleAssets(ctx, r.db.db, "ta.id IN ("+marks+")", args...)
}

func listCurrentDefaultStyleAssets(ctx context.Context, q productionPackageQuerier, predicate string, args ...interface{}) ([]repo.ProductionPackageAsset, error) {
	rows, err := q.QueryContext(ctx, `SELECT g.id, ds.id, 'single', ds.created_at, 0, 0, '', ta.id,
  t.id, t.task_no, `+productionPackageResolvedSKU+`, `+productionPackageResolvedName+`, g.scope_kind,
  ta.file_name, COALESCE(ta.original_filename, ''), COALESCE(ta.mime_type, ''), COALESCE(ta.file_size, 0),
  ta.storage_key, COALESCE(ta.whole_hash, ''), ta.created_at
  FROM task_asset_groups g
  JOIN task_default_style_revisions ds ON ds.id = g.current_default_style_id AND ds.group_id = g.id
  `+defaultStyleFileJoin+`
  JOIN tasks t ON t.id = g.task_id
  LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
  LEFT JOIN task_retouch_requirements rr ON rr.id = g.retouch_requirement_id AND rr.deleted_at IS NULL
  WHERE g.migration_incomplete = 0 AND `+predicate+`
  ORDER BY g.id`, args...)
	if err != nil {
		return nil, err
	}
	return scanProductionPackageAssets(rows)
}

func listWaivedFinalizedGroups(ctx context.Context, q productionPackageQuerier) ([]repo.ProductionPackageAsset, error) {
	rows, err := q.QueryContext(ctx, `SELECT g.id, r.id, r.mode, COALESCE(r.finalized_at, r.created_at),
  t.id, t.task_no, `+resourceGroupSKUIdentitySQL+`, `+productionPackageResolvedName+`, g.scope_kind
  FROM task_asset_groups g
  JOIN task_asset_group_revisions r ON r.id = g.finalized_revision_id AND r.group_id = g.id
  JOIN tasks t ON t.id = g.task_id
  LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
  WHERE g.migration_incomplete = 0 AND r.status = 'finalized' AND r.final_output_waived = 1
  ORDER BY g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []repo.ProductionPackageAsset{}
	for rows.Next() {
		var row repo.ProductionPackageAsset
		if err := rows.Scan(&row.GroupID, &row.RevisionID, &row.RevisionMode, &row.RevisionFinalizedAt,
			&row.TaskID, &row.TaskNo, &row.SKUCode, &row.SKUName, &row.ScopeKind); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
