CREATE TABLE IF NOT EXISTS task_module_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_module_id BIGINT NOT NULL,
  event_type VARCHAR(48) NOT NULL,
  from_state VARCHAR(48) NULL,
  to_state VARCHAR(48) NULL,
  actor_id BIGINT NULL,
  actor_snapshot JSON NULL,
  payload JSON NOT NULL DEFAULT (JSON_OBJECT()),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_task_module_events_module_created (task_module_id, created_at),
  KEY idx_task_module_events_type_created (event_type, created_at),
  CONSTRAINT fk_task_module_events_module FOREIGN KEY (task_module_id) REFERENCES task_modules (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS task_module_events;
-- ROLLBACK-END
