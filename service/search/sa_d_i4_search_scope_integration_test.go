//go:build integration

package search

import (
	"testing"

	"workflow/domain"
)

func TestSADI4_SearchScope(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	sadCleanup(t, db, []int64{50004}, []int64{50004})
	t.Cleanup(func() { sadCleanup(t, db, []int64{50004}, []int64{50004}) })
	sadInsertUser(t, db, 50004, domain.RoleMember, "sad_i4_user")
	sadInsertTaskAsset(t, db, 50004, "SADSCOPE-50004", "SADSCOPE-SKU-50004", "sad_scope.psd")
	ctx, cancel := sadCtx(t)
	defer cancel()
	got, appErr := svc.Search(ctx, sadActor(50004, domain.RoleSuperAdmin), "SADSCOPE", "tasks", 20)
	if appErr != nil || len(got.Tasks) == 0 || len(got.Assets) != 0 || len(got.Products) != 0 || len(got.Users) != 0 {
		t.Fatalf("tasks scope got=%+v err=%+v", got, appErr)
	}
	got, appErr = svc.Search(ctx, sadActor(50004, domain.RoleMember), "sad_i4_user", "users", 20)
	if appErr != nil || len(got.Tasks) != 0 || len(got.Users) != 0 {
		t.Fatalf("low users scope got=%+v err=%+v", got, appErr)
	}
}
