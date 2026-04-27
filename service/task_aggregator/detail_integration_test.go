//go:build integration

package task_aggregator

import (
	"context"
	"testing"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service/blueprint"
	"workflow/testsupport/r35"
)

func TestTaskDetailModulesVisibleIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	taskID := int64(10050)
	taskType := domain.TaskTypeOriginalProductDevelopment
	r35.InsertTaskWithModules(t, db, taskID, string(taskType), string(domain.TaskPriorityNormal), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
		{Key: domain.ModuleKeyDesign, State: string(domain.ModuleStatePendingClaim), PoolTeamCode: r35.StrPtr(domain.TeamDesignStandard)},
		{Key: domain.ModuleKeyAudit, State: string(domain.ModuleStatePending)},
		{Key: domain.ModuleKeyWarehouse, State: string(domain.ModuleStatePending)},
	})
	defer r35.CleanupTaskIDs(t, db, taskID)

	mysqlDB := mysqlrepo.New(db)
	detail, err := NewDetailService(
		mysqlrepo.NewTaskRepo(mysqlDB),
		mysqlrepo.NewTaskModuleRepo(mysqlDB),
		mysqlrepo.NewTaskModuleEventRepo(mysqlDB),
		mysqlrepo.NewReferenceFileRefFlatRepo(mysqlDB),
	).Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("detail get: %v", err)
	}
	if detail == nil {
		t.Fatalf("detail is nil")
	}
	bp, err := blueprint.NewRegistry().MustGet(taskType)
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	if len(detail.Modules) != len(bp.Modules) {
		t.Fatalf("modules len=%d, want %d", len(detail.Modules), len(bp.Modules))
	}
	for _, module := range detail.Modules {
		if module.Visibility != "visible" {
			t.Fatalf("module %s visibility=%q, want visible", module.ModuleKey, module.Visibility)
		}
	}
}
