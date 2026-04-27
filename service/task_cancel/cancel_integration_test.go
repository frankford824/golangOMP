//go:build integration

package task_cancel

import (
	"context"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

func TestUserCancelUpdatesTaskAndEventsIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	taskID := int64(10040)
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
	})
	defer r35.CleanupTaskIDs(t, db, taskID)

	mysqlDB := mysqlrepo.New(db)
	svc := NewService(mysqlrepo.NewTaskRepo(mysqlDB), mysqlrepo.NewTaskModuleRepo(mysqlDB), mysqlrepo.NewTaskModuleEventRepo(mysqlDB), mysqlDB)
	dec := svc.Cancel(context.Background(), Request{
		Actor:  domain.RequestActor{ID: 10001, Team: "operations_r35", Roles: []domain.Role{domain.RoleMember}},
		TaskID: taskID,
		Reason: "user_cancel",
	})
	if !dec.OK {
		t.Fatalf("cancel failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}

	var status string
	if err := db.QueryRow(`SELECT task_status FROM tasks WHERE id=?`, taskID).Scan(&status); err != nil {
		t.Fatalf("select task status: %v", err)
	}
	if status != string(domain.TaskStatusCancelled) {
		t.Fatalf("task_status=%s, want Cancelled", status)
	}
	var events int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		  FROM task_module_events e
		  JOIN task_modules m ON m.id = e.task_module_id
		 WHERE m.task_id=? AND e.event_type='task_cancelled'`, taskID).Scan(&events); err != nil {
		t.Fatalf("count task_cancelled events: %v", err)
	}
	if events == 0 {
		t.Fatalf("task_cancelled events=0, want at least 1")
	}
}
