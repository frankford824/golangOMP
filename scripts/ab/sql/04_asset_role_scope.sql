SELECT 'asset_role_scope.bound_without_group' AS violation_code, CONCAT(a.id) AS entity_key, CONCAT('role=', COALESCE(a.bound_role, 'NULL')) AS detail
FROM task_assets a WHERE @ab_side = 'B' AND a.binding_state = 'bound' AND (a.bound_group_id IS NULL OR a.bound_role IS NULL)
UNION ALL
SELECT 'asset_role_scope.bound_group_task_mismatch', CONCAT(a.id), CONCAT('asset_task=', a.task_id, ',group_task=', g.task_id)
FROM task_assets a JOIN task_asset_groups g ON g.id = a.bound_group_id
WHERE @ab_side = 'B' AND a.binding_state = 'bound' AND a.task_id <> g.task_id
UNION ALL
SELECT 'asset_role_scope.source_wrong_type_or_binding', CONCAT(r.id), CONCAT('asset=', a.id, ',type=', a.asset_type, ',binding=', a.binding_state, ',role=', COALESCE(a.bound_role, ''))
FROM task_asset_group_revisions r JOIN task_asset_groups g ON g.id = r.group_id JOIN task_assets a ON a.id = r.source_task_asset_id
WHERE @ab_side = 'B' AND (a.asset_type <> 'source' OR a.binding_state <> 'bound' OR a.bound_group_id <> g.id OR a.bound_role <> 'source')
UNION ALL
SELECT 'asset_role_scope.final_wrong_type_or_binding', CONCAT(i.id), CONCAT('asset=', a.id, ',type=', a.asset_type, ',binding=', a.binding_state, ',role=', COALESCE(a.bound_role, ''))
FROM task_asset_group_revision_items i JOIN task_asset_group_revisions r ON r.id = i.revision_id
JOIN task_asset_groups g ON g.id = r.group_id JOIN task_assets a ON a.id = i.task_asset_id
WHERE @ab_side = 'B' AND (a.asset_type <> 'delivery' OR a.binding_state <> 'bound' OR a.bound_group_id <> g.id OR a.bound_role <> 'final')
UNION ALL
SELECT 'asset_role_scope.source_reused_as_final', CONCAT(r.id), CONCAT('asset=', r.source_task_asset_id)
FROM task_asset_group_revisions r JOIN task_asset_group_revision_items i ON i.revision_id = r.id AND i.task_asset_id = r.source_task_asset_id
WHERE @ab_side = 'B'
UNION ALL
SELECT 'asset_role_scope.design_revision_missing_source', CONCAT(r.id), CONCAT('group=', r.group_id, ',stage=', r.source_stage)
FROM task_asset_group_revisions r JOIN task_asset_groups g ON g.id = r.group_id
WHERE @ab_side = 'B' AND g.scope_kind <> 'retouch_requirement' AND r.source_task_asset_id IS NULL
  AND r.status IN ('submitted', 'finalized')
UNION ALL
SELECT 'asset_role_scope.mode_item_count_invalid', CONCAT(r.id), CONCAT('mode=', r.mode, ',items=', COUNT(i.id))
FROM task_asset_group_revisions r LEFT JOIN task_asset_group_revision_items i ON i.revision_id = r.id
WHERE @ab_side = 'B'
GROUP BY r.id, r.mode
HAVING (r.mode = 'single' AND COUNT(i.id) <> 1) OR (r.mode = 'set' AND COUNT(i.id) < 2)
UNION ALL
SELECT 'asset_role_scope.final_sort_not_contiguous', CONCAT(i.revision_id), CONCAT('min=', MIN(i.sort_order), ',max=', MAX(i.sort_order), ',count=', COUNT(*))
FROM task_asset_group_revision_items i
WHERE @ab_side = 'B'
GROUP BY i.revision_id
HAVING MIN(i.sort_order) <> 0 OR MAX(i.sort_order) <> COUNT(*) - 1
UNION ALL
SELECT 'asset_role_scope.staged_two_scopes', CONCAT(a.id), 'both sku and retouch requirement are set'
FROM task_assets a WHERE @ab_side = 'B' AND a.binding_state = 'staged' AND a.staged_task_sku_item_id IS NOT NULL AND a.staged_retouch_requirement_id IS NOT NULL
UNION ALL
SELECT 'asset_role_scope.bundle_contents_not_sql_verifiable', CONCAT(a.id), 'hard_blocked: ZIP membership and internal hashes require object-manifest verification'
FROM task_asset_group_revisions r JOIN task_assets a ON a.id = r.source_task_asset_id
WHERE @ab_side = 'B' AND LOWER(a.file_name) LIKE '%.zip'
  AND NOT EXISTS (
    SELECT 1 FROM ab_manifest_entities m
    WHERE m.run_id = @ab_run_id AND m.gate_name = 'G06'
      AND m.review_state = 'pass' AND m.expected_state = 'verified'
  )
ORDER BY 1, 2, 3;
