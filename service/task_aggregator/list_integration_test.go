//go:build integration

package task_aggregator

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

func TestTaskListPriorityOrderingIntegration(t *testing.T) {
	db := r35.MustOpenTestDB(t)
	defer db.Close()

	lowTaskID := int64(10060)
	highTaskID := int64(10061)
	r35.InsertTaskWithModules(t, db, lowTaskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityLow), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
	})
	r35.InsertTaskWithModules(t, db, highTaskID, string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskPriorityHigh), []r35.ModuleFixture{
		{Key: domain.ModuleKeyBasicInfo, State: string(domain.ModuleStateActive)},
	})
	defer r35.CleanupTaskIDs(t, db, lowTaskID, highTaskID)
	if _, err := db.Exec(`UPDATE tasks SET task_no = CASE id WHEN ? THEN 'R35-LIST-LOW' WHEN ? THEN 'R35-LIST-HIGH' END WHERE id IN (?, ?)`, lowTaskID, highTaskID, lowTaskID, highTaskID); err != nil {
		t.Fatalf("rename list fixtures: %v", err)
	}

	items, _, err := mysqlrepo.NewTaskRepo(mysqlrepo.New(db)).List(context.Background(), repo.TaskListFilter{
		Keyword:      "R35-LIST",
		ScopeViewAll: true,
		Page:         1,
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	var lowIndex, highIndex = -1, -1
	for i, item := range items {
		switch item.ID {
		case lowTaskID:
			lowIndex = i
		case highTaskID:
			highIndex = i
		}
	}
	if lowIndex < 0 || highIndex < 0 {
		t.Fatalf("list missing fixtures: low_index=%d high_index=%d items=%d", lowIndex, highIndex, len(items))
	}
	if highIndex > lowIndex {
		t.Fatalf("priority order wrong: high_index=%d low_index=%d, want high before low", highIndex, lowIndex)
	}
}
