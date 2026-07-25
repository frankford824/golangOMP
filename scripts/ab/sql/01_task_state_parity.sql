-- A is the immutable external baseline. State migration assertions apply only
-- to B; exact per-task approved state is independently bound by gate 11/G01.
SELECT CONVERT('task_state.retired_status_present' USING utf8mb4) COLLATE utf8mb4_unicode_ci AS violation_code,
       CONVERT(CONCAT(t.id) USING utf8mb4) COLLATE utf8mb4_unicode_ci AS entity_key,
       CONVERT(CONCAT('status=', t.task_status) USING utf8mb4) COLLATE utf8mb4_unicode_ci AS detail
FROM tasks t
WHERE @ab_side = 'B' AND t.task_status IN (
  'PendingAuditA','RejectedByAuditA','PendingAuditB','RejectedByAuditB',
  'PendingOutsource','Outsourcing','PendingOutsourceReview',
  'PendingCustomizationReview','PendingCustomizationProduction',
  'PendingEffectReview','PendingEffectRevision','PendingProductionTransfer',
  'PendingWarehouseQC','RejectedByWarehouse','PendingWarehouseReceive','PendingClose'
)
UNION ALL
SELECT CONVERT('task_state.purchase_task_present' USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT(CONCAT(t.id) USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT('purchase_task must be migrated to sku_planning' USING utf8mb4) COLLATE utf8mb4_unicode_ci
FROM tasks t WHERE @ab_side = 'B' AND t.task_type = 'purchase_task'
UNION ALL
SELECT CONVERT('task_state.completed_with_open_module' USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT(CONCAT(t.id) USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT(CONCAT('module=', tm.module_key, ',state=', tm.state) USING utf8mb4) COLLATE utf8mb4_unicode_ci
FROM tasks t JOIN task_modules tm ON tm.task_id = t.id
WHERE @ab_side = 'B' AND t.task_status = 'Completed'
  AND tm.state NOT IN ('completed', 'closed', 'forcibly_closed', 'closed_by_admin')
UNION ALL
SELECT CONVERT('task_state.claim_timestamp_without_actor' USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT(CONCAT(tm.id) USING utf8mb4) COLLATE utf8mb4_unicode_ci,
       CONVERT(CONCAT('module=', tm.module_key) USING utf8mb4) COLLATE utf8mb4_unicode_ci
FROM task_modules tm WHERE @ab_side = 'B' AND tm.claimed_at IS NOT NULL AND tm.claimed_by IS NULL
ORDER BY 1, 2, 3;
