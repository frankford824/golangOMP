-- Migration: 034_v7_design_asset_center_boundary.sql
-- V7 Step-67: task design asset center boundary for asset/version/upload-session integration.

CREATE TABLE IF NOT EXISTS design_assets (
  id                  BIGINT       NOT NULL AUTO_INCREMENT,
  task_id             BIGINT       NOT NULL,
  asset_no            VARCHAR(64)  NOT NULL,
  asset_type          VARCHAR(32)  NOT NULL,
  current_version_id  BIGINT       NULL,
  created_by          BIGINT       NOT NULL,
  created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_design_assets_task_asset_no (task_id, asset_no),
  KEY idx_design_assets_task_id (task_id),
  KEY idx_design_assets_current_version_id (current_version_id),
  CONSTRAINT fk_design_assets_task FOREIGN KEY (task_id) REFERENCES tasks (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 task-scoped design asset center root records';

ALTER TABLE task_assets
  ADD COLUMN asset_id BIGINT NULL AFTER task_id,
  ADD COLUMN asset_version_no INT NULL AFTER version_no,
  ADD COLUMN original_filename VARCHAR(255) NULL AFTER file_name,
  ADD COLUMN storage_key VARCHAR(255) NULL AFTER file_path,
  ADD COLUMN upload_status VARCHAR(32) NULL AFTER whole_hash,
  ADD COLUMN preview_status VARCHAR(32) NULL AFTER upload_status,
  ADD COLUMN uploaded_at DATETIME NULL AFTER uploaded_by;

ALTER TABLE task_assets
  ADD KEY idx_task_assets_asset_id (asset_id),
  ADD KEY idx_task_assets_asset_version_no (asset_id, asset_version_no);

UPDATE task_assets
SET
  original_filename = COALESCE(original_filename, file_name),
  asset_version_no = COALESCE(asset_version_no, version_no),
  storage_key = COALESCE(storage_key, file_path),
  upload_status = COALESCE(upload_status, 'uploaded'),
  preview_status = COALESCE(preview_status, 'pending'),
  uploaded_at = COALESCE(uploaded_at, created_at)
WHERE original_filename IS NULL
   OR asset_version_no IS NULL
   OR upload_status IS NULL
   OR preview_status IS NULL
   OR uploaded_at IS NULL;

ALTER TABLE upload_requests
  ADD COLUMN task_id BIGINT NULL AFTER owner_id,
  ADD COLUMN asset_id BIGINT NULL AFTER task_id,
  ADD COLUMN upload_mode VARCHAR(32) NULL AFTER storage_adapter,
  ADD COLUMN expected_size BIGINT NULL AFTER file_size,
  ADD COLUMN storage_provider VARCHAR(32) NULL AFTER ref_type,
  ADD COLUMN session_status VARCHAR(32) NULL AFTER status,
  ADD COLUMN remote_upload_id VARCHAR(128) NULL AFTER session_status,
  ADD COLUMN created_by BIGINT NULL AFTER bound_ref_id,
  ADD COLUMN expires_at DATETIME NULL AFTER created_by;

ALTER TABLE upload_requests
  ADD KEY idx_upload_requests_task_id (task_id),
  ADD KEY idx_upload_requests_asset_id (asset_id),
  ADD KEY idx_upload_requests_session_status (session_status);

UPDATE upload_requests
SET
  task_id = CASE WHEN owner_type = 'task' THEN owner_id ELSE task_id END,
  upload_mode = COALESCE(upload_mode, 'small'),
  expected_size = COALESCE(expected_size, file_size),
  storage_provider = COALESCE(storage_provider, 'nas'),
  session_status = COALESCE(
    session_status,
    CASE status
      WHEN 'bound' THEN 'completed'
      WHEN 'cancelled' THEN 'cancelled'
      WHEN 'expired' THEN 'expired'
      ELSE 'created'
    END
  )
WHERE task_id IS NULL
   OR upload_mode IS NULL
   OR storage_provider IS NULL
   OR session_status IS NULL;
