-- Migration: 083_v1_4_asset_flow_review_and_cleanup.sql
-- Persist asset-version review state so asset center/global search can show
-- whether a file is currently approved for warehouse use without inferring from
-- task status at read time.

ALTER TABLE task_assets
  ADD COLUMN flow_review_status VARCHAR(32) NOT NULL DEFAULT 'not_applicable' AFTER archived_by,
  ADD COLUMN approved_at DATETIME NULL AFTER flow_review_status,
  ADD COLUMN approved_by BIGINT NULL AFTER approved_at,
  ADD COLUMN rejected_at DATETIME NULL AFTER approved_by,
  ADD COLUMN rejected_by BIGINT NULL AFTER rejected_at,
  ADD COLUMN superseded_by_version_id BIGINT NULL AFTER rejected_by,
  ADD COLUMN superseded_at DATETIME NULL AFTER superseded_by_version_id,
  ADD COLUMN cleanup_after_at DATETIME NULL AFTER superseded_at,
  ADD COLUMN source_asset_version_id BIGINT NULL AFTER cleanup_after_at,
  ADD KEY idx_task_assets_flow_review (flow_review_status, asset_type, task_id),
  ADD KEY idx_task_assets_cleanup_after (cleanup_after_at, cleaned_at, deleted_at),
  ADD KEY idx_task_assets_superseded (superseded_by_version_id, superseded_at),
  ADD KEY idx_task_assets_source_asset_version (source_asset_version_id);

UPDATE task_assets ta
LEFT JOIN tasks t ON t.id = ta.task_id
LEFT JOIN design_assets da ON da.id = ta.asset_id
SET
  ta.flow_review_status = CASE
    WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return')
      AND ta.cleaned_at IS NOT NULL THEN 'cleaned'
    WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return')
      AND da.current_version_id = ta.id
      AND t.task_status IN ('PendingWarehouseReceive', 'PendingClose', 'Completed') THEN 'approved'
    WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return')
      AND da.current_version_id IS NOT NULL
      AND da.current_version_id <> ta.id THEN 'superseded'
    WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return') THEN 'pending_review'
    ELSE 'not_applicable'
  END,
  ta.approved_at = CASE
    WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return')
      AND da.current_version_id = ta.id
      AND t.task_status IN ('PendingWarehouseReceive', 'PendingClose', 'Completed')
    THEN COALESCE(ta.approved_at, t.updated_at)
    ELSE ta.approved_at
  END
WHERE ta.deleted_at IS NULL;

-- ROLLBACK-BEGIN
ALTER TABLE task_assets DROP INDEX idx_task_assets_source_asset_version;
ALTER TABLE task_assets DROP INDEX idx_task_assets_superseded;
ALTER TABLE task_assets DROP INDEX idx_task_assets_cleanup_after;
ALTER TABLE task_assets DROP INDEX idx_task_assets_flow_review;
ALTER TABLE task_assets DROP COLUMN source_asset_version_id;
ALTER TABLE task_assets DROP COLUMN cleanup_after_at;
ALTER TABLE task_assets DROP COLUMN superseded_at;
ALTER TABLE task_assets DROP COLUMN superseded_by_version_id;
ALTER TABLE task_assets DROP COLUMN rejected_by;
ALTER TABLE task_assets DROP COLUMN rejected_at;
ALTER TABLE task_assets DROP COLUMN approved_by;
ALTER TABLE task_assets DROP COLUMN approved_at;
ALTER TABLE task_assets DROP COLUMN flow_review_status;
-- ROLLBACK-END
