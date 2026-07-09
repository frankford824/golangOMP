-- Materialize product-management lookup fields used by hot read paths.

SET SESSION group_concat_max_len = 1048576;

SET @has_latest_cost_snapshot_id := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND column_name = 'latest_cost_snapshot_id'
);
SET @sql := IF(
    @has_latest_cost_snapshot_id = 0,
    'ALTER TABLE erp_product_sync_records ADD COLUMN latest_cost_snapshot_id BIGINT NULL AFTER updated_at',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_latest_erp_trace_id := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND column_name = 'latest_erp_trace_id'
);
SET @sql := IF(
    @has_latest_erp_trace_id = 0,
    'ALTER TABLE erp_product_sync_records ADD COLUMN latest_erp_trace_id BIGINT NULL AFTER latest_cost_snapshot_id',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_combo_search_text := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND column_name = 'combo_search_text'
);
SET @sql := IF(
    @has_combo_search_text = 0,
    'ALTER TABLE erp_product_sync_records ADD COLUMN combo_search_text TEXT NULL AFTER latest_erp_trace_id',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_cost_legacy_alias_fallback := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND column_name = 'cost_legacy_alias_fallback'
);
SET @sql := IF(
    @has_cost_legacy_alias_fallback = 0,
    'ALTER TABLE erp_product_sync_records ADD COLUMN cost_legacy_alias_fallback TINYINT(1) NOT NULL DEFAULT 0 AFTER combo_search_text',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_cost_area_spec_abnormal := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND column_name = 'cost_area_spec_abnormal'
);
SET @sql := IF(
    @has_cost_area_spec_abnormal = 0,
    'ALTER TABLE erp_product_sync_records ADD COLUMN cost_area_spec_abnormal TINYINT(1) NOT NULL DEFAULT 0 AFTER cost_legacy_alias_fallback',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_latest_cost_snapshot := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND index_name = 'idx_erp_product_sync_records_latest_cost'
);
SET @sql := IF(
    @has_idx_latest_cost_snapshot = 0,
    'ALTER TABLE erp_product_sync_records ADD INDEX idx_erp_product_sync_records_latest_cost (latest_cost_snapshot_id)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx_latest_erp_trace := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND index_name = 'idx_erp_product_sync_records_latest_trace'
);
SET @sql := IF(
    @has_idx_latest_erp_trace = 0,
    'ALTER TABLE erp_product_sync_records ADD INDEX idx_erp_product_sync_records_latest_trace (latest_erp_trace_id)',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

UPDATE erp_product_sync_records pm
   SET pm.latest_cost_snapshot_id = (
         SELECT s.id
           FROM omp_sku_cost_snapshots s
          WHERE s.sku_code = pm.sku_code
            AND (
              (pm.task_sku_item_id IS NOT NULL AND s.task_sku_item_id = pm.task_sku_item_id)
              OR (pm.task_sku_item_id IS NULL AND s.task_id = pm.task_id AND s.task_sku_item_id IS NULL)
              OR s.task_id = pm.task_id
              OR s.task_id IS NULL
            )
          ORDER BY
            CASE
              WHEN pm.task_sku_item_id IS NOT NULL AND s.task_sku_item_id = pm.task_sku_item_id THEN 0
              WHEN pm.task_sku_item_id IS NULL AND s.task_id = pm.task_id AND s.task_sku_item_id IS NULL THEN 1
              WHEN s.task_id = pm.task_id THEN 2
              ELSE 3
            END,
            s.created_at DESC,
            s.id DESC
          LIMIT 1
       );

UPDATE erp_product_sync_records pm
   SET pm.latest_erp_trace_id = (
         SELECT l.id
           FROM omp_sku_erp_trace_logs l
          WHERE l.sku_code = pm.sku_code
            AND (
              (pm.task_sku_item_id IS NOT NULL AND l.task_sku_item_id = pm.task_sku_item_id)
              OR (pm.task_sku_item_id IS NULL AND l.task_id = pm.task_id AND l.task_sku_item_id IS NULL)
              OR l.task_id = pm.task_id
              OR l.task_id IS NULL
            )
          ORDER BY l.created_at DESC, l.id DESC
          LIMIT 1
       );

UPDATE erp_product_sync_records pm
LEFT JOIN (
    SELECT
      limited.child_sku_code,
      GROUP_CONCAT(DISTINCT limited.search_token ORDER BY limited.search_token SEPARATOR ' ') AS combo_search_text
      FROM (
        SELECT ranked.child_sku_code, ranked.search_token
          FROM (
            SELECT
              rel.child_sku_code,
              LEFT(CONCAT_WS(' ', rel.combo_sku_code, rec.erp_i_id, rec.name, rec.short_name), 256) AS search_token,
              ROW_NUMBER() OVER (PARTITION BY rel.child_sku_code ORDER BY rel.combo_sku_code, COALESCE(rec.erp_i_id, '')) AS rn
              FROM omp_sku_combo_relations rel
              LEFT JOIN omp_sku_combo_records rec ON rec.combo_sku_code = rel.combo_sku_code
          ) ranked
         WHERE ranked.rn <= 200
      ) limited
     GROUP BY limited.child_sku_code
) combo ON combo.child_sku_code = pm.sku_code
   SET pm.combo_search_text = COALESCE(combo.combo_search_text, '');

SET @has_ft_combo_search := (
    SELECT COUNT(1)
    FROM information_schema.STATISTICS
    WHERE table_schema = DATABASE()
      AND table_name = 'erp_product_sync_records'
      AND index_name = 'ft_erp_product_sync_records_combo_ngram'
);
SET @sql := IF(
    @has_ft_combo_search = 0,
    'ALTER TABLE erp_product_sync_records ADD FULLTEXT INDEX ft_erp_product_sync_records_combo_ngram (combo_search_text) WITH PARSER ngram',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

UPDATE erp_product_sync_records pm
LEFT JOIN omp_sku_cost_snapshots cost_snapshot ON cost_snapshot.id = pm.latest_cost_snapshot_id
LEFT JOIN task_details pm_td ON pm_td.task_id = pm.task_id
LEFT JOIN task_sku_items pm_tsi ON pm.task_sku_item_id IS NOT NULL AND pm_tsi.id = pm.task_sku_item_id
   SET pm.cost_legacy_alias_fallback = CASE
         WHEN JSON_VALID(cost_snapshot.calculation_snapshot_json)
          AND JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.legacy_alias_fallback')) = 'true'
         THEN 1 ELSE 0 END,
       pm.cost_area_spec_abnormal = CASE
         WHEN pm.cost_price IS NOT NULL AND pm.cost_price > 0
          AND COALESCE(pm_td.area, 0) <= 0
          AND (COALESCE(pm_td.width, 0) <= 0 OR COALESCE(pm_td.height, 0) <= 0)
          AND (
            pm.task_sku_item_id IS NULL
            OR NOT JSON_VALID(pm_tsi.variant_json)
            OR (
                 COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.area')) AS DECIMAL(12,4)), 0) <= 0
             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.width')) AS DECIMAL(12,4)), 0) <= 0
             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.height')) AS DECIMAL(12,4)), 0) <= 0
            )
          )
         THEN 1 ELSE 0 END;

-- ROLLBACK-BEGIN
ALTER TABLE erp_product_sync_records DROP INDEX ft_erp_product_sync_records_combo_ngram;
ALTER TABLE erp_product_sync_records DROP INDEX idx_erp_product_sync_records_latest_trace;
ALTER TABLE erp_product_sync_records DROP INDEX idx_erp_product_sync_records_latest_cost;
ALTER TABLE erp_product_sync_records DROP COLUMN cost_area_spec_abnormal;
ALTER TABLE erp_product_sync_records DROP COLUMN cost_legacy_alias_fallback;
ALTER TABLE erp_product_sync_records DROP COLUMN combo_search_text;
ALTER TABLE erp_product_sync_records DROP COLUMN latest_erp_trace_id;
ALTER TABLE erp_product_sync_records DROP COLUMN latest_cost_snapshot_id;
-- ROLLBACK-END
