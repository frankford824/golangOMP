CREATE TABLE IF NOT EXISTS reference_file_refs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  sku_item_id BIGINT NULL,
  ref_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  owner_module_key VARCHAR(32) NOT NULL,
  context VARCHAR(64) NULL,
  attached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_reference_file_refs_task_ref_sku (task_id, ref_id, sku_item_id),
  KEY idx_reference_file_refs_owner_task (owner_module_key, task_id),
  KEY idx_reference_file_refs_ref_id (ref_id),
  CONSTRAINT fk_reference_file_refs_task FOREIGN KEY (task_id) REFERENCES tasks (id),
  CONSTRAINT fk_reference_file_refs_ref FOREIGN KEY (ref_id) REFERENCES asset_storage_refs (ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS reference_file_refs;
-- ROLLBACK-END
