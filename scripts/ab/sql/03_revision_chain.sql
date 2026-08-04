SELECT 'revision_chain.finalized_pointer_missing_or_not_finalized' AS violation_code, CONCAT(g.id) AS entity_key,
       CONCAT('revision=', COALESCE(g.finalized_revision_id, 0), ',status=', COALESCE(r.status, 'missing')) AS detail
FROM task_asset_groups g LEFT JOIN task_asset_group_revisions r ON r.id = g.finalized_revision_id AND r.group_id = g.id
WHERE @ab_side = 'B' AND g.finalized_revision_id IS NOT NULL AND (r.id IS NULL OR r.status <> 'finalized')
UNION ALL
SELECT 'revision_chain.working_pointer_invalid', CONCAT(g.id), CONCAT('revision=', COALESCE(g.working_revision_id, 0), ',status=', COALESCE(r.status, 'missing'))
FROM task_asset_groups g LEFT JOIN task_asset_group_revisions r ON r.id = g.working_revision_id AND r.group_id = g.id
WHERE @ab_side = 'B' AND g.working_revision_id IS NOT NULL AND (r.id IS NULL OR r.status IN ('rejected', 'superseded'))
UNION ALL
SELECT 'revision_chain.non_contiguous_revision_no', CONCAT(r.group_id), CONCAT('min=', MIN(r.revision_no), ',max=', MAX(r.revision_no), ',count=', COUNT(*))
FROM task_asset_group_revisions r
WHERE @ab_side = 'B'
GROUP BY r.group_id
HAVING MIN(r.revision_no) <> 1 OR MAX(r.revision_no) <> COUNT(*)
UNION ALL
SELECT 'revision_chain.duplicate_revision_no', CONCAT(r.group_id, ':', r.revision_no), CONCAT('count=', COUNT(*))
FROM task_asset_group_revisions r
WHERE @ab_side = 'B'
GROUP BY r.group_id, r.revision_no HAVING COUNT(*) > 1
UNION ALL
SELECT 'revision_chain.finalized_timestamp_missing', CONCAT(r.id), CONCAT('group=', r.group_id)
FROM task_asset_group_revisions r
WHERE @ab_side = 'B' AND r.status = 'finalized' AND r.finalized_at IS NULL
UNION ALL
SELECT 'revision_chain.submitted_timestamp_missing', CONCAT(r.id), CONCAT('group=', r.group_id, ',status=', r.status)
FROM task_asset_group_revisions r
WHERE @ab_side = 'B' AND r.status IN ('submitted', 'finalized') AND r.submitted_at IS NULL
ORDER BY 1, 2, 3;
