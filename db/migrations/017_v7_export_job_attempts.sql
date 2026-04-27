-- Migration: 017_v7_export_job_attempts.sql
-- Adds placeholder execution-attempt visibility for export jobs.

CREATE TABLE export_job_attempts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  attempt_id VARCHAR(64) NOT NULL,
  export_job_id BIGINT NOT NULL,
  attempt_no INT NOT NULL,
  trigger_source VARCHAR(64) NOT NULL DEFAULT '',
  execution_mode VARCHAR(64) NOT NULL,
  adapter_key VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  started_at DATETIME NOT NULL,
  finished_at DATETIME NULL,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  adapter_note VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_export_job_attempts_attempt_id (attempt_id),
  UNIQUE KEY uq_export_job_attempts_job_attempt_no (export_job_id, attempt_no),
  KEY idx_export_job_attempts_job_created_at (export_job_id, created_at),
  KEY idx_export_job_attempts_job_status (export_job_id, status),
  CONSTRAINT fk_export_job_attempts_job FOREIGN KEY (export_job_id) REFERENCES export_jobs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Export job execution attempts over the placeholder runner-adapter boundary';
