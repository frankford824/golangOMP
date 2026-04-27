//go:build integration

package handler

import (
	"net/http"
	"testing"

	"workflow/domain"
)

func TestSABI4_GetMyOrg_ReturnsManagedScopeByRole(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	deptAdminID := int64(30041)
	memberID := int64(30042)
	saBCleanupUsers(t, db, deptAdminID, memberID)
	defer saBCleanupUsers(t, db, deptAdminID, memberID)

	saBInsertUser(t, db, saBUserFixture{
		ID:                 deptAdminID,
		Username:           "sab_i4_dept_admin",
		DisplayName:        "SA-B I4 DeptAdmin",
		Department:         string(domain.DepartmentOperations),
		Team:               "淘系一组",
		Password:           "ChangeMeAdmin123",
		Roles:              []domain.Role{domain.RoleMember, domain.RoleDeptAdmin},
		ManagedDepartments: []string{string(domain.DepartmentOperations)},
	})
	saBInsertUser(t, db, saBUserFixture{
		ID:          memberID,
		Username:    "sab_i4_member",
		DisplayName: "SA-B I4 Member",
		Department:  string(domain.DepartmentOperations),
		Team:        "淘系二组",
		Password:    "ChangeMeAdmin123",
		Roles:       []domain.Role{domain.RoleMember},
	})
	deptToken := saBCreateSession(t, db, deptAdminID, "sab-i4-dept-token")
	memberToken := saBCreateSession(t, db, memberID, "sab-i4-member-token")

	router := saBAuthRouter(svc)
	authH := NewAuthHandler(svc)
	router.GET("/v1/me/org", authH.GetMyOrg)

	var deptResp struct {
		Data struct {
			ManagedDepartments []string `json:"managed_departments"`
			ManagedTeams       []string `json:"managed_teams"`
		} `json:"data"`
	}
	rec := saBPerformJSON(router, http.MethodGet, "/v1/me/org", deptToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dept admin GET /v1/me/org status=%d body=%s", rec.Code, rec.Body.String())
	}
	saBDecode(t, rec, &deptResp)

	var memberResp struct {
		Data struct {
			ManagedDepartments []string `json:"managed_departments"`
			ManagedTeams       []string `json:"managed_teams"`
		} `json:"data"`
	}
	rec = saBPerformJSON(router, http.MethodGet, "/v1/me/org", memberToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("member GET /v1/me/org status=%d body=%s", rec.Code, rec.Body.String())
	}
	saBDecode(t, rec, &memberResp)
	if len(deptResp.Data.ManagedDepartments) == 0 || len(memberResp.Data.ManagedDepartments) != 0 || len(memberResp.Data.ManagedTeams) != 0 {
		t.Fatalf("managed scopes dept=%+v member=%+v", deptResp.Data, memberResp.Data)
	}
}
