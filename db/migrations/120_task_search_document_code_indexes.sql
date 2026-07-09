-- Add BTREE indexes for code-style task search recall so exact/prefix lookups on
-- task_search_documents no longer reverse-scan idx_task_search_updated.
-- product_i_id already has idx_task_search_iid (migration 069), so it is omitted.

SET @has_idx_task_search_task_no := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_search_documents'
      AND index_name = 'idx_task_search_task_no'
);
SET @sql := IF(
    @has_idx_task_search_task_no = 0,
    'ALTER TABLE task_search_documents ADD INDEX idx_task_search_task_no (task_no)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_task_search_sku_code := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_search_documents'
      AND index_name = 'idx_task_search_sku_code'
);
SET @sql := IF(
    @has_idx_task_search_sku_code = 0,
    'ALTER TABLE task_search_documents ADD INDEX idx_task_search_sku_code (sku_code)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_task_search_primary_sku := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_search_documents'
      AND index_name = 'idx_task_search_primary_sku'
);
SET @sql := IF(
    @has_idx_task_search_primary_sku = 0,
    'ALTER TABLE task_search_documents ADD INDEX idx_task_search_primary_sku (primary_sku_code)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ROLLBACK-BEGIN
ALTER TABLE task_search_documents DROP INDEX idx_task_search_primary_sku;
ALTER TABLE task_search_documents DROP INDEX idx_task_search_sku_code;
ALTER TABLE task_search_documents DROP INDEX idx_task_search_task_no;
-- ROLLBACK-END
