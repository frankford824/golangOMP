-- Migration: 113_external_asset_source_modified_at.sql
-- Track source file modified time from AList/NAS scans in a side table so OSS
-- copies can be re-queued when a local file is replaced in place without
-- changing its path. This avoids copying the large FULLTEXT-indexed
-- external_asset_records table.

CREATE TABLE IF NOT EXISTS external_asset_source_fingerprints (
  origin_path_hash CHAR(64) NOT NULL,
  file_size BIGINT NOT NULL DEFAULT 0,
  source_modified_at DATETIME NULL,
  last_scanned_at DATETIME NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (origin_path_hash),
  KEY idx_external_asset_source_fp_scanned (last_scanned_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='External asset source fingerprint snapshots for OSS realignment';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS external_asset_source_fingerprints;
-- ROLLBACK-END
