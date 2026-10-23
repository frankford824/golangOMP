package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
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
	rows, err = r.List(ctx, repo.CostMaintenanceFilter{PageSize: 50})
	if err != nil || !rows[0].BaseSyncRequired {
		t.Fatal("pending metadata was hidden behind cost convergence", err)
	}
	exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(72,'QA-MAINT-OLD','original_product_development','Completed','test','QA-MAINT-OLD',1,'QA-MAINT-OLD',1,'historical')`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,width,height,area) VALUES(72,'','','','{}','{}','','','','[]','',7.77,40,50,.2)`)
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
}
