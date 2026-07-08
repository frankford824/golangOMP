-- Migration: 113_external_asset_source_modified_at.sql
-- Track source file modified time from AList/NAS scans so OSS copies can be
-- re-queued when a local file is replaced in place without changing its path.

ALTER TABLE external_asset_records
  ADD COLUMN source_modified_at DATETIME NULL;

-- ROLLBACK-BEGIN
ALTER TABLE external_asset_records DROP COLUMN source_modified_at;
-- ROLLBACK-END
