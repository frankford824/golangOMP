-- Migration: 022_v7_cost_rule_governance_hardening.sql
-- Hardens cost-rule governance/versioning and task-side prefill/override audit boundaries.

ALTER TABLE cost_rules
  ADD COLUMN rule_version INT NOT NULL DEFAULT 1 AFTER rule_name,
  ADD COLUMN supersedes_rule_id BIGINT NULL AFTER effective_to,
  ADD COLUMN governance_note VARCHAR(255) NOT NULL DEFAULT '' AFTER supersedes_rule_id;

CREATE INDEX idx_cost_rules_supersedes_rule_id ON cost_rules (supersedes_rule_id);
CREATE INDEX idx_cost_rules_category_version ON cost_rules (category_code, rule_version);

ALTER TABLE task_details
  ADD COLUMN matched_rule_version INT NULL AFTER cost_rule_source,
  ADD COLUMN prefill_source VARCHAR(128) NOT NULL DEFAULT '' AFTER matched_rule_version,
  ADD COLUMN prefill_at DATETIME NULL AFTER prefill_source,
  ADD COLUMN override_actor VARCHAR(128) NOT NULL DEFAULT '' AFTER manual_cost_override_reason,
  ADD COLUMN override_at DATETIME NULL AFTER override_actor;

CREATE INDEX idx_task_details_prefill_at ON task_details (prefill_at);
