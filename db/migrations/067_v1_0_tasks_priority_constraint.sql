ALTER TABLE tasks
  ADD CONSTRAINT chk_tasks_priority_v1 CHECK (priority IN ('low', 'normal', 'high', 'critical')),
  ADD KEY idx_tasks_priority_created (priority, created_at);

-- ROLLBACK-BEGIN
ALTER TABLE tasks DROP INDEX idx_tasks_priority_created;
ALTER TABLE tasks DROP CHECK chk_tasks_priority_v1;
-- ROLLBACK-END
