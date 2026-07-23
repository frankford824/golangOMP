SELECT 'planning_retouch.planning_settings_missing' AS violation_code, CONCAT(t.id) AS entity_key, 'sku_planning task has no settings' AS detail
FROM tasks t LEFT JOIN task_planning_settings p ON p.task_id = t.id
WHERE @ab_side = 'B' AND t.task_type = 'sku_planning' AND p.task_id IS NULL
UNION ALL
SELECT 'planning_retouch.planning_detail_missing', CONCAT(s.task_id, ':', s.id), CONCAT('sku=', s.sku_code)
FROM task_sku_items s JOIN tasks t ON t.id = s.task_id AND t.task_type = 'sku_planning'
LEFT JOIN task_planning_sku_details d ON d.task_sku_item_id = s.id
WHERE @ab_side = 'B' AND d.task_sku_item_id IS NULL
UNION ALL
SELECT 'planning_retouch.planning_current_revision_missing', CONCAT(s.task_id, ':', s.id), CONCAT('sku=', s.sku_code)
FROM task_sku_items s JOIN tasks t ON t.id = s.task_id AND t.task_type = 'sku_planning'
JOIN task_planning_sku_details d ON d.task_sku_item_id = s.id
WHERE @ab_side = 'B' AND d.current_revision_id IS NULL
UNION ALL
SELECT 'planning_retouch.current_revision_item_mismatch', CONCAT(d.task_sku_item_id), CONCAT('current_revision=', d.current_revision_id)
FROM task_planning_sku_details d JOIN task_planning_sku_revisions r ON r.id = d.current_revision_id
WHERE @ab_side = 'B' AND r.task_sku_item_id <> d.task_sku_item_id
UNION ALL
SELECT 'planning_retouch.planning_revision_non_contiguous', CONCAT(r.task_sku_item_id), CONCAT('min=', MIN(r.version_no), ',max=', MAX(r.version_no), ',count=', COUNT(*))
FROM task_planning_sku_revisions r
WHERE @ab_side = 'B'
GROUP BY r.task_sku_item_id
HAVING MIN(r.version_no) <> 1 OR MAX(r.version_no) <> COUNT(*)
UNION ALL
SELECT 'planning_retouch.planning_current_image_missing', CONCAT(r.task_sku_item_id), CONCAT('revision=', r.id)
FROM task_planning_sku_details d JOIN task_planning_sku_revisions r ON r.id = d.current_revision_id
LEFT JOIN task_planning_sku_revision_images i ON i.revision_id = r.id
WHERE @ab_side = 'B' AND i.revision_id IS NULL
UNION ALL
SELECT 'planning_retouch.planning_task_type_mismatch', CONCAT(p.task_id), CONCAT('task_type=', t.task_type)
FROM task_planning_settings p JOIN tasks t ON t.id = p.task_id
WHERE @ab_side = 'B' AND t.task_type <> 'sku_planning'
UNION ALL
SELECT 'planning_retouch.asset_retouch_task_mismatch', CONCAT(a.id), CONCAT('asset_task=', a.task_id, ',retouch_task=', r.task_id)
FROM task_assets a JOIN task_retouch_requirements r ON r.id = a.retouch_requirement_id
WHERE @ab_side = 'B' AND a.task_id <> r.task_id
UNION ALL
SELECT 'planning_retouch.completed_requirement_without_final', CONCAT(q.task_id, ':', q.id), CONCAT('group=', COALESCE(g.id, 0))
FROM task_retouch_requirements q JOIN tasks t ON t.id = q.task_id AND t.task_status = 'Completed'
LEFT JOIN task_asset_groups g ON g.task_id = q.task_id AND g.scope_kind = 'retouch_requirement' AND g.retouch_requirement_id = q.id
LEFT JOIN task_asset_group_revision_items i ON i.revision_id = g.finalized_revision_id
WHERE @ab_side = 'B'
GROUP BY q.task_id, q.id, g.id
HAVING COUNT(i.id) = 0
ORDER BY 1, 2, 3;
