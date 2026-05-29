-- Migration: 081_v1_3_external_asset_index.sql
-- Store searchable references to AList-backed external resources. This keeps
-- external/NAS resources separate from system OSS task assets while allowing
-- asset-center and global search to surface them.

CREATE TABLE IF NOT EXISTS external_asset_records (
  id BIGINT NOT NULL AUTO_INCREMENT,
  provider VARCHAR(32) NOT NULL DEFAULT 'alist',
  kind VARCHAR(32) NOT NULL DEFAULT 'netdisk' COMMENT 'netdisk, nas_local',
  driver VARCHAR(64) NOT NULL DEFAULT '',
  mount_path VARCHAR(255) NOT NULL DEFAULT '',
  origin_path_hash CHAR(64) NOT NULL,
  origin_path TEXT NOT NULL,
  parent_path TEXT NULL,
  file_name VARCHAR(1024) NOT NULL DEFAULT '',
  file_ext VARCHAR(32) NOT NULL DEFAULT '',
  mime_type VARCHAR(255) NOT NULL DEFAULT '',
  file_size BIGINT NOT NULL DEFAULT 0,
  is_dir TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'indexed',
  raw_url TEXT NULL,
  raw_url_expires_at DATETIME NULL,
  direct_url_status VARCHAR(32) NOT NULL DEFAULT '',
  oss_original_key VARCHAR(1024) NOT NULL DEFAULT '',
  oss_preview_key VARCHAR(1024) NOT NULL DEFAULT '',
  oss_thumb_key VARCHAR(1024) NOT NULL DEFAULT '',
  oss_sync_status VARCHAR(32) NOT NULL DEFAULT 'none',
  preview_status VARCHAR(32) NOT NULL DEFAULT 'none',
  last_seen_at DATETIME NULL,
  last_scanned_at DATETIME NULL,
  last_link_checked_at DATETIME NULL,
  last_prepare_error TEXT NULL,
  searchable_text MEDIUMTEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_external_asset_origin (origin_path_hash),
  KEY idx_external_asset_kind_mount (kind, mount_path),
  KEY idx_external_asset_status (status, oss_sync_status, preview_status),
  KEY idx_external_asset_updated_at (updated_at),
  KEY idx_external_asset_file_ext (file_ext)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='External AList/NAS resource index for search and controlled download routing';

CREATE TABLE IF NOT EXISTS external_asset_sync_runs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  run_type VARCHAR(32) NOT NULL DEFAULT '',
  mount_path VARCHAR(255) NOT NULL DEFAULT '',
  keyword VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'running',
  scanned_count INT NOT NULL DEFAULT 0,
  upserted_count INT NOT NULL DEFAULT 0,
  error_message TEXT NULL,
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at DATETIME NULL,
  PRIMARY KEY (id),
  KEY idx_external_asset_sync_runs_started (started_at),
  KEY idx_external_asset_sync_runs_mount (mount_path, run_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='External asset sync/search run audit';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS external_asset_sync_runs;
DROP TABLE IF EXISTS external_asset_records;
-- ROLLBACK-END
