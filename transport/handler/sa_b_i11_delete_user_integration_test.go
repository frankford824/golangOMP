//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI11_DeleteUser_SoftDeleteBySuperAdminOnly(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	ids := []int64{30110, 30111, 30112, 30113, 30114}
	saBCleanupUsers(t, db, ids...)
	defer saBCleanupUsers(t, db, ids...)

	saBInsertUser(t, db, saBUserFixture{ID: 30110, Username: "sab_i11_super", Department: string(domain.DepartmentHR), Team: "人事管理组", Roles: []domain.Role{domain.RoleSuperAdmin}})
	saBInsertUser(t, db, saBUserFixture{ID: 30111, Username: "sab_i11_delete_target", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}})
	saBInsertUser(t, db, saBUserFixture{ID: 30112, Username: "sab_i11_hr", Department: string(domain.DepartmentHR), Team: "人事管理组", Roles: []domain.Role{domain.RoleHRAdmin}})
	saBInsertUser(t, db, saBUserFixture{ID: 30113, Username: "sab_i11_non_super_target", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}})
	saBInsertUser(t, db, saBUserFixture{ID: 30114, Username: "sab_i11_missing_reason_target", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}})
	superToken := saBCreateSession(t, db, 30110, "sab-i11-super-token")
	hrToken := saBCreateSession(t, db, 30112, "sab-i11-hr-token")

	router := saBAuthRouter(svc)
	userH := NewUserAdminHandler(svc, nil, nil)
	router.DELETE("/v1/users/:id", userH.Delete)

	rec := saBPerformJSON(router, http.MethodDelete, "/v1/users/30111", superToken, `{"reason":"SA-B.1 I11 soft delete"}`)
	if rec.Code != http.StatusNoContent || saBUserStatus(t, db, 30111) != string(domain.UserStatusDeleted) {
		t.Fatalf("super delete status=%d user_status=%s body=%s", rec.Code, saBUserStatus(t, db, 30111), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodDelete, "/v1/users/30113", hrToken, `{"reason":"SA-B.1 I11 denied"}`)
	if rec.Code != http.StatusForbidden || saBDenyCode(t, rec) != "module_action_role_denied" {
		t.Fatalf("non-super delete status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodDelete, "/v1/users/30114", superToken, `{"reason":""}`)
	if rec.Code != http.StatusBadRequest || saBDenyCode(t, rec) != "reason_required" {
		t.Fatalf("missing reason delete status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
}
