-- Add a targeted browse index for asset-workbench external material folders.
-- The material browser filters by mount_path/is_dir and navigates by
-- parent_path prefix/equality. parent_path is TEXT, so use a bounded prefix
-- that stays below InnoDB's utf8mb4 key length while covering normal folders.

CREATE TABLE IF NOT EXISTS external_asset_directory_index (
  id BIGINT NOT NULL AUTO_INCREMENT,
  provider VARCHAR(32) NOT NULL DEFAULT 'alist',
  kind VARCHAR(32) NOT NULL DEFAULT 'netdisk',
  driver VARCHAR(64) NOT NULL DEFAULT '',
  mount_path VARCHAR(255) NOT NULL DEFAULT '',
  path_hash CHAR(64) NOT NULL,
  parent_path_hash CHAR(64) NOT NULL,
  path TEXT NOT NULL,
  parent_path TEXT NULL,
  name VARCHAR(1024) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'indexed',
  descendant_file_count BIGINT NOT NULL DEFAULT 0,
  direct_file_count BIGINT NOT NULL DEFAULT 0,
  last_seen_at DATETIME NULL,
  last_scanned_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_external_asset_directory_path (path_hash),
  KEY idx_external_asset_directory_parent (mount_path, parent_path_hash, status, name(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Derived external asset directory index for material browsing';

SET @has_external_browse_parent := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE table_schema = DATABASE()
    AND table_name = 'external_asset_records'
    AND index_name = 'idx_external_asset_browse_parent'
);
SET @sql_external_browse_parent := IF(
  @has_external_browse_parent = 0,
  'ALTER TABLE external_asset_records ADD KEY idx_external_asset_browse_parent (mount_path, is_dir, parent_path(480))',
  'SELECT 1'
);
PREPARE stmt_external_browse_parent FROM @sql_external_browse_parent;
EXECUTE stmt_external_browse_parent;
DEALLOCATE PREPARE stmt_external_browse_parent;

SET SESSION cte_max_recursion_depth = 100;

INSERT INTO external_asset_directory_index (
  path_hash, parent_path_hash, provider, kind, driver, mount_path, path, parent_path, name,
  status, descendant_file_count, direct_file_count, last_seen_at, last_scanned_at
)
WITH RECURSIVE file_dirs AS (
  SELECT provider, kind, driver, mount_path, parent_path AS path, parent_path AS direct_parent_path,
         last_seen_at, last_scanned_at
    FROM external_asset_records
   WHERE status <> 'missing'
     AND is_dir = 0
     AND parent_path <> ''
     AND parent_path <> '/'
     AND parent_path NOT LIKE '%/@eaDir/%'
     AND parent_path NOT LIKE '%/#recycle/%'
     AND file_name NOT LIKE '%@Syno%'
  UNION ALL
  SELECT provider, kind, driver, mount_path,
         CASE
           WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
           ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
         END AS path,
         direct_parent_path, last_seen_at, last_scanned_at
    FROM file_dirs
   WHERE path <> ''
     AND (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) > 1
)
SELECT SHA2(CONCAT(LOWER(TRIM(provider)), '|', mount_path, '|', path), 256) AS path_hash,
       SHA2(CASE
         WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
         ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
       END, 256) AS parent_path_hash,
       provider,
       kind,
       driver,
       mount_path,
       path,
       CASE
         WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
         ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
       END AS parent_path,
       SUBSTRING_INDEX(path, '/', -1) AS name,
       'indexed' AS status,
       COUNT(*) AS descendant_file_count,
       SUM(path = direct_parent_path) AS direct_file_count,
       MAX(last_seen_at) AS last_seen_at,
       MAX(last_scanned_at) AS last_scanned_at
  FROM file_dirs
 WHERE path <> ''
 GROUP BY provider, kind, driver, mount_path, path
ON DUPLICATE KEY UPDATE
  kind = VALUES(kind),
  driver = VALUES(driver),
  parent_path_hash = VALUES(parent_path_hash),
  parent_path = VALUES(parent_path),
  name = VALUES(name),
  status = VALUES(status),
  descendant_file_count = VALUES(descendant_file_count),
  direct_file_count = VALUES(direct_file_count),
  last_seen_at = VALUES(last_seen_at),
  last_scanned_at = VALUES(last_scanned_at);

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS external_asset_directory_index;
ALTER TABLE external_asset_records DROP INDEX idx_external_asset_browse_parent;
-- ROLLBACK-END
