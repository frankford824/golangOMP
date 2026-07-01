ALTER TABLE asset_workbench_error_records
  ADD COLUMN difficulty_class VARCHAR(64) NOT NULL DEFAULT '' AFTER order_no,
  ADD COLUMN occurred_date DATE NULL AFTER difficulty_class;

CREATE INDEX idx_aw_error_records_payee_difficulty_month
  ON asset_workbench_error_records (payee_user_id, business_month, difficulty_class);
