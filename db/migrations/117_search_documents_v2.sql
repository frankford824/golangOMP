-- Search document read models for global asset/product search.

SET SESSION group_concat_max_len = 1048576;

CREATE TABLE IF NOT EXISTS asset_search_documents (
  asset_id BIGINT NOT NULL,
  task_asset_id BIGINT NOT NULL,
  task_id BIGINT NOT NULL,
  asset_type VARCHAR(32) NOT NULL DEFAULT '',
  flow_review_status VARCHAR(32) NOT NULL DEFAULT '',
  sort_time DATETIME NOT NULL,
  search_text TEXT NULL,
  source_updated_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (asset_id),
  KEY idx_asset_search_documents_task_asset (task_asset_id),
  KEY idx_asset_search_documents_task (task_id),
  KEY idx_asset_search_documents_sort (sort_time DESC, asset_id DESC),
  FULLTEXT KEY ft_asset_search_documents_text (search_text) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Global search read model for current task assets';

CREATE TABLE IF NOT EXISTS product_search_documents (
  sku_code VARCHAR(64) NOT NULL,
  product_name VARCHAR(512) NOT NULL DEFAULT '',
  i_id VARCHAR(255) NOT NULL DEFAULT '',
  category VARCHAR(255) NOT NULL DEFAULT '',
  search_text TEXT NULL,
  source_updated_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (sku_code),
  KEY idx_product_search_documents_iid (i_id),
  KEY idx_product_search_documents_category (category),
  KEY idx_product_search_documents_updated (source_updated_at DESC),
  FULLTEXT KEY ft_product_search_documents_text (search_text) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Global search read model for ERP products';

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
WHERE ta.deleted_at IS NULL
  AND ta.cleaned_at IS NULL
  AND COALESCE(ta.is_archived, 0) = 0
ON DUPLICATE KEY UPDATE
  task_asset_id = VALUES(task_asset_id),
  task_id = VALUES(task_id),
  asset_type = VALUES(asset_type),
  flow_review_status = VALUES(flow_review_status),
  sort_time = VALUES(sort_time),
  search_text = VALUES(search_text),
  source_updated_at = VALUES(source_updated_at);

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
WHERE COALESCE(p.sku_code, '') <> ''
ON DUPLICATE KEY UPDATE
  product_name = VALUES(product_name),
  i_id = VALUES(i_id),
  category = VALUES(category),
  search_text = VALUES(search_text),
  source_updated_at = VALUES(source_updated_at);

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS product_search_documents;
DROP TABLE IF EXISTS asset_search_documents;
-- ROLLBACK-END
