CREATE TABLE IF NOT EXISTS task_reference_asset_bindings (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  ref_id VARCHAR(64) NOT NULL,
  design_asset_id BIGINT NOT NULL,
  task_asset_id BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_reference_asset_bindings_task_ref (task_id, ref_id),
  KEY idx_task_reference_asset_bindings_task (task_id),
  KEY idx_task_reference_asset_bindings_design_asset (design_asset_id),
  KEY idx_task_reference_asset_bindings_task_asset (task_asset_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS task_reference_asset_bindings;
-- ROLLBACK-END
