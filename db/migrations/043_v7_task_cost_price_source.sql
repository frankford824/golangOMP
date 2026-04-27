-- Migration: 043_v7_task_cost_price_source.sql
-- Add cost_price_source on task_details for explicit cost origin tracking.

ALTER TABLE task_details
  ADD COLUMN cost_price_source VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'cost price source tag';

