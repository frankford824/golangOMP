-- Migration: 007_v7_procurement_records.sql
-- Adds explicit procurement persistence for purchase-task preparation.

CREATE TABLE IF NOT EXISTS procurement_records (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  task_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'preparing' COMMENT 'preparing | ready | completed',
  procurement_price DECIMAL(10,2) NULL,
  supplier_name VARCHAR(255) NOT NULL DEFAULT '',
  purchase_remark TEXT NULL,
  expected_delivery_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_procurement_records_task_id (task_id),
  CONSTRAINT fk_procurement_records_task FOREIGN KEY (task_id) REFERENCES tasks (id)
);

INSERT INTO procurement_records (
  task_id,
  status,
  procurement_price,
  supplier_name,
  purchase_remark,
  expected_delivery_at,
  created_at,
  updated_at
)
SELECT
  t.id,
  CASE
    WHEN td.procurement_price IS NOT NULL THEN 'ready'
    ELSE 'preparing'
  END,
  td.procurement_price,
  '',
  '',
  NULL,
  CURRENT_TIMESTAMP,
  CURRENT_TIMESTAMP
FROM tasks t
JOIN task_details td ON td.task_id = t.id
LEFT JOIN procurement_records pr ON pr.task_id = t.id
WHERE t.task_type = 'purchase_task'
  AND pr.task_id IS NULL
  AND td.procurement_price IS NOT NULL;
