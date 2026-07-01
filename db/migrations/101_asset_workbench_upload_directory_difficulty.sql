ALTER TABLE asset_workbench_upload_directories
  ADD COLUMN difficulty_class VARCHAR(64) NOT NULL DEFAULT 'A' AFTER description,
  ADD KEY idx_aw_upload_directories_difficulty (enabled, difficulty_class, sort_order, id);

ALTER TABLE asset_workbench_upload_sessions
  ADD COLUMN upload_directory_difficulty_class VARCHAR(64) NOT NULL DEFAULT '' AFTER upload_directory_prefix;

ALTER TABLE asset_workbench_submission_files
  ADD COLUMN upload_directory_difficulty_class VARCHAR(64) NOT NULL DEFAULT '' AFTER upload_directory_prefix;
