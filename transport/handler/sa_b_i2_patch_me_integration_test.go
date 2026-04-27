//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI2_PatchMe_IgnoresPlaceholderFields(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	userID := int64(30002)
	saBCleanupUsers(t, db, userID)
	defer saBCleanupUsers(t, db, userID)

	saBInsertUser(t, db, saBUserFixture{
		ID:                 userID,
		Username:           "sab_i2_patch_me",
		DisplayName:        "SA-B I2 Before",
		Department:         string(domain.DepartmentOperations),
		Team:               "淘系一组",
		Mobile:             "13900030002",
		Email:              "sab_i2_before@example.test",
		Password:           "ChangeMeAdmin123",
		Roles:              []domain.Role{domain.RoleMember},
		ManagedDepartments: []string{string(domain.DepartmentOperations)},
		ManagedTeams:       []string{"淘系一组"},
	})
	token := saBCreateSession(t, db, userID, "sab-i2-token")

	var before struct {
		DisplayName        string
		Mobile             string
		Email              string
		Department         string
		Team               string
		ManagedDepartments *string
		ManagedTeams       *string
	}
	if err := db.QueryRow(`
		SELECT display_name, mobile, email, department, team, managed_departments_json, managed_teams_json
		FROM users WHERE id = ?`, userID).Scan(&before.DisplayName, &before.Mobile, &before.Email, &before.Department, &before.Team, &before.ManagedDepartments, &before.ManagedTeams); err != nil {
		t.Fatalf("select before user: %v", err)
	}

	router := saBAuthRouter(svc)
	authH := NewAuthHandler(svc)
	router.PATCH("/v1/me", authH.PatchMe)

	body := `{"display_name":"SA-B I2 After","mobile":"13910030002","email":"sab_i2_after@example.test","avatar":"ignored.png","team_codes":["SHOULD_NOT_WRITE"],"primary_team_code":"SHOULD_NOT_WRITE"}`
	rec := saBPerformJSON(router, http.MethodPatch, "/v1/me", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/me status = %d body=%s", rec.Code, rec.Body.String())
	}

	var after struct {
		DisplayName        string
		Mobile             string
		Email              string
		Department         string
		Team               string
		ManagedDepartments *string
		ManagedTeams       *string
	}
	if err := db.QueryRow(`
		SELECT display_name, mobile, email, department, team, managed_departments_json, managed_teams_json
		FROM users WHERE id = ?`, userID).Scan(&after.DisplayName, &after.Mobile, &after.Email, &after.Department, &after.Team, &after.ManagedDepartments, &after.ManagedTeams); err != nil {
		t.Fatalf("select after user: %v", err)
	}
	if before.DisplayName != "SA-B I2 Before" || after.DisplayName != "SA-B I2 After" ||
		before.Mobile != "13900030002" || after.Mobile != "13910030002" ||
		before.Email != "sab_i2_before@example.test" || after.Email != "sab_i2_after@example.test" ||
		before.Department != after.Department || before.Team != after.Team ||
		(before.ManagedDepartments == nil) != (after.ManagedDepartments == nil) ||
		(before.ManagedDepartments != nil && *before.ManagedDepartments != *after.ManagedDepartments) ||
		(before.ManagedTeams == nil) != (after.ManagedTeams == nil) ||
		(before.ManagedTeams != nil && *before.ManagedTeams != *after.ManagedTeams) ||
		saBCountColumns(t, db, "avatar_url", "avatar", "team_codes", "primary_team_code") != 0 {
		t.Fatalf("PATCH /v1/me before=%+v after=%+v placeholder columns should be absent and org placeholders unchanged", before, after)
	}
}
