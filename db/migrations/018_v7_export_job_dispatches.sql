-- Migration: 018_v7_export_job_dispatches.sql
-- Adds placeholder adapter-dispatch handoff persistence for export jobs.

CREATE TABLE export_job_dispatches (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  dispatch_id VARCHAR(64) NOT NULL,
  export_job_id BIGINT NOT NULL,
  dispatch_no INT NOT NULL,
  trigger_source VARCHAR(64) NOT NULL DEFAULT '',
  execution_mode VARCHAR(64) NOT NULL,
  adapter_key VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  submitted_at DATETIME NOT NULL,
  received_at DATETIME NULL,
  finished_at DATETIME NULL,
  expires_at DATETIME NULL,
  status_reason VARCHAR(255) NOT NULL DEFAULT '',
  adapter_note VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_export_job_dispatches_dispatch_id (dispatch_id),
  UNIQUE KEY uq_export_job_dispatches_job_dispatch_no (export_job_id, dispatch_no),
  KEY idx_export_job_dispatches_job_status (export_job_id, status),
  KEY idx_export_job_dispatches_job_submitted_at (export_job_id, submitted_at),
  CONSTRAINT fk_export_job_dispatches_job FOREIGN KEY (export_job_id) REFERENCES export_jobs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Export job adapter-dispatch handoff records for placeholder scheduler boundary';

ALTER TABLE export_job_attempts
  ADD COLUMN dispatch_id VARCHAR(64) NULL AFTER export_job_id,
  ADD KEY idx_export_job_attempts_dispatch_id (dispatch_id),
  ADD CONSTRAINT fk_export_job_attempts_dispatch FOREIGN KEY (dispatch_id) REFERENCES export_job_dispatches (dispatch_id);
