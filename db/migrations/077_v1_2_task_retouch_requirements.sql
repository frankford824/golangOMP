-- Migration: 077_v1_2_task_retouch_requirements.sql
-- Phase 1A: structured retouch_task requirement lines (text only; no asset binding).

CREATE TABLE IF NOT EXISTS task_retouch_requirements (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  description TEXT NOT NULL,
  sku_code VARCHAR(64) NULL,
  spec VARCHAR(255) NULL,
  remark TEXT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_by BIGINT NULL,
  updated_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  PRIMARY KEY (id),
  KEY idx_retouch_req_task (task_id),
  KEY idx_retouch_req_task_sort (task_id, sort_order, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='P图任务需求明细（Phase 1A 仅文字）';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS task_retouch_requirements;
-- ROLLBACK-END
