package mysqlrepo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

type maintenanceERPReadOnlyStub struct {
	service.ERPBridgeClient
	reads int
}

func (s *maintenanceERPReadOnlyStub) GetProductByID(_ context.Context, sku string) (*domain.ERPProduct, error) {
	s.reads++
	price := 9.9
	return &domain.ERPProduct{SKUID: sku, CostPrice: &price}, nil
}

type deadlockMaintenanceProjection struct {
	repo.CostMaintenanceRepo
	entered       chan struct{}
	sourceStarted chan struct{}
	calls         int
}

func (r *deadlockMaintenanceProjection) Project(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem) error {
	r.calls++
	if r.calls == 1 {
		close(r.entered)
		select {
		case <-r.sourceStarted:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.CostMaintenanceRepo.Project(ctx, tx, item)
}

func TestCostMaintenanceDeadlockIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_COST_DEADLOCK_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	var database string
	if err = db.QueryRow(`SELECT DATABASE()`).Scan(&database); err != nil || !strings.HasPrefix(database, "codex_cost_deadlock_test_") {
		t.Fatal("refusing non-test database")
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`SET FOREIGN_KEY_CHECKS=0`)
	for _, table := range []string{"tasks", "task_details", "task_sku_items", "erp_product_sync_records", "sku_cost_sync_states", "sku_cost_sync_audit", "cost_maintenance_logs", "task_event_logs", "task_event_sequences", "task_erp_outbox"} {
		exec("DELETE FROM " + table)
	}
	for _, id := range []int64{71, 72} {
		exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(?,?,'original_product_development','Completed','test',?,7,?,1,'original')`, id, fmt.Sprintf("QA-DEADLOCK-%d", id), fmt.Sprintf("QA-DEADLOCK-%d", id), fmt.Sprintf("QA-DEADLOCK-%d", id))
		exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,width,height,area) VALUES(?,'','','','{}','{}','','','','[]','',3.3,20,30,.06)`, id)
	}
	wrapper := mysqlrepo.New(db)
	tasks := mysqlrepo.NewTaskRepo(wrapper)
	price := 3.3
	item := &domain.TaskSKUItem{TaskID: 71, SequenceNo: 1, SKUCode: "QA-DEADLOCK-71", SKUStatus: domain.TaskSKUStatusGenerated, ProductNameSnapshot: "original", VariantJSON: json.RawMessage(`{"width":20,"height":30,"area":0.06}`), CostPrice: &price, ManualCostOverride: true}
	if err = wrapper.RunInTx(context.Background(), func(tx repo.Tx) error {
		return tasks.CreateSKUItems(context.Background(), tx, []*domain.TaskSKUItem{item})
	}); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM task_sku_items WHERE task_id=71`).Scan(&item.ID); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO sku_cost_sync_states(sku_code,local_cost,local_confirmed,erp_cost,revision,manual_lock,manual_origin,status) VALUES('QA-DEADLOCK-71',3.3,1,9.9,2,1,'local','synced')`)
	if err = mysqlrepo.NewProductManagementRepo(wrapper).RefreshReadModel(context.Background()); err != nil {
		t.Fatal(err)
	}
	var target, unrelated int64
	if err = db.QueryRow(`SELECT id FROM erp_product_sync_records WHERE task_sku_item_id=?`, item.ID).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM erp_product_sync_records WHERE task_id=72`).Scan(&unrelated); err != nil {
		t.Fatal(err)
	}
	projection := mysqlrepo.NewCostMaintenanceRepo(wrapper)
	// Reproduce the reported PRIMARY scan: a lock on an unrelated mirror row
	// blocks the old predicate. The replacement must finish while it stays held.
	other, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.Exec(`UPDATE erp_product_sync_records SET creator_name='held' WHERE id=?`, unrelated); err != nil {
		t.Fatal(err)
	}
	probe, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	old, beginErr := db.BeginTx(probe, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	_, err = old.ExecContext(probe, `UPDATE erp_product_sync_records FORCE INDEX(PRIMARY) SET cost_price=3.3 WHERE task_sku_item_id=?`, item.ID)
	_ = old.Rollback()
	cancel()
	if err == nil {
		t.Fatal("old scan did not reproduce unrelated-row blocking")
	}
	probe, cancel = context.WithTimeout(context.Background(), time.Second)
	err = wrapper.RunInReadCommittedTx(probe, func(tx repo.Tx) error { return projection.Project(probe, tx, item) })
	cancel()
	_ = other.Rollback()
	if err != nil {
		t.Fatalf("primary-key projection still blocked on unrelated row: %v", err)
	}
	// Give the background transaction more undo work so InnoDB deterministically
	// selects the small foreground transaction as its deadlock victim.
	for id := 1000; id < 1100; id++ {
		exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,product_name_snapshot) VALUES(?,?,'original_product_development','Completed','test','',7,'ballast')`, id, fmt.Sprintf("QA-BALLAST-%d", id))
	}
	background, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = background.Exec(`UPDATE tasks SET product_name_snapshot='background refresh' WHERE id BETWEEN 1000 AND 1099`); err != nil {
		t.Fatal(err)
	}
	if _, err = background.Exec(`UPDATE erp_product_sync_records SET creator_name='background refresh' WHERE id=?`, target); err != nil {
		t.Fatal(err)
	}
	store := &deadlockMaintenanceProjection{CostMaintenanceRepo: projection, entered: make(chan struct{}), sourceStarted: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		select {
		case <-store.entered:
		case <-ctx.Done():
			_ = background.Rollback()
			done <- ctx.Err()
			return
		}
		close(store.sourceStarted)
		var v float64
		e := background.QueryRowContext(ctx, `SELECT cost_price FROM task_details WHERE task_id=71 FOR SHARE`).Scan(&v)
		if e != nil {
			_ = background.Rollback()
			done <- e
			return
		}
		done <- background.Commit()
	}()
	observer := &maintenanceERPReadOnlyStub{}
	sync := service.NewCostSyncService(mysqlrepo.NewCostSyncRepo(wrapper), observer)
	svc := service.NewTaskService(tasks, nil, mysqlrepo.NewTaskEventRepo(wrapper), nil, nil, wrapper, service.WithTaskCostSyncService(sync), service.WithTaskCostMaintenanceRepo(store)).(service.CostMaintenanceService)
	actor := domain.RequestActor{ID: 7, Permissions: []domain.PermissionCode{domain.PermissionCatalogManage, domain.PermissionERPManage}, EffectiveAccess: &domain.EffectiveAccess{UserID: 7, Permissions: []domain.PermissionCode{domain.PermissionCatalogManage, domain.PermissionERPManage}, Assignments: []domain.AccessAssignment{{RoleID: 9, ScopeMode: domain.AccessScopeGlobal}}, Sources: []domain.EffectiveAccessNote{{RoleID: 9, Permission: domain.PermissionCatalogManage}, {RoleID: 9, Permission: domain.PermissionERPManage}}}}
	ctx = domain.WithRequestActor(ctx, actor)
	list, appErr := svc.ListCostMaintenance(ctx, repo.CostMaintenanceFilter{PageSize: 50, ReviewState: "all"})
	if appErr != nil || len(list.Data) != 1 {
		t.Fatalf("fixture read: %v", appErr)
	}
	row := list.Data[0]
	request := domain.CostMaintenanceRequest{Action: "confirm", Token: row.Token, Revision: row.Revision, ERPCost: row.ERPCost}
	result, appErr := svc.MaintainCost(ctx, item.ID, request)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if err = <-done; err != nil {
		t.Fatal("background transaction did not finish", err)
	}
	if store.calls < 2 || !result.Reviewed || *result.CostPrice != 3.3 || observer.reads != 1 {
		t.Fatalf("retry/read/commit result: attempts=%d ERP reads=%d result=%+v", store.calls, observer.reads, result)
	}
	for _, query := range []string{`SELECT COUNT(*) FROM cost_maintenance_logs`, `SELECT COUNT(*) FROM task_event_logs WHERE event_type='cost_maintenance'`, `SELECT COUNT(*) FROM sku_cost_sync_audit WHERE action='manual_saved'`} {
		var n int
		if err = db.QueryRow(query).Scan(&n); err != nil || n != 1 {
			t.Fatalf("rollback leaked or duplicated audit: %s: %d %v", query, n, err)
		}
	}
	var revision int64
	if err = db.QueryRow(`SELECT revision FROM sku_cost_sync_states WHERE sku_code='QA-DEADLOCK-71'`).Scan(&revision); err != nil || revision != 3 {
		t.Fatalf("intent revision duplicated: %d %v", revision, err)
	}
	_, appErr = svc.MaintainCost(ctx, item.ID, request)
	if appErr == nil || appErr.Code != domain.ErrCodeConflict || observer.reads != 1 {
		t.Fatal("a stale client request replayed the successful confirmation", appErr)
	}
}
