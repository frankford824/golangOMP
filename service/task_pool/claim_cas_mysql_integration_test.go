//go:build integration

package task_pool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service/permission"
	"workflow/testsupport/r35"
)

func TestClaimCAS_100Concurrent_MySQL(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	taskID := int64(10010)
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
	})
	defer r35.CleanupTaskIDs(t, db, taskID)

	mysqlDB := mysqlrepo.New(db)
	svc := NewClaimService(
		mysqlrepo.NewTaskRepo(mysqlDB),
		mysqlrepo.NewTaskModuleRepo(mysqlDB),
		mysqlrepo.NewTaskModuleEventRepo(mysqlDB),
		mysqlDB,
	)

	var successCount int64
	var conflictCount int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actor := domain.RequestActor{ID: int64(i + 1), Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}
			dec := svc.Claim(context.Background(), actor, taskID, domain.ModuleKeyDesign, domain.TeamDesignStandard)
			if dec.OK {
				atomic.AddInt64(&successCount, 1)
				return
			}
			if dec.DenyCode == permission.DenyModuleClaimConflict {
				atomic.AddInt64(&conflictCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 || conflictCount != 99 {
		t.Fatalf("real MySQL CAS: success=%d conflict=%d, want 1/99", successCount, conflictCount)
	}
	var finalState string
	var claimedBy int64
	if err := db.QueryRow(`SELECT state, claimed_by FROM task_modules WHERE task_id=? AND module_key=?`, taskID, domain.ModuleKeyDesign).Scan(&finalState, &claimedBy); err != nil {
		t.Fatalf("select final module state: %v", err)
	}
	if finalState != string(domain.ModuleStateInProgress) {
		t.Fatalf("state=%s, want in_progress", finalState)
	}
	if claimedBy < 1 || claimedBy > 100 {
		t.Fatalf("claimed_by=%d out of range", claimedBy)
	}
	var claimEvents int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		  FROM task_module_events e
		  JOIN task_modules tm ON tm.id = e.task_module_id
		 WHERE tm.task_id=? AND tm.module_key=? AND e.event_type='claimed'`,
		taskID, domain.ModuleKeyDesign).Scan(&claimEvents); err != nil {
		t.Fatalf("count claimed events: %v", err)
	}
	if claimEvents != 1 {
		t.Fatalf("claimed events=%d, want 1", claimEvents)
	}
}

func TestClaimCAS_TwoClaims_MySQLIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	taskID := int64(10011)
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
	})
	defer r35.CleanupTaskIDs(t, db, taskID)

	mysqlDB := mysqlrepo.New(db)
	svc := NewClaimService(mysqlrepo.NewTaskRepo(mysqlDB), mysqlrepo.NewTaskModuleRepo(mysqlDB), mysqlrepo.NewTaskModuleEventRepo(mysqlDB), mysqlDB)
	first := svc.Claim(context.Background(), domain.RequestActor{ID: 1, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}, taskID, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if !first.OK {
		t.Fatalf("first claim failed: code=%s message=%s", first.DenyCode, first.Message)
	}
	second := svc.Claim(context.Background(), domain.RequestActor{ID: 2, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}, taskID, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if second.OK || second.DenyCode != permission.DenyModuleClaimConflict {
		t.Fatalf("second claim = ok:%t code:%s, want 409/module_claim_conflict behavior", second.OK, second.DenyCode)
	}
}
