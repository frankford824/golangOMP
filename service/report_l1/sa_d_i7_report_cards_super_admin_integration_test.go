//go:build integration

package report_l1

import (
	"testing"

	"workflow/domain"
)

func TestSADI7_ReportCardsSuperAdmin(t *testing.T) {
	_, svc := sadReportDBSvc(t)
	ctx, cancel := sadReportCtx(t)
	defer cancel()
	cards, appErr := svc.Cards(ctx, sadReportActor(50007, domain.RoleSuperAdmin))
	if appErr != nil {
		t.Fatal(appErr)
	}
	if len(cards) < 3 {
		t.Fatalf("cards=%+v", cards)
	}
	seen := map[string]bool{}
	for _, c := range cards {
		if c.Key == "" || c.Title == "" {
			t.Fatalf("bad card=%+v", c)
		}
		seen[c.Key] = true
	}
	for _, key := range []string{"tasks_in_progress", "tasks_completed_today", "archived_total"} {
		if !seen[key] {
			t.Fatalf("missing card %s in %+v", key, cards)
		}
	}
}
