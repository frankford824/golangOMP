-- Migration: 080_v1_3_cost_precision_3dp.sql
-- Preserve OMP-calculated cost amounts to 3 decimal places across task,
-- procurement, override audit, and SKU traceability storage.

ALTER TABLE task_details
  MODIFY COLUMN procurement_price DECIMAL(12,3) NULL,
  MODIFY COLUMN cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,3) NULL;

ALTER TABLE procurement_records
  MODIFY COLUMN procurement_price DECIMAL(12,3) NULL;

ALTER TABLE procurement_record_items
  MODIFY COLUMN cost_price DECIMAL(12,3) NULL;

ALTER TABLE task_sku_items
  MODIFY COLUMN cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,3) NULL;

ALTER TABLE cost_override_events
  MODIFY COLUMN previous_estimated_cost DECIMAL(12,3) NULL,
  MODIFY COLUMN previous_cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN override_cost DECIMAL(12,3) NULL,
  MODIFY COLUMN result_cost_price DECIMAL(12,3) NULL;

ALTER TABLE cost_rules
  MODIFY COLUMN base_price DECIMAL(12,3) NULL,
  MODIFY COLUMN surcharge_amount DECIMAL(12,3) NULL,
  MODIFY COLUMN special_process_price DECIMAL(12,3) NULL;

ALTER TABLE omp_sku_records
  MODIFY COLUMN cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,3) NULL;

ALTER TABLE omp_sku_cost_snapshots
  MODIFY COLUMN cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,3) NULL;

ALTER TABLE omp_sku_erp_trace_logs
  MODIFY COLUMN request_cost_price DECIMAL(12,3) NULL,
  MODIFY COLUMN response_cost_price DECIMAL(12,3) NULL;

-- ROLLBACK-BEGIN
ALTER TABLE omp_sku_erp_trace_logs
  MODIFY COLUMN request_cost_price DECIMAL(12,2) NULL,
  MODIFY COLUMN response_cost_price DECIMAL(12,2) NULL;

ALTER TABLE omp_sku_cost_snapshots
  MODIFY COLUMN cost_price DECIMAL(12,2) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,2) NULL;

ALTER TABLE omp_sku_records
  MODIFY COLUMN cost_price DECIMAL(12,2) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,2) NULL;

ALTER TABLE cost_rules
  MODIFY COLUMN base_price DECIMAL(10,2) NULL,
  MODIFY COLUMN surcharge_amount DECIMAL(10,2) NULL,
  MODIFY COLUMN special_process_price DECIMAL(10,2) NULL;

ALTER TABLE cost_override_events
  MODIFY COLUMN previous_estimated_cost DECIMAL(10,2) NULL,
  MODIFY COLUMN previous_cost_price DECIMAL(10,2) NULL,
  MODIFY COLUMN override_cost DECIMAL(10,2) NULL,
  MODIFY COLUMN result_cost_price DECIMAL(10,2) NULL;

ALTER TABLE task_sku_items
  MODIFY COLUMN cost_price DECIMAL(12,2) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(12,2) NULL;

ALTER TABLE procurement_record_items
  MODIFY COLUMN cost_price DECIMAL(12,2) NULL;

ALTER TABLE procurement_records
  MODIFY COLUMN procurement_price DECIMAL(10,2) NULL;

ALTER TABLE task_details
  MODIFY COLUMN procurement_price DECIMAL(10,2) NULL,
  MODIFY COLUMN cost_price DECIMAL(10,2) NULL,
  MODIFY COLUMN estimated_cost DECIMAL(10,2) NULL;
-- ROLLBACK-END
