//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI2_PatchMe_UpdatesProfile_RejectsAvatarFields_IgnoresOrgPlaceholderFields(t *testing.T) {
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
		AvatarURL          *string
		Department         string
		Team               string
		ManagedDepartments *string
		ManagedTeams       *string
	}
	if err := db.QueryRow(`
		SELECT display_name, mobile, email, avatar_url, department, team, managed_departments_json, managed_teams_json
		FROM users WHERE id = ?`, userID).Scan(&before.DisplayName, &before.Mobile, &before.Email, &before.AvatarURL, &before.Department, &before.Team, &before.ManagedDepartments, &before.ManagedTeams); err != nil {
		t.Fatalf("select before user: %v", err)
	}

	router := saBAuthRouter(svc)
	authH := NewAuthHandler(svc)
	router.PATCH("/v1/me", authH.PatchMe)

	body := `{"display_name":"SA-B I2 After","mobile":"13910030002","email":"sab_i2_after@example.test","team_codes":["SHOULD_NOT_WRITE"],"primary_team_code":"SHOULD_NOT_WRITE"}`
	rec := saBPerformJSON(router, http.MethodPatch, "/v1/me", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/me status = %d body=%s", rec.Code, rec.Body.String())
	}

	var after struct {
		DisplayName        string
		Mobile             string
		Email              string
		AvatarURL          *string
		Department         string
		Team               string
		ManagedDepartments *string
		ManagedTeams       *string
	}
	if err := db.QueryRow(`
		SELECT display_name, mobile, email, avatar_url, department, team, managed_departments_json, managed_teams_json
		FROM users WHERE id = ?`, userID).Scan(&after.DisplayName, &after.Mobile, &after.Email, &after.AvatarURL, &after.Department, &after.Team, &after.ManagedDepartments, &after.ManagedTeams); err != nil {
		t.Fatalf("select after user: %v", err)
	}
	if before.DisplayName != "SA-B I2 Before" || after.DisplayName != "SA-B I2 After" ||
		before.Mobile != "13900030002" || after.Mobile != "13910030002" ||
		before.Email != "sab_i2_before@example.test" || after.Email != "sab_i2_after@example.test" ||
		before.AvatarURL != nil || after.AvatarURL != nil ||
		before.Department != after.Department || before.Team != after.Team ||
		(before.ManagedDepartments == nil) != (after.ManagedDepartments == nil) ||
		(before.ManagedDepartments != nil && *before.ManagedDepartments != *after.ManagedDepartments) ||
		(before.ManagedTeams == nil) != (after.ManagedTeams == nil) ||
		(before.ManagedTeams != nil && *before.ManagedTeams != *after.ManagedTeams) ||
		saBCountColumns(t, db, "avatar", "team_codes", "primary_team_code") != 0 {
		t.Fatalf("PATCH /v1/me before=%+v after=%+v profile fields should persist and org placeholders should remain absent", before, after)
	}

	var storedAvatar *string
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "managed-looking avatar url", body: `{"avatar_url":"/v1/me/avatar-files/avatar-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png"}`},
		{name: "external avatar url", body: `{"avatar":"https://example.com/avatar.png"}`},
		{name: "empty avatar value", body: `{"avatar":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec = saBPerformJSON(router, http.MethodPatch, "/v1/me", token, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("PATCH /v1/me avatar field status = %d body=%s, want 400", rec.Code, rec.Body.String())
			}
			if err := db.QueryRow(`SELECT avatar_url FROM users WHERE id = ?`, userID).Scan(&storedAvatar); err != nil {
				t.Fatalf("select avatar after rejected update: %v", err)
			}
			if storedAvatar != nil {
				t.Fatalf("stored avatar after rejected update = %v, want nil", storedAvatar)
			}
		})
	}
}
