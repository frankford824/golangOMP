-- Migration: 019_v7_integration_call_logs.sql
-- Adds placeholder integration-center API call log persistence.

CREATE TABLE integration_call_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  connector_key VARCHAR(64) NOT NULL,
  operation_key VARCHAR(128) NOT NULL,
  direction VARCHAR(16) NOT NULL,
  resource_type VARCHAR(64) NOT NULL DEFAULT '',
  resource_id BIGINT NULL,
  status VARCHAR(32) NOT NULL,
  requested_by_actor_id BIGINT NOT NULL,
  requested_by_roles_json JSON NULL,
  requested_by_source VARCHAR(64) NOT NULL DEFAULT '',
  requested_by_auth_mode VARCHAR(32) NOT NULL DEFAULT '',
  request_payload_json JSON NULL,
  response_payload_json JSON NULL,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  status_updated_at DATETIME NOT NULL,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_integration_call_logs_connector_status (connector_key, status),
  KEY idx_integration_call_logs_resource (resource_type, resource_id),
  KEY idx_integration_call_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Placeholder integration center API call logs';
