ALTER TABLE task_assets
  ADD COLUMN source_module_key VARCHAR(32) NOT NULL DEFAULT 'design',
  ADD COLUMN source_task_module_id BIGINT NULL,
  ADD COLUMN is_archived TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN archived_at DATETIME NULL,
  ADD COLUMN archived_by BIGINT NULL,
  ADD KEY idx_task_assets_source_task_module_id (source_task_module_id),
  ADD CONSTRAINT fk_task_assets_source_task_module FOREIGN KEY (source_task_module_id) REFERENCES task_modules (id);

-- ROLLBACK-BEGIN
ALTER TABLE task_assets DROP FOREIGN KEY fk_task_assets_source_task_module;
ALTER TABLE task_assets DROP INDEX idx_task_assets_source_task_module_id;
ALTER TABLE task_assets DROP COLUMN archived_by;
ALTER TABLE task_assets DROP COLUMN archived_at;
ALTER TABLE task_assets DROP COLUMN is_archived;
ALTER TABLE task_assets DROP COLUMN source_task_module_id;
ALTER TABLE task_assets DROP COLUMN source_module_key;
-- ROLLBACK-END
