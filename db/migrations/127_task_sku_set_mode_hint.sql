-- V8 unified task creation: operations may suggest a set, but design
-- task_asset_group_revisions.mode remains the authoritative decision.
ALTER TABLE task_details
  ADD COLUMN set_mode_hint TINYINT(1) NOT NULL DEFAULT 0 AFTER design_requirement;

ALTER TABLE task_sku_items
  ADD COLUMN set_mode_hint TINYINT(1) NOT NULL DEFAULT 0 AFTER design_requirement;

-- ROLLBACK
-- ALTER TABLE task_sku_items DROP COLUMN set_mode_hint;
-- ALTER TABLE task_details DROP COLUMN set_mode_hint;
