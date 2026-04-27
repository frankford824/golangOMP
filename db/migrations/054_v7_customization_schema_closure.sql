-- Migration: 054_v7_customization_schema_closure.sql
-- Purpose: lock customization MVP schema truth and dual pricing consistency.
-- NOTE: MySQL 8.x does not support `ADD COLUMN IF NOT EXISTS`; operator must ensure idempotency by checking `SHOW COLUMNS` before running on a live DB.

ALTER TABLE tasks
  ADD COLUMN customization_required TINYINT(1) NOT NULL DEFAULT 0 AFTER is_outsource,
  ADD COLUMN customization_source_type VARCHAR(32) NOT NULL DEFAULT '' AFTER customization_required,
  ADD COLUMN last_customization_operator_id BIGINT NULL AFTER customization_source_type,
  ADD COLUMN warehouse_reject_reason VARCHAR(255) NOT NULL DEFAULT '' AFTER last_customization_operator_id,
  ADD COLUMN warehouse_reject_category VARCHAR(64) NOT NULL DEFAULT '' AFTER warehouse_reject_reason;

CREATE INDEX IF NOT EXISTS idx_tasks_customization_status ON tasks (customization_required, task_status);
CREATE INDEX IF NOT EXISTS idx_tasks_last_customization_operator ON tasks (last_customization_operator_id);

CREATE TABLE IF NOT EXISTS customization_jobs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  source_asset_id BIGINT NULL,
  current_asset_id BIGINT NULL,
  customization_level_code VARCHAR(64) NOT NULL DEFAULT '',
  customization_level_name VARCHAR(128) NOT NULL DEFAULT '',
  unit_price DECIMAL(12,2) NULL,
  weight_factor DECIMAL(10,4) NULL,
  note TEXT NOT NULL,
  customization_review_decision VARCHAR(32) NOT NULL DEFAULT 'approved',
  decision_type VARCHAR(32) NOT NULL DEFAULT 'final',
  assigned_operator_id BIGINT NULL,
  last_operator_id BIGINT NULL,
  pricing_worker_type VARCHAR(32) NULL COMMENT 'pricing snapshot worker type: full_time | part_time',
  status VARCHAR(64) NOT NULL DEFAULT 'pending_customization_production',
  warehouse_reject_reason VARCHAR(255) NOT NULL DEFAULT '',
  warehouse_reject_category VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_customization_jobs_task_id (task_id),
  KEY idx_customization_jobs_status (status),
  KEY idx_customization_jobs_last_operator (last_operator_id),
  KEY idx_customization_jobs_assigned_operator (assigned_operator_id),
  CONSTRAINT fk_customization_jobs_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Customization workflow job records';

ALTER TABLE customization_jobs
  ADD COLUMN pricing_worker_type VARCHAR(32) NULL COMMENT 'pricing snapshot worker type: full_time | part_time' AFTER last_operator_id;

ALTER TABLE users
  ADD COLUMN employment_type VARCHAR(32) NOT NULL DEFAULT 'full_time' COMMENT 'full_time | part_time';

CREATE TABLE IF NOT EXISTS customization_pricing_rules (
  id BIGINT NOT NULL AUTO_INCREMENT,
  customization_level_code VARCHAR(64) NOT NULL,
  employment_type VARCHAR(32) NOT NULL COMMENT 'full_time | part_time',
  unit_price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  weight_factor DECIMAL(10,4) NOT NULL DEFAULT 1.0000,
  is_enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_customization_pricing_rules_level_worker (customization_level_code, employment_type),
  KEY idx_customization_pricing_rules_enabled (is_enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Customization dual piece-rate pricing rules by level and employment type';
