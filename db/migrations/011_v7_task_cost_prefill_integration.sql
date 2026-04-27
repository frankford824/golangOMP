-- Migration: 011_v7_task_cost_prefill_integration.sql
-- Extends task_details with minimal cost-prefill input/output persistence.

ALTER TABLE task_details
  ADD COLUMN width DECIMAL(10,4) NULL AFTER craft_text,
  ADD COLUMN height DECIMAL(10,4) NULL AFTER width,
  ADD COLUMN area DECIMAL(10,4) NULL AFTER height,
  ADD COLUMN quantity BIGINT NULL AFTER area,
  ADD COLUMN process VARCHAR(255) NOT NULL DEFAULT '' AFTER quantity,
  ADD COLUMN estimated_cost DECIMAL(10,2) NULL AFTER cost_price,
  ADD COLUMN requires_manual_review TINYINT(1) NOT NULL DEFAULT 0 AFTER cost_rule_source,
  ADD COLUMN manual_cost_override TINYINT(1) NOT NULL DEFAULT 0 AFTER requires_manual_review,
  ADD COLUMN manual_cost_override_reason VARCHAR(255) NOT NULL DEFAULT '' AFTER manual_cost_override;

CREATE INDEX idx_task_details_category_prefill ON task_details (category_code, requires_manual_review);
