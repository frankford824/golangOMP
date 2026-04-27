-- Migration: 056_v0_9_customization_review_reference_split.sql
-- Purpose: separate reviewer-stage reference pricing from execution-stage frozen settlement snapshot.
-- NOTE: MySQL 8.x does not support `ADD COLUMN IF NOT EXISTS`; operator must ensure idempotency by checking `SHOW COLUMNS` before running on a live DB.

ALTER TABLE customization_jobs
  ADD COLUMN review_reference_unit_price DECIMAL(12,2) NULL AFTER customization_level_name,
  ADD COLUMN review_reference_weight_factor DECIMAL(10,4) NULL AFTER review_reference_unit_price;
