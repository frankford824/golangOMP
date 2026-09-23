package mysqlrepo

import (
	"context"
	"strings"
	"time"
	"workflow/domain"
	"workflow/repo"
)

// Same precedence as pricing: ERP style binding first, then product style.
// Do not join historical calculation snapshots or require ERP filing success.
const costBoundSKUFrom = ` FROM task_sku_items s
 LEFT JOIN cost_rule_bindings eb ON eb.normalized_i_id_active =
 UPPER(REPLACE(REPLACE(TRIM(COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(s.variant_json,'$.erp_i_id')),''),NULLIF(JSON_UNQUOTE(JSON_EXTRACT(s.variant_json,'$.erp_product_i_id')),''),'')),' ',''),'　',''))
 LEFT JOIN cost_rule_bindings pb ON pb.normalized_i_id_active =
 UPPER(REPLACE(REPLACE(TRIM(COALESCE(NULLIF(s.product_i_id,''),NULLIF(JSON_UNQUOTE(JSON_EXTRACT(s.variant_json,'$.product_i_id')),''),NULLIF(JSON_UNQUOTE(JSON_EXTRACT(s.variant_json,'$.i_id')),''),'')),' ',''),'　',''))
 WHERE COALESCE(eb.rule_group,pb.rule_group) = ? AND s.sku_code <> ''
 AND EXISTS (SELECT 1 FROM cost_rules cr WHERE cr.category_code = COALESCE(eb.rule_group,pb.rule_group)
 AND cr.is_active = 1 AND (cr.effective_from IS NULL OR cr.effective_from <= ?)
 AND (cr.effective_to IS NULL OR cr.effective_to >= ?))`

func (r *costRuleBindingRepo) ListBoundSKUs(ctx context.Context, f repo.CostRuleBindingListFilter) ([]*domain.CostBoundSKU, int64, error) {
	ctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	from := costBoundSKUFrom
	now := time.Now().UTC()
	args := []interface{}{strings.TrimSpace(f.RuleGroup), now, now}
	if q := strings.TrimSpace(f.Keyword); q != "" {
		from += ` AND (s.sku_code LIKE ? OR s.product_name_snapshot LIKE ? OR COALESCE(eb.i_id_raw,pb.i_id_raw) LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(f.Page, f.PageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT s.id,s.sku_code,s.product_name_snapshot,COALESCE(eb.i_id_raw,pb.i_id_raw),s.cost_price,s.sku_status`+from+` ORDER BY s.id DESC LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.CostBoundSKU, 0)
	for rows.Next() {
		var item domain.CostBoundSKU
		if err := rows.Scan(&item.ID, &item.SKUCode, &item.ProductName, &item.StyleCode, &item.CostPrice, &item.Status); err != nil {
			return nil, 0, err
		}
		items = append(items, &item)
	}
	return items, total, rows.Err()
}
