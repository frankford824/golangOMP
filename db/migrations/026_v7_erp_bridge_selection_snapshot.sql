-- Migration: 026_v7_erp_bridge_selection_snapshot.sql
-- Persists additive ERP Bridge selection snapshots without breaking the
-- existing task-side product_selection provenance columns.

ALTER TABLE task_details
  ADD COLUMN product_selection_snapshot_json TEXT NOT NULL AFTER matched_mapping_rule_json;
