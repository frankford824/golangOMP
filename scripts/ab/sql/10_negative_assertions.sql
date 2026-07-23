SELECT 'negative.unresolved_group_migration' AS violation_code, CONCAT(g.id) AS entity_key, g.migration_issue AS detail
FROM task_asset_groups g WHERE @ab_side = 'B' AND g.migration_incomplete = 1
UNION ALL
SELECT 'negative.legacy_task_asset', CONCAT(a.id), CONCAT('task=', a.task_id, ',type=', a.asset_type)
FROM task_assets a WHERE @ab_side = 'B' AND a.binding_state = 'legacy'
UNION ALL
SELECT 'negative.unconfirmed_revision_metadata', CONCAT(r.id), LEFT(r.reason, 240)
FROM task_asset_group_revisions r
WHERE @ab_side = 'B' AND (r.reason LIKE '%confidence=proposed_review%' OR r.reason LIKE '%confidence=hard_blocked%')
UNION ALL
SELECT 'negative.orphan_task_asset_group_revision', CONCAT(r.id), CONCAT('group_id=', r.group_id)
FROM task_asset_group_revisions r LEFT JOIN task_asset_groups g ON g.id = r.group_id
WHERE @ab_side = 'B' AND g.id IS NULL
UNION ALL
SELECT 'negative.orphan_task_asset_group_revision_item', CONCAT(i.id), CONCAT('revision_id=', i.revision_id)
FROM task_asset_group_revision_items i LEFT JOIN task_asset_group_revisions r ON r.id = i.revision_id
WHERE @ab_side = 'B' AND r.id IS NULL
UNION ALL
SELECT 'negative.orphan_task_asset_group_reference', CONCAT(r.id), CONCAT('reference_file_ref_id=', r.reference_file_ref_id)
FROM task_asset_group_revision_references r LEFT JOIN reference_file_refs f ON f.id = r.reference_file_ref_id
WHERE @ab_side = 'B' AND f.id IS NULL
UNION ALL
SELECT 'negative.broken_working_pointer', CONCAT(g.id), CONCAT('revision=', g.working_revision_id)
FROM task_asset_groups g LEFT JOIN task_asset_group_revisions r ON r.id = g.working_revision_id AND r.group_id = g.id
WHERE @ab_side = 'B' AND g.working_revision_id IS NOT NULL AND r.id IS NULL
UNION ALL
SELECT 'negative.broken_finalized_pointer', CONCAT(g.id), CONCAT('revision=', g.finalized_revision_id)
FROM task_asset_groups g LEFT JOIN task_asset_group_revisions r ON r.id = g.finalized_revision_id AND r.group_id = g.id
WHERE @ab_side = 'B' AND g.finalized_revision_id IS NOT NULL AND r.id IS NULL
UNION ALL
SELECT 'negative.active_revision_asset_unavailable', CONCAT(a.id), CONCAT('binding=', a.binding_state, ',revoked=', IF(a.access_revoked_at IS NULL, 0, 1), ',deleted=', IF(a.object_deleted_at IS NULL, 0, 1))
FROM task_assets a
WHERE @ab_side = 'B' AND (a.access_revoked_at IS NOT NULL OR a.object_deleted_at IS NOT NULL OR a.binding_state <> 'bound') AND (
  EXISTS (SELECT 1 FROM task_asset_group_revisions r WHERE r.source_task_asset_id = a.id)
  OR EXISTS (SELECT 1 FROM task_asset_group_revision_items i WHERE i.task_asset_id = a.id)
)
ORDER BY 1, 2, 3;
