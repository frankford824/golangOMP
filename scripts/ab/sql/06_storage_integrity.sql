SELECT 'storage_integrity.task_asset_missing_storage_ref' AS violation_code, CONCAT(a.id) AS entity_key, CONCAT('storage_ref_id=', a.storage_ref_id) AS detail
FROM task_assets a LEFT JOIN asset_storage_refs s ON s.ref_id = a.storage_ref_id
WHERE @ab_side = 'B' AND a.storage_ref_id IS NOT NULL AND s.ref_id IS NULL
UNION ALL
SELECT 'storage_integrity.revision_asset_without_storage_ref', CONCAT(a.id), CONCAT('revision=', r.id, ',role=source')
FROM task_asset_group_revisions r JOIN task_assets a ON a.id = r.source_task_asset_id
WHERE @ab_side = 'B' AND (a.storage_ref_id IS NULL OR a.storage_ref_id = '')
UNION ALL
SELECT 'storage_integrity.revision_asset_without_storage_ref', CONCAT(a.id), CONCAT('revision=', i.revision_id, ',role=final')
FROM task_asset_group_revision_items i JOIN task_assets a ON a.id = i.task_asset_id
WHERE @ab_side = 'B' AND (a.storage_ref_id IS NULL OR a.storage_ref_id = '')
UNION ALL
SELECT 'storage_integrity.bound_upload_request_mismatch', CONCAT(a.id), CONCAT('asset_storage_ref=', COALESCE(a.storage_ref_id, ''), ',request_bound_ref=', COALESCE(u.bound_ref_id, ''))
FROM task_assets a JOIN upload_requests u ON u.request_id = a.upload_request_id
WHERE @ab_side = 'B' AND a.storage_ref_id IS NOT NULL AND u.bound_ref_id <> '' AND u.bound_ref_id <> a.storage_ref_id
UNION ALL
SELECT 'storage_integrity.storage_ref_missing_upload_request', s.ref_id, CONCAT('upload_request_id=', s.upload_request_id)
FROM asset_storage_refs s LEFT JOIN upload_requests u ON u.request_id = s.upload_request_id
WHERE @ab_side = 'B' AND s.upload_request_id IS NOT NULL AND u.request_id IS NULL
UNION ALL
SELECT 'storage_integrity.required_object_queued_for_deletion', CONCAT(o.id), CONCAT('task_asset=', o.task_asset_id, ',storage_ref=', COALESCE(o.storage_ref_id, ''))
FROM asset_object_deletion_outbox o
WHERE @ab_side = 'B' AND o.status IN ('pending', 'processing', 'retry') AND (
  EXISTS (SELECT 1 FROM task_asset_group_revisions r WHERE r.source_task_asset_id = o.task_asset_id)
  OR EXISTS (SELECT 1 FROM task_asset_group_revision_items i WHERE i.task_asset_id = o.task_asset_id)
  OR EXISTS (SELECT 1 FROM task_asset_group_revision_references rr WHERE rr.formal_task_asset_id = o.task_asset_id)
)
UNION ALL
SELECT 'storage_integrity.object_bytes_not_sql_verifiable', '*', 'hard_blocked: object existence, MIME, size and SHA-256 require the object-manifest verifier'
WHERE @ab_side = 'B' AND NOT EXISTS (
  SELECT 1 FROM ab_manifest_entities m
  WHERE m.run_id = @ab_run_id AND m.gate_name = 'G06'
    AND m.review_state = 'pass' AND m.expected_state = 'verified'
)
ORDER BY 1, 2, 3;
