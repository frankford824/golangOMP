-- Migration: 085_v1_5_erp_product_sync_records.sql
-- Local read model for SKU-level ERP product image/cost sync management.

CREATE TABLE IF NOT EXISTS erp_product_sync_records (
  id BIGINT NOT NULL AUTO_INCREMENT,
  record_key VARCHAR(96) NOT NULL,
  task_id BIGINT NOT NULL,
  task_sku_item_id BIGINT NULL,
  task_no VARCHAR(64) NOT NULL DEFAULT '',
  sku_code VARCHAR(64) NOT NULL DEFAULT '',
  product_i_id VARCHAR(128) NOT NULL DEFAULT '',
  product_name VARCHAR(512) NOT NULL DEFAULT '',
  cost_price DECIMAL(12,3) NULL,
  creator_id BIGINT NOT NULL DEFAULT 0,
  creator_name VARCHAR(128) NOT NULL DEFAULT '',
  task_created_at DATETIME NOT NULL,
  image_source VARCHAR(32) NOT NULL DEFAULT 'missing',
  image_selection_mode VARCHAR(16) NOT NULL DEFAULT 'auto',
  image_asset_id BIGINT NULL,
  image_asset_version_id BIGINT NULL,
  image_filename VARCHAR(512) NOT NULL DEFAULT '',
  image_mime_type VARCHAR(128) NOT NULL DEFAULT '',
  image_missing_reason VARCHAR(255) NOT NULL DEFAULT 'ERP 图片待补充',
  erp_sync_status VARCHAR(32) NOT NULL DEFAULT 'pending_sync',
  last_erp_checked_at DATETIME NULL,
  last_erp_synced_at DATETIME NULL,
  sync_cooldown_until DATETIME NULL,
  sync_claim_token VARCHAR(96) NOT NULL DEFAULT '',
  last_sync_error VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_erp_product_sync_records_record_key (record_key),
  KEY idx_erp_product_sync_records_task (task_id),
  KEY idx_erp_product_sync_records_sku (sku_code),
  KEY idx_erp_product_sync_records_iid (product_i_id),
  KEY idx_erp_product_sync_records_creator (creator_id),
  KEY idx_erp_product_sync_records_image_source (image_source),
  KEY idx_erp_product_sync_records_sync_status (erp_sync_status),
  KEY idx_erp_product_sync_records_sync_claim (sync_claim_token),
  KEY idx_erp_product_sync_records_task_created (task_created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT IGNORE INTO erp_product_sync_records (
  record_key, task_id, task_sku_item_id, task_no, sku_code, product_i_id,
  product_name, cost_price, creator_id, creator_name, task_created_at,
  erp_sync_status, last_erp_synced_at
)
SELECT
  CONCAT('task:', t.id, ':main'),
  t.id,
  NULL,
  COALESCE(t.task_no, ''),
  COALESCE(t.sku_code, ''),
  COALESCE(
    NULLIF(td.category, ''),
    NULLIF(td.category_name, ''),
    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
    ''
  ),
  COALESCE(NULLIF(td.product_short_name, ''), NULLIF(t.product_name_snapshot, ''), ''),
  td.cost_price,
  COALESCE(t.creator_id, 0),
  COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''),
  t.created_at,
  CASE
    WHEN td.filing_status = 'filed' THEN 'synced'
    WHEN td.filing_status = 'filing_failed' THEN 'failed'
    ELSE 'pending_sync'
  END,
  td.last_filed_at
FROM tasks t
JOIN task_details td ON td.task_id = t.id
LEFT JOIN users u ON u.id = t.creator_id
WHERE COALESCE(t.sku_code, '') <> ''
  AND NOT EXISTS (
    SELECT 1 FROM task_sku_items tsi WHERE tsi.task_id = t.id
  );

INSERT IGNORE INTO erp_product_sync_records (
  record_key, task_id, task_sku_item_id, task_no, sku_code, product_i_id,
  product_name, cost_price, creator_id, creator_name, task_created_at,
  erp_sync_status, last_erp_synced_at
)
SELECT
  CONCAT('task:', t.id, ':sku:', tsi.id),
  t.id,
  tsi.id,
  COALESCE(t.task_no, ''),
  COALESCE(tsi.sku_code, ''),
  COALESCE(
    NULLIF(JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')), ''),
    NULLIF(JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')), ''),
    ''
  ),
  COALESCE(NULLIF(tsi.product_short_name, ''), NULLIF(tsi.product_name_snapshot, ''), NULLIF(t.product_name_snapshot, ''), ''),
  tsi.cost_price,
  COALESCE(t.creator_id, 0),
  COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''),
  t.created_at,
  CASE
    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filed' THEN 'synced'
    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filing_failed' THEN 'failed'
    ELSE 'pending_sync'
  END,
  tsi.last_filed_at
FROM task_sku_items tsi
JOIN tasks t ON t.id = tsi.task_id
JOIN task_details td ON td.task_id = t.id
LEFT JOIN users u ON u.id = t.creator_id
WHERE COALESCE(tsi.sku_code, '') <> '';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS erp_product_sync_records;
-- ROLLBACK-END
