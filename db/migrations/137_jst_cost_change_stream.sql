-- Migration: 137_jst_cost_change_stream.sql
-- Capture immutable current-cost changes from the independently maintained
-- 8082 jst_inventory table so the 8081 Bridge can expose an auditable feed.
-- Production prerequisite: jst_inventory already exists in the shared jst_erp
-- database. This migration intentionally does not take ownership of that table.

CREATE TABLE IF NOT EXISTS jst_cost_changes (
  id BIGINT NOT NULL AUTO_INCREMENT,
  sku_id VARCHAR(100) NOT NULL,
  sku_type VARCHAR(20) NULL,
  old_cost_price DECIMAL(14,4) NULL,
  new_cost_price DECIMAL(14,4) NULL,
  source_modified_at DATETIME(3) NULL,
  changed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_jst_cost_changes_changed_id (changed_at, id),
  KEY idx_jst_cost_changes_sku_changed (sku_id, changed_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Immutable cost changes captured from the 8082 jst_inventory projection';

DROP TRIGGER IF EXISTS trg_jst_inventory_cost_insert;
CREATE TRIGGER trg_jst_inventory_cost_insert
AFTER INSERT ON jst_inventory
FOR EACH ROW
INSERT INTO jst_cost_changes (
  sku_id, sku_type, old_cost_price, new_cost_price, source_modified_at, changed_at
)
SELECT
  NEW.sku_id, NULLIF(TRIM(NEW.sku_type), ''), NULL, NEW.cost_price,
  NEW.local_updated_at, CURRENT_TIMESTAMP(3)
WHERE NEW.cost_price IS NOT NULL;

DROP TRIGGER IF EXISTS trg_jst_inventory_cost_update;
CREATE TRIGGER trg_jst_inventory_cost_update
AFTER UPDATE ON jst_inventory
FOR EACH ROW
INSERT INTO jst_cost_changes (
  sku_id, sku_type, old_cost_price, new_cost_price, source_modified_at, changed_at
)
SELECT
  NEW.sku_id, NULLIF(TRIM(NEW.sku_type), ''), OLD.cost_price, NEW.cost_price,
  NEW.local_updated_at, CURRENT_TIMESTAMP(3)
WHERE NOT (OLD.cost_price <=> NEW.cost_price);

-- ROLLBACK-BEGIN
DROP TRIGGER IF EXISTS trg_jst_inventory_cost_update;
DROP TRIGGER IF EXISTS trg_jst_inventory_cost_insert;
DROP TABLE IF EXISTS jst_cost_changes;
-- ROLLBACK-END
