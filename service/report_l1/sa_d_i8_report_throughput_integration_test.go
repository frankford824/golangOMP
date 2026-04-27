//go:build integration

package report_l1

import (
	"testing"
	"time"

	"workflow/domain"
)

func TestSADI8_ReportThroughput(t *testing.T) {
	db, svc := sadReportDBSvc(t)
	tasks := []int64{50080, 50081, 50082}
	sadReportCleanup(t, db, tasks, nil)
	t.Cleanup(func() { sadReportCleanup(t, db, tasks, nil) })
	for i, taskID := range tasks {
		sadReportSeedTask(t, db, taskID)
		sadReportInsertEvent(t, db, taskID, "task_detail", "created", time.Date(2026, 4, 20+i, 10, 0, 0, 0, time.UTC))
	}
	ctx, cancel := sadReportCtx(t)
	defer cancel()
	points, appErr := svc.Throughput(ctx, sadReportActor(50008, domain.RoleSuperAdmin), time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC), nil, nil)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if len(points) == 0 {
		t.Fatalf("points empty")
	}
	for _, p := range points {
		if p.Date < "2026-04-20" || p.Date > "2026-04-22" {
			t.Fatalf("out of range point=%+v", p)
		}
	}
}
