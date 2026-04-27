ALTER TABLE task_assets
  ADD COLUMN cleaned_at DATETIME NULL,
  ADD COLUMN deleted_at DATETIME NULL,
  ADD KEY idx_task_assets_archived_deleted (is_archived, deleted_at);

-- ROLLBACK-BEGIN
ALTER TABLE task_assets DROP INDEX idx_task_assets_archived_deleted;
ALTER TABLE task_assets DROP COLUMN deleted_at;
ALTER TABLE task_assets DROP COLUMN cleaned_at;
-- ROLLBACK-END
