//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI6_ActivateUser_TeamLeadWithinGroupOnly(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	ids := []int64{30061, 30062, 30063}
	saBCleanupUsers(t, db, ids...)
	defer saBCleanupUsers(t, db, ids...)

	saBInsertUser(t, db, saBUserFixture{ID: 30061, Username: "sab_i6_team_lead", Department: string(domain.DepartmentOperations), Team: "淘系一组", Roles: []domain.Role{domain.RoleMember, domain.RoleTeamLead}, ManagedTeams: []string{"淘系一组"}})
	saBInsertUser(t, db, saBUserFixture{ID: 30062, Username: "sab_i6_same_team", Department: string(domain.DepartmentOperations), Team: "淘系一组", Status: string(domain.UserStatusDisabled), Roles: []domain.Role{domain.RoleMember}})
	saBInsertUser(t, db, saBUserFixture{ID: 30063, Username: "sab_i6_other_team", Department: string(domain.DepartmentOperations), Team: "淘系二组", Status: string(domain.UserStatusDisabled), Roles: []domain.Role{domain.RoleMember}})
	token := saBCreateSession(t, db, 30061, "sab-i6-team-token")

	router := saBAuthRouter(svc)
	userH := NewUserAdminHandler(svc, nil, nil)
	router.POST("/v1/users/:id/activate", userH.Activate)

	rec := saBPerformJSON(router, http.MethodPost, "/v1/users/30062/activate", token, "")
	if rec.Code != http.StatusNoContent || saBUserStatus(t, db, 30062) != string(domain.UserStatusActive) {
		t.Fatalf("same-team activate status=%d user_status=%s body=%s", rec.Code, saBUserStatus(t, db, 30062), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodPost, "/v1/users/30063/activate", token, "")
	if rec.Code != http.StatusForbidden || saBDenyCode(t, rec) != "user_update_field_denied_by_scope" {
		t.Fatalf("other-team activate status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
}
