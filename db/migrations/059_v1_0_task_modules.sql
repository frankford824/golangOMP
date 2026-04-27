CREATE TABLE IF NOT EXISTS task_modules (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  module_key VARCHAR(32) NOT NULL,
  state VARCHAR(48) NOT NULL,
  pool_team_code VARCHAR(64) NULL,
  claimed_by BIGINT NULL,
  claimed_team_code VARCHAR(64) NULL,
  claimed_at DATETIME NULL,
  actor_org_snapshot JSON NULL,
  entered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  terminal_at DATETIME NULL,
  data JSON NOT NULL DEFAULT (JSON_OBJECT()),
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_modules_task_module (task_id, module_key),
  KEY idx_task_modules_task (task_id),
  KEY idx_task_modules_pool (module_key, state, pool_team_code, task_id),
  KEY idx_task_modules_claim (claimed_by, state, updated_at),
  CONSTRAINT fk_task_modules_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS task_modules;
-- ROLLBACK-END
