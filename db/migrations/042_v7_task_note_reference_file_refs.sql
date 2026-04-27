-- Migration: 042_v7_task_note_reference_file_refs.sql
-- Add independent note/reference_file_refs fields for task_details.

ALTER TABLE task_details
  ADD COLUMN note TEXT NOT NULL COMMENT 'independent note field',
  ADD COLUMN reference_file_refs_json TEXT NOT NULL COMMENT 'JSON array of reference file refs';

