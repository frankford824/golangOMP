-- Authorized canonicalization of completed original-product codes only.
-- Keep the latest completed task for duplicate historical codes. Never change
-- original task/detail prices, historical snapshots, existing SKU identities or bindings.
START TRANSACTION;
INSERT INTO task_sku_items (
 task_id,sequence_no,sku_code,sku_status,filing_status,erp_sync_status,erp_sync_required,erp_sync_version,last_filed_at,product_name_snapshot,product_short_name,
 product_i_id,category_code,quantity,design_requirement,reference_file_refs_json,dedupe_key,
 cost_price,estimated_cost,cost_rule_id,cost_rule_name,cost_rule_source,matched_rule_version,
 requires_manual_review,manual_cost_override,manual_cost_override_reason,override_actor,override_at,
 variant_json,created_at,updated_at
)
SELECT t.id,1,t.sku_code,IF(d.filing_status='filed','filed','generated'),d.filing_status,d.filing_status,0,d.erp_sync_version,d.last_filed_at,t.product_name_snapshot,d.product_short_name,
 COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(d.product_selection_snapshot_json,'$.erp_product.i_id')),''),''),
 d.category_code,d.quantity,d.design_requirement,COALESCE(d.reference_file_refs_json,'[]'),'cost_maintenance_historical',
 d.cost_price,d.estimated_cost,d.cost_rule_id,d.cost_rule_name,d.cost_rule_source,d.matched_rule_version,
 d.requires_manual_review,d.manual_cost_override,d.manual_cost_override_reason,d.override_actor,d.override_at,
 JSON_OBJECT('width',d.width,'height',d.height,'area',d.area,'size_text',d.size_text,
 'product_i_id',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.product_selection_snapshot_json,'$.erp_product.i_id')),'')),t.created_at,t.updated_at
FROM tasks t JOIN task_details d ON d.task_id=t.id
WHERE t.task_type='original_product_development' AND t.task_status IN ('Completed','Archived') AND t.sku_code<>''
 AND NOT EXISTS(SELECT 1 FROM task_sku_items s WHERE s.sku_code=t.sku_code OR s.task_id=t.id)
 AND t.id=(SELECT MAX(t2.id) FROM tasks t2 WHERE t2.sku_code=t.sku_code AND t2.task_status IN ('Completed','Archived'));

UPDATE sku_cost_sync_states cs JOIN task_sku_items s ON s.sku_code=cs.sku_code
SET cs.status='conflict',cs.local_confirmed=0,cs.needs_check=0,
 cs.reason='历史编码已补齐，需人工核对后再同步ERP'
WHERE s.dedupe_key='cost_maintenance_historical';

UPDATE erp_product_sync_records pm JOIN task_sku_items s ON s.task_id=pm.task_id AND s.sku_code=pm.sku_code
SET pm.task_sku_item_id=s.id,pm.record_key=CONCAT('task:',s.task_id,':sku:',s.id)
WHERE s.dedupe_key='cost_maintenance_historical' AND pm.task_sku_item_id IS NULL;
COMMIT;
