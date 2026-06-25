-- Migration: 086_v1_6_product_management_erp_image_workflow.sql
-- Split ERP product base sync from ERP image sync and add task-scoped ERP product image workflow metadata.

SET @add_task_type_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'task_type') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN task_type VARCHAR(64) NOT NULL DEFAULT '' AFTER task_no",
  "SELECT 1"
);
PREPARE add_task_type_stmt FROM @add_task_type_sql;
EXECUTE add_task_type_stmt;
DEALLOCATE PREPARE add_task_type_stmt;

SET @add_source_mode_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'source_mode') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN source_mode VARCHAR(64) NOT NULL DEFAULT '' AFTER task_type",
  "SELECT 1"
);
PREPARE add_source_mode_stmt FROM @add_source_mode_sql;
EXECUTE add_source_mode_stmt;
DEALLOCATE PREPARE add_source_mode_stmt;

SET @add_erp_i_id_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'erp_i_id') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN erp_i_id VARCHAR(128) NOT NULL DEFAULT '' AFTER product_i_id",
  "SELECT 1"
);
PREPARE add_erp_i_id_stmt FROM @add_erp_i_id_sql;
EXECUTE add_erp_i_id_stmt;
DEALLOCATE PREPARE add_erp_i_id_stmt;

SET @add_category_name_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'category_name') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN category_name VARCHAR(255) NOT NULL DEFAULT '' AFTER erp_i_id",
  "SELECT 1"
);
PREPARE add_category_name_stmt FROM @add_category_name_sql;
EXECUTE add_category_name_stmt;
DEALLOCATE PREPARE add_category_name_stmt;

SET @add_product_family_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'product_family') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN product_family VARCHAR(255) NOT NULL DEFAULT '' AFTER category_name",
  "SELECT 1"
);
PREPARE add_product_family_stmt FROM @add_product_family_sql;
EXECUTE add_product_family_stmt;
DEALLOCATE PREPARE add_product_family_stmt;

SET @add_image_sync_source_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'image_sync_source') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN image_sync_source VARCHAR(32) NOT NULL DEFAULT '' AFTER image_missing_reason",
  "SELECT 1"
);
PREPARE add_image_sync_source_stmt FROM @add_image_sync_source_sql;
EXECUTE add_image_sync_source_stmt;
DEALLOCATE PREPARE add_image_sync_source_stmt;

SET @add_base_sync_status_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'base_sync_status') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN base_sync_status VARCHAR(32) NOT NULL DEFAULT 'pending_sync' AFTER erp_sync_status",
  "SELECT 1"
);
PREPARE add_base_sync_status_stmt FROM @add_base_sync_status_sql;
EXECUTE add_base_sync_status_stmt;
DEALLOCATE PREPARE add_base_sync_status_stmt;

SET @add_image_sync_status_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'image_sync_status') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN image_sync_status VARCHAR(32) NOT NULL DEFAULT 'waiting_image' AFTER base_sync_status",
  "SELECT 1"
);
PREPARE add_image_sync_status_stmt FROM @add_image_sync_status_sql;
EXECUTE add_image_sync_status_stmt;
DEALLOCATE PREPARE add_image_sync_status_stmt;

SET @add_last_base_synced_at_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'last_base_synced_at') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN last_base_synced_at DATETIME NULL AFTER last_erp_synced_at",
  "SELECT 1"
);
PREPARE add_last_base_synced_at_stmt FROM @add_last_base_synced_at_sql;
EXECUTE add_last_base_synced_at_stmt;
DEALLOCATE PREPARE add_last_base_synced_at_stmt;

SET @add_last_image_synced_at_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'last_image_synced_at') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN last_image_synced_at DATETIME NULL AFTER last_base_synced_at",
  "SELECT 1"
);
PREPARE add_last_image_synced_at_stmt FROM @add_last_image_synced_at_sql;
EXECUTE add_last_image_synced_at_stmt;
DEALLOCATE PREPARE add_last_image_synced_at_stmt;

SET @add_base_sync_error_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'base_sync_error') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN base_sync_error VARCHAR(512) NOT NULL DEFAULT '' AFTER last_sync_error",
  "SELECT 1"
);
PREPARE add_base_sync_error_stmt FROM @add_base_sync_error_sql;
EXECUTE add_base_sync_error_stmt;
DEALLOCATE PREPARE add_base_sync_error_stmt;

SET @add_image_sync_error_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'image_sync_error') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN image_sync_error VARCHAR(512) NOT NULL DEFAULT '' AFTER base_sync_error",
  "SELECT 1"
);
PREPARE add_image_sync_error_stmt FROM @add_image_sync_error_sql;
EXECUTE add_image_sync_error_stmt;
DEALLOCATE PREPARE add_image_sync_error_stmt;

SET @add_image_required_sql := IF(
  (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND COLUMN_NAME = 'image_required') = 0,
  "ALTER TABLE erp_product_sync_records ADD COLUMN image_required TINYINT(1) NOT NULL DEFAULT 1 AFTER image_sync_error",
  "SELECT 1"
);
PREPARE add_image_required_stmt FROM @add_image_required_sql;
EXECUTE add_image_required_stmt;
DEALLOCATE PREPARE add_image_required_stmt;

SET @add_idx_erp_iid_sql := IF(
  (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND INDEX_NAME = 'idx_erp_product_sync_records_erp_iid') = 0,
  "CREATE INDEX idx_erp_product_sync_records_erp_iid ON erp_product_sync_records (erp_i_id)",
  "SELECT 1"
);
PREPARE add_idx_erp_iid_stmt FROM @add_idx_erp_iid_sql;
EXECUTE add_idx_erp_iid_stmt;
DEALLOCATE PREPARE add_idx_erp_iid_stmt;

SET @add_idx_base_status_sql := IF(
  (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND INDEX_NAME = 'idx_erp_product_sync_records_base_status') = 0,
  "CREATE INDEX idx_erp_product_sync_records_base_status ON erp_product_sync_records (base_sync_status)",
  "SELECT 1"
);
PREPARE add_idx_base_status_stmt FROM @add_idx_base_status_sql;
EXECUTE add_idx_base_status_stmt;
DEALLOCATE PREPARE add_idx_base_status_stmt;

SET @add_idx_image_status_sql := IF(
  (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'erp_product_sync_records' AND INDEX_NAME = 'idx_erp_product_sync_records_image_status') = 0,
  "CREATE INDEX idx_erp_product_sync_records_image_status ON erp_product_sync_records (image_sync_status)",
  "SELECT 1"
);
PREPARE add_idx_image_status_stmt FROM @add_idx_image_status_sql;
EXECUTE add_idx_image_status_stmt;
DEALLOCATE PREPARE add_idx_image_status_stmt;

UPDATE erp_product_sync_records epsr
JOIN tasks t ON t.id = epsr.task_id
JOIN task_details td ON td.task_id = t.id
SET
  epsr.task_type = COALESCE(t.task_type, ''),
  epsr.source_mode = COALESCE(t.source_mode, ''),
  epsr.category_name = COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), epsr.category_name),
  epsr.product_family = COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), epsr.product_family),
  epsr.erp_i_id = COALESCE(
    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
    NULLIF(CASE
      WHEN NULLIF(epsr.product_i_id, '') IS NULL THEN ''
      WHEN BINARY epsr.product_i_id = BINARY COALESCE(td.category, '') THEN ''
      WHEN BINARY epsr.product_i_id = BINARY COALESCE(td.category_name, '') THEN ''
      ELSE epsr.product_i_id
    END, ''),
    ''
  ),
  epsr.product_i_id = COALESCE(
    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
    NULLIF(CASE
      WHEN NULLIF(epsr.product_i_id, '') IS NULL THEN ''
      WHEN BINARY epsr.product_i_id = BINARY COALESCE(td.category, '') THEN ''
      WHEN BINARY epsr.product_i_id = BINARY COALESCE(td.category_name, '') THEN ''
      ELSE epsr.product_i_id
    END, ''),
    ''
  ),
  epsr.base_sync_status = epsr.erp_sync_status,
  epsr.last_base_synced_at = epsr.last_erp_synced_at,
  epsr.image_sync_source = epsr.image_source,
  epsr.image_sync_status = CASE
    WHEN epsr.image_asset_id IS NULL OR epsr.image_asset_id <= 0 THEN 'waiting_image'
    WHEN epsr.erp_sync_status = 'synced' THEN 'synced'
    WHEN epsr.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN epsr.erp_sync_status
    ELSE 'pending_sync'
  END
WHERE epsr.task_sku_item_id IS NULL;

UPDATE erp_product_sync_records epsr
JOIN task_sku_items tsi ON tsi.id = epsr.task_sku_item_id
JOIN tasks t ON t.id = epsr.task_id
JOIN task_details td ON td.task_id = t.id
SET
  epsr.task_type = COALESCE(t.task_type, ''),
  epsr.source_mode = COALESCE(t.source_mode, ''),
  epsr.category_name = COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), epsr.category_name),
  epsr.product_family = COALESCE(NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_family')) ELSE '' END, ''), NULLIF(td.category_name, ''), NULLIF(td.category, ''), epsr.product_family),
  epsr.erp_i_id = COALESCE(
    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')) ELSE '' END, ''),
    NULLIF(epsr.product_i_id, ''),
    ''
  ),
  epsr.product_i_id = COALESCE(
    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END, ''),
    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')) ELSE '' END, ''),
    NULLIF(epsr.product_i_id, ''),
    ''
  ),
  epsr.base_sync_status = epsr.erp_sync_status,
  epsr.last_base_synced_at = epsr.last_erp_synced_at,
  epsr.image_sync_source = epsr.image_source,
  epsr.image_sync_status = CASE
    WHEN epsr.image_asset_id IS NULL OR epsr.image_asset_id <= 0 THEN 'waiting_image'
    WHEN epsr.erp_sync_status = 'synced' THEN 'synced'
    WHEN epsr.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN epsr.erp_sync_status
    ELSE 'pending_sync'
  END
WHERE epsr.task_sku_item_id IS NOT NULL;

-- ROLLBACK-BEGIN
ALTER TABLE erp_product_sync_records
  DROP COLUMN image_required,
  DROP COLUMN image_sync_error,
  DROP COLUMN base_sync_error,
  DROP COLUMN last_image_synced_at,
  DROP COLUMN last_base_synced_at,
  DROP COLUMN image_sync_status,
  DROP COLUMN base_sync_status,
  DROP COLUMN image_sync_source,
  DROP COLUMN product_family,
  DROP COLUMN category_name,
  DROP COLUMN erp_i_id,
  DROP COLUMN source_mode,
  DROP COLUMN task_type;
-- ROLLBACK-END
