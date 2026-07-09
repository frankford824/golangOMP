-- Add durable async batch jobs for asset-workbench long-running operations.

CREATE TABLE IF NOT EXISTS asset_workbench_batch_jobs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  job_id VARCHAR(64) NOT NULL,
  job_type VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'queued',
  action VARCHAR(32) NOT NULL DEFAULT '',
  selection_scope VARCHAR(32) NOT NULL DEFAULT '',
  requested_by BIGINT NOT NULL,
  request_payload_json JSON NOT NULL,
  result_payload_json JSON NULL,
  total_count INT NOT NULL DEFAULT 0,
  processed_count INT NOT NULL DEFAULT 0,
  created_count INT NOT NULL DEFAULT 0,
  updated_count INT NOT NULL DEFAULT 0,
  enabled_count INT NOT NULL DEFAULT 0,
  disabled_count INT NOT NULL DEFAULT 0,
  removed_count INT NOT NULL DEFAULT 0,
  skipped_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  lease_owner VARCHAR(128) NOT NULL DEFAULT '',
  lease_expires_at DATETIME NULL,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_batch_jobs_job_id (job_id),
  KEY idx_aw_batch_jobs_status_lease (status, lease_expires_at, id),
  KEY idx_aw_batch_jobs_requester_created (requested_by, created_at),
  KEY idx_aw_batch_jobs_type_created (job_type, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS asset_workbench_batch_jobs;
-- ROLLBACK-END
