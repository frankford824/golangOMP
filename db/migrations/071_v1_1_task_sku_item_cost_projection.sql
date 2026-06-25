-- Migration: 071_v1_1_task_sku_item_cost_projection.sql
-- Add per-SKU cost projection fields for batch task cost maintenance and ERP filing.

ALTER TABLE task_sku_items
  ADD COLUMN cost_price DECIMAL(12,2) NULL AFTER base_sale_price,
  ADD COLUMN estimated_cost DECIMAL(12,2) NULL AFTER cost_price,
  ADD COLUMN cost_rule_id BIGINT NULL AFTER estimated_cost,
  ADD COLUMN cost_rule_name VARCHAR(255) NOT NULL DEFAULT '' AFTER cost_rule_id,
  ADD COLUMN cost_rule_source VARCHAR(64) NOT NULL DEFAULT '' AFTER cost_rule_name,
  ADD COLUMN matched_rule_version INT NULL AFTER cost_rule_source,
  ADD COLUMN prefill_source VARCHAR(64) NOT NULL DEFAULT '' AFTER matched_rule_version,
  ADD COLUMN prefill_at DATETIME NULL AFTER prefill_source,
  ADD COLUMN requires_manual_review TINYINT(1) NOT NULL DEFAULT 0 AFTER prefill_at,
  ADD COLUMN manual_cost_override TINYINT(1) NOT NULL DEFAULT 0 AFTER requires_manual_review,
  ADD COLUMN manual_cost_override_reason VARCHAR(255) NOT NULL DEFAULT '' AFTER manual_cost_override,
  ADD COLUMN override_actor VARCHAR(64) NOT NULL DEFAULT '' AFTER manual_cost_override_reason,
  ADD COLUMN override_at DATETIME NULL AFTER override_actor,
  ADD KEY idx_task_sku_items_cost_rule_id (cost_rule_id),
  ADD KEY idx_task_sku_items_manual_cost_override (manual_cost_override);
