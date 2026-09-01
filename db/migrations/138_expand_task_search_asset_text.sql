-- Migration: 138_expand_task_search_asset_text.sql
-- Large batch tasks can accumulate enough authoritative and derived assets to
-- exceed the 64 KiB TEXT limit while refreshing the task search projection.
-- Search is a downstream read model and must not cap the business asset graph.

ALTER TABLE task_search_documents
  MODIFY COLUMN asset_text LONGTEXT NULL;

-- ROLLBACK-BEGIN
-- Keep rollback representable as TEXT even when production accumulated a
-- larger projection after this migration. 16k utf8mb4 characters stay below
-- the 65,535-byte TEXT limit.
UPDATE task_search_documents
SET asset_text = LEFT(asset_text, 16000)
WHERE OCTET_LENGTH(asset_text) > 64000;

ALTER TABLE task_search_documents
  MODIFY COLUMN asset_text TEXT NULL;
-- ROLLBACK-END
