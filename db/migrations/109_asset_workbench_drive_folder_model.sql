-- Add folder-upload metadata, editable display names, soft delete state, and
-- updated fulltext coverage for the asset-workbench drive model.

SET @has_aw_upload_batch := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_upload_sessions'
    AND column_name = 'upload_batch_id'
);
SET @sql_aw_upload_batch := IF(
  @has_aw_upload_batch = 0,
  'ALTER TABLE asset_workbench_upload_sessions
     ADD COLUMN upload_batch_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER upload_directory_difficulty_class,
     ADD COLUMN relative_path VARCHAR(1024) NOT NULL DEFAULT '''' AFTER upload_batch_id,
     ADD COLUMN is_folder_upload TINYINT(1) NOT NULL DEFAULT 0 AFTER relative_path,
     ADD COLUMN expected_business_month CHAR(7) NOT NULL DEFAULT '''' AFTER is_folder_upload',
  'SELECT 1'
);
PREPARE stmt_aw_upload_batch FROM @sql_aw_upload_batch;
EXECUTE stmt_aw_upload_batch;
DEALLOCATE PREPARE stmt_aw_upload_batch;

SET @has_aw_file_batch := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND column_name = 'upload_batch_id'
);
SET @sql_aw_file_batch := IF(
  @has_aw_file_batch = 0,
  'ALTER TABLE asset_workbench_submission_files
     ADD COLUMN upload_batch_id VARCHAR(64) NOT NULL DEFAULT '''' AFTER upload_directory_difficulty_class,
     ADD COLUMN relative_path VARCHAR(1024) NOT NULL DEFAULT '''' AFTER upload_batch_id,
     ADD COLUMN display_name VARCHAR(255) NOT NULL DEFAULT '''' AFTER relative_path,
     ADD COLUMN is_folder_upload TINYINT(1) NOT NULL DEFAULT 0 AFTER display_name,
     ADD COLUMN deleted_at DATETIME NULL AFTER updated_at,
     ADD COLUMN deleted_by BIGINT NULL AFTER deleted_at,
     ADD COLUMN delete_reason VARCHAR(512) NOT NULL DEFAULT '''' AFTER deleted_by',
  'SELECT 1'
);
PREPARE stmt_aw_file_batch FROM @sql_aw_file_batch;
EXECUTE stmt_aw_file_batch;
DEALLOCATE PREPARE stmt_aw_file_batch;

UPDATE asset_workbench_submission_files
SET display_name = original_filename
WHERE display_name = '';

SET @idx_aw_files_active_dir := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'idx_aw_files_active_directory_created'
);
SET @sql_aw_files_active_dir := IF(
  @idx_aw_files_active_dir = 0,
  'ALTER TABLE asset_workbench_submission_files
     ADD KEY idx_aw_files_active_directory_created (deleted_at, upload_directory_id, created_at, id)',
  'SELECT 1'
);
PREPARE stmt_aw_files_active_dir FROM @sql_aw_files_active_dir;
EXECUTE stmt_aw_files_active_dir;
DEALLOCATE PREPARE stmt_aw_files_active_dir;

SET @idx_aw_files_batch := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'idx_aw_files_upload_batch'
);
SET @sql_aw_files_batch := IF(
  @idx_aw_files_batch = 0,
  'ALTER TABLE asset_workbench_submission_files
     ADD KEY idx_aw_files_upload_batch (upload_batch_id, relative_path(255))',
  'SELECT 1'
);
PREPARE stmt_aw_files_batch FROM @sql_aw_files_batch;
EXECUTE stmt_aw_files_batch;
DEALLOCATE PREPARE stmt_aw_files_batch;

SET @idx_aw_upload_batch := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_upload_sessions'
    AND index_name = 'idx_aw_upload_sessions_batch'
);
SET @sql_aw_upload_batch_idx := IF(
  @idx_aw_upload_batch = 0,
  'ALTER TABLE asset_workbench_upload_sessions
     ADD KEY idx_aw_upload_sessions_batch (upload_batch_id, owner_user_id, status)',
  'SELECT 1'
);
PREPARE stmt_aw_upload_batch_idx FROM @sql_aw_upload_batch_idx;
EXECUTE stmt_aw_upload_batch_idx;
DEALLOCATE PREPARE stmt_aw_upload_batch_idx;

SET @has_aw_files_fulltext := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'ft_aw_files_search_text'
);
SET @sql_aw_files_fulltext_drop := IF(
  @has_aw_files_fulltext > 0,
  'ALTER TABLE asset_workbench_submission_files DROP INDEX ft_aw_files_search_text',
  'SELECT 1'
);
PREPARE stmt_aw_files_fulltext_drop FROM @sql_aw_files_fulltext_drop;
EXECUTE stmt_aw_files_fulltext_drop;
DEALLOCATE PREPARE stmt_aw_files_fulltext_drop;

ALTER TABLE asset_workbench_submission_files
  ADD FULLTEXT KEY ft_aw_files_search_text (
    original_filename,
    display_name,
    relative_path,
    file_type,
    mime_type,
    upload_directory_name
  ) WITH PARSER ngram;

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_submission_files DROP INDEX ft_aw_files_search_text;
ALTER TABLE asset_workbench_submission_files ADD FULLTEXT KEY ft_aw_files_search_text (original_filename, file_type, mime_type, upload_directory_name) WITH PARSER ngram;
ALTER TABLE asset_workbench_upload_sessions DROP INDEX idx_aw_upload_sessions_batch;
ALTER TABLE asset_workbench_submission_files DROP INDEX idx_aw_files_upload_batch;
ALTER TABLE asset_workbench_submission_files DROP INDEX idx_aw_files_active_directory_created;
ALTER TABLE asset_workbench_submission_files
  DROP COLUMN delete_reason,
  DROP COLUMN deleted_by,
  DROP COLUMN deleted_at,
  DROP COLUMN is_folder_upload,
  DROP COLUMN display_name,
  DROP COLUMN relative_path,
  DROP COLUMN upload_batch_id;
ALTER TABLE asset_workbench_upload_sessions
  DROP COLUMN expected_business_month,
  DROP COLUMN is_folder_upload,
  DROP COLUMN relative_path,
  DROP COLUMN upload_batch_id;
-- ROLLBACK-END
