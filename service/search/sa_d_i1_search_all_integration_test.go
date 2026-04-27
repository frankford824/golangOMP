//go:build integration

package search

import (
	"testing"

	"workflow/domain"
)

func TestSADI1_SearchAll(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	sadCleanup(t, db, []int64{50001}, []int64{50001})
	t.Cleanup(func() { sadCleanup(t, db, []int64{50001}, []int64{50001}) })
	sadInsertUser(t, db, 50001, domain.RoleMember, "sad_i1_operator")
	sadInsertTaskAsset(t, db, 50001, "TASK50001", "SAD-SKU-50001", "sad_asset_TASK50001.psd")
	ctx, cancel := sadCtx(t)
	defer cancel()
	got, appErr := svc.Search(ctx, sadActor(50001, domain.RoleMember), "TASK50001", "all", 20)
	if appErr != nil {
		t.Fatal(appErr)
	}
	if len(got.Tasks) == 0 || got.Tasks[0].ID != 50001 {
		t.Fatalf("tasks=%+v", got.Tasks)
	}
	if len(got.Users) != 0 {
		t.Fatalf("operator users=%+v want empty", got.Users)
	}
}
