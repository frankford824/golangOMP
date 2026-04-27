-- Migration: 041_v7_task_filing_status.sql
-- Formalize filing state machine on task_details.

ALTER TABLE task_details
  ADD COLUMN filing_status VARCHAR(32) NOT NULL DEFAULT 'not_filed' COMMENT 'not_filed | filing | filed | filing_failed',
  ADD COLUMN filing_error_message TEXT NOT NULL COMMENT 'latest filing failure reason';

UPDATE task_details
SET filing_status = 'filed'
WHERE filed_at IS NOT NULL;

