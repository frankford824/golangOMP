package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestUsersDesignersRouteUsesExplicitCandidateSelectorCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	required := []domain.PermissionCode{
		domain.PermissionTaskCreate,
		domain.PermissionTaskAssign,
		domain.PermissionTaskReassign,
		domain.PermissionTaskAuditHandover,
	}
	for _, tc := range []struct {
		name        string
		permissions []domain.PermissionCode
		roles       []domain.Role
		wantStatus  int
		wantReached bool
	}{
		{name: "task creator", permissions: []domain.PermissionCode{domain.PermissionTaskCreate}, wantStatus: http.StatusNoContent, wantReached: true},
		{name: "task assigner", permissions: []domain.PermissionCode{domain.PermissionTaskAssign}, wantStatus: http.StatusNoContent, wantReached: true},
		{name: "audit handover", permissions: []domain.PermissionCode{domain.PermissionTaskAuditHandover}, wantStatus: http.StatusNoContent, wantReached: true},
		{name: "legacy roles only", roles: []domain.Role{domain.RoleOps, domain.RoleAuditA, domain.RoleAdmin}, wantStatus: http.StatusForbidden},
		{name: "unrelated capability", permissions: []domain.PermissionCode{domain.PermissionTaskView}, wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			router := gin.New()
			router.Use(func(c *gin.Context) {
				actor := domain.RequestActor{ID: 99, Roles: tc.roles, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
				c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
				c.Next()
			})
			view := &domain.EffectiveAccess{UserID: 99, Permissions: tc.permissions}
			router.GET("/v1/users/designers", withCapabilityAccess(nil, effectiveAccessResolverStub{view: view}, domain.APIReadinessReadyForFrontend, required...), func(c *gin.Context) {
				reached = true
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/users/designers?workflow_lane=audit", nil))
			if recorder.Code != tc.wantStatus || reached != tc.wantReached {
				t.Fatalf("status=%d reached=%v, want status=%d reached=%v", recorder.Code, reached, tc.wantStatus, tc.wantReached)
			}
		})
	}
}

func TestUsersDesignersRouteAccessRuleContainsNoLegacyRoles(t *testing.T) {
	rule := domain.NewCapabilityRouteAccessRule(
		http.MethodGet,
		"/v1/users/designers",
		domain.APIReadinessReadyForFrontend,
		domain.PermissionTaskCreate,
		domain.PermissionTaskAssign,
		domain.PermissionTaskReassign,
		domain.PermissionTaskAuditHandover,
	)
	if len(rule.RequiredRoles) != 0 || len(rule.RequiredPermissions) != 4 {
		t.Fatalf("candidate selector route rule = %+v", rule)
	}
}
