//go:build integration

package handler

import (
	"fmt"
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI5_PatchUsers_FieldLevelAuthorizationDenyMatrix(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	ids := []int64{30051, 30052, 30053, 30054, 30055, 30056}
	saBCleanupUsers(t, db, ids...)
	defer saBCleanupUsers(t, db, ids...)

	fixtures := []saBUserFixture{
		{ID: 30051, Username: "sab_i5_dept_admin", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember, domain.RoleDeptAdmin}, ManagedDepartments: []string{string(domain.DepartmentOperations)}},
		{ID: 30052, Username: "sab_i5_other_dept", Department: string(domain.DepartmentHR), Team: "人事管理组", Roles: []domain.Role{domain.RoleMember}},
		{ID: 30053, Username: "sab_i5_hr_admin", Department: string(domain.DepartmentHR), Team: "人事管理组", Roles: []domain.Role{domain.RoleHRAdmin}},
		{ID: 30054, Username: "sab_i5_role_target", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}},
		{ID: 30055, Username: "sab_i5_team_lead", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember, domain.RoleTeamLead}, ManagedTeams: []string{"淘系一组"}},
		{ID: 30056, Username: "sab_i5_team_target", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember}},
	}
	for _, f := range fixtures {
		f.DisplayName = fmt.Sprintf("SA-B I5 %d", f.ID)
		f.Password = "ChangeMeAdmin123"
		saBInsertUser(t, db, f)
	}
	deptToken := saBCreateSession(t, db, 30051, "sab-i5-dept-token")
	hrToken := saBCreateSession(t, db, 30053, "sab-i5-hr-token")
	teamToken := saBCreateSession(t, db, 30055, "sab-i5-team-token")

	router := saBAuthRouter(svc)
	userH := NewUserAdminHandler(svc, nil, nil)
	router.PATCH("/v1/users/:id", userH.PatchUser)

	// SA-B.2(2026-04-24):DeptAdmin 访问跨部门用户会**先命中** read-scope gate
	// (identity_service.go:2121 → deny_code=department_scope_only),永远到不了 field-update gate。
	// scope-first 比 field-first 更安全(scope 外直接 403,而非 PATCH 到一半才发现);两种 deny_code
	// 均合法 403,语义等价。SA-B.1 / SA-B prompt §4/§8.2 已注明此次序。
	rec := saBPerformJSON(router, http.MethodPatch, "/v1/users/30052", deptToken, `{"display_name":"cross department denied"}`)
	deptDeny := saBDenyCode(t, rec)
	if rec.Code != http.StatusForbidden || (deptDeny != "department_scope_only" && deptDeny != "user_update_field_denied_by_scope") {
		t.Fatalf("DeptAdmin cross-dept status=%d deny=%q body=%s", rec.Code, deptDeny, rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodPatch, "/v1/users/30054", hrToken, `{"roles":["SuperAdmin"]}`)
	if rec.Code != http.StatusForbidden || saBDenyCode(t, rec) != "role_assignment_denied_by_scope" {
		t.Fatalf("HRAdmin assign SuperAdmin status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodPatch, "/v1/users/30056", teamToken, `{"display_name":"team lead denied"}`)
	if rec.Code != http.StatusForbidden || saBDenyCode(t, rec) != "user_update_field_denied_by_scope" {
		t.Fatalf("TeamLead display_name status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
}
