-- Netdisk (Drive) virtual directory aggregation relies on grouping/filtering
-- submission files by upload directory and (for non-admins) by owner. Add
-- covering indexes so directory/order aggregation and owner-scoped browsing
-- stay fast on large datasets.
--
-- Idempotent: some environments may already carry one of these indexes, so we
-- guard each ADD KEY with an information_schema existence check instead of a
-- plain ALTER (MySQL has no ADD KEY IF NOT EXISTS).

SET @idx_owner_dir := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'idx_aw_files_owner_directory'
);
SET @sql_owner_dir := IF(
  @idx_owner_dir = 0,
  'ALTER TABLE asset_workbench_submission_files ADD KEY idx_aw_files_owner_directory (owner_user_id, upload_directory_id)',
  'SELECT 1'
);
PREPARE stmt_owner_dir FROM @sql_owner_dir;
EXECUTE stmt_owner_dir;
DEALLOCATE PREPARE stmt_owner_dir;

SET @idx_dir := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'asset_workbench_submission_files'
    AND index_name = 'idx_aw_files_directory'
);
SET @sql_dir := IF(
  @idx_dir = 0,
  'ALTER TABLE asset_workbench_submission_files ADD KEY idx_aw_files_directory (upload_directory_id)',
  'SELECT 1'
);
PREPARE stmt_dir FROM @sql_dir;
EXECUTE stmt_dir;
DEALLOCATE PREPARE stmt_dir;
