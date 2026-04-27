CREATE TABLE IF NOT EXISTS task_customization_orders (
  task_id BIGINT NOT NULL,
  online_order_no VARCHAR(64) NOT NULL DEFAULT '',
  requirement_note TEXT NOT NULL,
  ordered_at DATETIME NULL,
  erp_product_code VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (task_id),
  KEY idx_task_customization_orders_order_no (online_order_no),
  KEY idx_task_customization_orders_erp_code (erp_product_code),
  CONSTRAINT fk_task_customization_orders_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS task_customization_orders;
-- ROLLBACK-END
