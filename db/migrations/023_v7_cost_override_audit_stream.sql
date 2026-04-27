-- Migration: 023_v7_cost_override_audit_stream.sql
-- Adds a dedicated governance audit stream for task-side cost overrides.

CREATE TABLE IF NOT EXISTS cost_override_event_sequences (
  task_id BIGINT NOT NULL,
  last_sequence BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (task_id),
  CONSTRAINT fk_cost_override_event_sequences_task
    FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cost_override_events (
  event_id VARCHAR(64) NOT NULL,
  task_id BIGINT NOT NULL,
  task_detail_id BIGINT NULL,
  sequence BIGINT NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  category_code VARCHAR(64) NOT NULL DEFAULT '',
  matched_rule_id BIGINT NULL,
  matched_rule_version INT NULL,
  matched_rule_source VARCHAR(128) NOT NULL DEFAULT '',
  governance_status VARCHAR(32) NOT NULL DEFAULT 'no_match',
  previous_estimated_cost DECIMAL(10,2) NULL,
  previous_cost_price DECIMAL(10,2) NULL,
  override_cost DECIMAL(10,2) NULL,
  result_cost_price DECIMAL(10,2) NULL,
  override_reason VARCHAR(255) NOT NULL DEFAULT '',
  override_actor VARCHAR(128) NOT NULL DEFAULT '',
  override_at DATETIME NOT NULL,
  source VARCHAR(128) NOT NULL DEFAULT '',
  note VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  PRIMARY KEY (event_id),
  UNIQUE KEY uk_cost_override_events_task_sequence (task_id, sequence),
  KEY idx_cost_override_events_task_id (task_id),
  KEY idx_cost_override_events_override_at (override_at),
  KEY idx_cost_override_events_rule (matched_rule_id, matched_rule_version),
  CONSTRAINT fk_cost_override_events_task
    FOREIGN KEY (task_id) REFERENCES tasks (id),
  CONSTRAINT fk_cost_override_events_task_detail
    FOREIGN KEY (task_detail_id) REFERENCES task_details (id),
  CONSTRAINT fk_cost_override_events_rule
    FOREIGN KEY (matched_rule_id) REFERENCES cost_rules (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
