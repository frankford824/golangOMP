package mysqlrepo_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

func TestTaskCostSpecificationIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_TASK_COST_SPEC_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var database string
	if err = db.QueryRow(`SELECT DATABASE()`).Scan(&database); err != nil || !strings.HasPrefix(database, "codex_task_cost_spec_test_") {
		t.Fatal("refusing non-test database")
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`SET FOREIGN_KEY_CHECKS=0`)
	for _, table := range []string{"tasks", "task_details", "task_sku_items", "cost_rules", "cost_rule_bindings", "task_event_logs", "task_event_sequences", "cost_maintenance_logs", "search_reindex_outbox", "task_search_documents", "task_erp_outbox"} {
		exec("DELETE FROM " + table)
	}
	exec(`INSERT INTO cost_rules(rule_name,category_code,rule_type,formula_expression,remark) VALUES('area','AREA','cost_model','{"version":1,"material":"test","basis":"area","unit_price":10,"multiplier":1}',''),('piece','PIECE','cost_model','{"version":1,"material":"test","basis":"piece","unit_price":10,"multiplier":1}',''),('manual','MAN','manual_quote','',''),('old area','MIX','fixed_unit_price','',''),('new piece','MIX','cost_model','{"version":1,"material":"test","basis":"piece","unit_price":10,"multiplier":1}','')`)
	wrapper := mysqlrepo.New(db)
	tasks := mysqlrepo.NewTaskRepo(wrapper)
	ctx := context.Background()
	for _, id := range []int64{71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81} {
		creator := int64(7)
		if id == 72 {
			creator = 8
		}
		status := "Completed"
		if id == 73 {
			status = "PendingAudit"
		}
		if id == 75 {
			status = "Archived"
		}
		group := "AREA"
		if id == 74 {
			group = "PIECE"
		}
		if id == 78 {
			group = "MAN"
		}
		if id == 79 {
			group = "MIX"
		}
		sku := fmt.Sprintf("QA-SPEC-%d", id)
		exec(`INSERT INTO tasks(id,task_no,task_type,task_status,source_mode,sku_code,creator_id,primary_sku_code,batch_item_count,product_name_snapshot) VALUES(?,?,'original_product_development',?,'test',?,?,?,1,'test')`, id, sku, status, sku, creator, sku)
		exec(`INSERT INTO task_details(task_id,demand_text,copy_text,remark,matched_mapping_rule_json,product_selection_snapshot_json,spec_text,change_request,design_requirement,reference_images_json,reference_link,cost_price,category_code) VALUES(?,'','','','{}','{}','','','','[]','',10,?)`, id, group)
		if id == 73 {
			continue
		}
		variant := json.RawMessage(`{}`)
		switch id {
		case 76:
			variant = json.RawMessage(`{"cost_input":{"area_mode":"faces","pieces":2,"faces":[{"width_m":0.1,"height_m":0.2,"count":2}]}}`)
		case 77:
			variant = json.RawMessage(`{"cost_input":{"area_mode":"flat","width_m":0.2,"height_m":0.3,"pieces":6}}`)
		case 80:
			variant = json.RawMessage(`{"cost_input":{"area_mode":"total","area_m2":0.5,"pieces":6}}`)
		case 81:
			variant = json.RawMessage(`{"width":20,"height":30,"area":0.06,"cost_input":{"area_mode":"total","area_m2":0,"pieces":1}}`)
		}
		price := 10.0
		item := &domain.TaskSKUItem{TaskID: id, SequenceNo: 1, SKUCode: sku, SKUStatus: domain.TaskSKUStatusGenerated, ProductNameSnapshot: "test", CategoryCode: group, VariantJSON: variant, CostPrice: &price, ManualCostOverride: true}
		if err := wrapper.RunInTx(ctx, func(tx repo.Tx) error { return tasks.CreateSKUItems(ctx, tx, []*domain.TaskSKUItem{item}) }); err != nil {
			t.Fatal(err)
		}
	}
	actor := domain.RequestActor{ID: 7, Permissions: []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskView}, EffectiveAccess: &domain.EffectiveAccess{UserID: 7, Permissions: []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskView}, Assignments: []domain.AccessAssignment{{RoleID: 9, ScopeMode: domain.AccessScopeGlobal}}, Sources: []domain.EffectiveAccessNote{{RoleID: 9, Permission: domain.PermissionTaskCreate}, {RoleID: 9, Permission: domain.PermissionTaskView}}}}
	ctx = domain.WithRequestActor(ctx, actor)
	svc := service.NewTaskService(tasks, nil, mysqlrepo.NewTaskEventRepo(wrapper), nil, nil, wrapper, service.WithTaskCostSpecificationRepo(mysqlrepo.NewTaskCostSpecificationRepo(wrapper)), service.WithTaskCostMaintenanceRepo(mysqlrepo.NewCostMaintenanceRepo(wrapper))).(service.TaskCostSpecificationService)
	queue, appErr := svc.ListCostSpecifications(ctx, 0, 1, 20)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if queue.Total != 3 {
		t.Fatalf("queue contains non-area/other creator/archived/already specified records: %+v", queue)
	}
	var target *domain.TaskCostSpecification
	for _, row := range queue.Data {
		if row.TaskID == 71 {
			target = row
		}
	}
	if target == nil || !target.CanEdit {
		t.Fatal("completed own task cannot be corrected")
	}
	result, appErr := svc.CorrectCostSpecification(ctx, 71, domain.TaskCostSpecificationUpdate{SKUItemID: target.SKUItemID, Token: target.Token, Reason: "补录面积", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .5, Pieces: 1}})
	if appErr != nil {
		t.Fatal(appErr)
	}
	if result.Data[0].MissingArea || !result.Data[0].CostReviewRequired || *result.Data[0].CostPrice != 10 || result.Data[0].TaskStatus != "Completed" {
		t.Fatalf("price/state changed or area unresolved: %+v", result)
	}
	_, appErr = svc.CorrectCostSpecification(ctx, 71, domain.TaskCostSpecificationUpdate{SKUItemID: target.SKUItemID, Token: target.Token, Reason: "旧页面补录", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .6, Pieces: 1}})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatal("stale input overwrote committed geometry")
	}
	var count int
	for _, query := range []string{`SELECT COUNT(*) FROM cost_maintenance_logs WHERE action='specification'`, `SELECT COUNT(*) FROM task_event_logs WHERE event_type='task.cost_specification_updated'`} {
		if err = db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatal("missing audit", err, count)
		}
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM task_erp_outbox`).Scan(&count); err != nil || count != 0 {
		t.Fatal("geometry correction enqueued ERP price/business writes")
	}
	queue, appErr = svc.ListCostSpecifications(ctx, 0, 1, 20)
	if appErr != nil || queue.Total != 2 {
		t.Fatalf("persistent queue did not resolve by data: %+v %v", queue, appErr)
	}
	rows, appErr := svc.ListCostSpecifications(ctx, 73, 1, 20)
	if appErr != nil {
		t.Fatal(appErr)
	}
	_, appErr = svc.CorrectCostSpecification(ctx, 73, domain.TaskCostSpecificationUpdate{Token: rows.Data[0].Token, Reason: "审核前补录面积", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .8, Pieces: 1}})
	if appErr != nil {
		t.Fatal(appErr)
	}
	queue, appErr = svc.ListCostSpecifications(ctx, 0, 1, 20)
	if appErr != nil || queue.Total != 1 {
		t.Fatalf("pre-canonical total area was not persisted: %+v %v", queue, appErr)
	}
	if err = wrapper.RunInTx(ctx, func(tx repo.Tx) error {
		return mysqlrepo.NewTaskResourceGroupRepo(wrapper).EnqueueTaskFinalized(ctx, tx, 81, 1, false, false)
	}); err != nil {
		t.Fatal(err)
	}
	var review, manual bool
	var price float64
	if err = db.QueryRow(`SELECT requires_manual_review,manual_cost_override,cost_price FROM task_sku_items WHERE task_id=81`).Scan(&review, &manual, &price); err != nil || !review || manual || price != 10 {
		t.Fatalf("completion did not withhold an unverified price: %v %v %v %v", review, manual, price, err)
	}
}
