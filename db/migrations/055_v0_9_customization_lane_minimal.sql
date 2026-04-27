-- Migration: 055_v0_9_customization_lane_minimal.sql
-- Purpose: extend customization lane with order matching and dedicated operator role support.
-- NOTE: MySQL 8.x does not support `ADD COLUMN IF NOT EXISTS`; operator must ensure idempotency by checking `SHOW COLUMNS` before running on a live DB.

ALTER TABLE customization_jobs
  ADD COLUMN order_no VARCHAR(64) NOT NULL DEFAULT '' AFTER task_id;

CREATE INDEX IF NOT EXISTS idx_customization_jobs_order_no ON customization_jobs (order_no);
