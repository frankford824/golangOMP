package mysqlrepo

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"testing"
	"time"
	"workflow/domain"
)

// Runs only against the explicitly isolated clone created by the release QA
// harness. It never accepts the production schema as its DSN target.
func TestCostSyncIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_COST_SYNC_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var database string
	if err = db.QueryRow(`SELECT DATABASE()`).Scan(&database); err != nil || !strings.HasPrefix(database, "codex_cost_sync_test_") {
		t.Fatal("refusing non-test database")
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// Idempotent fixture reset, after the explicit isolated-schema guard above.
	for _, q := range []string{
		`DELETE FROM task_event_logs WHERE task_id=1`, `DELETE FROM task_event_sequences WHERE task_id=1`,
		`DELETE FROM omp_sku_cost_snapshots WHERE sku_code='QA-COST-1'`, `DELETE FROM omp_sku_records WHERE sku_code='QA-COST-1'`,
		`DELETE FROM erp_product_sync_records WHERE record_key='qa'`, `DELETE FROM task_details WHERE task_id=1`,
		`DELETE FROM task_sku_items WHERE id=1 AND sku_code='QA-COST-1'`, `DELETE FROM tasks WHERE id=1 AND task_no='QA-COST'`,
		`DELETE FROM sku_cost_sync_states WHERE sku_code='QA-COST-1'`, `DELETE FROM sku_cost_sync_audit WHERE sku_code='QA-COST-1'`,
		`DELETE FROM jst_cost_changes WHERE sku_id='QA-COST-1'`,
	} {
		exec(q)
	}
	exec(`INSERT INTO tasks(id,task_no,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count) VALUES(1,'QA-COST','test','QA-COST-1',1,'QA-COST-1',1)`)
	exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price) VALUES(1,'','','','{}','{}','','','','[]','',10)`)
	exec(`INSERT INTO task_sku_items(id,task_id,sequence_no,sku_code,design_requirement,reference_file_refs_json,dedupe_key,cost_price) VALUES(1,1,1,'QA-COST-1','','[]','qa',10)`)
	exec(`INSERT INTO omp_sku_records(sku_code,cost_price) VALUES('QA-COST-1',10)`)
	exec(`INSERT INTO erp_product_sync_records(record_key,task_id,task_sku_item_id,sku_code,task_created_at,cost_price) VALUES('qa',1,1,'QA-COST-1',UTC_TIMESTAMP(),10)`)
	r := &costSyncRepo{db: &DB{db: db}}
	baseline, err := r.Get(ctx, "QA-BASELINE")
	if err != nil || baseline == nil || baseline.Status != "baseline" {
		t.Fatalf("baseline improperly claimed verified: %+v %v", baseline, err)
	}
	historical, err := r.Get(ctx, "QA-HISTORY-CONFLICT")
	if err != nil || historical == nil || historical.Status != "conflict" || *historical.LocalCost != 10 || *historical.ERPCost != 12 {
		t.Fatalf("history not quarantined %+v %v", historical, err)
	}
	sku := "QA-COST-1"
	value := func(n float64) *float64 { return &n }
	if err = r.Acknowledge(ctx, sku, 1, value(10)); err != nil {
		t.Fatal(err)
	}
	// A clean ERP edit imports exactly once, marks manual protection, and updates every local mirror.
	if err = r.Observe(ctx, sku, value(12.3456)); err != nil {
		t.Fatal(err)
	}
	s, _ := r.Get(ctx, sku)
	if s.CheckedAt == nil || time.Since(*s.CheckedAt) > time.Minute || time.Until(*s.CheckedAt) > time.Minute {
		t.Fatal("checked_at has a driver timezone shift")
	}
	var created time.Time
	if err := db.QueryRow(`SELECT created_at FROM omp_sku_cost_snapshots WHERE sku_code=? ORDER BY id DESC LIMIT 1`, sku).Scan(&created); err != nil || time.Since(created) > time.Minute {
		t.Fatalf("native snapshot clock mismatch: %v %v", created, err)
	}
	if s.Status != "synced" || !s.ManualLock || !domain.EqualCost(s.LocalCost, value(12.3456)) {
		t.Fatalf("bad import %+v", s)
	}
	for _, table := range []string{"task_details", "task_sku_items", "omp_sku_records", "erp_product_sync_records"} {
		var p float64
		if err = db.QueryRow(`SELECT cost_price FROM ` + table + ` LIMIT 1`).Scan(&p); err != nil || p != 12.3456 {
			t.Fatalf("mirror %s price=%v err=%v", table, p, err)
		}
	}
	// ERP echo neither creates another local revision nor another outbound write.
	rev := s.Revision
	if err = r.Observe(ctx, sku, value(12.3456)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Revision != rev || s.NeedsCheck {
		t.Fatal("echo loop")
	}
	if err = r.Observe(ctx, sku, value(13.1111)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "synced" || s.ManualOrigin != "erp" || *s.LocalCost != 13.1111 {
		t.Fatal("consecutive ERP edits stopped synchronizing")
	}
	exec(`UPDATE task_sku_items SET cost_price=14,override_actor='qa_local' WHERE id=1`)
	if err = r.Observe(ctx, sku, value(15)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "conflict" || *s.LocalCost != 14 {
		t.Fatal("conflicting manual value overwritten")
	}
	stale := domain.CostSyncResolution{SKUCode: sku, Revision: s.Revision - 1, ERPRevision: s.ERPRevision, Choice: "erp", Reason: "QA", ActorID: 1}
	if err = r.Resolve(ctx, stale); err == nil {
		t.Fatal("stale resolution accepted")
	}
	stale.Revision = s.Revision
	if err = r.Resolve(ctx, stale); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if *s.LocalCost != 15 || s.Status != "synced" {
		t.Fatal("ERP resolution failed")
	}
	exec(`UPDATE task_sku_items SET override_actor='qa_local' WHERE id=1`)
	exec(`INSERT INTO jst_cost_changes(sku_id,new_cost_price) VALUES('QA-COST-1',16)`)
	if err = r.Collect(ctx); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if !s.NeedsCheck {
		t.Fatal("ERP event not queued")
	}
	if err = r.Observe(ctx, sku, value(16)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if err = r.Resolve(ctx, domain.CostSyncResolution{SKUCode: sku, Revision: s.Revision, ERPRevision: s.ERPRevision, Choice: "local", Reason: "QA local", ActorID: 1}); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "pending" || !s.NeedsCheck || *s.LocalCost != 15 {
		t.Fatal("local resolution not queued")
	}
	if err = r.Acknowledge(ctx, sku, s.Revision-1, value(15)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status == "synced" {
		t.Fatal("stale acknowledgement accepted")
	}
	if err = r.Acknowledge(ctx, sku, s.Revision, s.LocalCost); err != nil {
		t.Fatal(err)
	}
	if err = r.Fail(ctx, sku, s.Revision-1, "stale callback"); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "synced" {
		t.Fatal("old failure overwrote a newer acknowledgement")
	}
	// Reproduce a fresh-SKU manual edit blocked by the first ERP price, then
	// explicitly review that ERP price and save atomically on the same SKU.
	exec(`UPDATE task_sku_items SET cost_price=3.3,manual_cost_override=1,override_actor='operator:1' WHERE id=1`)
	if err = r.Observe(ctx, sku, value(9.9)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	editRevision := s.Revision
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &MySQLTx{tx: tx}
	if err = r.LockManualRevision(ctx, wrapped, sku, editRevision); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err = tx.Exec(`UPDATE task_sku_items SET product_name_snapshot='current name',quantity=7,erp_sync_required=0 WHERE id=1`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	taskWriter := &taskRepo{db: r.db}
	if err = taskWriter.UpdateSKUCostOnly(ctx, wrapped, &domain.TaskSKUItem{ID: 1, TaskID: 1, ProductNameSnapshot: "stale name", CostPrice: value(3.3), ManualCostOverride: true, OverrideActor: "operator:1"}, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = r.StageManualCost(ctx, wrapped, sku, value(9.9), 1, "QA reviewed current ERP"); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var keptName string
	var keptQty int
	var filing bool
	if err = db.QueryRow(`SELECT product_name_snapshot,quantity,erp_sync_required FROM task_sku_items WHERE id=1`).Scan(&keptName, &keptQty, &filing); err != nil || keptName != "current name" || keptQty != 7 || filing {
		t.Fatalf("manual save replayed business fields %s %d %t %v", keptName, keptQty, filing, err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "pending" || !s.NeedsCheck || s.Revision <= editRevision || !domain.EqualCost(s.ERPCost, value(9.9)) {
		t.Fatalf("manual retry remained stuck %+v", s)
	}
	if err = r.Observe(ctx, sku, value(9.9)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "pending" {
		t.Fatalf("reviewed ERP baseline re-conflicted %+v", s)
	}
	// A stale tab cannot save after the reviewed intent advanced its revision.
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.LockManualRevision(ctx, &MySQLTx{tx: tx}, sku, editRevision); err == nil {
		tx.Rollback()
		t.Fatal("stale local baseline accepted")
	}
	tx.Rollback()
	if err = r.Observe(ctx, sku, value(10.1)); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, sku)
	if s.Status != "conflict" || *s.LocalCost != 3.3 {
		t.Fatal("post-save ERP edit overwritten")
	}
	t.Log("PASS triggers, 4dp mirrors, manual conflict, stale revisions, ERP event cursor, no echo")
}
