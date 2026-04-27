-- Migration: 024_v7_cost_override_placeholder_boundaries.sql
-- Adds approval/finance placeholder boundaries above the dedicated cost override audit stream.

CREATE TABLE IF NOT EXISTS cost_override_reviews (
  record_id BIGINT NOT NULL AUTO_INCREMENT,
  override_event_id VARCHAR(64) NOT NULL,
  task_id BIGINT NOT NULL,
  review_required TINYINT(1) NOT NULL DEFAULT 1,
  review_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  review_note VARCHAR(255) NOT NULL DEFAULT '',
  review_actor VARCHAR(128) NOT NULL DEFAULT '',
  reviewed_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (record_id),
  UNIQUE KEY uk_cost_override_reviews_event (override_event_id),
  KEY idx_cost_override_reviews_task_id (task_id),
  CONSTRAINT fk_cost_override_reviews_event
    FOREIGN KEY (override_event_id) REFERENCES cost_override_events (event_id),
  CONSTRAINT fk_cost_override_reviews_task
    FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cost_override_finance_flags (
  record_id BIGINT NOT NULL AUTO_INCREMENT,
  override_event_id VARCHAR(64) NOT NULL,
  task_id BIGINT NOT NULL,
  finance_required TINYINT(1) NOT NULL DEFAULT 1,
  finance_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  finance_note VARCHAR(255) NOT NULL DEFAULT '',
  finance_marked_by VARCHAR(128) NOT NULL DEFAULT '',
  finance_marked_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (record_id),
  UNIQUE KEY uk_cost_override_finance_flags_event (override_event_id),
  KEY idx_cost_override_finance_flags_task_id (task_id),
  CONSTRAINT fk_cost_override_finance_flags_event
    FOREIGN KEY (override_event_id) REFERENCES cost_override_events (event_id),
  CONSTRAINT fk_cost_override_finance_flags_task
    FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
