-- Migration: 049_v7_batch_sku_asset_scope.sql
-- Add formal per-SKU reference refs and design-asset SKU scope metadata.

ALTER TABLE task_sku_items
  ADD COLUMN reference_file_refs_json TEXT NULL COMMENT 'JSON array of per-SKU reference file refs' AFTER variant_json;

UPDATE task_sku_items
SET reference_file_refs_json = '[]'
WHERE reference_file_refs_json IS NULL;

ALTER TABLE task_sku_items
  MODIFY COLUMN reference_file_refs_json TEXT NOT NULL COMMENT 'JSON array of per-SKU reference file refs';

ALTER TABLE design_assets
  ADD COLUMN scope_sku_code VARCHAR(64) NULL COMMENT 'Optional SKU scope for batch-task design assets' AFTER asset_no;

ALTER TABLE design_assets
  ADD KEY idx_design_assets_task_scope_sku (task_id, scope_sku_code);

ALTER TABLE task_assets
  ADD COLUMN scope_sku_code VARCHAR(64) NULL COMMENT 'Optional SKU scope for version rows aligned with design_assets.scope_sku_code' AFTER asset_id;

ALTER TABLE task_assets
  ADD KEY idx_task_assets_task_scope_sku (task_id, scope_sku_code);

ALTER TABLE upload_requests
  ADD COLUMN target_sku_code VARCHAR(64) NULL COMMENT 'Optional target SKU scope captured at upload-session creation' AFTER asset_id;

ALTER TABLE upload_requests
  ADD KEY idx_upload_requests_target_sku_code (target_sku_code);
