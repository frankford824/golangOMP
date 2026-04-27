//go:build integration

package search

import (
	"testing"

	"workflow/domain"
)

func TestSADI2_SearchUsersLowPrivilege(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	sadCleanup(t, db, nil, []int64{50002})
	t.Cleanup(func() { sadCleanup(t, db, nil, []int64{50002}) })
	sadInsertUser(t, db, 50002, domain.RoleMember, "sad_i2_visible")
	ctx, cancel := sadCtx(t)
	defer cancel()
	for _, role := range []domain.Role{domain.RoleSuperAdmin, domain.RoleHRAdmin} {
		got, appErr := svc.Search(ctx, sadActor(50002, role), "sad_i2_visible", "users", 20)
		if appErr != nil || len(got.Users) == 0 {
			t.Fatalf("role=%s users=%+v err=%+v", role, got.Users, appErr)
		}
	}
	got, appErr := svc.Search(ctx, sadActor(50002, domain.RoleMember), "sad_i2_visible", "users", 20)
	if appErr != nil || len(got.Users) != 0 {
		t.Fatalf("operator users=%+v err=%+v", got.Users, appErr)
	}
}
