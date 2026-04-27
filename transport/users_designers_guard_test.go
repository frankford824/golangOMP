package transport

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

// TestUsersDesignersRouteGuardRoundC verifies the Round C widening of the
// `/v1/users/designers` route guard. Ops (the canonical task-creator role)
// must pass the guard, as do HRAdmin and SuperAdmin. DepartmentAdmin remains
// intentionally excluded at the route layer — cross-department designer
// lookup is an Ops-style capability and DepartmentAdmin scope belongs to the
// canonical `/v1/users` path governed by authorizeUserListFilter.
func TestUsersDesignersRouteGuardRoundC(t *testing.T) {
	const (
		pattern = "/v1/users/designers"
		method  = http.MethodGet
	)

	requiredRoles := []domain.Role{
		domain.RoleOps,
		domain.RoleDesigner,
		domain.RoleCustomizationOperator,
		domain.RoleAdmin,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
	}

	cases := []struct {
		name  string
		roles []domain.Role
		want  int
	}{
		{
			name:  "ops_with_department_query_passes_guard",
			roles: []domain.Role{domain.RoleOps},
			want:  http.StatusNoContent,
		},
		{
			name:  "ops_no_query_passes_guard",
			roles: []domain.Role{domain.RoleOps},
			want:  http.StatusNoContent,
		},
		{
			name:  "designer_alone_passes_guard",
			roles: []domain.Role{domain.RoleDesigner},
			want:  http.StatusNoContent,
		},
		{
			name:  "customization_operator_alone_passes_guard",
			roles: []domain.Role{domain.RoleCustomizationOperator},
			want:  http.StatusNoContent,
		},
		{
			name:  "hradmin_passes_guard",
			roles: []domain.Role{domain.RoleHRAdmin},
			want:  http.StatusNoContent,
		},
		{
			name:  "superadmin_passes_guard",
			roles: []domain.Role{domain.RoleSuperAdmin},
			want:  http.StatusNoContent,
		},
		{
			name:  "department_admin_blocked_by_route_guard",
			roles: []domain.Role{domain.RoleDeptAdmin},
			want:  http.StatusForbidden,
		},
		{
			name:  "team_lead_blocked_by_route_guard",
			roles: []domain.Role{domain.RoleTeamLead},
			want:  http.StatusForbidden,
		},
		{
			name:  "member_only_blocked_by_route_guard",
			roles: []domain.Role{domain.RoleMember},
			want:  http.StatusForbidden,
		},
		{
			name:  "anonymous_blocked_by_route_guard",
			roles: nil,
			want:  http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(injectRequestActor(tokenResolverStub{
				actor: &domain.RequestActor{
					ID:       99,
					Username: "designers-guard-test",
					Roles:    tc.roles,
					Source:   domain.RequestActorSourceSessionToken,
					AuthMode: domain.AuthModeSessionTokenRoleEnforced,
				},
			}))
			router.GET(pattern,
				withAccessMeta(domain.APIReadinessReadyForFrontend, requiredRoles...),
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)

			requestPath := pattern
			if tc.name == "ops_with_department_query_passes_guard" {
				requestPath = pattern + "?department=%E8%AE%BE%E8%AE%A1%E7%A0%94%E5%8F%91%E9%83%A8"
			}
			req := httptest.NewRequest(method, requestPath, nil)
			req.Header.Set(authorizationHeader, "Bearer session-token")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("%s %s roles=%v code=%d want=%d body=%s",
					method, requestPath, tc.roles, rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestUsersDesignersRouteGuardCustomizationOperatorReceivesCustomizationPool(t *testing.T) {
	const (
		pattern = "/v1/users/designers"
		method  = http.MethodGet
	)

	requiredRoles := []domain.Role{
		domain.RoleOps,
		domain.RoleDesigner,
		domain.RoleCustomizationOperator,
		domain.RoleAdmin,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       99,
			Username: "customization-operator",
			Roles:    []domain.Role{domain.RoleCustomizationOperator},
			Source:   domain.RequestActorSourceSessionToken,
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))
	router.GET(pattern,
		withAccessMeta(domain.APIReadinessReadyForFrontend, requiredRoles...),
		func(c *gin.Context) {
			if c.Query("workflow_lane") != "customization" {
				c.Status(http.StatusBadRequest)
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{
					{"id": 20, "username": "custom_operator_a", "display_name": "定制A"},
					{"id": 21, "username": "custom_operator_b", "display_name": "定制B"},
				},
				"pagination": gin.H{"page": 1, "page_size": 2, "total": 2},
			})
		},
	)

	req := httptest.NewRequest(method, pattern+"?workflow_lane=customization", nil)
	req.Header.Set(authorizationHeader, "Bearer session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s customization operator code=%d want=200 body=%s",
			method, pattern, rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"custom_operator_a"`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"total":2`)) {
		t.Fatalf("customization pool response missing expected payload: %s", rec.Body.String())
	}
}
