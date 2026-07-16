-- Migration: 125_task_resource_groups.sql
-- Canonical task/SKU/retouch resource groups, staging and workflow CAS.

ALTER TABLE tasks
  ADD COLUMN workflow_revision BIGINT NOT NULL DEFAULT 0 AFTER task_status;

ALTER TABLE task_sku_items
  ADD UNIQUE KEY uq_task_sku_items_task_id_id (task_id, id);

ALTER TABLE task_retouch_requirements
  ADD UNIQUE KEY uq_task_retouch_requirements_task_id_id (task_id, id);

CREATE TABLE task_asset_groups (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  scope_kind VARCHAR(32) NOT NULL,
  task_sku_item_id BIGINT NULL,
  retouch_requirement_id BIGINT NULL,
  scope_ref_id BIGINT GENERATED ALWAYS AS (COALESCE(task_sku_item_id, retouch_requirement_id, 0)) STORED,
  working_revision_id BIGINT NULL,
  finalized_revision_id BIGINT NULL,
  lock_version BIGINT NOT NULL DEFAULT 0,
  migration_incomplete TINYINT(1) NOT NULL DEFAULT 0,
  migration_issue VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_asset_group_scope (task_id, scope_kind, scope_ref_id),
  KEY idx_task_asset_groups_task (task_id, scope_kind, id),
  KEY idx_task_asset_groups_sku (task_sku_item_id),
  KEY idx_task_asset_groups_retouch (retouch_requirement_id),
  CONSTRAINT fk_task_asset_groups_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_task_asset_groups_sku FOREIGN KEY (task_id, task_sku_item_id) REFERENCES task_sku_items(task_id, id),
  CONSTRAINT fk_task_asset_groups_retouch FOREIGN KEY (task_id, retouch_requirement_id) REFERENCES task_retouch_requirements(task_id, id),
  CONSTRAINT chk_task_asset_group_scope_kind CHECK (scope_kind IN ('task','sku','retouch_requirement')),
  CONSTRAINT chk_task_asset_group_scope_shape CHECK (
    (scope_kind = 'task' AND task_sku_item_id IS NULL AND retouch_requirement_id IS NULL)
    OR (scope_kind = 'sku' AND task_sku_item_id IS NOT NULL AND retouch_requirement_id IS NULL)
    OR (scope_kind = 'retouch_requirement' AND task_sku_item_id IS NULL AND retouch_requirement_id IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Canonical task resource group shell';

-- Cutover shell backfill. Runtime reads stay pure: every legacy task receives
-- its deterministic scope identities here, while resource revisions are
-- populated by workflow-groups-migrate from confirmed mappings. The marker is
-- cleared only after that task's resource mapping is complete.
INSERT INTO task_asset_groups
  (task_id, scope_kind, retouch_requirement_id, migration_incomplete, migration_issue)
SELECT trr.task_id, 'retouch_requirement', trr.id, 1, 'legacy resource revision pending cutover mapping'
FROM task_retouch_requirements trr
JOIN tasks t ON t.id = trr.task_id AND t.task_type = 'retouch_task'
ON DUPLICATE KEY UPDATE id = task_asset_groups.id;

INSERT INTO task_asset_groups
  (task_id, scope_kind, task_sku_item_id, migration_incomplete, migration_issue)
SELECT tsi.task_id, 'sku', tsi.id, 1, 'legacy resource revision pending cutover mapping'
FROM task_sku_items tsi
JOIN tasks t ON t.id = tsi.task_id
WHERE t.task_type NOT IN ('retouch_task', 'purchase_task', 'sku_planning')
ON DUPLICATE KEY UPDATE id = task_asset_groups.id;

INSERT INTO task_asset_groups
  (task_id, scope_kind, migration_incomplete, migration_issue)
SELECT t.id, 'task', 1, 'legacy resource revision pending cutover mapping'
FROM tasks t
WHERE t.task_type NOT IN ('retouch_task', 'purchase_task', 'sku_planning')
  AND NOT EXISTS (SELECT 1 FROM task_sku_items tsi WHERE tsi.task_id = t.id)
ON DUPLICATE KEY UPDATE id = task_asset_groups.id;

CREATE TABLE task_asset_group_revisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  group_id BIGINT NOT NULL,
  revision_no INT NOT NULL,
  status VARCHAR(32) NOT NULL,
  mode VARCHAR(16) NOT NULL,
  source_task_asset_id BIGINT NULL,
  source_stage VARCHAR(32) NOT NULL,
  created_by BIGINT NOT NULL,
  reason VARCHAR(512) NOT NULL DEFAULT '',
  submitted_at DATETIME NULL,
  finalized_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_asset_group_revision_no (group_id, revision_no),
  UNIQUE KEY uq_task_asset_group_revision_group_id (group_id, id),
  KEY idx_task_asset_group_revision_status (group_id, status, id),
  KEY idx_task_asset_group_revision_source (source_task_asset_id),
  CONSTRAINT fk_task_asset_group_revision_group FOREIGN KEY (group_id) REFERENCES task_asset_groups(id),
  CONSTRAINT fk_task_asset_group_revision_source FOREIGN KEY (source_task_asset_id) REFERENCES task_assets(id),
  CONSTRAINT fk_task_asset_group_revision_actor FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT chk_task_asset_group_revision_status CHECK (status IN ('draft','submitted','finalized','rejected','superseded')),
  CONSTRAINT chk_task_asset_group_revision_mode CHECK (mode IN ('single','set')),
  CONSTRAINT chk_task_asset_group_revision_stage CHECK (source_stage IN ('design','audit','retouch','migration','reopen'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable task resource group revisions';

CREATE TABLE task_asset_group_revision_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  revision_id BIGINT NOT NULL,
  task_asset_id BIGINT NOT NULL,
  sort_order INT NOT NULL,
  item_name VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_asset_group_revision_item_order (revision_id, sort_order),
  UNIQUE KEY uq_task_asset_group_revision_item_asset (revision_id, task_asset_id),
  UNIQUE KEY uq_task_asset_group_revision_item_revision_id (revision_id, id),
  KEY idx_task_asset_group_revision_items_asset (task_asset_id),
  CONSTRAINT fk_task_asset_group_revision_item_revision FOREIGN KEY (revision_id) REFERENCES task_asset_group_revisions(id),
  CONSTRAINT fk_task_asset_group_revision_item_asset FOREIGN KEY (task_asset_id) REFERENCES task_assets(id),
  CONSTRAINT chk_task_asset_group_revision_item_order CHECK (sort_order >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Ordered final files in a resource group revision';

CREATE TABLE task_asset_group_revision_references (
  id BIGINT NOT NULL AUTO_INCREMENT,
  revision_id BIGINT NOT NULL,
  reference_file_ref_id BIGINT NOT NULL,
  formal_task_asset_id BIGINT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  ref_id_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  file_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  scope_snapshot VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_asset_group_revision_reference (revision_id, reference_file_ref_id),
  KEY idx_task_asset_group_revision_reference_asset (formal_task_asset_id),
  CONSTRAINT fk_task_asset_group_revision_reference_revision FOREIGN KEY (revision_id) REFERENCES task_asset_group_revisions(id),
  CONSTRAINT fk_task_asset_group_revision_reference_ref FOREIGN KEY (reference_file_ref_id) REFERENCES reference_file_refs(id),
  CONSTRAINT fk_task_asset_group_revision_reference_asset FOREIGN KEY (formal_task_asset_id) REFERENCES task_assets(id),
  CONSTRAINT chk_task_asset_group_revision_reference_order CHECK (sort_order >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Frozen reference images for a resource group revision';

ALTER TABLE task_asset_groups
  ADD CONSTRAINT fk_task_asset_groups_working_revision
    FOREIGN KEY (id, working_revision_id) REFERENCES task_asset_group_revisions(group_id, id),
  ADD CONSTRAINT fk_task_asset_groups_finalized_revision
    FOREIGN KEY (id, finalized_revision_id) REFERENCES task_asset_group_revisions(group_id, id);

ALTER TABLE task_assets
  ADD COLUMN binding_state VARCHAR(24) NOT NULL DEFAULT 'legacy' AFTER asset_type,
  ADD COLUMN bound_group_id BIGINT NULL AFTER binding_state,
  ADD COLUMN bound_role VARCHAR(16) NULL AFTER bound_group_id,
  ADD COLUMN staged_task_sku_item_id BIGINT NULL AFTER bound_role,
  ADD COLUMN staged_retouch_requirement_id BIGINT NULL AFTER staged_task_sku_item_id,
  ADD COLUMN staged_role VARCHAR(16) NULL AFTER staged_retouch_requirement_id,
  ADD COLUMN staged_by BIGINT NULL AFTER staged_role,
  ADD COLUMN upload_session_id VARCHAR(64) NULL AFTER staged_by,
  ADD COLUMN staged_expires_at DATETIME NULL AFTER upload_session_id,
  ADD COLUMN access_revoked_at DATETIME NULL AFTER staged_expires_at,
  ADD COLUMN access_revoked_reason VARCHAR(512) NOT NULL DEFAULT '' AFTER access_revoked_at,
  ADD COLUMN object_deleted_at DATETIME NULL AFTER access_revoked_reason,
  ADD KEY idx_task_assets_binding (binding_state, bound_group_id, bound_role),
  ADD KEY idx_task_assets_staging_expiry (binding_state, staged_expires_at),
  ADD KEY idx_task_assets_staged_sku (staged_task_sku_item_id),
  ADD KEY idx_task_assets_staged_retouch (staged_retouch_requirement_id),
  ADD CONSTRAINT fk_task_assets_bound_group FOREIGN KEY (bound_group_id) REFERENCES task_asset_groups(id),
  ADD CONSTRAINT fk_task_assets_staged_sku FOREIGN KEY (staged_task_sku_item_id) REFERENCES task_sku_items(id),
  ADD CONSTRAINT fk_task_assets_staged_retouch FOREIGN KEY (staged_retouch_requirement_id) REFERENCES task_retouch_requirements(id),
  ADD CONSTRAINT fk_task_assets_staged_by FOREIGN KEY (staged_by) REFERENCES users(id),
  ADD CONSTRAINT chk_task_assets_binding_state CHECK (binding_state IN ('legacy','staged','bound','discarded')),
  ADD CONSTRAINT chk_task_assets_bound_role CHECK (bound_role IS NULL OR bound_role IN ('source','final')),
  ADD CONSTRAINT chk_task_assets_staged_role CHECK (staged_role IS NULL OR staged_role IN ('source','final')),
  ADD CONSTRAINT chk_task_assets_staged_scope CHECK (staged_task_sku_item_id IS NULL OR staged_retouch_requirement_id IS NULL);

CREATE TABLE task_asset_staging_drafts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  task_sku_item_id BIGINT NULL,
  retouch_requirement_id BIGINT NULL,
  scope_ref_key VARCHAR(64) GENERATED ALWAYS AS (
    CASE
      WHEN task_sku_item_id IS NOT NULL THEN CONCAT('sku:', task_sku_item_id)
      WHEN retouch_requirement_id IS NOT NULL THEN CONCAT('retouch:', retouch_requirement_id)
      ELSE 'task:0'
    END
  ) STORED,
  client_draft_id VARCHAR(64) NOT NULL,
  mode VARCHAR(16) NOT NULL,
  ordered_task_asset_ids_json JSON NOT NULL,
  created_by BIGINT NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_asset_staging_draft (task_id, scope_ref_key, client_draft_id, created_by),
  KEY idx_task_asset_staging_draft_expiry (expires_at),
  CONSTRAINT fk_task_asset_staging_draft_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_task_asset_staging_draft_sku FOREIGN KEY (task_sku_item_id) REFERENCES task_sku_items(id),
  CONSTRAINT fk_task_asset_staging_draft_retouch FOREIGN KEY (retouch_requirement_id) REFERENCES task_retouch_requirements(id),
  CONSTRAINT fk_task_asset_staging_draft_actor FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT chk_task_asset_staging_draft_mode CHECK (mode IN ('single','set')),
  CONSTRAINT chk_task_asset_staging_draft_scope CHECK (task_sku_item_id IS NULL OR retouch_requirement_id IS NULL)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Server-side resumable resource staging manifests';

CREATE TABLE workflow_action_idempotency (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  action_type VARCHAR(64) NOT NULL,
  actor_id BIGINT NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  request_hash VARCHAR(64) NOT NULL,
  response_json JSON NULL,
  completed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_workflow_action_idempotency (task_id, action_type, actor_id, idempotency_key),
  KEY idx_workflow_action_idempotency_created (created_at),
  CONSTRAINT fk_workflow_action_idempotency_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_workflow_action_idempotency_actor FOREIGN KEY (actor_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Idempotent workflow action response snapshots';

CREATE TABLE asset_object_deletion_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_asset_id BIGINT NULL,
  storage_ref_id VARCHAR(64) NULL,
  storage_adapter VARCHAR(32) NOT NULL,
  storage_is_placeholder TINYINT(1) NOT NULL DEFAULT 0,
  storage_key VARCHAR(1024) NOT NULL,
  dedupe_key VARCHAR(255) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  attempt INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME NULL,
  lease_token VARCHAR(64) NULL,
  lease_until DATETIME NULL,
  last_error TEXT NULL,
  alert_status VARCHAR(24) NOT NULL DEFAULT 'none',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_asset_object_deletion_dedupe (dedupe_key),
  KEY idx_asset_object_deletion_claim (status, next_retry_at, lease_until, id),
  CONSTRAINT fk_asset_object_deletion_asset FOREIGN KEY (task_asset_id) REFERENCES task_assets(id),
  CONSTRAINT chk_asset_object_deletion_status CHECK (status IN ('pending','processing','succeeded','retry'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Durable adapter-aware object deletion queue; storage 404 is success';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS asset_object_deletion_outbox;
DROP TABLE IF EXISTS workflow_action_idempotency;
DROP TABLE IF EXISTS task_asset_staging_drafts;
ALTER TABLE task_assets DROP FOREIGN KEY fk_task_assets_staged_by;
ALTER TABLE task_assets DROP FOREIGN KEY fk_task_assets_staged_retouch;
ALTER TABLE task_assets DROP FOREIGN KEY fk_task_assets_staged_sku;
ALTER TABLE task_assets DROP FOREIGN KEY fk_task_assets_bound_group;
ALTER TABLE task_assets DROP INDEX idx_task_assets_staged_retouch;
ALTER TABLE task_assets DROP INDEX idx_task_assets_staged_sku;
ALTER TABLE task_assets DROP INDEX idx_task_assets_staging_expiry;
ALTER TABLE task_assets DROP INDEX idx_task_assets_binding;
ALTER TABLE task_assets DROP COLUMN object_deleted_at;
ALTER TABLE task_assets DROP COLUMN access_revoked_reason;
ALTER TABLE task_assets DROP COLUMN access_revoked_at;
ALTER TABLE task_assets DROP COLUMN staged_expires_at;
ALTER TABLE task_assets DROP COLUMN upload_session_id;
ALTER TABLE task_assets DROP COLUMN staged_by;
ALTER TABLE task_assets DROP COLUMN staged_role;
ALTER TABLE task_assets DROP COLUMN staged_retouch_requirement_id;
ALTER TABLE task_assets DROP COLUMN staged_task_sku_item_id;
ALTER TABLE task_assets DROP COLUMN bound_role;
ALTER TABLE task_assets DROP COLUMN bound_group_id;
ALTER TABLE task_assets DROP COLUMN binding_state;
ALTER TABLE task_asset_groups DROP FOREIGN KEY fk_task_asset_groups_finalized_revision;
ALTER TABLE task_asset_groups DROP FOREIGN KEY fk_task_asset_groups_working_revision;
DROP TABLE IF EXISTS task_asset_group_revision_references;
DROP TABLE IF EXISTS task_asset_group_revision_items;
DROP TABLE IF EXISTS task_asset_group_revisions;
DROP TABLE IF EXISTS task_asset_groups;
ALTER TABLE task_retouch_requirements DROP INDEX uq_task_retouch_requirements_task_id_id;
ALTER TABLE task_sku_items DROP INDEX uq_task_sku_items_task_id_id;
ALTER TABLE tasks DROP COLUMN workflow_revision;
-- ROLLBACK-END
