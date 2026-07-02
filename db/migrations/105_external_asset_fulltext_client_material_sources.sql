-- Add DB-level search indexes for external resources and make asset-workbench
-- client materials source-aware. Existing rows remain system-asset materials.

SET @has_external_visible_updated := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'external_asset_records'
    AND index_name = 'idx_external_asset_visible_updated'
);
SET @sql_external_visible_updated := IF(
  @has_external_visible_updated = 0,
  'ALTER TABLE external_asset_records ADD KEY idx_external_asset_visible_updated (is_dir, status, updated_at, id)',
  'SELECT 1'
);
PREPARE stmt_external_visible_updated FROM @sql_external_visible_updated;
EXECUTE stmt_external_visible_updated;
DEALLOCATE PREPARE stmt_external_visible_updated;

SET @has_external_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'external_asset_records'
    AND index_name = 'ft_external_asset_search_text'
);
SET @sql_external_fulltext := IF(
  @has_external_fulltext = 0,
  'ALTER TABLE external_asset_records ADD FULLTEXT KEY ft_external_asset_search_text (file_name, origin_path, parent_path, searchable_text)',
  'SELECT 1'
);
PREPARE stmt_external_fulltext FROM @sql_external_fulltext;
EXECUTE stmt_external_fulltext;
DEALLOCATE PREPARE stmt_external_fulltext;

SET @has_client_material_source_type := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_client_materials'
    AND column_name = 'source_type'
);
SET @sql_client_material_source_type := IF(
  @has_client_material_source_type = 0,
  'ALTER TABLE asset_workbench_client_materials ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT ''system'' AFTER asset_id',
  'SELECT 1'
);
PREPARE stmt_client_material_source_type FROM @sql_client_material_source_type;
EXECUTE stmt_client_material_source_type;
DEALLOCATE PREPARE stmt_client_material_source_type;

SET @has_client_material_source_ref := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_client_materials'
    AND column_name = 'source_ref'
);
SET @sql_client_material_source_ref := IF(
  @has_client_material_source_ref = 0,
  'ALTER TABLE asset_workbench_client_materials ADD COLUMN source_ref VARCHAR(64) NOT NULL DEFAULT '''' AFTER source_type',
  'SELECT 1'
);
PREPARE stmt_client_material_source_ref FROM @sql_client_material_source_ref;
EXECUTE stmt_client_material_source_ref;
DEALLOCATE PREPARE stmt_client_material_source_ref;

UPDATE asset_workbench_client_materials
   SET source_type = CASE WHEN source_type = '' THEN 'system' ELSE source_type END,
       source_ref = CASE WHEN source_ref = '' THEN CAST(asset_id AS CHAR) ELSE source_ref END
 WHERE source_type = '' OR source_ref = '';

SET @has_legacy_client_material_asset_unique := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_client_materials'
    AND index_name = 'uk_aw_client_materials_asset'
);
SET @sql_drop_legacy_client_material_asset_unique := IF(
  @has_legacy_client_material_asset_unique > 0,
  'ALTER TABLE asset_workbench_client_materials DROP INDEX uk_aw_client_materials_asset',
  'SELECT 1'
);
PREPARE stmt_drop_legacy_client_material_asset_unique FROM @sql_drop_legacy_client_material_asset_unique;
EXECUTE stmt_drop_legacy_client_material_asset_unique;
DEALLOCATE PREPARE stmt_drop_legacy_client_material_asset_unique;

SET @has_client_material_source_unique := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_client_materials'
    AND index_name = 'uk_aw_client_materials_source'
);
SET @sql_client_material_source_unique := IF(
  @has_client_material_source_unique = 0,
  'ALTER TABLE asset_workbench_client_materials ADD UNIQUE KEY uk_aw_client_materials_source (source_type, source_ref)',
  'SELECT 1'
);
PREPARE stmt_client_material_source_unique FROM @sql_client_material_source_unique;
EXECUTE stmt_client_material_source_unique;
DEALLOCATE PREPARE stmt_client_material_source_unique;

SET @has_client_material_source_enabled := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_client_materials'
    AND index_name = 'idx_aw_client_materials_source_enabled'
);
SET @sql_client_material_source_enabled := IF(
  @has_client_material_source_enabled = 0,
  'ALTER TABLE asset_workbench_client_materials ADD KEY idx_aw_client_materials_source_enabled (source_type, enabled, sort_order, id)',
  'SELECT 1'
);
PREPARE stmt_client_material_source_enabled FROM @sql_client_material_source_enabled;
EXECUTE stmt_client_material_source_enabled;
DEALLOCATE PREPARE stmt_client_material_source_enabled;

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_client_materials DROP INDEX idx_aw_client_materials_source_enabled;
ALTER TABLE asset_workbench_client_materials DROP INDEX uk_aw_client_materials_source;
ALTER TABLE asset_workbench_client_materials ADD UNIQUE KEY uk_aw_client_materials_asset (asset_id);
ALTER TABLE asset_workbench_client_materials DROP COLUMN source_ref;
ALTER TABLE asset_workbench_client_materials DROP COLUMN source_type;
ALTER TABLE external_asset_records DROP INDEX ft_external_asset_search_text;
ALTER TABLE external_asset_records DROP INDEX idx_external_asset_visible_updated;
-- ROLLBACK-END
