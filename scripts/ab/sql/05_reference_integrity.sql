SELECT 'reference_integrity.sku_scope_task_mismatch' AS violation_code, CONCAT(r.id) AS entity_key, CONCAT('sku_item=', r.sku_item_id, ',task=', r.task_id) AS detail
FROM reference_file_refs r JOIN task_sku_items s ON s.id = r.sku_item_id
WHERE @ab_side = 'B' AND s.task_id <> r.task_id
UNION ALL
SELECT 'reference_integrity.retouch_scope_task_mismatch', CONCAT(r.id), CONCAT('retouch_requirement=', r.retouch_requirement_id, ',task=', r.task_id)
FROM reference_file_refs r JOIN task_retouch_requirements q ON q.id = r.retouch_requirement_id
WHERE @ab_side = 'B' AND q.task_id <> r.task_id
UNION ALL
SELECT 'reference_integrity.binding_task_asset_mismatch', CONCAT(b.id), CONCAT('binding_task=', b.task_id, ',asset_task=', a.task_id)
FROM task_reference_asset_bindings b JOIN task_assets a ON a.id = b.task_asset_id
WHERE @ab_side = 'B' AND a.task_id <> b.task_id
UNION ALL
SELECT 'reference_integrity.revision_reference_task_mismatch', CONCAT(rr.id), CONCAT('reference_task=', f.task_id, ',group_task=', g.task_id)
FROM task_asset_group_revision_references rr JOIN task_asset_group_revisions r ON r.id = rr.revision_id
JOIN task_asset_groups g ON g.id = r.group_id JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
WHERE @ab_side = 'B' AND f.task_id <> g.task_id
UNION ALL
SELECT 'reference_integrity.revision_reference_scope_mismatch', CONCAT(rr.id), CONCAT('group_kind=', g.scope_kind, ',group_ref=', g.scope_ref_id)
FROM task_asset_group_revision_references rr JOIN task_asset_group_revisions r ON r.id = rr.revision_id
JOIN task_asset_groups g ON g.id = r.group_id JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
WHERE @ab_side = 'B' AND NOT (
  (f.sku_item_id IS NULL AND f.retouch_requirement_id IS NULL)
  OR (g.scope_kind = 'sku' AND f.sku_item_id = g.task_sku_item_id AND f.retouch_requirement_id IS NULL)
  OR (g.scope_kind = 'retouch_requirement' AND f.sku_item_id IS NULL AND f.retouch_requirement_id = g.retouch_requirement_id)
)
UNION ALL
SELECT 'reference_integrity.snapshot_identity_mismatch', CONCAT(rr.id), CONCAT('ref_snapshot=', rr.ref_id_snapshot, ',actual=', f.ref_id)
FROM task_asset_group_revision_references rr JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
WHERE @ab_side = 'B' AND rr.ref_id_snapshot <> f.ref_id
UNION ALL
SELECT 'reference_integrity.formal_asset_task_mismatch', CONCAT(rr.id), CONCAT('asset_task=', a.task_id, ',group_task=', g.task_id)
FROM task_asset_group_revision_references rr JOIN task_asset_group_revisions r ON r.id = rr.revision_id
JOIN task_asset_groups g ON g.id = r.group_id JOIN task_assets a ON a.id = rr.formal_task_asset_id
WHERE @ab_side = 'B' AND a.task_id <> g.task_id
ORDER BY 1, 2, 3;
