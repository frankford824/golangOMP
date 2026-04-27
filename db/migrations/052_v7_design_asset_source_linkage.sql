-- Migration: 052_v7_design_asset_source_linkage.sql
-- Add explicit source linkage for backend-owned preview/thumb asset modeling.

ALTER TABLE design_assets
  ADD COLUMN source_asset_id BIGINT NULL COMMENT 'Optional source asset linkage for preview/thumb assets' AFTER asset_no;

ALTER TABLE design_assets
  ADD KEY idx_design_assets_source_asset_id (source_asset_id);

ALTER TABLE upload_requests
  ADD COLUMN source_asset_id BIGINT NULL COMMENT 'Optional source asset linkage carried by upload sessions for preview/thumb assets' AFTER asset_id;

ALTER TABLE upload_requests
  ADD KEY idx_upload_requests_source_asset_id (source_asset_id);
