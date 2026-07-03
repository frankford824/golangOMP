-- Replace the external asset fulltext index with an ngram parser variant and
-- add asset-workbench drive/overview fulltext indexes. These keep Chinese
-- two-character searches on the index path instead of falling back to broad
-- contains-LIKE scans.

SET @has_external_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'external_asset_records'
    AND index_name = 'ft_external_asset_search_text'
);
SET @sql_drop_external_fulltext := IF(
  @has_external_fulltext > 0,
  'ALTER TABLE external_asset_records DROP INDEX ft_external_asset_search_text',
  'SELECT 1'
);
PREPARE stmt_drop_external_fulltext FROM @sql_drop_external_fulltext;
EXECUTE stmt_drop_external_fulltext;
DEALLOCATE PREPARE stmt_drop_external_fulltext;

ALTER TABLE external_asset_records
  ADD FULLTEXT KEY ft_external_asset_search_text (file_name, origin_path, parent_path, searchable_text) WITH PARSER ngram;

SET @has_aw_submissions_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submissions'
    AND index_name = 'ft_aw_submissions_search_text'
);
SET @sql_aw_submissions_fulltext := IF(
  @has_aw_submissions_fulltext = 0,
  'ALTER TABLE asset_workbench_submissions ADD FULLTEXT KEY ft_aw_submissions_search_text (submission_no, notes) WITH PARSER ngram',
  'SELECT 1'
);
PREPARE stmt_aw_submissions_fulltext FROM @sql_aw_submissions_fulltext;
EXECUTE stmt_aw_submissions_fulltext;
DEALLOCATE PREPARE stmt_aw_submissions_fulltext;

SET @has_aw_items_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_items'
    AND index_name = 'ft_aw_items_search_text'
);
SET @sql_aw_items_fulltext := IF(
  @has_aw_items_fulltext = 0,
  'ALTER TABLE asset_workbench_submission_items ADD FULLTEXT KEY ft_aw_items_search_text (order_no, template_name_snapshot, category_snapshot, difficulty_class) WITH PARSER ngram',
  'SELECT 1'
);
PREPARE stmt_aw_items_fulltext FROM @sql_aw_items_fulltext;
EXECUTE stmt_aw_items_fulltext;
DEALLOCATE PREPARE stmt_aw_items_fulltext;

SET @has_aw_files_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'ft_aw_files_search_text'
);
SET @sql_aw_files_fulltext := IF(
  @has_aw_files_fulltext = 0,
  'ALTER TABLE asset_workbench_submission_files ADD FULLTEXT KEY ft_aw_files_search_text (original_filename, file_type, mime_type, upload_directory_name) WITH PARSER ngram',
  'SELECT 1'
);
PREPARE stmt_aw_files_fulltext FROM @sql_aw_files_fulltext;
EXECUTE stmt_aw_files_fulltext;
DEALLOCATE PREPARE stmt_aw_files_fulltext;

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_submission_files DROP INDEX ft_aw_files_search_text;
ALTER TABLE asset_workbench_submission_items DROP INDEX ft_aw_items_search_text;
ALTER TABLE asset_workbench_submissions DROP INDEX ft_aw_submissions_search_text;
ALTER TABLE external_asset_records DROP INDEX ft_external_asset_search_text;
ALTER TABLE external_asset_records ADD FULLTEXT KEY ft_external_asset_search_text (file_name, origin_path, parent_path, searchable_text);
-- ROLLBACK-END
