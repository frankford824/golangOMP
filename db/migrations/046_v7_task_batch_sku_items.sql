-- Migration: 046_v7_task_batch_sku_items.sql
-- Add task batch-SKU metadata plus task_sku_items / procurement_record_items tables.

ALTER TABLE tasks
  ADD COLUMN is_batch_task TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Whether the task contains multiple SKU child items',
  ADD COLUMN batch_item_count INT NOT NULL DEFAULT 0 COMMENT 'Number of SKU child items under the task',
  ADD COLUMN batch_mode VARCHAR(32) NOT NULL DEFAULT 'single' COMMENT 'single | multi_sku',
  ADD COLUMN primary_sku_code VARCHAR(64) NULL COMMENT 'Primary SKU used by compatibility list/read projections',
  ADD COLUMN sku_generation_status VARCHAR(32) NOT NULL DEFAULT 'not_applicable' COMMENT 'not_applicable | pending | partial | completed | failed';

UPDATE tasks
SET
  batch_item_count = CASE WHEN COALESCE(sku_code, '') = '' THEN 0 ELSE 1 END,
  primary_sku_code = CASE WHEN COALESCE(primary_sku_code, '') = '' THEN sku_code ELSE primary_sku_code END,
  sku_generation_status = CASE
    WHEN task_type IN ('new_product_development', 'purchase_task') THEN 'completed'
    ELSE 'not_applicable'
  END
WHERE batch_item_count = 0 OR COALESCE(primary_sku_code, '') = '' OR sku_generation_status = 'not_applicable';

CREATE TABLE IF NOT EXISTS task_sku_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  sequence_no INT NOT NULL,
  sku_code VARCHAR(64) NOT NULL,
  sku_status VARCHAR(32) NOT NULL DEFAULT 'generated',
  product_id BIGINT NULL,
  erp_product_id VARCHAR(64) NULL,
  product_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
  product_short_name VARCHAR(255) NOT NULL DEFAULT '',
  category_code VARCHAR(64) NOT NULL DEFAULT '',
  material_mode VARCHAR(32) NOT NULL DEFAULT '',
  cost_price_mode VARCHAR(32) NOT NULL DEFAULT '',
  quantity BIGINT NULL,
  base_sale_price DECIMAL(12,2) NULL,
  design_requirement TEXT NOT NULL,
  variant_json JSON NULL,
  dedupe_key VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_sku_items_sku_code (sku_code),
  UNIQUE KEY uq_task_sku_items_task_dedupe (task_id, dedupe_key),
  UNIQUE KEY uq_task_sku_items_task_sequence (task_id, sequence_no),
  KEY idx_task_sku_items_task_id (task_id),
  CONSTRAINT fk_task_sku_items_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Task-level SKU child items for single and multi-SKU task creation';

CREATE TABLE IF NOT EXISTS procurement_record_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  procurement_record_id BIGINT NOT NULL,
  task_id BIGINT NOT NULL,
  task_sku_item_id BIGINT NOT NULL,
  sku_code VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  quantity BIGINT NULL,
  cost_price DECIMAL(12,2) NULL,
  base_sale_price DECIMAL(12,2) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_procurement_record_items_task_sku_item_id (task_sku_item_id),
  KEY idx_procurement_record_items_record_id (procurement_record_id),
  KEY idx_procurement_record_items_task_id (task_id),
  CONSTRAINT fk_procurement_record_items_record FOREIGN KEY (procurement_record_id) REFERENCES procurement_records (id),
  CONSTRAINT fk_procurement_record_items_task FOREIGN KEY (task_id) REFERENCES tasks (id),
  CONSTRAINT fk_procurement_record_items_task_sku_item FOREIGN KEY (task_sku_item_id) REFERENCES task_sku_items (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Procurement child items aligned with task_sku_items for purchase tasks';
