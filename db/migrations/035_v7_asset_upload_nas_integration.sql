-- Migration: 035_v7_asset_upload_nas_integration.sql
-- V7 Step-68: persist NAS upload-service remote file identity and session sync metadata.

ALTER TABLE task_assets
  ADD COLUMN remote_file_id VARCHAR(128) NULL AFTER original_filename;

ALTER TABLE upload_requests
  ADD COLUMN remote_file_id VARCHAR(128) NULL AFTER remote_upload_id,
  ADD COLUMN last_synced_at DATETIME NULL AFTER expires_at;

ALTER TABLE upload_requests
  ADD KEY idx_upload_requests_remote_file_id (remote_file_id),
  ADD KEY idx_upload_requests_last_synced_at (last_synced_at);

UPDATE upload_requests
SET last_synced_at = COALESCE(last_synced_at, updated_at)
WHERE last_synced_at IS NULL;
