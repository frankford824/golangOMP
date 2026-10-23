package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"workflow/domain"
	"workflow/repo"
)

type costMaintenanceRepo struct{ db *DB }

func NewCostMaintenanceRepo(db *DB) repo.CostMaintenanceRepo { return &costMaintenanceRepo{db} }

const maintenanceColumns = `s.id,s.task_id,s.sku_code,s.product_name_snapshot,COALESCE(s.variant_json,JSON_OBJECT()),s.product_i_id,s.cost_price,s.updated_at,
 COALESCE(cs.revision,0),cs.erp_cost,COALESCE(cs.status,'baseline'),d.width,d.height,d.area,COALESCE(d.size_text,''),t.id*1000+s.sequence_no`
const maintenanceFrom = ` FROM task_sku_items s JOIN tasks t ON t.id=s.task_id
 LEFT JOIN task_details d ON d.task_id=s.task_id AND (t.batch_item_count=1 OR t.primary_sku_code=s.sku_code OR t.sku_code=s.sku_code) LEFT JOIN sku_cost_sync_states cs ON cs.sku_code=s.sku_code
 WHERE (t.task_status IN ('Completed','Archived') OR EXISTS(SELECT 1 FROM tasks history WHERE history.sku_code=s.sku_code AND history.task_status IN ('Completed','Archived'))) AND s.sku_code<>''`

func scanMaintenanceRow(s interface{ Scan(...interface{}) error }) (*domain.CostMaintenanceRow, error) {
	r := new(domain.CostMaintenanceRow)
	err := s.Scan(&r.ID, &r.TaskID, &r.SKUCode, &r.ProductName, &r.VariantJSON, &r.ProductIID, &r.CostPrice, &r.UpdatedAt, &r.Revision, &r.ERPCost, &r.Status, &r.DetailWidth, &r.DetailHeight, &r.DetailArea, &r.DetailSize, &r.SortID)
	return r, err
}
func (r *costMaintenanceRepo) List(ctx context.Context, f repo.CostMaintenanceFilter) ([]*domain.CostMaintenanceRow, error) {
	ctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	where := maintenanceFrom
	args := []interface{}{}
	if f.BeforeID > 0 {
		where += ` AND t.id*1000+s.sequence_no<?`
		args = append(args, f.BeforeID)
	}
	if f.Keyword != "" {
		where += ` AND (s.sku_code LIKE ? OR s.product_name_snapshot LIKE ? OR s.variant_json->>'$.product_i_id' LIKE ? OR s.variant_json->>'$.erp_i_id' LIKE ?)`
		for i := 0; i < 4; i++ {
			args = append(args, "%"+f.Keyword+"%")
		}
	}
	if f.Status != "" {
		where += ` AND COALESCE(cs.status,'baseline')=?`
		args = append(args, f.Status)
	}
	args = append(args, f.PageSize+1)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+maintenanceColumns+where+` ORDER BY t.id DESC,s.sequence_no DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.CostMaintenanceRow{}
	for rows.Next() {
		v, e := scanMaintenanceRow(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *costMaintenanceRepo) LockRow(ctx context.Context, tx repo.Tx, id int64) (*domain.CostMaintenanceRow, error) {
	v, err := scanMaintenanceRow(Unwrap(tx).QueryRowContext(ctx, `SELECT `+maintenanceColumns+maintenanceFrom+` AND s.id=? FOR UPDATE`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}
func (r *costMaintenanceRepo) ListLogs(ctx context.Context, f repo.CostMaintenanceFilter) ([]*domain.CostMaintenanceLog, int64, error) {
	ctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	where := ` FROM cost_maintenance_logs e LEFT JOIN users u ON u.id=e.operator_id WHERE 1=1`
	args := []interface{}{}
	if f.Keyword != "" {
		where += ` AND e.sku_code LIKE ?`
		args = append(args, f.Keyword+"%")
	}
	if f.Reason != "" {
		where += ` AND e.reason_code=?`
		args = append(args, f.Reason)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT CAST(e.id AS CHAR),e.task_id,e.operator_id,COALESCE(NULLIF(u.display_name,''),u.username,''),e.created_at,e.payload`+where+` ORDER BY e.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*domain.CostMaintenanceLog{}
	for rows.Next() {
		v := new(domain.CostMaintenanceLog)
		if err = rows.Scan(&v.ID, &v.TaskID, &v.OperatorID, &v.OperatorName, &v.CreatedAt, &v.Payload); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r *costMaintenanceRepo) Project(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem) error {
	q := Unwrap(tx)
	if !item.ERPSyncRequired {
		_, err := q.ExecContext(ctx, `UPDATE erp_product_sync_records SET cost_price=?,latest_cost_snapshot_id=(SELECT MAX(id) FROM omp_sku_cost_snapshots WHERE sku_code=?) WHERE task_sku_item_id=?`, item.CostPrice, item.SKUCode, item.ID)
		return err
	}
	// Update existing read mirrors in the same transaction. No full-catalog refresh.
	_, err := q.ExecContext(ctx, `UPDATE erp_product_sync_records SET product_name=?,product_i_id=?,erp_i_id=?,cost_price=?,base_sync_status=IF(?,'queued',base_sync_status),erp_sync_status=IF(?,'queued',erp_sync_status),sync_claim_token=IF(?,'',sync_claim_token),base_sync_error='',updated_at=UTC_TIMESTAMP() WHERE task_sku_item_id=?`, item.ProductNameSnapshot, taskStyle(item), taskStyle(item), item.CostPrice, item.ERPSyncRequired, item.ERPSyncRequired, item.ERPSyncRequired, item.ID)
	if err != nil {
		return err
	}
	if _, err = q.ExecContext(ctx, `UPDATE erp_product_sync_records SET latest_cost_snapshot_id=(SELECT MAX(id) FROM omp_sku_cost_snapshots WHERE sku_code=?) WHERE task_sku_item_id=?`, item.SKUCode, item.ID); err != nil {
		return err
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	if _, err = q.ExecContext(ctx, `INSERT INTO task_erp_outbox(task_id,job_type,dedupe_key,payload_json)
 SELECT task_id,'task_filing',CONCAT('cost_maintenance_base:',id,':',erp_sync_version),JSON_OBJECT('task_id',task_id,'operator_id',?,'source','business_info_patch','target_sku_item_ids',JSON_ARRAY(id)) FROM task_sku_items WHERE id=?
 ON DUPLICATE KEY UPDATE dedupe_key=task_erp_outbox.dedupe_key`, actor.ID, item.ID); err != nil {
		return err
	}
	return reindexTaskSearchDocument(ctx, q, item.TaskID)
}
func taskStyle(item *domain.TaskSKUItem) string { return strings.TrimSpace(item.ProductIID) }

func (r *costMaintenanceRepo) Save(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem, primary, business bool) error {
	q := Unwrap(tx)
	if business {
		if _, err := q.ExecContext(ctx, `UPDATE task_sku_items SET product_name_snapshot=?,product_short_name=?,product_i_id=?,variant_json=?,erp_sync_required=1,erp_sync_version=erp_sync_version+1 WHERE id=?`, item.ProductNameSnapshot, item.ProductShortName, item.ProductIID, item.VariantJSON, item.ID); err != nil {
			return err
		}
	}
	if err := (&taskRepo{db: r.db}).UpdateSKUCostOnly(ctx, tx, item, primary); err != nil {
		return err
	}
	if primary && business {
		if _, err := q.ExecContext(ctx, `UPDATE tasks SET product_name_snapshot=? WHERE id=?`, item.ProductNameSnapshot, item.TaskID); err != nil {
			return err
		}
		// Primary detail mirrors only explicitly supplied SKU dimensions; complex
		// per-SKU cost_input remains authoritative in variant_json.
		if _, err := q.ExecContext(ctx, `UPDATE task_details SET width=COALESCE(CAST(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(?, '$.width')),'null') AS DECIMAL(14,4)),width),height=COALESCE(CAST(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(?,'$.height')),'null') AS DECIMAL(14,4)),height),area=COALESCE(CAST(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(?,'$.area')),'null') AS DECIMAL(14,6)),area),updated_at=UTC_TIMESTAMP() WHERE task_id=?`, item.VariantJSON, item.VariantJSON, item.VariantJSON, item.TaskID); err != nil {
			return err
		}
	}
	item.ERPSyncRequired = business
	return nil
}
func (r *costMaintenanceRepo) AppendLog(ctx context.Context, tx repo.Tx, taskID, itemID int64, sku, action, reason string, actor int64, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = Unwrap(tx).ExecContext(ctx, `INSERT INTO cost_maintenance_logs(task_id,sku_item_id,sku_code,action,reason_code,operator_id,payload,created_at) VALUES(?,?,?,?,?,?,?,?)`, taskID, itemID, sku, action, reason, actor, raw, time.Now().UTC())
	return err
}
