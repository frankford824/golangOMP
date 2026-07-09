-- Phase 1 performance indexes for main-ops read paths.

SET @has_idx_task_event_logs_event_created_task := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_event_logs'
      AND index_name = 'idx_task_event_logs_event_created_task'
);
SET @sql := IF(
    @has_idx_task_event_logs_event_created_task = 0,
    'ALTER TABLE task_event_logs ADD INDEX idx_task_event_logs_event_created_task (event_type, created_at, task_id)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_task_assets_asset_version_desc := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_assets'
      AND index_name = 'idx_task_assets_asset_version_desc'
);
SET @sql := IF(
    @has_idx_task_assets_asset_version_desc = 0,
    'ALTER TABLE task_assets ADD INDEX idx_task_assets_asset_version_desc (asset_id, asset_version_no DESC, id DESC)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_products_i_id_gen := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND column_name = 'i_id_gen'
);
SET @sql := IF(
    @has_products_i_id_gen = 0,
    'ALTER TABLE products ADD COLUMN i_id_gen VARCHAR(255) GENERATED ALWAYS AS (NULLIF(TRIM(CASE WHEN JSON_VALID(spec_json) THEN JSON_UNQUOTE(JSON_EXTRACT(spec_json, ''$.i_id'')) ELSE '''' END), '''')) STORED AFTER spec_json',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_products_i_id_gen := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'products'
      AND index_name = 'idx_products_i_id_gen'
);
SET @sql := IF(
    @has_idx_products_i_id_gen = 0,
    'ALTER TABLE products ADD INDEX idx_products_i_id_gen (i_id_gen)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_erp_product_sync_records_sort := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND index_name = 'idx_erp_product_sync_records_sort'
);
SET @sql := IF(
    @has_idx_erp_product_sync_records_sort = 0,
    'ALTER TABLE erp_product_sync_records ADD INDEX idx_erp_product_sync_records_sort (updated_at DESC, task_created_at DESC, id DESC)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_task_search_documents := (
    SELECT COUNT(1)
    FROM information_schema.TABLES
    WHERE table_schema = DATABASE()
      AND table_name = 'task_search_documents'
);
SET @has_ft_task_search_text := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_search_documents'
      AND index_name = 'ft_task_search_text'
);
SET @sql := IF(
    @has_task_search_documents > 0 AND @has_ft_task_search_text > 0,
    'ALTER TABLE task_search_documents DROP INDEX ft_task_search_text',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
    @has_task_search_documents > 0,
    'ALTER TABLE task_search_documents ADD FULLTEXT KEY ft_task_search_text (search_text) WITH PARSER ngram',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ROLLBACK-BEGIN
ALTER TABLE task_search_documents DROP INDEX ft_task_search_text;
ALTER TABLE task_search_documents ADD FULLTEXT KEY ft_task_search_text (search_text);
ALTER TABLE erp_product_sync_records DROP INDEX idx_erp_product_sync_records_sort;
ALTER TABLE products DROP INDEX idx_products_i_id_gen;
ALTER TABLE products DROP COLUMN i_id_gen;
ALTER TABLE task_assets DROP INDEX idx_task_assets_asset_version_desc;
ALTER TABLE task_event_logs DROP INDEX idx_task_event_logs_event_created_task;
-- ROLLBACK-END
