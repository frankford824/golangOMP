//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI3_ChangePassword_ValidatesOldAndConfirm(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	userID := int64(30003)
	saBCleanupUsers(t, db, userID)
	defer saBCleanupUsers(t, db, userID)

	saBInsertUser(t, db, saBUserFixture{
		ID:          userID,
		Username:    "sab_i3_password",
		DisplayName: "SA-B I3 Password",
		Department:  string(domain.DepartmentOperations),
		Team:        "淘系一组",
		Password:    "ChangeMeAdmin123",
		Roles:       []domain.Role{domain.RoleMember},
	})
	token := saBCreateSession(t, db, userID, "sab-i3-token")
	beforeHash := saBPasswordHash(t, db, userID)

	router := saBAuthRouter(svc)
	authH := NewAuthHandler(svc)
	router.POST("/v1/me/change-password", authH.ChangeMyPassword)

	rec := saBPerformJSON(router, http.MethodPost, "/v1/me/change-password", token, `{"old_password":"wrong520","new_password":"ChangeMeNew123","confirm":"ChangeMeNew123"}`)
	if rec.Code != http.StatusBadRequest || saBDenyCode(t, rec) != "old_password_mismatch" {
		t.Fatalf("old password mismatch status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodPost, "/v1/me/change-password", token, `{"old_password":"ChangeMeAdmin123","new_password":"ChangeMeNew123","confirm":"ChangeMeOther123"}`)
	if rec.Code != http.StatusBadRequest || saBDenyCode(t, rec) != "password_confirmation_mismatch" {
		t.Fatalf("confirmation mismatch status=%d deny=%q body=%s", rec.Code, saBDenyCode(t, rec), rec.Body.String())
	}
	rec = saBPerformJSON(router, http.MethodPost, "/v1/me/change-password", token, `{"old_password":"ChangeMeAdmin123","new_password":"ChangeMeNew123","confirm":"ChangeMeNew123"}`)
	afterHash := saBPasswordHash(t, db, userID)
	if rec.Code != http.StatusNoContent || beforeHash == afterHash {
		t.Fatalf("valid change status=%d beforeHash==afterHash:%v body=%s", rec.Code, beforeHash == afterHash, rec.Body.String())
	}
}
