-- Migration: 006_v7_task_business_info.sql
-- Adds PRD V2.0 front-loaded product/business-info and cost-maintenance fields.

ALTER TABLE task_details
  ADD COLUMN category VARCHAR(128) NOT NULL DEFAULT '' AFTER risk_flags_json,
  ADD COLUMN spec_text TEXT NOT NULL AFTER category,
  ADD COLUMN material VARCHAR(255) NOT NULL DEFAULT '' AFTER spec_text,
  ADD COLUMN size_text VARCHAR(255) NOT NULL DEFAULT '' AFTER material,
  ADD COLUMN craft_text VARCHAR(255) NOT NULL DEFAULT '' AFTER size_text,
  ADD COLUMN procurement_price DECIMAL(10,2) NULL AFTER craft_text,
  ADD COLUMN cost_price DECIMAL(10,2) NULL AFTER procurement_price,
  ADD COLUMN filed_at DATETIME NULL AFTER cost_price;
