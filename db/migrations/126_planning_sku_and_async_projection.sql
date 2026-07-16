-- Migration: 126_planning_sku_and_async_projection.sql
-- One canonical SKU identity, versioned code rules, planning details, ERP/search outboxes,
-- and pinned resource-group publication on the existing client-material table.

CREATE TABLE code_rule_revisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  rule_id BIGINT NOT NULL,
  version_no INT NOT NULL,
  prefix VARCHAR(32) NOT NULL DEFAULT '',
  date_format VARCHAR(32) NOT NULL DEFAULT '',
  site_code VARCHAR(16) NOT NULL DEFAULT '',
  biz_code VARCHAR(16) NOT NULL DEFAULT '',
  separator_text VARCHAR(8) NOT NULL DEFAULT '',
  seq_length INT NOT NULL DEFAULT 6,
  reset_cycle VARCHAR(16) NOT NULL DEFAULT 'none',
  dimension_mode VARCHAR(32) NOT NULL DEFAULT 'none',
  config_json JSON NOT NULL,
  created_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_code_rule_revision_version (rule_id, version_no),
  UNIQUE KEY uq_code_rule_revision_rule_id (rule_id, id),
  CONSTRAINT fk_code_rule_revision_rule FOREIGN KEY (rule_id) REFERENCES code_rules(id),
  CONSTRAINT fk_code_rule_revision_actor FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT chk_code_rule_revision_reset CHECK (reset_cycle IN ('none','daily','monthly','yearly')),
  CONSTRAINT chk_code_rule_revision_dimension CHECK (dimension_mode IN ('none','category_code')),
  CONSTRAINT chk_code_rule_revision_seq_length CHECK (seq_length BETWEEN 1 AND 18)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable code rule revisions';

INSERT INTO code_rule_revisions
  (rule_id, version_no, prefix, date_format, site_code, biz_code, separator_text, seq_length, reset_cycle, dimension_mode, config_json)
SELECT id, 1, prefix, date_format, site_code, biz_code, '', seq_length, reset_cycle,
       CASE WHEN rule_type = 'new_sku' THEN 'category_code' ELSE 'none' END,
       config_json
FROM code_rules;

ALTER TABLE code_rules
  ADD COLUMN active_revision_id BIGINT NULL AFTER is_enabled,
  ADD KEY idx_code_rules_active_revision (active_revision_id);

UPDATE code_rules r
JOIN code_rule_revisions rev ON rev.rule_id = r.id AND rev.version_no = 1
SET r.active_revision_id = rev.id;

ALTER TABLE code_rules
  ADD CONSTRAINT fk_code_rules_active_revision
    FOREIGN KEY (id, active_revision_id) REFERENCES code_rule_revisions(rule_id, id);

CREATE TABLE code_rule_revision_sequences (
  rule_revision_id BIGINT NOT NULL,
  dimension_key VARCHAR(128) NOT NULL DEFAULT '',
  period_key VARCHAR(32) NOT NULL DEFAULT '',
  next_value BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (rule_revision_id, dimension_key, period_key),
  CONSTRAINT fk_code_rule_revision_sequence_revision FOREIGN KEY (rule_revision_id) REFERENCES code_rule_revisions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Single atomic sequence engine for all code-rule revisions';

INSERT INTO code_rule_revision_sequences (rule_revision_id, dimension_key, period_key, next_value)
SELECT r.active_revision_id, '', '', s.last_seq + 1
FROM code_rule_sequences s
JOIN code_rules r ON r.id = s.rule_id
WHERE r.active_revision_id IS NOT NULL;

INSERT INTO code_rules
  (rule_type, rule_name, prefix, date_format, site_code, biz_code, seq_length, reset_cycle, is_enabled, config_json)
SELECT 'task_product_code', 'Task Product Code', 'NS', '', '', '', 6, 'none', 1,
       JSON_OBJECT('dimension_mode','category_code','migrated_from','product_code_sequences')
WHERE NOT EXISTS (SELECT 1 FROM code_rules WHERE rule_type = 'task_product_code');

INSERT INTO code_rule_revisions
  (rule_id, version_no, prefix, date_format, site_code, biz_code, separator_text, seq_length, reset_cycle, dimension_mode, config_json)
SELECT r.id, 1, r.prefix, r.date_format, r.site_code, r.biz_code, '', r.seq_length, r.reset_cycle,
       'category_code', r.config_json
FROM code_rules r
WHERE r.rule_type = 'task_product_code'
  AND NOT EXISTS (SELECT 1 FROM code_rule_revisions rev WHERE rev.rule_id = r.id);

UPDATE code_rules r
JOIN code_rule_revisions rev ON rev.rule_id = r.id AND rev.version_no = 1
SET r.active_revision_id = rev.id
WHERE r.rule_type = 'task_product_code' AND r.active_revision_id IS NULL;

INSERT INTO code_rule_revision_sequences (rule_revision_id, dimension_key, period_key, next_value)
SELECT r.active_revision_id, pcs.category_code, '', pcs.next_value
FROM product_code_sequences pcs
JOIN code_rules r ON r.rule_type = 'task_product_code' AND r.is_enabled = 1
WHERE r.active_revision_id IS NOT NULL
ON DUPLICATE KEY UPDATE next_value = GREATEST(code_rule_revision_sequences.next_value, VALUES(next_value));

INSERT INTO code_rules
  (rule_type, rule_name, prefix, date_format, site_code, biz_code, seq_length, reset_cycle, is_enabled, config_json)
SELECT 'sku_planning', '策划 SKU 编号规则（待配置）', 'SKU', '', '', '', 6, 'none', 0,
       JSON_OBJECT('dimension_mode','none','requires_admin_activation',true)
WHERE NOT EXISTS (SELECT 1 FROM code_rules WHERE rule_type = 'sku_planning');

INSERT INTO code_rule_revisions
  (rule_id, version_no, prefix, date_format, site_code, biz_code, separator_text, seq_length, reset_cycle, dimension_mode, config_json)
SELECT r.id, 1, r.prefix, r.date_format, r.site_code, r.biz_code, '-', r.seq_length, r.reset_cycle,
       'none', r.config_json
FROM code_rules r
WHERE r.rule_type = 'sku_planning'
  AND NOT EXISTS (SELECT 1 FROM code_rule_revisions rev WHERE rev.rule_id = r.id);

UPDATE code_rules r
JOIN code_rule_revisions rev ON rev.rule_id = r.id AND rev.version_no = 1
SET r.active_revision_id = rev.id
WHERE r.rule_type = 'sku_planning' AND r.active_revision_id IS NULL;

ALTER TABLE task_sku_items
  ADD COLUMN product_i_id VARCHAR(128) NOT NULL DEFAULT '' AFTER product_name_snapshot,
  ADD COLUMN sku_origin VARCHAR(32) NOT NULL DEFAULT 'native' AFTER sku_status,
  ADD KEY idx_task_sku_items_product_i_id (product_i_id),
  ADD CONSTRAINT chk_task_sku_items_origin CHECK (sku_origin IN ('native','legacy_migration'));

UPDATE task_sku_items
SET product_i_id = COALESCE(
  NULLIF(JSON_UNQUOTE(JSON_EXTRACT(variant_json, '$.product_i_id')), 'null'),
  NULLIF(JSON_UNQUOTE(JSON_EXTRACT(variant_json, '$.i_id')), 'null'),
  ''
)
WHERE product_i_id = '' AND variant_json IS NOT NULL;

CREATE TABLE task_planning_settings (
  task_id BIGINT NOT NULL,
  erp_sync_mode VARCHAR(16) NOT NULL DEFAULT 'none',
  code_rule_revision_id BIGINT NOT NULL,
  client_create_id VARCHAR(128) NOT NULL,
  created_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (task_id),
  UNIQUE KEY uq_task_planning_settings_client_create (created_by, client_create_id),
  KEY idx_task_planning_settings_rule_revision (code_rule_revision_id),
  CONSTRAINT fk_task_planning_settings_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_task_planning_settings_rule_revision FOREIGN KEY (code_rule_revision_id) REFERENCES code_rule_revisions(id),
  CONSTRAINT fk_task_planning_settings_actor FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT chk_task_planning_settings_erp_mode CHECK (erp_sync_mode IN ('none','async'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Task-level planning SKU settings';

CREATE TABLE task_planning_sku_details (
  task_sku_item_id BIGINT NOT NULL,
  current_revision_id BIGINT NULL,
  lock_version BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (task_sku_item_id),
  CONSTRAINT fk_task_planning_sku_detail_item FOREIGN KEY (task_sku_item_id) REFERENCES task_sku_items(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Planning extension for canonical task SKU items';

CREATE TABLE task_planning_sku_revisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_sku_item_id BIGINT NOT NULL,
  version_no INT NOT NULL,
  description_spec TEXT NOT NULL,
  quantity BIGINT NOT NULL,
  target_price DECIMAL(12,2) NULL,
  currency CHAR(3) NOT NULL DEFAULT 'CNY',
  note TEXT NOT NULL,
  reference_url VARCHAR(2048) NOT NULL DEFAULT '',
  erp_product_i_id VARCHAR(128) NOT NULL DEFAULT '',
  erp_product_name VARCHAR(255) NOT NULL DEFAULT '',
  reason VARCHAR(512) NOT NULL,
  created_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_planning_sku_revision_version (task_sku_item_id, version_no),
  UNIQUE KEY uq_task_planning_sku_revision_item_id (task_sku_item_id, id),
  CONSTRAINT fk_task_planning_sku_revision_item FOREIGN KEY (task_sku_item_id) REFERENCES task_sku_items(id),
  CONSTRAINT fk_task_planning_sku_revision_actor FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT chk_task_planning_sku_revision_target CHECK (target_price IS NULL OR target_price > 0),
  CONSTRAINT chk_task_planning_sku_revision_quantity CHECK (quantity > 0),
  CONSTRAINT chk_task_planning_sku_revision_currency CHECK (currency = 'CNY')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable planning SKU corrections';

ALTER TABLE task_planning_sku_details
  ADD CONSTRAINT fk_task_planning_sku_detail_current_revision
    FOREIGN KEY (task_sku_item_id, current_revision_id) REFERENCES task_planning_sku_revisions(task_sku_item_id, id);

CREATE TABLE task_planning_sku_revision_images (
  revision_id BIGINT NOT NULL,
  storage_ref_id VARCHAR(64) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (revision_id),
  KEY idx_task_planning_sku_revision_image_ref (storage_ref_id),
  CONSTRAINT fk_task_planning_sku_revision_image_revision FOREIGN KEY (revision_id) REFERENCES task_planning_sku_revisions(id),
  CONSTRAINT fk_task_planning_sku_revision_image_ref FOREIGN KEY (storage_ref_id) REFERENCES asset_storage_refs(ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Planning product image pinned to a planning revision';

ALTER TABLE upload_requests
  ADD COLUMN client_create_id VARCHAR(128) NOT NULL DEFAULT '' AFTER owner_id,
  ADD COLUMN client_item_id VARCHAR(128) NOT NULL DEFAULT '' AFTER client_create_id,
  ADD KEY idx_upload_requests_planning_draft (owner_type, owner_id, client_create_id, client_item_id);

CREATE TABLE task_erp_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  task_sku_item_id BIGINT NULL,
  job_type VARCHAR(32) NOT NULL,
  generation INT NOT NULL DEFAULT 1,
  dedupe_key VARCHAR(255) NOT NULL,
  payload_json JSON NOT NULL,
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
  UNIQUE KEY uq_task_erp_outbox_dedupe (dedupe_key),
  KEY idx_task_erp_outbox_claim (status, next_retry_at, lease_until, id),
  KEY idx_task_erp_outbox_task (task_id, task_sku_item_id, id),
  CONSTRAINT fk_task_erp_outbox_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_task_erp_outbox_item FOREIGN KEY (task_sku_item_id) REFERENCES task_sku_items(id),
  CONSTRAINT chk_task_erp_outbox_job_type CHECK (job_type IN ('task_filing','task_image_sync','planning_sku_sync','planning_sku_resync')),
  CONSTRAINT chk_task_erp_outbox_status CHECK (status IN ('pending','processing','succeeded','retry'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Durable ERP work independent of task completion';

CREATE TABLE task_asset_group_search_documents (
  group_id BIGINT NOT NULL,
  task_id BIGINT NOT NULL,
  finalized_revision_id BIGINT NULL,
  internal_text LONGTEXT NOT NULL,
  final_text LONGTEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id),
  KEY idx_task_asset_group_search_task (task_id, group_id),
  FULLTEXT KEY ft_task_asset_group_search_internal (internal_text),
  FULLTEXT KEY ft_task_asset_group_search_final (final_text),
  CONSTRAINT fk_task_asset_group_search_group FOREIGN KEY (group_id) REFERENCES task_asset_groups(id),
  CONSTRAINT fk_task_asset_group_search_task FOREIGN KEY (task_id) REFERENCES tasks(id),
  CONSTRAINT fk_task_asset_group_search_revision FOREIGN KEY (group_id, finalized_revision_id) REFERENCES task_asset_group_revisions(group_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Resource-group search projection with internal/final text';

CREATE TABLE search_reindex_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT NOT NULL,
  dedupe_key VARCHAR(255) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  attempt INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME NULL,
  lease_token VARCHAR(64) NULL,
  lease_until DATETIME NULL,
  last_error TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_search_reindex_outbox_dedupe (dedupe_key),
  KEY idx_search_reindex_outbox_claim (status, next_retry_at, lease_until, id),
  CONSTRAINT chk_search_reindex_outbox_entity CHECK (entity_type IN ('task','task_resource_group')),
  CONSTRAINT chk_search_reindex_outbox_status CHECK (status IN ('pending','processing','succeeded','retry'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Durable task/resource-group search refresh queue';

ALTER TABLE asset_workbench_client_materials
  MODIFY COLUMN asset_id BIGINT NULL,
  ADD COLUMN resource_group_id BIGINT NULL AFTER source_ref,
  ADD COLUMN finalized_revision_id BIGINT NULL AFTER resource_group_id,
  ADD COLUMN cover_revision_item_id BIGINT NULL AFTER finalized_revision_id,
  ADD KEY idx_aw_client_materials_resource_group (resource_group_id, enabled, id),
  ADD KEY idx_aw_client_materials_revision (resource_group_id, finalized_revision_id),
  ADD KEY idx_aw_client_materials_cover_revision (finalized_revision_id, cover_revision_item_id),
  ADD CONSTRAINT fk_aw_client_materials_resource_group FOREIGN KEY (resource_group_id) REFERENCES task_asset_groups(id),
  ADD CONSTRAINT fk_aw_client_materials_revision FOREIGN KEY (resource_group_id, finalized_revision_id) REFERENCES task_asset_group_revisions(group_id, id),
  ADD CONSTRAINT fk_aw_client_materials_cover_item FOREIGN KEY (finalized_revision_id, cover_revision_item_id) REFERENCES task_asset_group_revision_items(revision_id, id),
  ADD CONSTRAINT chk_aw_client_material_source_shape CHECK (
	(source_type IN ('external','external_asset') AND asset_id IS NOT NULL AND resource_group_id IS NULL AND finalized_revision_id IS NULL AND cover_revision_item_id IS NULL)
	OR (source_type = 'system' AND asset_id IS NOT NULL AND resource_group_id IS NULL AND finalized_revision_id IS NULL AND cover_revision_item_id IS NULL)
	OR (source_type = 'task_resource_group' AND asset_id IS NULL AND resource_group_id IS NOT NULL AND finalized_revision_id IS NOT NULL AND cover_revision_item_id IS NOT NULL)
  );

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_client_materials DROP FOREIGN KEY fk_aw_client_materials_cover_item;
ALTER TABLE asset_workbench_client_materials DROP FOREIGN KEY fk_aw_client_materials_revision;
ALTER TABLE asset_workbench_client_materials DROP FOREIGN KEY fk_aw_client_materials_resource_group;
ALTER TABLE asset_workbench_client_materials DROP INDEX idx_aw_client_materials_cover_revision;
ALTER TABLE asset_workbench_client_materials DROP INDEX idx_aw_client_materials_revision;
ALTER TABLE asset_workbench_client_materials DROP INDEX idx_aw_client_materials_resource_group;
ALTER TABLE asset_workbench_client_materials DROP COLUMN cover_revision_item_id;
ALTER TABLE asset_workbench_client_materials DROP COLUMN finalized_revision_id;
ALTER TABLE asset_workbench_client_materials DROP COLUMN resource_group_id;
ALTER TABLE asset_workbench_client_materials MODIFY COLUMN asset_id BIGINT NOT NULL;
DROP TABLE IF EXISTS search_reindex_outbox;
DROP TABLE IF EXISTS task_asset_group_search_documents;
DROP TABLE IF EXISTS task_erp_outbox;
ALTER TABLE upload_requests DROP INDEX idx_upload_requests_planning_draft;
ALTER TABLE upload_requests DROP COLUMN client_item_id;
ALTER TABLE upload_requests DROP COLUMN client_create_id;
DROP TABLE IF EXISTS task_planning_sku_revision_images;
ALTER TABLE task_planning_sku_details DROP FOREIGN KEY fk_task_planning_sku_detail_current_revision;
DROP TABLE IF EXISTS task_planning_sku_revisions;
DROP TABLE IF EXISTS task_planning_sku_details;
DROP TABLE IF EXISTS task_planning_settings;
ALTER TABLE task_sku_items DROP INDEX idx_task_sku_items_product_i_id;
ALTER TABLE task_sku_items DROP COLUMN sku_origin;
ALTER TABLE task_sku_items DROP COLUMN product_i_id;
DROP TABLE IF EXISTS code_rule_revision_sequences;
ALTER TABLE code_rules DROP FOREIGN KEY fk_code_rules_active_revision;
ALTER TABLE code_rules DROP INDEX idx_code_rules_active_revision;
ALTER TABLE code_rules DROP COLUMN active_revision_id;
DROP TABLE IF EXISTS code_rule_revisions;
-- ROLLBACK-END
