//go:build integration

package report_l1

import (
	"testing"
	"time"

	"workflow/domain"
)

func TestSADI10_ReportDateRangeInvalid(t *testing.T) {
	_, svc := sadReportDBSvc(t)
	ctx, cancel := sadReportCtx(t)
	defer cancel()
	from := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	if _, appErr := svc.Throughput(ctx, sadReportActor(50010, domain.RoleSuperAdmin), from, to, nil, nil); appErr == nil || appErr.Code != CodeInvalidDateRange {
		t.Fatalf("throughput appErr=%+v", appErr)
	}
	if _, appErr := svc.ModuleDwell(ctx, sadReportActor(50010, domain.RoleSuperAdmin), from, to, nil, nil); appErr == nil || appErr.Code != CodeInvalidDateRange {
		t.Fatalf("dwell appErr=%+v", appErr)
	}
}
