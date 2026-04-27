-- Migration: 013_v7_task_product_picker_integration.sql
-- Persists original-product picker provenance on task_details.

ALTER TABLE task_details
  ADD COLUMN source_product_id BIGINT NULL AFTER category_name,
  ADD COLUMN source_product_name VARCHAR(255) NOT NULL DEFAULT '' AFTER source_product_id,
  ADD COLUMN source_search_entry_code VARCHAR(64) NOT NULL DEFAULT '' AFTER source_product_name,
  ADD COLUMN source_match_type VARCHAR(64) NOT NULL DEFAULT '' AFTER source_search_entry_code,
  ADD COLUMN source_match_rule VARCHAR(255) NOT NULL DEFAULT '' AFTER source_match_type,
  ADD COLUMN matched_category_code VARCHAR(64) NOT NULL DEFAULT '' AFTER source_match_rule,
  ADD COLUMN matched_search_entry_code VARCHAR(64) NOT NULL DEFAULT '' AFTER matched_category_code,
  ADD COLUMN matched_mapping_rule_json TEXT NOT NULL AFTER matched_search_entry_code;

CREATE INDEX idx_task_details_source_product_id ON task_details (source_product_id);
CREATE INDEX idx_task_details_source_search_entry ON task_details (source_search_entry_code, matched_search_entry_code);
