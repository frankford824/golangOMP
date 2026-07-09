package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
)

func assetSearchDocumentsTableExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlTableExists(ctx, q, "asset_search_documents")
}

func productSearchDocumentsTableExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlTableExists(ctx, q, "product_search_documents")
}

func assetSearchDocumentsSemanticTextExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlColumnExists(ctx, q, "asset_search_documents", "semantic_text")
}

func productSearchDocumentsSemanticTextExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlColumnExists(ctx, q, "product_search_documents", "semantic_text")
}

func reindexAssetSearchDocument(ctx context.Context, q taskSearchDocumentSQL, assetID int64) error {
	if assetID <= 0 || !assetSearchDocumentsTableExists(ctx, q) {
		return nil
	}
	if _, err := q.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return fmt.Errorf("set asset search group_concat_max_len: %w", err)
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM asset_search_documents WHERE asset_id = ?`, assetID); err != nil {
		return fmt.Errorf("delete asset search document: %w", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO asset_search_documents (
		  asset_id, task_asset_id, task_id, asset_type, flow_review_status, sort_time, search_text, source_updated_at
		)
		SELECT
		  da.id,
		  ta.id,
		  ta.task_id,
		  COALESCE(ta.asset_type, ''),
		  COALESCE(ta.flow_review_status, ''),
		  COALESCE(ta.sort_time, ta.uploaded_at, ta.created_at),
		  CONCAT_WS(' ',
		    da.id, da.asset_no, ta.id, ta.file_name, ta.original_filename, ta.storage_key, ta.source_module_key,
		    t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
		    t.owner_team, t.owner_department, t.owner_org_team,
		    creator.username, creator.display_name, designer.username, designer.display_name
		  ),
		  GREATEST(ta.created_at, da.updated_at, t.updated_at)
		FROM design_assets da
		JOIN task_assets ta ON ta.id = da.current_version_id
		JOIN tasks t ON t.id = ta.task_id
		LEFT JOIN users creator ON creator.id = t.creator_id
		LEFT JOIN users designer ON designer.id = t.designer_id
		WHERE da.id = ?
		  AND ta.deleted_at IS NULL
		  AND ta.cleaned_at IS NULL
		  AND COALESCE(ta.is_archived, 0) = 0`, assetID); err != nil {
		return fmt.Errorf("upsert asset search document: %w", err)
	}
	return nil
}

func reindexAssetSearchDocumentsByTaskID(ctx context.Context, q taskSearchDocumentSQL, taskID int64) error {
	if taskID <= 0 || !assetSearchDocumentsTableExists(ctx, q) {
		return nil
	}
	if _, err := q.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return fmt.Errorf("set asset search group_concat_max_len by task: %w", err)
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM asset_search_documents WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete asset search documents by task: %w", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO asset_search_documents (
		  asset_id, task_asset_id, task_id, asset_type, flow_review_status, sort_time, search_text, source_updated_at
		)
		SELECT
		  da.id,
		  ta.id,
		  ta.task_id,
		  COALESCE(ta.asset_type, ''),
		  COALESCE(ta.flow_review_status, ''),
		  COALESCE(ta.sort_time, ta.uploaded_at, ta.created_at),
		  CONCAT_WS(' ',
		    da.id, da.asset_no, ta.id, ta.file_name, ta.original_filename, ta.storage_key, ta.source_module_key,
		    t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
		    t.owner_team, t.owner_department, t.owner_org_team,
		    creator.username, creator.display_name, designer.username, designer.display_name
		  ),
		  GREATEST(ta.created_at, da.updated_at, t.updated_at)
		FROM design_assets da
		JOIN task_assets ta ON ta.id = da.current_version_id
		JOIN tasks t ON t.id = ta.task_id
		LEFT JOIN users creator ON creator.id = t.creator_id
		LEFT JOIN users designer ON designer.id = t.designer_id
		WHERE ta.task_id = ?
		  AND ta.deleted_at IS NULL
		  AND ta.cleaned_at IS NULL
		  AND COALESCE(ta.is_archived, 0) = 0`, taskID); err != nil {
		return fmt.Errorf("upsert asset search documents by task: %w", err)
	}
	return nil
}

func reindexProductSearchDocument(ctx context.Context, q taskSearchDocumentSQL, skuCode string) error {
	skuCode = strings.TrimSpace(skuCode)
	if skuCode == "" || !productSearchDocumentsTableExists(ctx, q) {
		return nil
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM product_search_documents WHERE sku_code = ?`, skuCode); err != nil {
		return fmt.Errorf("delete product search document: %w", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO product_search_documents (
		  sku_code, product_name, i_id, category, search_text, source_updated_at
		)
		SELECT
		  p.sku_code,
		  COALESCE(p.product_name, ''),
		  COALESCE(p.i_id_gen, ''),
		  COALESCE(NULLIF(CASE WHEN JSON_VALID(p.spec_json) THEN JSON_UNQUOTE(JSON_EXTRACT(p.spec_json, '$.category_name')) ELSE '' END, ''), NULLIF(p.category, ''), ''),
		  CONCAT_WS(' ',
		    p.sku_code,
		    p.product_name,
		    p.category,
		    p.i_id_gen,
		    CASE WHEN JSON_VALID(p.spec_json) THEN JSON_UNQUOTE(JSON_EXTRACT(p.spec_json, '$.category_name')) ELSE '' END
		  ),
		  p.updated_at
		FROM products p
		WHERE p.sku_code = ?`, skuCode); err != nil {
		return fmt.Errorf("upsert product search document: %w", err)
	}
	return nil
}
