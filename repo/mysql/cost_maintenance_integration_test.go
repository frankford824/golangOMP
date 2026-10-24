package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

func TestCostMaintenanceIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_COST_SYNC_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var name string
	if err = db.QueryRow(`SELECT DATABASE()`).Scan(&name); err != nil || !strings.HasPrefix(name, "codex_cost_sync_test_") {
		t.Fatal("refusing non-test database")
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`SET FOREIGN_KEY_CHECKS=0`)
	for _, q := range []string{`DELETE FROM task_erp_outbox`, `DELETE FROM cost_maintenance_logs`, `DELETE FROM task_sku_items`, `DELETE FROM task_details`, `DELETE FROM tasks`, `DELETE FROM sku_cost_sync_states`} {
		exec(q)
	}
	exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(71,'QA-MAINT','original_product_development','Completed','test','QA-MAINT-1',1,'QA-MAINT-1',1,'original')`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,width,height,area) VALUES(71,'','','','{}','{}','','','','[]','',3.3,20,30,.06)`)
	wrapper := &DB{db: db}
	ctx := context.Background()
	tx, sqlTx, err := wrapper.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = ensureCompletedCostSKU(ctx, tx, 71); err != nil {
		t.Fatal(err)
	}
	if err = ensureCompletedCostSKU(ctx, tx, 71); err != nil {
		t.Fatal(err)
	}
	if err = sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM task_sku_items WHERE sku_code='QA-MAINT-1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate completion identity %d %v", count, err)
	}
	r := &costMaintenanceRepo{db: wrapper}
	rows, err := r.List(ctx, repo.CostMaintenanceFilter{PageSize: 50})
	if err != nil || len(rows) != 1 {
		t.Fatalf("canonical list %+v %v", rows, err)
	}
	if string(rows[0].ReviewJSON) != "{}" {
		t.Fatalf("new SKU unexpectedly has a maintenance audit: %s", rows[0].ReviewJSON)
	}
	for _, commit := range []bool{false, true} {
		tx, sqlTx, err = wrapper.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		locked, e := r.LockRow(ctx, tx, rows[0].ID)
		if e != nil || locked == nil {
			t.Fatal(e)
		}
		price := 4.4
		item := &domain.TaskSKUItem{ID: locked.ID, TaskID: 71, SKUCode: locked.SKUCode, ProductNameSnapshot: "original", VariantJSON: json.RawMessage(`{"width":20,"height":30}`), CostPrice: &price, ManualCostOverride: true, ManualCostOverrideReason: "cost_incorrect", OverrideActor: "1"}
		if e = r.Save(ctx, tx, item, true, false); e != nil {
			t.Fatal(e)
		}
		if e = r.Project(ctx, tx, item); e != nil {
			t.Fatal(e)
		}
		if e = r.AppendLog(ctx, tx, 71, item.ID, item.SKUCode, "modify", "cost_incorrect", 1, map[string]interface{}{"before": locked, "after": map[string]interface{}{"cost_price": price}}); e != nil {
			t.Fatal(e)
		}
		if commit {
			err = sqlTx.Commit()
		} else {
			err = sqlTx.Rollback()
		}
		if err != nil {
			t.Fatal(err)
		}
		var cost float64
		db.QueryRow(`SELECT cost_price FROM task_sku_items WHERE id=?`, item.ID).Scan(&cost)
		db.QueryRow(`SELECT COUNT(*) FROM cost_maintenance_logs`).Scan(&count)
		if commit && (cost != 4.4 || count != 1) || !commit && (cost != 3.3 || count != 0) {
			t.Fatalf("partial commit cost=%v logs=%d commit=%v", cost, count, commit)
		}
		listed, listErr := r.List(ctx, repo.CostMaintenanceFilter{PageSize: 50})
		if listErr != nil || len(listed) != 1 {
			t.Fatalf("read committed review: %v", listErr)
		}
		var audit struct {
			After struct {
				CostPrice *float64 `json:"cost_price"`
			} `json:"after"`
		}
		if err = json.Unmarshal(listed[0].ReviewJSON, &audit); err != nil || (audit.After.CostPrice != nil) != commit {
			t.Fatalf("review did not follow atomic commit/rollback: %s %v", listed[0].ReviewJSON, err)
		}
	}
	var outboxCount int
	db.QueryRow(`SELECT COUNT(*) FROM task_erp_outbox WHERE task_id=71`).Scan(&outboxCount)
	if outboxCount != 0 {
		t.Fatal("price-only confirmation replayed metadata")
	}
	tx, sqlTx, err = wrapper.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	price := 4.4
	changed := &domain.TaskSKUItem{ID: rows[0].ID, TaskID: 71, SKUCode: "QA-MAINT-1", ProductNameSnapshot: "renamed", ProductShortName: "renamed", ProductIID: "KT", VariantJSON: json.RawMessage(`{"width":null,"height":null,"area":null}`), CostPrice: &price, ManualCostOverride: true}
	if err = r.Save(ctx, tx, changed, true, true); err != nil {
		t.Fatal(err)
	}
	if err = r.Project(ctx, tx, changed); err != nil {
		t.Fatal(err)
	}
	if err = sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	var width float64
	var target int64
	db.QueryRow(`SELECT width FROM task_details WHERE task_id=71`).Scan(&width)
	db.QueryRow(`SELECT JSON_EXTRACT(payload_json,'$.target_sku_item_ids[0]') FROM task_erp_outbox WHERE task_id=71 ORDER BY id DESC LIMIT 1`).Scan(&target)
	if width != 20 || target != changed.ID {
		t.Fatalf("null dimension or durable metadata scope: %v %v", width, target)
	}
	var shortName, style string
	db.QueryRow(`SELECT product_short_name,JSON_UNQUOTE(JSON_EXTRACT(product_selection_snapshot_json,'$.erp_product.i_id')) FROM task_details WHERE task_id=71`).Scan(&shortName, &style)
	if shortName != "renamed" || style != "KT" {
		t.Fatalf("original-product metadata projection lost: %s %s", shortName, style)
	}
	rows, err = r.List(ctx, repo.CostMaintenanceFilter{PageSize: 50})
	if err != nil || !rows[0].BaseSyncRequired {
		t.Fatal("pending metadata was hidden behind cost convergence", err)
	}
	exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(72,'QA-MAINT-OLD','original_product_development','Completed','test','QA-MAINT-OLD',1,'QA-MAINT-OLD',1,'historical')`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,width,height,area) VALUES(72,'','','','{}','{}','','','','[]','',7.77,NULL,NULL,NULL)`)
	path := os.Getenv("TEST_COST_MAINTENANCE_MIGRATION")
	if path == "" {
		path = "../../db/migrations/145_cost_maintenance_historical_skus.sql"
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		for _, q := range strings.Split(string(raw), ";") {
			if strings.TrimSpace(q) != "" {
				exec(q)
			}
		}
	}
	var original, canonical float64
	var state string
	db.QueryRow(`SELECT d.cost_price,s.cost_price,cs.status FROM task_details d JOIN task_sku_items s ON s.task_id=d.task_id JOIN sku_cost_sync_states cs ON cs.sku_code=s.sku_code WHERE d.task_id=72`).Scan(&original, &canonical, &state)
	if original != 7.77 || canonical != original || state != "conflict" {
		t.Fatalf("historical preservation failed %v %v %s", original, canonical, state)
	}
	raw, e = os.ReadFile(filepath.Join(filepath.Dir(path), "146_cost_maintenance_unknown_dimensions.sql"))
	if e != nil {
		t.Fatal(e)
	}
	for _, statement := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(statement) != "" {
			exec(statement)
		}
	}
	var unknown int
	db.QueryRow(`SELECT COUNT(*) FROM task_sku_items WHERE task_id=72 AND JSON_TYPE(JSON_EXTRACT(variant_json,'$.width')) IS NULL`).Scan(&unknown)
	if unknown != 1 {
		t.Fatal("unknown width was not kept absent")
	}
	exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(73,'QA-MAINT-NEW','original_product_development','Completed','test','QA-MAINT-NEW',1,'QA-MAINT-NEW',1,'new completion')`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price) VALUES(73,'','','','{}','{}','','','','[]','',8.88)`)
	tx, sqlTx, err = wrapper.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = ensureCompletedCostSKU(ctx, tx, 73); err != nil {
		t.Fatal(err)
	}
	if err = sqlTx.Commit(); err != nil {
		t.Fatal(err)
	}
	// Regression: migration and future completion must both remain compatible
	// with the existing strict numeric dashboard materialization.
	if err = NewProductManagementRepo(wrapper).RefreshReadModel(ctx); err != nil {
		t.Fatalf("cost dashboard refresh regressed: %v", err)
	}
}
