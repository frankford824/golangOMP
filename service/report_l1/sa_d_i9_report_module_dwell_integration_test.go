//go:build integration

package report_l1

import (
	"testing"
	"time"

	"workflow/domain"
)

func TestSADI9_ReportModuleDwell(t *testing.T) {
	db, svc := sadReportDBSvc(t)
	sadReportCleanup(t, db, []int64{50090}, nil)
	t.Cleanup(func() { sadReportCleanup(t, db, []int64{50090}, nil) })
	sadReportSeedTask(t, db, 50090)
	start := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	sadReportInsertEvent(t, db, 50090, "design", "entered", start)
	sadReportInsertEvent(t, db, 50090, "design", "closed", start.Add(10*time.Minute))
	ctx, cancel := sadReportCtx(t)
	defer cancel()
	points, appErr := svc.ModuleDwell(ctx, sadReportActor(50009, domain.RoleSuperAdmin), time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC), nil, nil)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if len(points) != 5 {
		t.Fatalf("points len=%d %+v", len(points), points)
	}
	for _, p := range points {
		if p.AvgDwellSeconds < 0 || p.P95DwellSeconds < 0 || p.Samples < 0 {
			t.Fatalf("negative point=%+v", p)
		}
	}
}
