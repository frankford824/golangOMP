-- Migration: 070_v1_1_task_sku_item_filing_projection.sql
-- Add per-SKU ERP filing projection for batch new-product tasks.

ALTER TABLE task_sku_items
  ADD COLUMN filing_status VARCHAR(32) NOT NULL DEFAULT 'pending_filing' COMMENT 'pending_filing | filing | filed | filing_failed' AFTER erp_product_id,
  ADD COLUMN erp_sync_status VARCHAR(32) NOT NULL DEFAULT 'pending_filing' COMMENT 'Per-SKU ERP sync projection, mirrors filing_status' AFTER filing_status,
  ADD COLUMN erp_sync_required TINYINT(1) NOT NULL DEFAULT 1 COMMENT 'Whether this SKU still needs ERP sync' AFTER erp_sync_status,
  ADD COLUMN erp_sync_version BIGINT NOT NULL DEFAULT 0 COMMENT 'Per-SKU ERP sync version inherited from task filing version' AFTER erp_sync_required,
  ADD COLUMN last_filed_at DATETIME NULL COMMENT 'Last successful ERP filing time for this SKU' AFTER erp_sync_version,
  ADD COLUMN filing_error_message TEXT NULL COMMENT 'Latest per-SKU ERP filing error' AFTER last_filed_at,
  ADD KEY idx_task_sku_items_filing_status (filing_status),
  ADD KEY idx_task_sku_items_erp_sync_required (erp_sync_required);

