-- Migration: 036_v7_design_asset_flow_semantics.sql
-- Formalize canonical design-asset semantics around reference/source/delivery/preview.

ALTER TABLE task_assets
  ADD COLUMN upload_mode VARCHAR(32) NULL AFTER asset_version_no;

UPDATE design_assets
SET asset_type = 'source'
WHERE asset_type = 'original';

UPDATE design_assets
SET asset_type = 'delivery'
WHERE asset_type IN ('draft', 'revised', 'final', 'outsource_return');

UPDATE task_assets
SET asset_type = 'source'
WHERE asset_type = 'original';

UPDATE task_assets
SET asset_type = 'delivery'
WHERE asset_type IN ('draft', 'revised', 'final', 'outsource_return');

UPDATE upload_requests
SET task_asset_type = 'source'
WHERE task_asset_type = 'original';

UPDATE upload_requests
SET task_asset_type = 'delivery'
WHERE task_asset_type IN ('draft', 'revised', 'final', 'outsource_return');

UPDATE task_assets ta
LEFT JOIN upload_requests ur ON ur.request_id = ta.upload_request_id
SET ta.upload_mode = COALESCE(
  ta.upload_mode,
  ur.upload_mode,
  CASE
    WHEN ta.asset_type = 'reference' THEN 'small'
    WHEN ta.asset_type = 'source' THEN 'multipart'
    ELSE 'small'
  END
)
WHERE ta.upload_mode IS NULL;
