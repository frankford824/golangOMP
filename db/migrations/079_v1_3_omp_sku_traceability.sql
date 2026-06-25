-- Migration: 079_v1_3_omp_sku_traceability.sql
-- Add OMP-owned SKU traceability ledgers. These tables are append/merge
-- projections for SKUs touched by OMP, not replacements for task_details,
-- task_sku_items, integration_call_logs, or the 8082 jst_inventory cache.

CREATE TABLE IF NOT EXISTS omp_sku_records (
  sku_code VARCHAR(64) NOT NULL,
  sku_kind VARCHAR(32) NOT NULL DEFAULT 'ordinary' COMMENT 'ordinary, combo, unknown',
  first_task_id BIGINT NULL,
  last_task_id BIGINT NULL,
  first_task_sku_item_id BIGINT NULL,
  last_task_sku_item_id BIGINT NULL,
  source_mode VARCHAR(32) NOT NULL DEFAULT '',
  task_type VARCHAR(64) NOT NULL DEFAULT '',
  product_name VARCHAR(512) NOT NULL DEFAULT '',
  product_i_id VARCHAR(128) NOT NULL DEFAULT '',
  category_code VARCHAR(64) NOT NULL DEFAULT '',
  category_name VARCHAR(128) NOT NULL DEFAULT '',
  cost_price DECIMAL(12,2) NULL,
  estimated_cost DECIMAL(12,2) NULL,
  cost_rule_id BIGINT NULL,
  cost_rule_name VARCHAR(255) NOT NULL DEFAULT '',
  cost_rule_source VARCHAR(128) NOT NULL DEFAULT '',
  manual_cost_override TINYINT(1) NOT NULL DEFAULT 0,
  requires_manual_review TINYINT(1) NOT NULL DEFAULT 0,
  last_erp_sync_status VARCHAR(32) NOT NULL DEFAULT '',
  last_erp_call_log_id BIGINT NULL,
  created_by BIGINT NULL,
  last_operator_id BIGINT NULL,
  first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  trace_version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (sku_code),
  KEY idx_omp_sku_records_kind (sku_kind),
  KEY idx_omp_sku_records_last_task (last_task_id),
  KEY idx_omp_sku_records_iid (product_i_id),
  KEY idx_omp_sku_records_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OMP-owned SKU identity projection for traceability';

CREATE TABLE IF NOT EXISTS omp_sku_cost_snapshots (
  id BIGINT NOT NULL AUTO_INCREMENT,
  sku_code VARCHAR(64) NOT NULL,
  sku_kind VARCHAR(32) NOT NULL DEFAULT 'ordinary',
  task_id BIGINT NULL,
  task_sku_item_id BIGINT NULL,
  event_source VARCHAR(64) NOT NULL DEFAULT '',
  event_reason VARCHAR(128) NOT NULL DEFAULT '',
  operator_id BIGINT NULL,
  cost_price DECIMAL(12,2) NULL,
  cost_price_present TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 means OMP explicitly observed/calculated cost_price, including 0',
  estimated_cost DECIMAL(12,2) NULL,
  estimated_cost_present TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 means OMP explicitly observed/calculated estimated_cost, including 0',
  cost_rule_id BIGINT NULL,
  cost_rule_name VARCHAR(255) NOT NULL DEFAULT '',
  cost_rule_source VARCHAR(128) NOT NULL DEFAULT '',
  matched_rule_version INT NULL,
  prefill_source VARCHAR(64) NOT NULL DEFAULT '',
  requires_manual_review TINYINT(1) NOT NULL DEFAULT 0,
  manual_cost_override TINYINT(1) NOT NULL DEFAULT 0,
  manual_cost_override_reason VARCHAR(255) NOT NULL DEFAULT '',
  input_snapshot_json LONGTEXT NULL,
  calculation_snapshot_json LONGTEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_omp_sku_cost_snapshots_sku_time (sku_code, created_at),
  KEY idx_omp_sku_cost_snapshots_task (task_id, task_sku_item_id),
  KEY idx_omp_sku_cost_snapshots_source (event_source, event_reason)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Append-only OMP SKU cost calculation/override snapshots';

CREATE TABLE IF NOT EXISTS omp_sku_erp_trace_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  sku_code VARCHAR(64) NOT NULL,
  sku_kind VARCHAR(32) NOT NULL DEFAULT 'ordinary',
  task_id BIGINT NULL,
  task_sku_item_id BIGINT NULL,
  call_log_id BIGINT NULL,
  connector_key VARCHAR(64) NOT NULL DEFAULT '',
  operation_key VARCHAR(128) NOT NULL DEFAULT '',
  direction VARCHAR(16) NOT NULL DEFAULT 'outbound',
  status VARCHAR(32) NOT NULL DEFAULT '',
  request_cost_price DECIMAL(12,2) NULL,
  request_cost_price_present TINYINT(1) NOT NULL DEFAULT 0,
  response_cost_price DECIMAL(12,2) NULL,
  response_cost_price_present TINYINT(1) NOT NULL DEFAULT 0,
  request_payload_hash VARCHAR(128) NOT NULL DEFAULT '',
  request_payload_json LONGTEXT NULL,
  response_payload_json LONGTEXT NULL,
  error_message TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_omp_sku_erp_trace_sku_time (sku_code, created_at),
  KEY idx_omp_sku_erp_trace_task (task_id, task_sku_item_id),
  KEY idx_omp_sku_erp_trace_call_log (call_log_id),
  KEY idx_omp_sku_erp_trace_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SKU-indexed ERP request/response trace rows created by OMP';

CREATE TABLE IF NOT EXISTS omp_sku_combo_relations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  combo_sku_code VARCHAR(64) NOT NULL,
  child_sku_code VARCHAR(64) NOT NULL,
  quantity DECIMAL(12,4) NOT NULL DEFAULT 1,
  source VARCHAR(64) NOT NULL DEFAULT '',
  source_call_log_id BIGINT NULL,
  raw_payload_json LONGTEXT NULL,
  first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_omp_sku_combo_relation (combo_sku_code, child_sku_code, source),
  KEY idx_omp_sku_combo_child (child_sku_code),
  KEY idx_omp_sku_combo_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Combo SKU to ordinary SKU relationship ledger from OMP/JST readback';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS omp_sku_combo_relations;
DROP TABLE IF EXISTS omp_sku_erp_trace_logs;
DROP TABLE IF EXISTS omp_sku_cost_snapshots;
DROP TABLE IF EXISTS omp_sku_records;
-- ROLLBACK-END
