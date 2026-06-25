-- Migration: 082_v1_4_workflow_trace_events.sql
-- Lightweight full-chain event ledger for API, frontend, task, SKU, asset, and integration traceability.

CREATE TABLE IF NOT EXISTS workflow_trace_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_id VARCHAR(36) NOT NULL,
  trace_id VARCHAR(64) NOT NULL DEFAULT '',
  event_source VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'api | frontend | system | integration',
  event_type VARCHAR(64) NOT NULL,
  action VARCHAR(128) NOT NULL DEFAULT '',

  actor_id BIGINT NULL,
  actor_username VARCHAR(64) NOT NULL DEFAULT '',
  actor_source VARCHAR(64) NOT NULL DEFAULT '',
  actor_auth_mode VARCHAR(64) NOT NULL DEFAULT '',
  actor_roles_json JSON NOT NULL DEFAULT (JSON_ARRAY()),
  actor_department VARCHAR(128) NOT NULL DEFAULT '',
  actor_team VARCHAR(128) NOT NULL DEFAULT '',

  route_method VARCHAR(16) NOT NULL DEFAULT '',
  route_path VARCHAR(255) NOT NULL DEFAULT '',
  route_full_path VARCHAR(512) NOT NULL DEFAULT '',
  http_status INT NULL,
  latency_ms BIGINT NULL,
  client_ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(512) NOT NULL DEFAULT '',

  page_url VARCHAR(512) NOT NULL DEFAULT '',
  page_name VARCHAR(128) NOT NULL DEFAULT '',
  component_id VARCHAR(128) NOT NULL DEFAULT '',

  task_id BIGINT NULL,
  task_module_id BIGINT NULL,
  module_key VARCHAR(32) NOT NULL DEFAULT '',
  sku_code VARCHAR(64) NOT NULL DEFAULT '',
  task_sku_item_id BIGINT NULL,
  asset_id BIGINT NULL,
  design_asset_id BIGINT NULL,
  task_asset_id BIGINT NULL,
  integration_call_log_id BIGINT NULL,

  resource_type VARCHAR(64) NOT NULL DEFAULT '',
  resource_id VARCHAR(128) NOT NULL DEFAULT '',
  outcome VARCHAR(32) NOT NULL DEFAULT '',
  payload_json JSON NULL,
  occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uq_workflow_trace_events_event_id (event_id),
  KEY idx_workflow_trace_trace_id (trace_id),
  KEY idx_workflow_trace_actor_time (actor_id, occurred_at),
  KEY idx_workflow_trace_department_time (actor_department, occurred_at),
  KEY idx_workflow_trace_type_time (event_type, occurred_at),
  KEY idx_workflow_trace_source_time (event_source, occurred_at),
  KEY idx_workflow_trace_task_time (task_id, occurred_at),
  KEY idx_workflow_trace_sku_time (sku_code, occurred_at),
  KEY idx_workflow_trace_asset_time (asset_id, occurred_at),
  KEY idx_workflow_trace_route_time (route_path, occurred_at),
  KEY idx_workflow_trace_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Unified lightweight workflow trace events for audit and AI context';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS workflow_trace_events;
-- ROLLBACK-END
