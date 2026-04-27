-- Migration: 028_v7_runtime_distribution_event_tables.sql
-- Fills the runtime-schema gap left by the additive V7 pack by creating the
-- legacy SKU-scoped event/distribution tables still required by the binary.
-- These tables intentionally avoid new foreign keys to older V6 tables because
-- the repository does not currently ship the full V6 baseline migration set.

CREATE TABLE IF NOT EXISTS sku_sequences (
  sku_id         BIGINT       NOT NULL,
  last_sequence  BIGINT       NOT NULL DEFAULT 0,
  PRIMARY KEY (sku_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Per-SKU event sequence counter for legacy event_logs';

CREATE TABLE IF NOT EXISTS event_logs (
  id          VARCHAR(36)  NOT NULL COMMENT 'UUID',
  sku_id       BIGINT       NOT NULL,
  sequence     BIGINT       NOT NULL COMMENT 'Monotonically increasing per sku_id',
  event_type   VARCHAR(128) NOT NULL,
  payload      JSON         NOT NULL DEFAULT (JSON_OBJECT()),
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_event_logs_sku_sequence (sku_id, sequence),
  KEY idx_event_logs_created_at_id (created_at, id),
  KEY idx_event_logs_event_type (event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Legacy SKU-scoped runtime event log';

CREATE TABLE IF NOT EXISTS distribution_jobs (
  id                 BIGINT       NOT NULL AUTO_INCREMENT,
  idempotent_key     VARCHAR(255) NOT NULL COMMENT 'action_id + target uniqueness key',
  action_id          VARCHAR(128) NOT NULL,
  sku_id             BIGINT       NOT NULL,
  asset_ver_id       BIGINT       NOT NULL,
  target             VARCHAR(128) NOT NULL,
  status             VARCHAR(32)  NOT NULL DEFAULT 'PendingVerify',
  verify_status      VARCHAR(32)  NOT NULL DEFAULT 'NotRequested',
  retry_count        INT          NOT NULL DEFAULT 0,
  max_retries        INT          NOT NULL DEFAULT 3,
  current_attempt_id VARCHAR(36)  NULL,
  next_retry_at      DATETIME     NULL,
  created_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_distribution_jobs_idempotent_key (idempotent_key),
  KEY idx_distribution_jobs_sku_id (sku_id),
  KEY idx_distribution_jobs_status_created_at (status, created_at),
  KEY idx_distribution_jobs_status_retry (status, next_retry_at),
  KEY idx_distribution_jobs_current_attempt_id (current_attempt_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Legacy distribution job queue required by workers';

CREATE TABLE IF NOT EXISTS job_attempts (
  id                VARCHAR(36)  NOT NULL COMMENT 'UUID',
  job_id            BIGINT       NOT NULL,
  agent_id          VARCHAR(128) NOT NULL,
  lease_expires_at  DATETIME     NOT NULL,
  heartbeat_at      DATETIME     NULL,
  acked_at          DATETIME     NULL,
  created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_job_attempts_job_id (job_id),
  KEY idx_job_attempts_lease_expires_at (lease_expires_at),
  CONSTRAINT fk_job_attempts_distribution_job FOREIGN KEY (job_id) REFERENCES distribution_jobs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Execution attempts for legacy distribution jobs';
