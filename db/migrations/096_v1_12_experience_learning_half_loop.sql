-- Migration: 096_v1_12_experience_learning_half_loop.sql
-- Stable-first experience capture and observation half-loop.
-- All tables are side-channel only: no foreign keys to core task/asset/ERP tables, and no business state reads from these tables.

CREATE TABLE IF NOT EXISTS experience_reason_tags (
  id BIGINT NOT NULL AUTO_INCREMENT,
  scene VARCHAR(64) NOT NULL,
  code VARCHAR(96) NOT NULL,
  name VARCHAR(128) NOT NULL,
  tag_group VARCHAR(64) NOT NULL DEFAULT '',
  severity VARCHAR(32) NOT NULL DEFAULT '',
  version INT NOT NULL DEFAULT 1,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  deleted_at DATETIME NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_by BIGINT NULL,
  updated_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_reason_tags_scene_code_version (scene, code, version),
  KEY idx_experience_reason_tags_scene_enabled (scene, enabled, deleted_at, sort_order),
  KEY idx_experience_reason_tags_group (tag_group, enabled, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Controlled versioned reason tag dictionary for experience learning';

CREATE TABLE IF NOT EXISTS experience_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_key VARCHAR(191) NOT NULL,
  schema_version INT NOT NULL DEFAULT 1,
  source_type VARCHAR(64) NOT NULL,
  source_id VARCHAR(128) NOT NULL DEFAULT '',
  task_id BIGINT NULL,
  action VARCHAR(96) NOT NULL,
  outcome VARCHAR(64) NOT NULL DEFAULT '',
  event_time DATETIME NOT NULL,
  actor_snapshot_json JSON NULL,
  business_snapshot_json JSON NULL,
  payload_json JSON NULL,
  data_classification VARCHAR(32) NOT NULL DEFAULT 'business_fact',
  ground_truth_status VARCHAR(32) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'queued',
  attempt_count INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  next_retry_at DATETIME NULL,
  claimed_by VARCHAR(128) NOT NULL DEFAULT '',
  claimed_at DATETIME NULL,
  processed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_outbox_event_key (event_key),
  KEY idx_experience_outbox_status_retry (status, next_retry_at, id),
  KEY idx_experience_outbox_task (task_id, event_time),
  KEY idx_experience_outbox_source (source_type, source_id),
  KEY idx_experience_outbox_created_at (created_at),
  CONSTRAINT ck_experience_outbox_status CHECK (status IN ('queued', 'processing', 'processed', 'dead_letter'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Experience event outbox consumed asynchronously by worker';

CREATE TABLE IF NOT EXISTS experience_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_key VARCHAR(191) NOT NULL,
  schema_version INT NOT NULL DEFAULT 1,
  event_time DATETIME NOT NULL,
  source_type VARCHAR(64) NOT NULL,
  source_id VARCHAR(128) NOT NULL DEFAULT '',
  task_id BIGINT NULL,
  action VARCHAR(96) NOT NULL,
  outcome VARCHAR(64) NOT NULL DEFAULT '',
  actor_snapshot_json JSON NULL,
  business_snapshot_json JSON NULL,
  payload_json JSON NULL,
  data_classification VARCHAR(32) NOT NULL DEFAULT 'business_fact',
  ground_truth_status VARCHAR(32) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_events_event_key (event_key),
  KEY idx_experience_events_time (event_time, id),
  KEY idx_experience_events_task (task_id, event_time),
  KEY idx_experience_events_source (source_type, source_id),
  KEY idx_experience_events_action_outcome (action, outcome, event_time),
  KEY idx_experience_events_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Immutable experience fact events for reporting, evaluation, and future model training';

CREATE TABLE IF NOT EXISTS ai_suggestion_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  suggestion_event_id VARCHAR(191) NOT NULL,
  suggestion_type VARCHAR(64) NOT NULL,
  suggestion_id VARCHAR(128) NOT NULL DEFAULT '',
  source VARCHAR(64) NOT NULL DEFAULT '',
  confidence DECIMAL(8,6) NULL,
  model VARCHAR(128) NOT NULL DEFAULT '',
  provider VARCHAR(64) NOT NULL DEFAULT '',
  model_version VARCHAR(128) NOT NULL DEFAULT '',
  input_summary_json JSON NULL,
  suggestion_json JSON NULL,
  target_type VARCHAR(64) NOT NULL DEFAULT '',
  target_id VARCHAR(128) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  displayed_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_ai_suggestion_events_event_id (suggestion_event_id),
  KEY idx_ai_suggestion_events_type_time (suggestion_type, displayed_at),
  KEY idx_ai_suggestion_events_target (target_type, target_id, displayed_at),
  KEY idx_ai_suggestion_events_actor (actor_id, displayed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Displayed AI or rule suggestion ledger for later evaluation';

CREATE TABLE IF NOT EXISTS ai_suggestion_feedback (
  id BIGINT NOT NULL AUTO_INCREMENT,
  suggestion_event_id VARCHAR(191) NOT NULL,
  feedback_value VARCHAR(32) NOT NULL,
  reason_code VARCHAR(96) NOT NULL DEFAULT '',
  reason_note VARCHAR(512) NOT NULL DEFAULT '',
  outcome_source_type VARCHAR(64) NOT NULL DEFAULT '',
  outcome_source_id VARCHAR(128) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  payload_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_ai_suggestion_feedback_event (suggestion_event_id),
  KEY idx_ai_suggestion_feedback_value_time (feedback_value, created_at),
  KEY idx_ai_suggestion_feedback_actor (actor_id, created_at),
  CONSTRAINT ck_ai_suggestion_feedback_value CHECK (feedback_value IN ('accepted', 'rejected', 'partially_accepted'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Human feedback and future ground-truth link for AI suggestions';

CREATE TABLE IF NOT EXISTS task_experience_profiles (
  task_id BIGINT NOT NULL,
  profile_version INT NOT NULL DEFAULT 1,
  source_event_watermark BIGINT NOT NULL DEFAULT 0,
  task_type VARCHAR(64) NOT NULL DEFAULT '',
  category_code VARCHAR(64) NOT NULL DEFAULT '',
  category_name VARCHAR(128) NOT NULL DEFAULT '',
  task_status VARCHAR(64) NOT NULL DEFAULT '',
  outcome VARCHAR(64) NOT NULL DEFAULT '',
  profile_json JSON NULL,
  rebuilt_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (task_id),
  KEY idx_task_experience_profiles_version_watermark (profile_version, source_event_watermark),
  KEY idx_task_experience_profiles_rebuilt_at (rebuilt_at),
  KEY idx_task_experience_profiles_task_type (task_type, task_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Rebuildable materialized task experience profile';

CREATE TABLE IF NOT EXISTS asset_quality_labels (
  id BIGINT NOT NULL AUTO_INCREMENT,
  asset_id BIGINT NULL,
  task_asset_id BIGINT NULL,
  submission_file_id BIGINT NULL,
  quality_label VARCHAR(32) NOT NULL,
  reason_code VARCHAR(96) NOT NULL DEFAULT '',
  reason_note VARCHAR(512) NOT NULL DEFAULT '',
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id VARCHAR(128) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  payload_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_asset_quality_labels_asset (asset_id, created_at),
  KEY idx_asset_quality_labels_task_asset (task_asset_id, created_at),
  KEY idx_asset_quality_labels_submission_file (submission_file_id, created_at),
  KEY idx_asset_quality_labels_label (quality_label, created_at),
  KEY idx_asset_quality_labels_source (source_type, source_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Asset quality and reuse labels for future evaluation';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS asset_quality_labels;
DROP TABLE IF EXISTS task_experience_profiles;
DROP TABLE IF EXISTS ai_suggestion_feedback;
DROP TABLE IF EXISTS ai_suggestion_events;
DROP TABLE IF EXISTS experience_events;
DROP TABLE IF EXISTS experience_outbox;
DROP TABLE IF EXISTS experience_reason_tags;
-- ROLLBACK-END
