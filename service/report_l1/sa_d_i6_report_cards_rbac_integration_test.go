//go:build integration

package report_l1

import (
	"testing"

	"workflow/domain"
)

func TestSADI6_ReportCardsRBAC(t *testing.T) {
	db, svc := sadReportDBSvc(t)
	sadReportCleanup(t, db, nil, []int64{50006})
	t.Cleanup(func() { sadReportCleanup(t, db, nil, []int64{50006}) })
	sadReportSeedUser(t, db, 50006, domain.RoleMember)
	ctx, cancel := sadReportCtx(t)
	defer cancel()
	_, appErr := svc.Cards(ctx, sadReportActor(50006, domain.RoleMember))
	if appErr == nil || denyCode(appErr) != domain.ErrDenyCodeReportsSuperAdminOnly {
		t.Fatalf("appErr=%+v", appErr)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM permission_logs WHERE actor_id=? AND action_type='report_access_denied'`, 50006).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("missing report_access_denied audit")
	}
}
