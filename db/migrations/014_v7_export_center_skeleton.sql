-- Migration: 014_v7_export_center_skeleton.sql
-- Adds minimal export-center persistence without real file generation/storage.

CREATE TABLE export_jobs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  template_key VARCHAR(64) NOT NULL DEFAULT '',
  export_type VARCHAR(64) NOT NULL,
  source_query_type VARCHAR(64) NOT NULL,
  source_filters_json TEXT NOT NULL,
  normalized_filters_json TEXT NOT NULL,
  query_template_json TEXT NOT NULL,
  requested_by_actor_id BIGINT NOT NULL,
  requested_by_roles_json TEXT NOT NULL,
  requested_by_source VARCHAR(64) NOT NULL DEFAULT '',
  requested_by_auth_mode VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  result_ref_json TEXT NOT NULL,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  finished_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX idx_export_jobs_status_created_at ON export_jobs (status, created_at);
CREATE INDEX idx_export_jobs_source_query_created_at ON export_jobs (source_query_type, created_at);
CREATE INDEX idx_export_jobs_requested_by_created_at ON export_jobs (requested_by_actor_id, created_at);
