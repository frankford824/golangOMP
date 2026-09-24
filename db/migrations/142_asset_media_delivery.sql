-- Additive, asset-only durable processing and NAS source reconciliation.
CREATE TABLE IF NOT EXISTS asset_media_jobs (
  job_id CHAR(36) NOT NULL,
  dedupe_key CHAR(64) NOT NULL,
  kind VARCHAR(48) NOT NULL,
  pool VARCHAR(16) NOT NULL,
  resource_id VARCHAR(128) NOT NULL,
  source_version VARCHAR(128) NOT NULL,
  recipe VARCHAR(64) NOT NULL,
  state VARCHAR(24) NOT NULL DEFAULT 'queued',
  phase VARCHAR(32) NOT NULL DEFAULT 'queued',
  priority INT NOT NULL DEFAULT 20,
  attempts INT NOT NULL DEFAULT 0,
  next_run_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  lease_owner VARCHAR(128) NOT NULL DEFAULT '',
  lease_epoch BIGINT NOT NULL DEFAULT 0,
  lease_expires_at DATETIME(6) NULL,
  processed_bytes BIGINT NOT NULL DEFAULT 0,
  total_bytes BIGINT NOT NULL DEFAULT 0,
  payload JSON NOT NULL,
  checkpoint JSON NULL,
  result JSON NULL,
  error_code VARCHAR(128) NOT NULL DEFAULT '',
  retryable TINYINT(1) NOT NULL DEFAULT 0,
  manual_retry_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (job_id),
  UNIQUE KEY uq_asset_media_dedupe (dedupe_key),
  KEY idx_asset_media_claim (pool, state, next_run_at, priority),
  KEY idx_asset_media_lease (state, lease_expires_at),
  KEY idx_asset_media_resource (resource_id, source_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS asset_media_requests (
  request_id CHAR(36) NOT NULL,
  job_id CHAR(36) NOT NULL,
  actor_id BIGINT NOT NULL,
  access_reference JSON NULL,
  cancelled TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (request_id),
  UNIQUE KEY uq_asset_media_request (job_id, actor_id),
  KEY idx_asset_media_request_actor (actor_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE external_asset_source_fingerprints
  ADD COLUMN modified_ns BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN changed_ns BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN file_identity VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN root_identity VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN source_version CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN sha256 CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN checksum_state VARCHAR(24) NOT NULL DEFAULT 'unverified',
  ADD COLUMN original_source_version CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN preview_source_version CHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN media_result JSON NULL,
  ADD COLUMN event_agent VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN event_epoch VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN event_seq BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN scan_id CHAR(36) NOT NULL DEFAULT '';

ALTER TABLE external_asset_sync_runs
  ADD COLUMN root_generation CHAR(36) NOT NULL DEFAULT '',
  ADD COLUMN generation_shards INT NOT NULL DEFAULT 0,
  ADD COLUMN scan_id CHAR(36) NULL,
  ADD COLUMN agent_id VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN agent_epoch VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN root_identity VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN start_sequence BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN end_sequence BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN shard_index INT NOT NULL DEFAULT 0,
  ADD COLUMN shard_count INT NOT NULL DEFAULT 1,
  ADD COLUMN expected_parts INT NOT NULL DEFAULT 0,
  ADD COLUMN expected_files BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN manifest_sha256 CHAR(64) NOT NULL DEFAULT '',
  ADD UNIQUE KEY uq_external_scan_id (scan_id),
  ADD KEY idx_external_scan_generation (root_generation,status);

CREATE TABLE IF NOT EXISTS external_asset_scan_parts (
  scan_id CHAR(36) NOT NULL,
  part_no INT NOT NULL,
  sha256 CHAR(64) NOT NULL,
  item_count INT NOT NULL,
  payload JSON NOT NULL,
  applied TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (scan_id, part_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Rollback is feature-flag based. Retain durable jobs, source evidence and data.
