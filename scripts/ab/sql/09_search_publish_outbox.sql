SELECT 'search_publish.task_document_missing' AS violation_code, CONCAT(t.id) AS entity_key, CONCAT('task_status=', t.task_status) AS detail
FROM tasks t LEFT JOIN task_search_documents d ON d.task_id = t.id
WHERE @ab_side = 'B' AND d.task_id IS NULL
UNION ALL
SELECT 'search_publish.task_document_mismatch', CONCAT(d.task_id), CONCAT('doc_status=', d.task_status, ',task_status=', t.task_status)
FROM task_search_documents d JOIN tasks t ON t.id = d.task_id
WHERE @ab_side = 'B' AND (d.task_status <> t.task_status OR d.task_type <> t.task_type OR NOT (d.current_handler_id <=> t.current_handler_id))
UNION ALL
SELECT 'search_publish.finalized_group_document_missing', CONCAT(g.id), CONCAT('revision=', g.finalized_revision_id)
FROM task_asset_groups g LEFT JOIN task_asset_group_search_documents d ON d.group_id = g.id
WHERE @ab_side = 'B' AND g.finalized_revision_id IS NOT NULL AND d.group_id IS NULL
UNION ALL
SELECT 'search_publish.group_document_not_finalized', CONCAT(d.group_id), CONCAT('document_revision=', d.finalized_revision_id, ',group_revision=', g.finalized_revision_id)
FROM task_asset_group_search_documents d JOIN task_asset_groups g ON g.id = d.group_id
WHERE @ab_side = 'B' AND NOT (d.finalized_revision_id <=> g.finalized_revision_id)
UNION ALL
SELECT 'search_publish.client_pin_invalid_shape', CONCAT(c.id), CONCAT('group=', COALESCE(c.resource_group_id,''), ',revision=', COALESCE(c.finalized_revision_id,''), ',cover=', COALESCE(c.cover_revision_item_id,''))
FROM asset_workbench_client_materials c
LEFT JOIN task_asset_groups g ON g.id = c.resource_group_id
LEFT JOIN task_asset_group_revisions r ON r.group_id = g.id AND r.id = c.finalized_revision_id
LEFT JOIN task_asset_group_revision_items i ON i.revision_id = r.id AND i.id = c.cover_revision_item_id
WHERE @ab_side = 'B' AND c.source_type = 'task_resource_group'
  AND (g.id IS NULL OR r.id IS NULL OR r.status NOT IN ('finalized','superseded') OR i.id IS NULL)
UNION ALL
SELECT 'search_publish.outbox_unknown_task', CONCAT(o.id), CONCAT('task_id=', o.task_id)
FROM task_erp_outbox o LEFT JOIN tasks t ON t.id = o.task_id
WHERE @ab_side = 'B' AND t.id IS NULL
UNION ALL
SELECT 'search_publish.erp_outbox_wrong_sku_scope', CONCAT(o.id), CONCAT('task_id=', o.task_id, ',sku_item_id=', o.task_sku_item_id)
FROM task_erp_outbox o JOIN task_sku_items si ON si.id = o.task_sku_item_id
WHERE @ab_side = 'B' AND si.task_id <> o.task_id
UNION ALL
SELECT 'search_publish.erp_outbox_retry', CONCAT(o.id), CONCAT('attempt=', o.attempt, ',error=', LEFT(COALESCE(o.last_error, ''), 160))
FROM task_erp_outbox o WHERE @ab_side = 'B' AND o.status = 'retry'
UNION ALL
SELECT 'search_publish.erp_outbox_permanent_failure', CONCAT(o.id), CONCAT('attempt=', o.attempt, ',alert=', o.alert_status, ',error=', LEFT(COALESCE(o.last_error, ''), 160))
FROM task_erp_outbox o
WHERE @ab_side = 'B' AND o.status = 'retry' AND (o.alert_status = 'alerted' OR o.attempt >= 5)
UNION ALL
SELECT 'search_publish.erp_outbox_duplicate_dedupe_key', o.dedupe_key, CONCAT('count=', COUNT(*))
FROM task_erp_outbox o WHERE @ab_side = 'B'
GROUP BY o.dedupe_key HAVING COUNT(*) > 1
UNION ALL
SELECT 'search_publish.erp_outbox_duplicate_business_key', CONCAT(o.task_id, ':', COALESCE(o.task_sku_item_id, 0), ':', o.job_type, ':', o.generation), CONCAT('count=', COUNT(*))
FROM task_erp_outbox o WHERE @ab_side = 'B'
GROUP BY o.task_id, o.task_sku_item_id, o.job_type, o.generation HAVING COUNT(*) > 1
UNION ALL
SELECT 'search_publish.reindex_outbox_retry', CONCAT(o.id), CONCAT('attempt=', o.attempt, ',error=', LEFT(COALESCE(o.last_error, ''), 160))
FROM search_reindex_outbox o WHERE @ab_side = 'B' AND o.status = 'retry'
UNION ALL
SELECT 'search_publish.reindex_outbox_permanent_failure', CONCAT(o.id), CONCAT('attempt=', o.attempt, ',error=', LEFT(COALESCE(o.last_error, ''), 160))
FROM search_reindex_outbox o WHERE @ab_side = 'B' AND o.status = 'retry' AND o.attempt >= 5
UNION ALL
SELECT 'search_publish.reindex_outbox_duplicate_dedupe_key', o.dedupe_key, CONCAT('count=', COUNT(*))
FROM search_reindex_outbox o WHERE @ab_side = 'B'
GROUP BY o.dedupe_key HAVING COUNT(*) > 1
UNION ALL
SELECT 'search_publish.reindex_outbox_invalid_entity', CONCAT(o.id), CONCAT('entity_type=', o.entity_type, ',entity_id=', o.entity_id)
FROM search_reindex_outbox o
WHERE @ab_side = 'B' AND (
  o.entity_type NOT IN ('task','task_resource_group')
  OR (o.entity_type = 'task' AND NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = o.entity_id))
  OR (o.entity_type = 'task_resource_group' AND NOT EXISTS (SELECT 1 FROM task_asset_groups g WHERE g.id = o.entity_id))
)
ORDER BY 1, 2, 3;
