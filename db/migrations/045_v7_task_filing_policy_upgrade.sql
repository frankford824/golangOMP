-- Migration: 045_v7_task_filing_policy_upgrade.sql
-- Upgrade task filing state machine metadata for auto-trigger/idempotency.

ALTER TABLE task_details
  ADD COLUMN IF NOT EXISTS filing_trigger_source VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'create | business_info_patch | procurement_update | procurement_advance | audit_final_approved | warehouse_complete_precheck | manual_retry' AFTER filing_error_message,
  ADD COLUMN IF NOT EXISTS last_filing_attempt_at DATETIME NULL COMMENT 'last ERP filing attempt timestamp' AFTER filing_trigger_source,
  ADD COLUMN IF NOT EXISTS last_filed_at DATETIME NULL COMMENT 'last successful ERP filing timestamp' AFTER last_filing_attempt_at,
  ADD COLUMN IF NOT EXISTS erp_sync_required TINYINT(1) NOT NULL DEFAULT 1 COMMENT 'whether ERP sync is still required for current effective payload' AFTER last_filed_at,
  ADD COLUMN IF NOT EXISTS erp_sync_version BIGINT NOT NULL DEFAULT 0 COMMENT 'logical ERP sync payload version' AFTER erp_sync_required,
  ADD COLUMN IF NOT EXISTS last_filing_payload_hash VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'sha256 hash of latest evaluated filing payload' AFTER erp_sync_version,
  ADD COLUMN IF NOT EXISTS last_filing_payload_json LONGTEXT NULL COMMENT 'latest evaluated filing payload snapshot' AFTER last_filing_payload_hash;

UPDATE task_details
SET filing_status = 'pending_filing'
WHERE filing_status = 'not_filed'
  AND (
    TRIM(COALESCE(category_code, '')) = ''
    OR TRIM(COALESCE(spec_text, '')) = ''
    OR cost_price IS NULL
  );

UPDATE task_details
SET last_filed_at = filed_at
WHERE filed_at IS NOT NULL
  AND last_filed_at IS NULL;
