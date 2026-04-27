-- Migration: 008_v7_procurement_flow_skeleton.sql
-- Purpose:
--   1. Extend procurement_records with minimal quantity support.
--   2. Normalize the skeleton status model to draft / prepared / in_progress / completed.

ALTER TABLE procurement_records
  ADD COLUMN quantity BIGINT NULL AFTER procurement_price;

UPDATE procurement_records
SET status = CASE status
  WHEN 'preparing' THEN 'draft'
  WHEN 'ready' THEN 'prepared'
  ELSE status
END;

ALTER TABLE procurement_records
  MODIFY COLUMN status VARCHAR(32) NOT NULL DEFAULT 'draft' COMMENT 'draft | prepared | in_progress | completed';
