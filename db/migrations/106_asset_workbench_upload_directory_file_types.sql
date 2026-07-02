-- Add optional per-upload-directory file type restrictions.
-- Empty or NULL allowed_file_types_json means the directory accepts all files.

SET @has_aw_upload_directory_file_types := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_upload_directories'
    AND column_name = 'allowed_file_types_json'
);
SET @sql_aw_upload_directory_file_types := IF(
  @has_aw_upload_directory_file_types = 0,
  'ALTER TABLE asset_workbench_upload_directories ADD COLUMN allowed_file_types_json JSON NULL AFTER difficulty_class',
  'SELECT 1'
);
PREPARE stmt_aw_upload_directory_file_types FROM @sql_aw_upload_directory_file_types;
EXECUTE stmt_aw_upload_directory_file_types;
DEALLOCATE PREPARE stmt_aw_upload_directory_file_types;

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_upload_directories DROP COLUMN allowed_file_types_json;
-- ROLLBACK-END
