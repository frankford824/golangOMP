-- Migration: 005_v7_erp_sync_runs.sql
-- Step 06 ERP sync placeholder: stores ERP sync execution history.

CREATE TABLE IF NOT EXISTS erp_sync_runs (
  id             BIGINT       NOT NULL AUTO_INCREMENT,
  trigger_mode   VARCHAR(32)  NOT NULL COMMENT 'manual | scheduled',
  source_mode    VARCHAR(32)  NOT NULL COMMENT 'stub',
  status         VARCHAR(32)  NOT NULL COMMENT 'success | noop | failed',
  total_received BIGINT       NOT NULL DEFAULT 0,
  total_upserted BIGINT       NOT NULL DEFAULT 0,
  error_message  TEXT         NULL,
  started_at     DATETIME     NOT NULL,
  finished_at    DATETIME     NOT NULL,
  created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_erp_sync_runs_created_at (created_at),
  KEY idx_erp_sync_runs_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ERP sync placeholder run history';
