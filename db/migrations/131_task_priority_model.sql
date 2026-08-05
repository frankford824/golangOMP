-- Migration: 131_task_priority_model.sql
-- Collapse the legacy low/normal/high/critical scale into the operational
-- normal/high/drawing model. Preserve rewritten values for deterministic rollback.

CREATE TABLE IF NOT EXISTS migration_131_task_priority_backup (
  task_id BIGINT NOT NULL,
  original_priority VARCHAR(32) NOT NULL,
  PRIMARY KEY (task_id),
  CONSTRAINT fk_migration_131_task_priority_task
    FOREIGN KEY (task_id) REFERENCES tasks(id)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO migration_131_task_priority_backup (task_id, original_priority)
SELECT id, priority
FROM tasks
WHERE priority IN ('low', 'critical')
ON DUPLICATE KEY UPDATE original_priority = VALUES(original_priority);

ALTER TABLE tasks DROP CHECK chk_tasks_priority_v1;

UPDATE tasks SET priority = 'normal' WHERE priority = 'low';
UPDATE tasks SET priority = 'high' WHERE priority = 'critical';

ALTER TABLE tasks
  ADD CONSTRAINT chk_tasks_priority_v1
  CHECK (priority IN ('normal', 'high', 'drawing'));

-- ROLLBACK-BEGIN
ALTER TABLE tasks DROP CHECK chk_tasks_priority_v1;

UPDATE tasks task_row
JOIN migration_131_task_priority_backup backup_row ON backup_row.task_id = task_row.id
SET task_row.priority = backup_row.original_priority;

ALTER TABLE tasks
  ADD CONSTRAINT chk_tasks_priority_v1
  CHECK (priority IN ('low', 'normal', 'high', 'critical'));

DROP TABLE migration_131_task_priority_backup;
-- ROLLBACK-END
