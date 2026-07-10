-- Link self-uploaded supplement work to the existing submission/file pipeline
-- while keeping it out of normal piecework settlement.

ALTER TABLE asset_workbench_submission_items
  ADD COLUMN entry_kind VARCHAR(32) NOT NULL DEFAULT 'normal' AFTER payee_user_id,
  ADD KEY idx_aw_items_entry_month (entry_kind, business_month, settlement_status, current_settlement_batch_id);

ALTER TABLE asset_workbench_settlement_supplements
  ADD COLUMN submission_item_id BIGINT NULL AFTER id,
  ADD UNIQUE KEY uk_aw_supplements_submission_item (submission_item_id),
  ADD CONSTRAINT fk_aw_supplements_submission_item
    FOREIGN KEY (submission_item_id) REFERENCES asset_workbench_submission_items(id);

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_settlement_supplements
  DROP FOREIGN KEY fk_aw_supplements_submission_item,
  DROP INDEX uk_aw_supplements_submission_item,
  DROP COLUMN submission_item_id;

ALTER TABLE asset_workbench_submission_items
  DROP INDEX idx_aw_items_entry_month,
  DROP COLUMN entry_kind;
-- ROLLBACK-END
