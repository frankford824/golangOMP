package mysqlrepo

import (
	"context"
	"workflow/repo"
)

// Completed original-product tasks historically lived only in task_details.
// New completions create their canonical maintenance identity in the finalizer's
// transaction. Existing codes always keep their existing canonical owner.
func ensureCompletedCostSKU(ctx context.Context, tx repo.Tx, taskID int64) error {
	q := Unwrap(tx)
	_, err := q.ExecContext(ctx, `INSERT INTO task_sku_items
 (task_id,sequence_no,sku_code,sku_status,filing_status,erp_sync_status,erp_sync_required,erp_sync_version,last_filed_at,product_name_snapshot,product_short_name,product_i_id,category_code,quantity,design_requirement,reference_file_refs_json,dedupe_key,cost_price,estimated_cost,cost_rule_id,cost_rule_name,cost_rule_source,matched_rule_version,requires_manual_review,manual_cost_override,manual_cost_override_reason,override_actor,override_at,variant_json)
 SELECT t.id,1,t.sku_code,IF(d.filing_status='filed','filed','generated'),d.filing_status,d.filing_status,0,d.erp_sync_version,d.last_filed_at,t.product_name_snapshot,d.product_short_name,
 COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(d.product_selection_snapshot_json,'$.erp_product.i_id')),''),''),d.category_code,d.quantity,d.design_requirement,COALESCE(d.reference_file_refs_json,'[]'),'cost_maintenance_completion',
 d.cost_price,d.estimated_cost,d.cost_rule_id,d.cost_rule_name,d.cost_rule_source,d.matched_rule_version,d.requires_manual_review,d.manual_cost_override,d.manual_cost_override_reason,d.override_actor,d.override_at,
 JSON_OBJECT('width',d.width,'height',d.height,'area',d.area,'size_text',d.size_text,'product_i_id',COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.product_selection_snapshot_json,'$.erp_product.i_id')),''))
 FROM tasks t JOIN task_details d ON d.task_id=t.id
 WHERE t.id=? AND t.task_type='original_product_development' AND t.sku_code<>''
 AND NOT EXISTS(SELECT 1 FROM task_sku_items s WHERE s.task_id=t.id OR s.sku_code=t.sku_code)
 ON DUPLICATE KEY UPDATE sku_code=VALUES(sku_code)`, taskID)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE erp_product_sync_records pm JOIN task_sku_items s ON s.task_id=pm.task_id AND s.sku_code=pm.sku_code SET pm.task_sku_item_id=s.id,pm.record_key=CONCAT('task:',s.task_id,':sku:',s.id) WHERE s.task_id=? AND s.dedupe_key='cost_maintenance_completion' AND pm.task_sku_item_id IS NULL`, taskID)
	return err
}
