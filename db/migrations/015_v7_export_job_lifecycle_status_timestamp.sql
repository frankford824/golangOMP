-- Migration: 015_v7_export_job_lifecycle_status_timestamp.sql
-- Adds explicit latest lifecycle timestamp for export jobs.

ALTER TABLE export_jobs
  ADD COLUMN status_updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER created_at;

UPDATE export_jobs
SET status_updated_at = COALESCE(finished_at, updated_at, created_at);

CREATE INDEX idx_export_jobs_status_updated_at ON export_jobs (status_updated_at);
