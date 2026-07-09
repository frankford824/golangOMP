-- Materialize asset-center current version and sort time invariants.

UPDATE design_assets da
   SET da.current_version_id = (
         SELECT ta.id
           FROM task_assets ta
          WHERE ta.asset_id = da.id
          ORDER BY ta.asset_version_no DESC, ta.id DESC
          LIMIT 1
       )
 WHERE da.current_version_id IS NULL
   AND EXISTS (
         SELECT 1
           FROM task_assets ta
          WHERE ta.asset_id = da.id
       );

SET @has_task_assets_sort_time := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_assets'
      AND column_name = 'sort_time'
);
SET @sql := IF(
    @has_task_assets_sort_time = 0,
    'ALTER TABLE task_assets ADD COLUMN sort_time DATETIME GENERATED ALWAYS AS (COALESCE(uploaded_at, created_at)) STORED AFTER uploaded_at',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_task_assets_archive_sort := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'task_assets'
      AND index_name = 'idx_task_assets_archive_sort'
);
SET @sql := IF(
    @has_idx_task_assets_archive_sort = 0,
    'ALTER TABLE task_assets ADD INDEX idx_task_assets_archive_sort (is_archived, sort_time DESC, id DESC)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- ROLLBACK-BEGIN
ALTER TABLE task_assets DROP INDEX idx_task_assets_archive_sort;
ALTER TABLE task_assets DROP COLUMN sort_time;
-- ROLLBACK-END
