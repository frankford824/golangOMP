package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"workflow/domain"
	"workflow/repo"
)

type taskCostSpecificationRepo struct{ db *DB }

func NewTaskCostSpecificationRepo(db *DB) repo.TaskCostSpecificationRepo {
	return &taskCostSpecificationRepo{db}
}

const costSpecFrom = ` FROM tasks t JOIN task_details d ON d.task_id=t.id LEFT JOIN task_sku_items s ON s.task_id=t.id
 LEFT JOIN cost_rule_bindings b ON b.is_active=1 AND b.normalized_i_id=UPPER(TRIM(COALESCE(NULLIF(s.variant_json->>'$.erp_i_id',''),NULLIF(s.variant_json->>'$.product_i_id',''),NULLIF(s.product_i_id,''),'')))`
const costSpecGroup = `COALESCE(NULLIF(b.rule_group,''),NULLIF(s.category_code,''),d.category_code,'')`
const costSpecActiveRule = `cr.category_code=` + costSpecGroup + ` AND cr.is_active=1 AND (cr.effective_from IS NULL OR cr.effective_from<=UTC_TIMESTAMP()) AND (cr.effective_to IS NULL OR cr.effective_to>=UTC_TIMESTAMP())
 AND NOT EXISTS(SELECT 1 FROM cost_rules newer WHERE newer.supersedes_rule_id=cr.id AND newer.is_active=1 AND (newer.effective_from IS NULL OR newer.effective_from<=UTC_TIMESTAMP()) AND (newer.effective_to IS NULL OR newer.effective_to>=UTC_TIMESTAMP()))`
const costSpecNeedsArea = `(EXISTS(SELECT 1 FROM cost_rules cr WHERE ` + costSpecActiveRule + ` AND cr.rule_type='cost_model' AND JSON_UNQUOTE(JSON_EXTRACT(IF(JSON_VALID(cr.formula_expression),cr.formula_expression,JSON_OBJECT()),'$.basis'))='area') OR
 (NOT EXISTS(SELECT 1 FROM cost_rules cr WHERE ` + costSpecActiveRule + ` AND cr.rule_type='cost_model') AND EXISTS(SELECT 1 FROM cost_rules cr WHERE ` + costSpecActiveRule + ` AND (cr.rule_type IN ('fixed_unit_price','minimum_billable_area','area_threshold_surcharge') OR (cr.rule_type='size_based_formula' AND cr.formula_expression LIKE 'keyword_area_unit_price:%')))))`
const costSpecPrimary = `(s.id IS NULL OR t.batch_item_count<=1 OR s.sku_code=t.primary_sku_code OR s.sku_code=t.sku_code)`

func costSpecNumber(expression string) string {
	return `(CASE WHEN JSON_UNQUOTE(` + expression + `) REGEXP '^[0-9]+([.][0-9]+)?$' THEN CAST(JSON_UNQUOTE(` + expression + `) AS DECIMAL(20,8)) ELSE NULL END)`
}
func costSpecAreaExpression() string {
	n := func(path string) string { return costSpecNumber(`JSON_EXTRACT(s.variant_json,'` + path + `')`) }
	legacy := `COALESCE(` + n(`$.area`) + `,IF(` + costSpecPrimary + `,d.area,NULL),` + n(`$.width`) + `*` + n(`$.height`) + `/10000,IF(` + costSpecPrimary + `,d.width*d.height/10000,NULL),0)`
	return `(CASE WHEN JSON_TYPE(JSON_EXTRACT(s.variant_json,'$.cost_input'))='OBJECT' THEN CASE s.variant_json->>'$.cost_input.area_mode'
 WHEN 'total' THEN COALESCE(` + n(`$.cost_input.area_m2`) + `,0)
 WHEN 'flat' THEN COALESCE(` + n(`$.cost_input.width_m`) + `*` + n(`$.cost_input.height_m`) + `*` + n(`$.cost_input.pieces`) + `,0)
 WHEN 'layout' THEN COALESCE(` + n(`$.cost_input.width_m`) + `*` + n(`$.cost_input.height_m`) + `,0)
 WHEN 'faces' THEN COALESCE((SELECT IF(COUNT(*)>0 AND SUM(IF(f.w>0 AND f.h>0 AND f.n>0,1,0))=COUNT(*),SUM(f.w*f.h*f.n),0)
 FROM JSON_TABLE(IF(JSON_TYPE(JSON_EXTRACT(s.variant_json,'$.cost_input.faces'))='ARRAY',JSON_EXTRACT(s.variant_json,'$.cost_input.faces'),JSON_ARRAY()),'$[*]' COLUMNS(w DOUBLE PATH '$.width_m' NULL ON ERROR,h DOUBLE PATH '$.height_m' NULL ON ERROR,n INT PATH '$.count' NULL ON ERROR)) f),0)
 ELSE 0 END ELSE ` + legacy + ` END)`
}
func costSpecColumns() string {
	return `t.id,t.task_no,t.task_status,t.creator_id,t.requester_id,t.designer_id,t.current_handler_id,t.owner_department_id,t.owner_team_id,t.task_type,COALESCE(s.id,0),COALESCE(NULLIF(s.sku_code,''),t.sku_code,''),COALESCE(NULLIF(s.product_name_snapshot,''),t.product_name_snapshot,''),COALESCE(s.variant_json,JSON_OBJECT()),IF(s.id IS NULL,d.cost_price,s.cost_price),COALESCE(s.requires_manual_review,d.requires_manual_review,0),
 COALESCE(` + costSpecNumber(`JSON_EXTRACT(s.variant_json,'$.width')`) + `,IF(` + costSpecPrimary + `,d.width,NULL)),COALESCE(` + costSpecNumber(`JSON_EXTRACT(s.variant_json,'$.height')`) + `,IF(` + costSpecPrimary + `,d.height,NULL)),` + costSpecAreaExpression() + `,` + costSpecNeedsArea
}
func scanCostSpec(scan interface{ Scan(...interface{}) error }) (*domain.TaskCostSpecification, error) {
	r := new(domain.TaskCostSpecification)
	err := scan.Scan(&r.TaskID, &r.TaskNo, &r.TaskStatus, &r.CreatorID, &r.RequesterID, &r.DesignerID, &r.CurrentHandlerID, &r.OwnerDepartmentID, &r.OwnerTeamID, &r.TaskType, &r.SKUItemID, &r.SKUCode, &r.ProductName, &r.VariantJSON, &r.CostPrice, &r.CostReviewRequired, &r.WidthCM, &r.HeightCM, &r.AreaM2, &r.RequiresArea)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}
func (r *taskCostSpecificationRepo) List(ctx context.Context, f repo.TaskCostSpecificationFilter) ([]*domain.TaskCostSpecification, int64, error) {
	ctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	where := []string{"1=1"}
	args := []interface{}{}
	where, args = appendResourceGroupAccessScope(where, args, f.Access)
	if f.TaskID > 0 {
		where = append(where, "t.id=?")
		args = append(args, f.TaskID)
	}
	if f.CreatorID > 0 {
		where = append(where, "t.creator_id=?")
		args = append(args, f.CreatorID)
	}
	if f.MissingOnly {
		where = append(where, "t.task_status IN ('PendingAudit','Completed')", "t.task_type IN ('original_product_development','new_product_development','retouch_task')", costSpecNeedsArea, costSpecAreaExpression()+"<=0")
	}
	from := costSpecFrom + " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.db.QueryContext(ctx, "SELECT "+costSpecColumns()+from+" ORDER BY t.id DESC,s.sequence_no DESC LIMIT ? OFFSET ?", append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*domain.TaskCostSpecification{}
	for rows.Next() {
		row, e := scanCostSpec(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
func (r *taskCostSpecificationRepo) Lock(ctx context.Context, tx repo.Tx, taskID, itemID int64) (*domain.TaskCostSpecification, error) {
	return scanCostSpec(Unwrap(tx).QueryRowContext(ctx, "SELECT "+costSpecColumns()+costSpecFrom+" WHERE t.id=? AND COALESCE(s.id,0)=? FOR UPDATE", taskID, itemID))
}
func (r *taskCostSpecificationRepo) Save(ctx context.Context, tx repo.Tx, row *domain.TaskCostSpecification, input domain.CostInput, area float64) error {
	q := Unwrap(tx)
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	var width, height *float64
	if input.AreaMode == "flat" || input.AreaMode == "layout" {
		w, h := input.WidthM*100, input.HeightM*100
		width, height = &w, &h
	}
	if row.SKUItemID > 0 {
		geometry := `JSON_SET(JSON_REMOVE(COALESCE(variant_json,JSON_OBJECT()),'$.cost_calculation','$.width','$.height'),'$.cost_input',CAST(? AS JSON),'$.area',?)`
		args := []interface{}{string(raw), area}
		if width != nil {
			geometry = `JSON_SET(` + geometry + `,'$.width',?,'$.height',?)`
			args = append(args, *width, *height)
		}
		args = append(args, row.SKUItemID, row.TaskID)
		_, err = q.ExecContext(ctx, `UPDATE task_sku_items SET variant_json=`+geometry+`,requires_manual_review=1,manual_cost_override=0,estimated_cost=NULL WHERE id=? AND task_id=?`, args...)
		if err != nil {
			return err
		}
	}
	// Detail fallback is authoritative for the task-level row before SKU creation.
	// Primary SKU mirrors update geometry only; monetary values stay untouched.
	_, err = q.ExecContext(ctx, `UPDATE task_details d JOIN tasks t ON t.id=d.task_id SET d.width=?,d.height=?,d.area=?,d.requires_manual_review=1,d.manual_cost_override=0,d.estimated_cost=NULL WHERE d.task_id=? AND (?=0 OR t.batch_item_count<=1 OR t.primary_sku_code=? OR t.sku_code=?)`, width, height, area, row.TaskID, row.SKUItemID, row.SKUCode, row.SKUCode)
	if err != nil {
		return err
	}
	return reindexTaskSearchDocument(ctx, q, row.TaskID)
}

// Completion still succeeds. Withhold unverified area-based prices from later
// filing jobs until the specification and cost have been reviewed explicitly.
func markMissingCostAreaPending(ctx context.Context, tx repo.Tx, taskID int64) error {
	where := ` WHERE t.id=? AND t.task_type IN ('original_product_development','new_product_development','retouch_task') AND ` + costSpecNeedsArea + ` AND ` + costSpecAreaExpression() + `<=0`
	joins := strings.TrimPrefix(costSpecFrom, " FROM ")
	if _, err := Unwrap(tx).ExecContext(ctx, `UPDATE `+joins+` SET s.requires_manual_review=1,s.manual_cost_override=0,s.estimated_cost=NULL`+where+` AND s.id IS NOT NULL`, taskID); err != nil {
		return err
	}
	_, err := Unwrap(tx).ExecContext(ctx, `UPDATE `+joins+` SET d.requires_manual_review=1,d.manual_cost_override=0,d.estimated_cost=NULL`+where+` AND `+costSpecPrimary, taskID)
	return err
}
