-- Migration: 004_v7_task_assets_assign_submit.sql
-- V7 Step-04: task_assets for task-scoped attachment/version timeline.
-- Strategy: additive; no V6 or prior V7 tables dropped.

CREATE TABLE IF NOT EXISTS task_assets (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  task_id      BIGINT      NOT NULL,
  asset_type   VARCHAR(32) NOT NULL COMMENT 'reference | draft | revised | final | outsource_return',
  version_no   INT         NOT NULL COMMENT 'Monotonic per task timeline sequence',
  file_name    VARCHAR(255) NOT NULL DEFAULT '',
  file_path    VARCHAR(1024) NULL,
  whole_hash   VARCHAR(255) NULL,
  uploaded_by  BIGINT      NOT NULL,
  remark       TEXT        NOT NULL,
  created_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_assets_task_version (task_id, version_no),
  KEY idx_task_assets_task_id (task_id),
  KEY idx_task_assets_type (asset_type),
  CONSTRAINT fk_task_assets_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 task-scoped asset timeline';
