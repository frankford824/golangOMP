-- Dedicated, append-only maintenance evidence. Existing prices are untouched.
CREATE TABLE cost_maintenance_logs (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 task_id BIGINT NOT NULL,
 sku_item_id BIGINT NOT NULL,
 sku_code VARCHAR(64) NOT NULL,
 action VARCHAR(16) NOT NULL,
 reason_code VARCHAR(32) NOT NULL DEFAULT '',
 operator_id BIGINT NOT NULL,
 payload JSON NOT NULL,
 created_at DATETIME(3) NOT NULL,
 KEY idx_cost_maintenance_time (created_at,id),
 KEY idx_cost_maintenance_sku (sku_code,id),
 KEY idx_cost_maintenance_reason (reason_code,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
