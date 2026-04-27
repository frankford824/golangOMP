-- Migration: 021_v7_integration_execution_boundary.sql
-- Adds placeholder integration execution persistence beneath integration call logs.

CREATE TABLE IF NOT EXISTS integration_call_executions (
  execution_id VARCHAR(64) PRIMARY KEY,
  call_log_id BIGINT NOT NULL,
  connector_key VARCHAR(64) NOT NULL,
  execution_no INT NOT NULL,
  execution_mode VARCHAR(32) NOT NULL,
  trigger_source VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  status_updated_at DATETIME NOT NULL,
  started_at DATETIME NOT NULL,
  finished_at DATETIME NULL,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  adapter_note VARCHAR(255) NOT NULL DEFAULT '',
  retryable TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_integration_call_executions_call_log_no (call_log_id, execution_no),
  KEY idx_integration_call_executions_call_log (call_log_id),
  KEY idx_integration_call_executions_call_log_status (call_log_id, status),
  KEY idx_integration_call_executions_connector_status (connector_key, status),
  CONSTRAINT fk_integration_call_executions_call_log FOREIGN KEY (call_log_id) REFERENCES integration_call_logs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Placeholder integration execution attempts beneath integration call logs';
