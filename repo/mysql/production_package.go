package mysqlrepo

import (
	"context"
	"fmt"
	"strings"

	"workflow/repo"
)

const productionPackageResolvedSKU = `UPPER(COALESCE(
	NULLIF(CONVERT(tsi.sku_code USING utf8mb4) COLLATE utf8mb4_unicode_ci, ''),
	NULLIF(CONVERT(rr.sku_code USING utf8mb4) COLLATE utf8mb4_unicode_ci, ''),
	NULLIF(CONVERT(ta.scope_sku_code USING utf8mb4) COLLATE utf8mb4_unicode_ci, ''),
	NULLIF(CONVERT(t.primary_sku_code USING utf8mb4) COLLATE utf8mb4_unicode_ci, ''),
	NULLIF(CONVERT(t.sku_code USING utf8mb4) COLLATE utf8mb4_unicode_ci, ''),
	''
))`

const productionPackageResolvedName = `COALESCE(
	NULLIF(tsi.product_name_snapshot, ''),
	NULLIF(t.product_name_snapshot, ''),
	''
)`

type productionPackageRepo struct{ db *DB }

func NewProductionPackageRepo(db *DB) repo.ProductionPackageStore {
	return &productionPackageRepo{db: db}
}

func (r *productionPackageRepo) ListFinalizedAssets(ctx context.Context, query repo.ProductionPackageQuery) ([]repo.ProductionPackageAsset, error) {
	codes := normalizedProductionPackageTerms(query.SKUCodes, true)
	names := normalizedProductionPackageTerms(query.SKUNames, false)
	if len(codes) == 0 && len(names) == 0 {
		return []repo.ProductionPackageAsset{}, nil
	}

	matchClauses := make([]string, 0, 2)
	args := make([]interface{}, 0, len(codes)+len(names))
	if len(codes) > 0 {
		filenameClauses := make([]string, 0, len(codes))
		for _, code := range codes {
			args = append(args, code)
		}
		for _, code := range codes {
			filenameClauses = append(filenameClauses, "UPPER(CONCAT_WS(' ', ta.file_name, ta.original_filename, ri.item_name)) LIKE ?")
			args = append(args, "%"+code+"%")
		}
		matchClauses = append(matchClauses, "("+productionPackageResolvedSKU+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(codes)), ",")+") OR ("+strings.Join(filenameClauses, " OR ")+"))")
	}
	if len(names) > 0 {
		parts := make([]string, 0, len(names))
		for _, name := range names {
			parts = append(parts, "LOWER("+productionPackageResolvedName+") LIKE ?")
			args = append(args, "%"+strings.ToLower(name)+"%")
		}
		matchClauses = append(matchClauses, "("+strings.Join(parts, " OR ")+")")
	}

	return r.listFinalizedAssets(ctx, "("+strings.Join(matchClauses, " OR ")+")", args...)
}

func (r *productionPackageRepo) ListAllFinalizedAssets(ctx context.Context) ([]repo.ProductionPackageAsset, error) {
	return r.listFinalizedAssets(ctx, "1 = 1")
}

func (r *productionPackageRepo) ListFinalizedAssetsByIDs(ctx context.Context, taskAssetIDs []int64) ([]repo.ProductionPackageAsset, error) {
	if len(taskAssetIDs) == 0 {
		return []repo.ProductionPackageAsset{}, nil
	}
	args := make([]interface{}, 0, len(taskAssetIDs))
	for _, id := range taskAssetIDs {
		args = append(args, id)
	}
	return r.listFinalizedAssets(ctx, "ta.id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+")", args...)
}

func (r *productionPackageRepo) listFinalizedAssets(ctx context.Context, matchClause string, args ...interface{}) ([]repo.ProductionPackageAsset, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT g.id,
		       r.id,
		       r.mode,
		       COALESCE(r.finalized_at, r.created_at),
		       ri.id,
		       ri.sort_order,
		       ri.item_name,
		       ta.id,
		       t.id,
		       t.task_no,
		       `+productionPackageResolvedSKU+` AS resolved_sku,
		       `+productionPackageResolvedName+` AS resolved_name,
		       g.scope_kind,
		       ta.file_name,
		       COALESCE(ta.original_filename, ''),
		       COALESCE(ta.mime_type, ''),
		       COALESCE(ta.file_size, 0),
		       ta.storage_key,
		       COALESCE(ta.whole_hash, ''),
		       ta.created_at
		  FROM task_asset_groups g
		  JOIN task_asset_group_revisions r
		    ON r.id = g.finalized_revision_id
		   AND r.status = 'finalized'
		  JOIN task_asset_group_revision_items ri ON ri.revision_id = r.id
		  JOIN task_assets ta ON ta.id = ri.task_asset_id
		  JOIN tasks t ON t.id = g.task_id
		  LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		  LEFT JOIN task_retouch_requirements rr
		    ON rr.id = g.retouch_requirement_id
		   AND rr.deleted_at IS NULL
		 WHERE `+matchClause+`
		   AND g.migration_incomplete = 0
		   AND ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND ta.is_archived = 0
		   AND ta.upload_status = 'uploaded'
		   AND COALESCE(ta.storage_key, '') <> ''
		   AND ta.asset_type IN ('delivery','draft','revised','final','outsource_return')
		 ORDER BY COALESCE(r.finalized_at, r.created_at) DESC, g.id DESC, ri.sort_order ASC, ri.id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list finalized production package assets: %w", err)
	}
	defer rows.Close()

	out := make([]repo.ProductionPackageAsset, 0)
	for rows.Next() {
		var item repo.ProductionPackageAsset
		if err := rows.Scan(
			&item.GroupID,
			&item.RevisionID,
			&item.RevisionMode,
			&item.RevisionFinalizedAt,
			&item.RevisionItemID,
			&item.SortOrder,
			&item.ItemName,
			&item.TaskAssetID,
			&item.TaskID,
			&item.TaskNo,
			&item.SKUCode,
			&item.SKUName,
			&item.ScopeKind,
			&item.FileName,
			&item.OriginalFilename,
			&item.MimeType,
			&item.FileSize,
			&item.StorageKey,
			&item.WholeHash,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan finalized production package asset: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func normalizedProductionPackageTerms(values []string, upper bool) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if upper {
			value = strings.ToUpper(value)
		}
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
