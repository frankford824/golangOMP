package mysqlrepo

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

// Uses an isolated schema with the five production table definitions. Never run
// against the application database: this test deliberately changes fixture data.
func TestProductRefreshMemoryIntegration(t *testing.T) {
	dsn := os.Getenv("PRODUCT_REFRESH_TEST_DSN")
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
	if err = db.QueryRow("SELECT DATABASE()").Scan(&name); err != nil || !strings.HasPrefix(name, "codex_product_refresh_test_") {
		t.Fatal("refusing non-test database", err)
	}
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec("SET FOREIGN_KEY_CHECKS=0")
	// TABLES caches AUTO_INCREMENT for 24 hours by default. Read the live
	// counter so this assertion fails for the original duplicate-insert query.
	exec("SET SESSION information_schema_stats_expiry=0")
	for _, table := range []string{"erp_product_sync_records", "task_sku_items", "task_details", "tasks", "users"} {
		exec("DELETE FROM " + table)
	}
	exec(`INSERT INTO users(id,username,display_name,team,password_hash) VALUES(1,'test','Test','','test-only')`)
	exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(71,'QA-MEM','original_product_development','Completed','test','QA-MEM-1',1,'QA-MEM-1',1,'Original')`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,width,height,area) VALUES(71,'','','','{}','{}','','','','[]','',3.3,20,30,.06)`)
	r := &productManagementRepo{db: New(db)}
	refresh := func() {
		t.Helper()
		if err := r.refreshMainTaskRecords(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := r.refreshSKUItemRecords(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	checkIdle := func() {
		t.Helper()
		var before, after uint64
		q := `SELECT AUTO_INCREMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='erp_product_sync_records'`
		if err := db.QueryRow(q).Scan(&before); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 30; i++ {
			refresh()
		}
		if err := db.QueryRow(q).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatalf("unchanged refresh still attempts inserts: auto_increment %d -> %d", before, after)
		}
	}
	refresh()
	checkIdle()
	exec(`UPDATE tasks SET product_name_snapshot='original' WHERE id=71`)
	refresh()
	var product string
	if err := db.QueryRow(`SELECT product_name FROM erp_product_sync_records WHERE record_key='task:71:main'`).Scan(&product); err != nil || product != "original" {
		t.Fatal("case-only edit lost", product, err)
	}
	exec(`UPDATE task_details SET cost_price=NULL WHERE task_id=71`)
	refresh()
	var cost sql.NullFloat64
	if err := db.QueryRow(`SELECT cost_price FROM erp_product_sync_records WHERE record_key='task:71:main'`).Scan(&cost); err != nil || cost.Valid {
		t.Fatal("NULL cost lost", cost, err)
	}
	exec(`INSERT INTO task_sku_items(id,task_id,sequence_no,sku_code,design_requirement,reference_file_refs_json,dedupe_key,cost_price) VALUES(1,71,1,'QA-MEM-SKU','','[]','qa',10)`)
	refresh()
	checkIdle()
	exec(`UPDATE erp_product_sync_records SET erp_sync_status='queued',base_sync_status='queued' WHERE task_sku_item_id=1`)
	exec(`UPDATE task_sku_items SET cost_price=12 WHERE id=1`)
	refresh()
	var status string
	if err := db.QueryRow(`SELECT cost_price,base_sync_status FROM erp_product_sync_records WHERE task_sku_item_id=1`).Scan(&cost, &status); err != nil || cost.Float64 != 12 || status != "queued" {
		t.Fatal("queued edit regressed", cost, status, err)
	}
	exec(`UPDATE erp_product_sync_records SET erp_sync_status='failed',base_sync_status='failed' WHERE task_sku_item_id=1`)
	exec(`UPDATE task_sku_items SET erp_sync_status='filed',filing_status='filed' WHERE id=1`)
	refresh()
	if err := db.QueryRow(`SELECT base_sync_status FROM erp_product_sync_records WHERE task_sku_item_id=1`).Scan(&status); err != nil || status != "synced" {
		t.Fatal("filing success lost", status, err)
	}
	checkIdle()
}
