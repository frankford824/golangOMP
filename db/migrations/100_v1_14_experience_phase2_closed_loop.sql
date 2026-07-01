-- v1.14 experience phase 2: client-visible feedback config, behavior capture,
-- stable suggestion keys, observer baselines, attribution evidence, and guardable
-- retention primitives.
--
-- Experience side-channel tables intentionally do not declare foreign keys to
-- core workflow tables. Core tables remain read-only observer sources.

ALTER TABLE experience_outbox
  ADD COLUMN target_type VARCHAR(64) NOT NULL DEFAULT '' AFTER task_id,
  ADD COLUMN target_id VARCHAR(128) NOT NULL DEFAULT '' AFTER target_type,
  ADD COLUMN source_watermark VARCHAR(191) NOT NULL DEFAULT '' AFTER target_id,
  ADD COLUMN observed_from VARCHAR(64) NOT NULL DEFAULT '' AFTER source_watermark,
  ADD COLUMN observed_id VARCHAR(128) NOT NULL DEFAULT '' AFTER observed_from;

ALTER TABLE experience_outbox ADD KEY idx_experience_outbox_target_time (target_type, target_id, event_time);
ALTER TABLE experience_outbox ADD KEY idx_experience_outbox_observed (observed_from, observed_id);

ALTER TABLE experience_events
  ADD COLUMN target_type VARCHAR(64) NOT NULL DEFAULT '' AFTER task_id,
  ADD COLUMN target_id VARCHAR(128) NOT NULL DEFAULT '' AFTER target_type,
  ADD COLUMN source_watermark VARCHAR(191) NOT NULL DEFAULT '' AFTER target_id,
  ADD COLUMN observed_from VARCHAR(64) NOT NULL DEFAULT '' AFTER source_watermark,
  ADD COLUMN observed_id VARCHAR(128) NOT NULL DEFAULT '' AFTER observed_from;

ALTER TABLE experience_events ADD KEY idx_experience_events_target_time (target_type, target_id, event_time);
ALTER TABLE experience_events ADD KEY idx_experience_events_source_action_time (source_type, action, event_time);
ALTER TABLE experience_events ADD KEY idx_experience_events_observed (observed_from, observed_id);

ALTER TABLE ai_suggestion_events
  ADD COLUMN suggestion_stable_key VARCHAR(191) NOT NULL DEFAULT '' AFTER suggestion_event_id,
  ADD COLUMN attribution_eligible TINYINT(1) NOT NULL DEFAULT 1 AFTER suggestion_stable_key;

ALTER TABLE ai_suggestion_events ADD KEY idx_ai_suggestion_events_stable_time (suggestion_stable_key, displayed_at);
ALTER TABLE ai_suggestion_events ADD KEY idx_ai_suggestion_events_attribution_time (attribution_eligible, displayed_at);

ALTER TABLE tasks ADD KEY idx_tasks_experience_observer_updated (updated_at, id);
ALTER TABLE audit_records ADD KEY idx_audit_records_experience_observer (created_at, id);
ALTER TABLE task_module_events ADD KEY idx_task_module_events_experience_observer (created_at, id);
ALTER TABLE task_assets ADD KEY idx_task_assets_experience_observer_created (created_at, id);
ALTER TABLE task_assets ADD KEY idx_task_assets_experience_observer_approved (approved_at, id);
ALTER TABLE task_assets ADD KEY idx_task_assets_experience_observer_rejected (rejected_at, id);
ALTER TABLE task_assets ADD KEY idx_task_assets_experience_observer_archived (archived_at, id);
ALTER TABLE task_assets ADD KEY idx_task_assets_experience_observer_cleaned (cleaned_at, id);
ALTER TABLE task_details ADD KEY idx_task_details_experience_observer_updated (updated_at, id);
ALTER TABLE task_sku_items ADD KEY idx_task_sku_items_experience_observer_updated (updated_at, id);

CREATE TABLE IF NOT EXISTS experience_behavior_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_key VARCHAR(191) NOT NULL,
  client_event_id VARCHAR(191) NOT NULL,
  page_instance_id VARCHAR(191) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  surface VARCHAR(64) NOT NULL DEFAULT '',
  action VARCHAR(64) NOT NULL,
  target_type VARCHAR(64) NOT NULL DEFAULT '',
  target_id VARCHAR(128) NOT NULL DEFAULT '',
  task_id BIGINT NULL,
  suggestion_event_id VARCHAR(191) NOT NULL DEFAULT '',
  suggestion_stable_key VARCHAR(191) NOT NULL DEFAULT '',
  occurred_at DATETIME NOT NULL,
  received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  route_name VARCHAR(128) NOT NULL DEFAULT '',
  component VARCHAR(128) NOT NULL DEFAULT '',
  dwell_ms INT NOT NULL DEFAULT 0,
  payload_json JSON NULL,
  data_classification VARCHAR(32) NOT NULL DEFAULT 'behavior',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_behavior_event_key (event_key),
  KEY idx_experience_behavior_actor_time (actor_id, occurred_at),
  KEY idx_experience_behavior_suggestion_event (suggestion_event_id, occurred_at),
  KEY idx_experience_behavior_stable (suggestion_stable_key, occurred_at),
  KEY idx_experience_behavior_target (target_type, target_id, occurred_at),
  KEY idx_experience_behavior_action_time (action, occurred_at),
  CONSTRAINT ck_experience_behavior_action CHECK (action IN (
    'impression', 'visible', 'expand', 'click', 'jump', 'dismiss', 'refresh', 'copy',
    'related_action_done', 'ignored_after_timeout'
  ))
);

CREATE TABLE IF NOT EXISTS experience_observed_entity_states (
  id BIGINT NOT NULL AUTO_INCREMENT,
  source_name VARCHAR(64) NOT NULL,
  entity_type VARCHAR(64) NOT NULL,
  entity_id VARCHAR(128) NOT NULL,
  observed_value_json JSON NULL,
  observed_hash CHAR(40) NOT NULL DEFAULT '',
  terminal_state VARCHAR(64) NOT NULL DEFAULT '',
  terminal_observed_at DATETIME NULL,
  source_updated_at DATETIME NULL,
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tombstoned TINYINT(1) NOT NULL DEFAULT 0,
  tombstone_payload_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_observed_entity (source_name, entity_type, entity_id),
  KEY idx_experience_observed_terminal (source_name, entity_type, terminal_state, terminal_observed_at),
  KEY idx_experience_observed_last_seen (last_seen_at),
  KEY idx_experience_observed_source_updated (source_name, source_updated_at, id)
);

CREATE TABLE IF NOT EXISTS experience_worker_watermarks (
  worker_name VARCHAR(96) NOT NULL,
  source_name VARCHAR(96) NOT NULL,
  last_seen_at DATETIME NULL,
  last_seen_id BIGINT NOT NULL DEFAULT 0,
  source_watermark VARCHAR(191) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  metadata_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (worker_name, source_name),
  KEY idx_experience_worker_watermarks_updated (updated_at)
);

CREATE TABLE IF NOT EXISTS experience_worker_runs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  worker_name VARCHAR(96) NOT NULL,
  source_name VARCHAR(96) NOT NULL DEFAULT '',
  started_at DATETIME NOT NULL,
  finished_at DATETIME NULL,
  status VARCHAR(32) NOT NULL,
  scanned_count INT NOT NULL DEFAULT 0,
  enqueued_count INT NOT NULL DEFAULT 0,
  skipped_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  metadata_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_experience_worker_runs_worker_time (worker_name, started_at),
  KEY idx_experience_worker_runs_status (status, started_at)
);

CREATE TABLE IF NOT EXISTS experience_attributions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  suggestion_event_id VARCHAR(191) NOT NULL,
  suggestion_stable_key VARCHAR(191) NOT NULL DEFAULT '',
  candidate_event_key VARCHAR(191) NOT NULL DEFAULT '',
  outcome_event_key VARCHAR(191) NOT NULL DEFAULT '',
  status VARCHAR(48) NOT NULL,
  confidence VARCHAR(32) NOT NULL DEFAULT '',
  score DECIMAL(8,4) NOT NULL DEFAULT 0,
  computed_at DATETIME NOT NULL,
  evidence_summary_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_attribution_candidate (suggestion_event_id, candidate_event_key, outcome_event_key),
  KEY idx_experience_attributions_stable_time (suggestion_stable_key, computed_at),
  KEY idx_experience_attributions_status_time (status, computed_at),
  KEY idx_experience_attributions_outcome (outcome_event_key)
);

CREATE TABLE IF NOT EXISTS experience_micro_question_answers (
  id BIGINT NOT NULL AUTO_INCREMENT,
  answer_event_key VARCHAR(191) NOT NULL,
  suggestion_event_id VARCHAR(191) NOT NULL DEFAULT '',
  suggestion_stable_key VARCHAR(191) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  surface VARCHAR(64) NOT NULL DEFAULT '',
  target_type VARCHAR(64) NOT NULL DEFAULT '',
  target_id VARCHAR(128) NOT NULL DEFAULT '',
  answer_value VARCHAR(64) NOT NULL,
  reason_code VARCHAR(96) NOT NULL DEFAULT '',
  payload_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_micro_question_answer (answer_event_key),
  KEY idx_experience_micro_question_actor_time (actor_id, created_at),
  KEY idx_experience_micro_question_stable_time (suggestion_stable_key, created_at),
  KEY idx_experience_micro_question_reason (reason_code, created_at)
);

CREATE TABLE IF NOT EXISTS experience_review_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  item_key VARCHAR(191) NOT NULL,
  item_type VARCHAR(64) NOT NULL,
  status VARCHAR(48) NOT NULL DEFAULT 'open',
  priority VARCHAR(32) NOT NULL DEFAULT '',
  evidence_summary_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_experience_review_item_key (item_key),
  KEY idx_experience_review_items_status (status, priority, created_at)
);

CREATE TABLE IF NOT EXISTS experience_review_decisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  review_item_key VARCHAR(191) NOT NULL,
  decision VARCHAR(64) NOT NULL,
  reason_code VARCHAR(96) NOT NULL DEFAULT '',
  actor_id BIGINT NULL,
  payload_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_experience_review_decisions_item (review_item_key, created_at),
  KEY idx_experience_review_decisions_actor (actor_id, created_at)
);

CREATE TABLE IF NOT EXISTS experience_rate_limits (
  limit_key VARCHAR(191) NOT NULL,
  actor_id BIGINT NULL,
  bucket_name VARCHAR(64) NOT NULL,
  period_start DATETIME NOT NULL,
  period_end DATETIME NOT NULL,
  count INT NOT NULL DEFAULT 0,
  hard_cap INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (limit_key),
  KEY idx_experience_rate_limits_bucket (bucket_name, period_start),
  KEY idx_experience_rate_limits_actor (actor_id, bucket_name, period_start),
  KEY idx_experience_rate_limits_period_end (period_end)
);

INSERT INTO experience_reason_tags (scene, code, name, tag_group, severity, sort_order)
SELECT scene, code, name, tag_group, severity, sort_order
FROM (
  SELECT 'ai_suggestion_micro_question' AS scene, 'temporarily_not_needed' AS code, '暂时不需要' AS name, 'micro_question_reason' AS tag_group, 'low' AS severity, 10 AS sort_order
  UNION ALL SELECT 'ai_suggestion_micro_question', 'will_handle_later', '稍后处理', 'micro_question_reason', 'low', 20
  UNION ALL SELECT 'ai_suggestion_micro_question', 'already_handled', '已处理', 'micro_question_reason', 'low', 30
  UNION ALL SELECT 'ai_suggestion_micro_question', 'not_relevant', '不相关', 'micro_question_reason', 'medium', 40
  UNION ALL SELECT 'ai_suggestion_micro_question', 'missing_context', '缺少上下文', 'micro_question_reason', 'medium', 50
  UNION ALL SELECT 'ai_suggestion_micro_question', 'stage_not_applicable', '当前阶段不适用', 'micro_question_reason', 'medium', 60
  UNION ALL SELECT 'ai_suggestion_micro_question', 'customer_special_case', '客户特例', 'micro_question_reason', 'medium', 70
  UNION ALL SELECT 'ai_suggestion_micro_question', 'suggestion_outdated', '建议已过时', 'micro_question_reason', 'medium', 80
) seed
WHERE NOT EXISTS (
  SELECT 1
  FROM experience_reason_tags t
  WHERE t.scene = seed.scene AND t.code = seed.code AND t.deleted_at IS NULL
);

-- Down migration (manual only)
-- DELETE FROM experience_reason_tags WHERE scene = 'ai_suggestion_micro_question';
-- DROP TABLE IF EXISTS experience_rate_limits;
-- DROP TABLE IF EXISTS experience_review_decisions;
-- DROP TABLE IF EXISTS experience_review_items;
-- DROP TABLE IF EXISTS experience_micro_question_answers;
-- DROP TABLE IF EXISTS experience_attributions;
-- DROP TABLE IF EXISTS experience_worker_runs;
-- DROP TABLE IF EXISTS experience_worker_watermarks;
-- DROP TABLE IF EXISTS experience_observed_entity_states;
-- DROP TABLE IF EXISTS experience_behavior_events;
