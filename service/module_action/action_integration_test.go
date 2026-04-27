//go:build integration

package module_action

import (
	"context"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service/blueprint"
	"workflow/testsupport/r35"
)

func TestAuditApproveEntersWarehouseIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	taskID := int64(10030)
	actorID := int64(3001)
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStateSubmitted), ClaimedBy: r35.Int64Ptr(3000), ClaimedTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
		{Key: domain.ModuleKeyAudit, State: string(domain.ModuleStateInProgress), ClaimedBy: &actorID, ClaimedTeamCode: r35.StrPtr(domain.TeamAuditStandard)},
		{Key: domain.ModuleKeyWarehouse, State: string(domain.ModuleStatePending)},
	})
	defer r35.CleanupTaskIDs(t, db, taskID)

	mysqlDB := mysqlrepo.New(db)
	modules := mysqlrepo.NewTaskModuleRepo(mysqlDB)
	events := mysqlrepo.NewTaskModuleEventRepo(mysqlDB)
	svc := NewActionService(
		mysqlrepo.NewTaskRepo(mysqlDB),
		modules,
		events,
		mysqlrepo.NewReferenceFileRefFlatRepo(mysqlDB),
		mysqlDB,
		blueprint.NewRuleEngine(blueprint.NewRegistry(), modules, events),
	)
	dec := svc.Apply(context.Background(), ActionRequest{
		Actor:     domain.RequestActor{ID: actorID, Team: domain.TeamAuditStandard, Roles: []domain.Role{domain.RoleMember}},
		TaskID:    taskID,
		ModuleKey: domain.ModuleKeyAudit,
		Action:    domain.ModuleActionApprove,
	})
	if !dec.OK {
		t.Fatalf("audit approve failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}

	var auditState, warehouseState string
	if err := db.QueryRow(`SELECT state FROM task_modules WHERE task_id=? AND module_key='audit'`, taskID).Scan(&auditState); err != nil {
		t.Fatalf("select audit state: %v", err)
	}
	if err := db.QueryRow(`SELECT state FROM task_modules WHERE task_id=? AND module_key='warehouse'`, taskID).Scan(&warehouseState); err != nil {
		t.Fatalf("select warehouse state: %v", err)
	}
	if auditState != string(domain.ModuleStateClosed) {
		t.Fatalf("audit state=%s, want closed", auditState)
	}
	if warehouseState != string(domain.ModuleStatePendingClaim) {
		t.Fatalf("warehouse state=%s, want pending_claim", warehouseState)
	}

	var evidence int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		  FROM task_module_events e
		  JOIN task_modules m ON m.id = e.task_module_id
		 WHERE m.task_id = ?
		   AND ((m.module_key='audit' AND e.event_type='approved')
		     OR (m.module_key='warehouse' AND e.event_type='entered'))`, taskID).Scan(&evidence); err != nil {
		t.Fatalf("count approve/entered events: %v", err)
	}
	if evidence != 2 {
		t.Fatalf("approved+entered evidence events=%d, want 2", evidence)
	}
}
