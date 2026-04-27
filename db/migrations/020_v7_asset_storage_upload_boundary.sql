-- Migration: 020_v7_asset_storage_upload_boundary.sql
-- V7 Step-37: task asset storage/upload adapter placeholder boundary.

CREATE TABLE IF NOT EXISTS upload_requests (
  request_id        VARCHAR(64)  NOT NULL,
  owner_type        VARCHAR(32)  NOT NULL,
  owner_id          BIGINT       NOT NULL,
  task_asset_type   VARCHAR(32)  NULL,
  storage_adapter   VARCHAR(32)  NOT NULL,
  ref_type          VARCHAR(32)  NOT NULL,
  file_name         VARCHAR(255) NOT NULL DEFAULT '',
  mime_type         VARCHAR(255) NOT NULL DEFAULT '',
  file_size         BIGINT       NULL,
  checksum_hint     VARCHAR(255) NOT NULL DEFAULT '',
  status            VARCHAR(32)  NOT NULL,
  is_placeholder    TINYINT(1)   NOT NULL DEFAULT 1,
  bound_asset_id    BIGINT       NULL,
  bound_ref_id      VARCHAR(64)  NOT NULL DEFAULT '',
  remark            VARCHAR(255) NOT NULL DEFAULT '',
  created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (request_id),
  KEY idx_upload_requests_owner (owner_type, owner_id),
  KEY idx_upload_requests_status (status),
  KEY idx_upload_requests_bound_asset (bound_asset_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 placeholder upload request boundary';

CREATE TABLE IF NOT EXISTS asset_storage_refs (
  ref_id             VARCHAR(64)  NOT NULL,
  asset_id           BIGINT       NULL,
  owner_type         VARCHAR(32)  NOT NULL,
  owner_id           BIGINT       NOT NULL,
  upload_request_id  VARCHAR(64)  NULL,
  storage_adapter    VARCHAR(32)  NOT NULL,
  ref_type           VARCHAR(32)  NOT NULL,
  ref_key            VARCHAR(255) NOT NULL,
  file_name          VARCHAR(255) NOT NULL DEFAULT '',
  mime_type          VARCHAR(255) NOT NULL DEFAULT '',
  file_size          BIGINT       NULL,
  is_placeholder     TINYINT(1)   NOT NULL DEFAULT 1,
  checksum_hint      VARCHAR(255) NOT NULL DEFAULT '',
  status             VARCHAR(32)  NOT NULL,
  created_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (ref_id),
  KEY idx_asset_storage_refs_asset (asset_id),
  KEY idx_asset_storage_refs_owner (owner_type, owner_id),
  KEY idx_asset_storage_refs_upload_request (upload_request_id),
  KEY idx_asset_storage_refs_status (status),
  CONSTRAINT fk_asset_storage_refs_upload_request FOREIGN KEY (upload_request_id) REFERENCES upload_requests (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 placeholder storage reference boundary';

ALTER TABLE task_assets
  ADD COLUMN upload_request_id VARCHAR(64) NULL AFTER version_no,
  ADD COLUMN storage_ref_id VARCHAR(64) NULL AFTER upload_request_id,
  ADD COLUMN mime_type VARCHAR(255) NULL AFTER file_name,
  ADD COLUMN file_size BIGINT NULL AFTER mime_type;

ALTER TABLE task_assets
  ADD KEY idx_task_assets_upload_request_id (upload_request_id),
  ADD KEY idx_task_assets_storage_ref_id (storage_ref_id);
