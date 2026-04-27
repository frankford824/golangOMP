-- Round W-2 SKU reference_file_refs storage_key backfill
-- Status: NOT EXECUTED -- pending human review.
--
-- Diagnostic decision: RECOVERABLE.
-- The affected SKU-level refs use the legacy proxy URL shape:
--   /v1/assets/files/<storage_key>
-- This matches service/reference_file_refs_download_enricher.go recovery logic.
--
-- Actual production schema stores SKU refs in task_sku_items.reference_file_refs_json,
-- keyed by (task_id, sku_code, ref_index). No business table update has been run.

START TRANSACTION;

UPDATE task_sku_items AS tsi
JOIN (
	SELECT
		462 AS task_id,
		'NSKT000156' AS sku_code,
		0 AS ref_index,
		'7aa72ca5-59ce-45ac-b6fc-40d945cd0bc3' AS asset_id,
		CONVERT(UNHEX('7461736b732f7461736b2d6372656174652d7265666572656e63652f6173736574732f5052454352454154452d5245464552454e43452f76312f646572697665642fe3809034e69da1e8a385e38091e6af95e4b89ae6898be68c81e6a8aae5b985e7bb84e59088412831292e6a7067') USING utf8mb4) AS recovered_storage_key
	UNION ALL
	SELECT
		466 AS task_id,
		'NSKT000158' AS sku_code,
		0 AS ref_index,
		'3de01ebb-60e1-4f33-998b-0f6f8b8b0e18' AS asset_id,
		'tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/1776738654630862145_e7c25501.jpg' AS recovered_storage_key
) AS recovery
	ON recovery.task_id = tsi.task_id
	AND recovery.sku_code = tsi.sku_code
SET tsi.reference_file_refs_json = JSON_SET(
	CAST(tsi.reference_file_refs_json AS JSON),
	CONCAT('$[', recovery.ref_index, '].storage_key'),
	recovery.recovered_storage_key
)
WHERE JSON_VALID(tsi.reference_file_refs_json)
  AND JSON_UNQUOTE(JSON_EXTRACT(
		CAST(tsi.reference_file_refs_json AS JSON),
		CONCAT('$[', recovery.ref_index, '].asset_id')
	)) = recovery.asset_id
  AND (
		JSON_EXTRACT(
			CAST(tsi.reference_file_refs_json AS JSON),
			CONCAT('$[', recovery.ref_index, '].storage_key')
		) IS NULL
		OR JSON_UNQUOTE(JSON_EXTRACT(
			CAST(tsi.reference_file_refs_json AS JSON),
			CONCAT('$[', recovery.ref_index, '].storage_key')
		)) = ''
  );

-- Human reviewer should inspect ROW_COUNT() before replacing ROLLBACK with COMMIT.
SELECT ROW_COUNT() AS candidate_rows_to_backfill;

ROLLBACK;
