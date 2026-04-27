//go:build integration

package task_pool

import (
	"context"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

func TestPoolQueryFiltersBackfillPlaceholderIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	visibleTaskID := int64(10020)
	placeholderTaskID := int64(10021)
	r35.InsertTaskWithModules(t, db, visibleTaskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityHigh), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard), Data: `{}`},
	})
	r35.InsertTaskWithModules(t, db, placeholderTaskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityCritical), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard), Data: `{"backfill_placeholder":true}`},
	})
	defer r35.CleanupTaskIDs(t, db, visibleTaskID, placeholderTaskID)

	svc := NewPoolQueryService(mysqlrepo.New(db))
	rows, err := svc.List(context.Background(), domain.RequestActor{ID: 1, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}, domain.ModuleKeyDesign, domain.TeamDesignStandard, 100, 0)
	if err != nil {
		t.Fatalf("pool list: %v", err)
	}
	foundVisible := false
	for _, row := range rows {
		if row.TaskID == placeholderTaskID {
			t.Fatalf("pool returned backfill placeholder task_id=%d", placeholderTaskID)
		}
		if row.TaskID == visibleTaskID {
			foundVisible = true
		}
	}
	if !foundVisible {
		t.Fatalf("pool did not return non-placeholder task_id=%d", visibleTaskID)
	}
}
