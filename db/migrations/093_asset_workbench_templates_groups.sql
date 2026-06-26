-- Migration: 093_asset_workbench_templates_groups.sql
-- Two-identity asset workbench: work type templates, worker groups, assignment, and item template snapshots.

CREATE TABLE IF NOT EXISTS asset_workbench_groups (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_groups_name (name),
  KEY idx_aw_groups_enabled (enabled),
  KEY idx_aw_groups_created_by (created_by),
  CONSTRAINT fk_aw_groups_created_by FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_workbench_group_members (
  group_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, user_id),
  KEY idx_aw_group_members_user (user_id),
  CONSTRAINT fk_aw_group_members_group FOREIGN KEY (group_id) REFERENCES asset_workbench_groups(id),
  CONSTRAINT fk_aw_group_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_workbench_templates (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  category VARCHAR(64) NOT NULL DEFAULT '',
  difficulty_class VARCHAR(64) NOT NULL,
  worker_type VARCHAR(32) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_templates_name (name),
  KEY idx_aw_templates_lookup (enabled, worker_type, difficulty_class, sort_order),
  KEY idx_aw_templates_created_by (created_by),
  CONSTRAINT fk_aw_templates_created_by FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_workbench_template_assignments (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  template_id BIGINT NOT NULL,
  target_type VARCHAR(16) NOT NULL,
  target_id BIGINT NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  assigned_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_template_assignment (template_id, target_type, target_id),
  KEY idx_aw_template_assignment_target (target_type, target_id, enabled),
  KEY idx_aw_template_assignment_template (template_id, enabled),
  CONSTRAINT fk_aw_template_assignments_template FOREIGN KEY (template_id) REFERENCES asset_workbench_templates(id),
  CONSTRAINT fk_aw_template_assignments_assigned_by FOREIGN KEY (assigned_by) REFERENCES users(id),
  CONSTRAINT ck_aw_template_assignment_target CHECK (target_type IN ('user', 'group'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE asset_workbench_submission_items
  ADD COLUMN template_id BIGINT NULL AFTER order_no,
  ADD COLUMN template_name_snapshot VARCHAR(128) NOT NULL DEFAULT '' AFTER template_id,
  ADD COLUMN category_snapshot VARCHAR(64) NOT NULL DEFAULT '' AFTER template_name_snapshot,
  ADD KEY idx_aw_items_template (template_id),
  ADD CONSTRAINT fk_aw_items_template FOREIGN KEY (template_id) REFERENCES asset_workbench_templates(id);
