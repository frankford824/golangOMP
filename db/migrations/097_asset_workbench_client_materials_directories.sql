CREATE TABLE IF NOT EXISTS asset_workbench_upload_directories (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  oss_prefix VARCHAR(255) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL,
  updated_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_upload_directories_prefix (oss_prefix),
  KEY idx_aw_upload_directories_enabled_sort (enabled, sort_order, id),
  CONSTRAINT fk_aw_upload_directories_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT fk_aw_upload_directories_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE asset_workbench_upload_sessions
  ADD COLUMN upload_directory_id BIGINT NULL AFTER owner_user_id,
  ADD COLUMN upload_directory_name VARCHAR(128) NOT NULL DEFAULT '' AFTER upload_directory_id,
  ADD COLUMN upload_directory_prefix VARCHAR(255) NOT NULL DEFAULT '' AFTER upload_directory_name,
  ADD KEY idx_aw_upload_sessions_directory (upload_directory_id),
  ADD CONSTRAINT fk_aw_upload_sessions_directory FOREIGN KEY (upload_directory_id) REFERENCES asset_workbench_upload_directories(id);

ALTER TABLE asset_workbench_submission_files
  ADD COLUMN upload_directory_id BIGINT NULL AFTER owner_user_id,
  ADD COLUMN upload_directory_name VARCHAR(128) NOT NULL DEFAULT '' AFTER upload_directory_id,
  ADD COLUMN upload_directory_prefix VARCHAR(255) NOT NULL DEFAULT '' AFTER upload_directory_name,
  ADD KEY idx_aw_files_directory (upload_directory_id),
  ADD CONSTRAINT fk_aw_files_directory FOREIGN KEY (upload_directory_id) REFERENCES asset_workbench_upload_directories(id);

CREATE TABLE IF NOT EXISTS asset_workbench_client_materials (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  asset_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL DEFAULT '',
  description VARCHAR(1024) NOT NULL DEFAULT '',
  filename_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  mime_type_snapshot VARCHAR(128) NOT NULL DEFAULT '',
  file_size_snapshot BIGINT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  published_by BIGINT NOT NULL,
  updated_by BIGINT NULL,
  published_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_client_materials_asset (asset_id),
  KEY idx_aw_client_materials_enabled_sort (enabled, sort_order, id),
  CONSTRAINT fk_aw_client_materials_published_by FOREIGN KEY (published_by) REFERENCES users(id),
  CONSTRAINT fk_aw_client_materials_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
