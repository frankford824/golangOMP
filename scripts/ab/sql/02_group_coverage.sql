SELECT 'group_coverage.missing_task_group' AS violation_code, CONCAT(t.id) AS entity_key, CONCAT('task_type=', t.task_type) AS detail
FROM tasks t
WHERE @ab_side = 'B'
  AND t.task_type NOT IN ('retouch_task', 'purchase_task', 'sku_planning')
  AND NOT EXISTS (SELECT 1 FROM task_sku_items s WHERE s.task_id = t.id)
  AND NOT EXISTS (SELECT 1 FROM task_asset_groups g WHERE g.task_id = t.id AND g.scope_kind = 'task')
UNION ALL
SELECT 'group_coverage.missing_sku_group', CONCAT(s.task_id, ':', s.id), CONCAT('sku_code=', s.sku_code)
FROM task_sku_items s JOIN tasks t ON t.id = s.task_id
WHERE @ab_side = 'B'
  AND t.task_type NOT IN ('retouch_task', 'purchase_task', 'sku_planning')
  AND NOT EXISTS (SELECT 1 FROM task_asset_groups g WHERE g.task_id = s.task_id AND g.scope_kind = 'sku' AND g.task_sku_item_id = s.id)
UNION ALL
SELECT 'group_coverage.missing_retouch_group', CONCAT(r.task_id, ':', r.id), CONCAT('requirement=', r.id)
FROM task_retouch_requirements r JOIN tasks t ON t.id = r.task_id AND t.task_type = 'retouch_task'
WHERE @ab_side = 'B'
  AND NOT EXISTS (SELECT 1 FROM task_asset_groups g WHERE g.task_id = r.task_id AND g.scope_kind = 'retouch_requirement' AND g.retouch_requirement_id = r.id)
UNION ALL
SELECT 'group_coverage.group_for_planning_task', CONCAT(g.id), CONCAT('task=', g.task_id, ',type=', t.task_type)
FROM task_asset_groups g JOIN tasks t ON t.id = g.task_id
WHERE @ab_side = 'B' AND t.task_type IN ('purchase_task', 'sku_planning')
UNION ALL
SELECT 'group_coverage.scope_shape_invalid', CONCAT(g.id), CONCAT('kind=', g.scope_kind, ',sku=', COALESCE(g.task_sku_item_id, 0), ',retouch=', COALESCE(g.retouch_requirement_id, 0))
FROM task_asset_groups g
WHERE @ab_side = 'B' AND NOT (
  (g.scope_kind = 'task' AND g.task_sku_item_id IS NULL AND g.retouch_requirement_id IS NULL)
  OR (g.scope_kind = 'sku' AND g.task_sku_item_id IS NOT NULL AND g.retouch_requirement_id IS NULL)
  OR (g.scope_kind = 'retouch_requirement' AND g.task_sku_item_id IS NULL AND g.retouch_requirement_id IS NOT NULL)
)
UNION ALL
SELECT 'group_coverage.sku_scope_task_mismatch', CONCAT(g.id), CONCAT('sku_item=', g.task_sku_item_id, ',task=', g.task_id)
FROM task_asset_groups g JOIN task_sku_items s ON s.id = g.task_sku_item_id
WHERE @ab_side = 'B' AND g.scope_kind = 'sku' AND s.task_id <> g.task_id
UNION ALL
SELECT 'group_coverage.retouch_scope_task_mismatch', CONCAT(g.id), CONCAT('retouch_requirement=', g.retouch_requirement_id, ',task=', g.task_id)
FROM task_asset_groups g JOIN task_retouch_requirements r ON r.id = g.retouch_requirement_id
WHERE @ab_side = 'B' AND g.scope_kind = 'retouch_requirement' AND r.task_id <> g.task_id
UNION ALL
SELECT 'group_coverage.completed_or_audit_empty_shell', CONCAT(g.id), CONCAT('task=', g.task_id, ',status=', t.task_status)
FROM task_asset_groups g JOIN tasks t ON t.id = g.task_id
WHERE @ab_side = 'B' AND t.task_status IN ('Completed', 'PendingAudit')
  AND g.working_revision_id IS NULL AND g.finalized_revision_id IS NULL
UNION ALL
SELECT 'group_coverage.unresolved_migration', CONCAT(g.id), g.migration_issue
FROM task_asset_groups g WHERE @ab_side = 'B' AND g.migration_incomplete = 1
ORDER BY 1, 2, 3;
