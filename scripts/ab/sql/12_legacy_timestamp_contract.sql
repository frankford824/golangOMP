-- Fail closed when legacy wall-clock relationships no longer match either the
-- proven historical UTC+8 writer or the corrected UTC writer. The migration
-- generator normalizes the +8 hour cohort before replay; it never rewrites the
-- immutable legacy rows checked here.
SELECT
  'legacy_timestamp.asset_created_delta_unclassified' AS violation_code,
  CONCAT('task_asset:', ta.id) AS entity_key,
  CONCAT('delta_seconds=', TIMESTAMPDIFF(SECOND, ta.created_at, e.created_at)) AS detail
FROM task_event_logs e
JOIN task_assets ta
  ON ta.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(e.payload, '$.asset_version_id')) AS UNSIGNED)
WHERE e.event_type = 'task.asset.version.created'
  AND @ab_side = 'B'
  AND TIMESTAMPDIFF(SECOND, ta.created_at, e.created_at) NOT BETWEEN -2 AND 2
  AND TIMESTAMPDIFF(SECOND, ta.created_at, e.created_at) NOT BETWEEN 28798 AND 28805

UNION ALL

SELECT
  'legacy_timestamp.superseded_delta_unclassified',
  CONCAT('task_asset:', old_asset.id),
  CONCAT('delta_seconds=', TIMESTAMPDIFF(SECOND, successor.created_at, old_asset.superseded_at))
FROM task_assets old_asset
JOIN task_assets successor ON successor.id = old_asset.superseded_by_version_id
WHERE old_asset.superseded_at IS NOT NULL
  AND @ab_side = 'B'
  AND TIMESTAMPDIFF(SECOND, successor.created_at, old_asset.superseded_at) NOT BETWEEN -3 AND 3
  AND TIMESTAMPDIFF(SECOND, successor.created_at, old_asset.superseded_at) NOT BETWEEN 28797 AND 28803
ORDER BY entity_key, violation_code;
