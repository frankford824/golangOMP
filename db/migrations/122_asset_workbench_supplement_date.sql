-- Promote manual supplement date from duplicate_hint_json to a first-class
-- indexed date column for server-side filtering, sorting, and pagination.

ALTER TABLE asset_workbench_settlement_supplements
  ADD COLUMN supplement_date DATE NULL AFTER order_no;

UPDATE asset_workbench_settlement_supplements
SET supplement_date = STR_TO_DATE(JSON_UNQUOTE(JSON_EXTRACT(duplicate_hint_json, '$.supplement_date')), '%Y-%m-%d')
WHERE supplement_date IS NULL
  AND JSON_VALID(duplicate_hint_json)
  AND JSON_UNQUOTE(JSON_EXTRACT(duplicate_hint_json, '$.supplement_date')) REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}$';

CREATE INDEX idx_aw_supplements_month_date_payee
  ON asset_workbench_settlement_supplements (business_month, supplement_date, payee_user_id, id);

CREATE INDEX idx_aw_supplements_date_status
  ON asset_workbench_settlement_supplements (supplement_date, status, id);

-- ROLLBACK-BEGIN
DROP INDEX idx_aw_supplements_date_status ON asset_workbench_settlement_supplements;
DROP INDEX idx_aw_supplements_month_date_payee ON asset_workbench_settlement_supplements;
ALTER TABLE asset_workbench_settlement_supplements DROP COLUMN supplement_date;
-- ROLLBACK-END
