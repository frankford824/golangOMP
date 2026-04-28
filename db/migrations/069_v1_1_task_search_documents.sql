CREATE TABLE IF NOT EXISTS task_search_documents (
  task_id BIGINT NOT NULL,
  task_no VARCHAR(64) NOT NULL,
  product_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  sku_code VARCHAR(64) NOT NULL DEFAULT '',
  primary_sku_code VARCHAR(64) NOT NULL DEFAULT '',
  product_i_id VARCHAR(128) NOT NULL DEFAULT '',
  task_type VARCHAR(64) NOT NULL DEFAULT '',
  task_status VARCHAR(64) NOT NULL DEFAULT '',
  priority VARCHAR(32) NOT NULL DEFAULT '',
  owner_department VARCHAR(128) NOT NULL DEFAULT '',
  owner_team VARCHAR(128) NOT NULL DEFAULT '',
  owner_org_team VARCHAR(128) NOT NULL DEFAULT '',
  creator_id BIGINT NULL,
  creator_name VARCHAR(128) NOT NULL DEFAULT '',
  requester_id BIGINT NULL,
  requester_name VARCHAR(128) NOT NULL DEFAULT '',
  designer_id BIGINT NULL,
  designer_name VARCHAR(128) NOT NULL DEFAULT '',
  current_handler_id BIGINT NULL,
  current_handler_name VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME NULL,
  updated_at DATETIME NULL,
  deadline_at DATETIME NULL,
  asset_text TEXT NULL,
  search_text LONGTEXT NOT NULL,
  PRIMARY KEY (task_id),
  KEY idx_task_search_updated (updated_at, task_id),
  KEY idx_task_search_status_updated (task_status, updated_at, task_id),
  KEY idx_task_search_iid (product_i_id),
  KEY idx_task_search_creator (creator_id),
  KEY idx_task_search_designer (designer_id),
  FULLTEXT KEY ft_task_search_text (search_text)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Task global-search read model';

SET SESSION group_concat_max_len = 1048576;

INSERT INTO task_search_documents (
  task_id, task_no, product_name_snapshot, sku_code, primary_sku_code, product_i_id,
  task_type, task_status, priority, owner_department, owner_team, owner_org_team,
  creator_id, creator_name, requester_id, requester_name, designer_id, designer_name,
  current_handler_id, current_handler_name, created_at, updated_at, deadline_at, asset_text, search_text
)
SELECT
  t.id,
  t.task_no,
  COALESCE(t.product_name_snapshot, ''),
  COALESCE(t.sku_code, ''),
  COALESCE(t.primary_sku_code, ''),
  COALESCE(
    NULLIF(td.category, ''),
    NULLIF(td.category_name, ''),
    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
    ''
  ),
  COALESCE(t.task_type, ''),
  COALESCE(t.task_status, ''),
  COALESCE(t.priority, ''),
  COALESCE(t.owner_department, ''),
  COALESCE(t.owner_team, ''),
  COALESCE(t.owner_org_team, ''),
  t.creator_id,
  COALESCE(NULLIF(creator.display_name, ''), creator.username, ''),
  t.requester_id,
  COALESCE(NULLIF(requester.display_name, ''), requester.username, ''),
  t.designer_id,
  COALESCE(NULLIF(designer.display_name, ''), designer.username, ''),
  t.current_handler_id,
  COALESCE(NULLIF(handler.display_name, ''), handler.username, ''),
  t.created_at,
  t.updated_at,
  t.deadline_at,
  COALESCE(assets.asset_text, ''),
  CONCAT_WS(' ',
    t.id, t.task_no, t.product_name_snapshot, t.sku_code, t.primary_sku_code,
    t.task_type, t.task_status, t.priority, t.owner_department, t.owner_team, t.owner_org_team,
    COALESCE(NULLIF(td.category, ''), NULLIF(td.category_name, ''), ''),
    td.category_code, td.product_short_name, td.demand_text, td.copy_text, td.remark,
    td.change_request, td.design_requirement, td.material, td.spec_text, td.size_text,
    td.craft_text, td.process, td.reference_link,
    COALESCE(NULLIF(creator.display_name, ''), creator.username, ''),
    COALESCE(NULLIF(requester.display_name, ''), requester.username, ''),
    COALESCE(NULLIF(designer.display_name, ''), designer.username, ''),
    COALESCE(NULLIF(handler.display_name, ''), handler.username, ''),
    DATE_FORMAT(t.created_at, '%Y-%m-%d'), DATE_FORMAT(t.created_at, '%Y%m%d'),
    DATE_FORMAT(t.deadline_at, '%Y-%m-%d'), COALESCE(assets.asset_text, '')
  )
FROM tasks t
LEFT JOIN task_details td ON td.task_id = t.id
LEFT JOIN users creator ON creator.id = t.creator_id
LEFT JOIN users requester ON requester.id = t.requester_id
LEFT JOIN users designer ON designer.id = t.designer_id
LEFT JOIN users handler ON handler.id = t.current_handler_id
LEFT JOIN (
  SELECT task_id, GROUP_CONCAT(CONCAT_WS(' ', file_name, original_filename, storage_key, source_module_key) SEPARATOR ' ') AS asset_text
  FROM task_assets
  WHERE COALESCE(deleted_at, cleaned_at) IS NULL
  GROUP BY task_id
) assets ON assets.task_id = t.id
ON DUPLICATE KEY UPDATE
  task_no = VALUES(task_no),
  product_name_snapshot = VALUES(product_name_snapshot),
  sku_code = VALUES(sku_code),
  primary_sku_code = VALUES(primary_sku_code),
  product_i_id = VALUES(product_i_id),
  task_type = VALUES(task_type),
  task_status = VALUES(task_status),
  priority = VALUES(priority),
  owner_department = VALUES(owner_department),
  owner_team = VALUES(owner_team),
  owner_org_team = VALUES(owner_org_team),
  creator_id = VALUES(creator_id),
  creator_name = VALUES(creator_name),
  requester_id = VALUES(requester_id),
  requester_name = VALUES(requester_name),
  designer_id = VALUES(designer_id),
  designer_name = VALUES(designer_name),
  current_handler_id = VALUES(current_handler_id),
  current_handler_name = VALUES(current_handler_name),
  created_at = VALUES(created_at),
  updated_at = VALUES(updated_at),
  deadline_at = VALUES(deadline_at),
  asset_text = VALUES(asset_text),
  search_text = VALUES(search_text);
