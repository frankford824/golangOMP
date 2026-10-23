-- Preserve unknown dimensions as absent keys. A JSON null is not a numeric
-- zero and causes the existing strict DECIMAL materialization to fail.
START TRANSACTION;
UPDATE task_sku_items SET variant_json=JSON_REMOVE(variant_json,'$.width')
WHERE dedupe_key IN ('cost_maintenance_historical','cost_maintenance_completion')
 AND JSON_TYPE(JSON_EXTRACT(variant_json,'$.width'))='NULL';
UPDATE task_sku_items SET variant_json=JSON_REMOVE(variant_json,'$.height')
WHERE dedupe_key IN ('cost_maintenance_historical','cost_maintenance_completion')
 AND JSON_TYPE(JSON_EXTRACT(variant_json,'$.height'))='NULL';
UPDATE task_sku_items SET variant_json=JSON_REMOVE(variant_json,'$.area')
WHERE dedupe_key IN ('cost_maintenance_historical','cost_maintenance_completion')
 AND JSON_TYPE(JSON_EXTRACT(variant_json,'$.area'))='NULL';

-- Warm the existing batch READ observer, without approving any price. This
-- supplies a usable ERP baseline before an operator confirms a historical SKU.
UPDATE sku_cost_sync_states cs JOIN task_sku_items s ON s.sku_code=cs.sku_code
SET cs.status='baseline',cs.needs_check=1,cs.next_check_at=UTC_TIMESTAMP(3),
 cs.reason='历史编码待核对；只读取ERP，不自动改价'
WHERE s.dedupe_key='cost_maintenance_historical' AND cs.local_confirmed=0
 AND cs.status='conflict' AND cs.erp_cost IS NULL;
COMMIT;
