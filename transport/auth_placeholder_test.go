package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

type legacyRoleGuardRouteCase struct {
	name                   string
	method                 string
	pattern                string
	requestPath            string
	requiredRoles          []domain.Role
	departmentAdminAllowed bool
}

func TestInjectRequestActorPlaceholderUsesHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActorPlaceholder())
	router.GET("/whoami", func(c *gin.Context) {
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok {
			t.Fatalf("request actor missing from context")
		}
		c.JSON(http.StatusOK, actor)
	})

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set(debugActorIDHeader, "42")
	req.Header.Set(debugActorRolesHeader, "Designer, Audit_A, unknown, Designer")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /whoami code = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get(workflowAuthModeHeader); got != string(domain.AuthModeDebugHeaderRoleEnforced) {
		t.Fatalf("%s = %q, want %q", workflowAuthModeHeader, got, domain.AuthModeDebugHeaderRoleEnforced)
	}
	if got := rec.Header().Get(workflowActorIDHeader); got != "42" {
		t.Fatalf("%s = %q, want 42", workflowActorIDHeader, got)
	}
	if got := rec.Header().Get(workflowActorRolesHeader); got != "Designer,Audit_A" {
		t.Fatalf("%s = %q, want Designer,Audit_A", workflowActorRolesHeader, got)
	}

	var actor domain.RequestActor
	if err := json.Unmarshal(rec.Body.Bytes(), &actor); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if actor.ID != 42 || actor.Source != domain.RequestActorSourceDebugHeader {
		t.Fatalf("actor = %+v, want id=42 source=header_placeholder", actor)
	}
	if len(actor.Roles) != 2 || actor.Roles[0] != domain.RoleDesigner || actor.Roles[1] != domain.RoleAuditA {
		t.Fatalf("actor roles = %+v, want [Designer Audit_A]", actor.Roles)
	}
}

func TestInjectRequestActorLeavesNormalRequestsAnonymousWithoutFallbackHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(nil))
	router.GET("/whoami", func(c *gin.Context) {
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok {
			t.Fatalf("request actor missing from context")
		}
		c.JSON(http.StatusOK, actor)
	})

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /whoami code = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get(workflowActorIDHeader); got != "" {
		t.Fatalf("%s = %q, want empty", workflowActorIDHeader, got)
	}

	var actor domain.RequestActor
	if err := json.Unmarshal(rec.Body.Bytes(), &actor); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if actor.ID != 0 || actor.Source != domain.RequestActorSourceAnonymous {
		t.Fatalf("actor = %+v, want anonymous actor without fallback id", actor)
	}
}

func TestWithAccessMetaRejectsActorWithoutRequiredRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActorPlaceholder())
	router.POST("/v1/products/sync/run", withAccessMeta(domain.APIReadinessInternalPlaceholder, domain.RoleERP, domain.RoleAdmin), func(c *gin.Context) {
		t.Fatalf("handler should not run when role enforcement blocks request")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/products/sync/run", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /v1/products/sync/run code = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get(workflowReadinessHeader); got != string(domain.APIReadinessInternalPlaceholder) {
		t.Fatalf("%s = %q, want %q", workflowReadinessHeader, got, domain.APIReadinessInternalPlaceholder)
	}
	if got := rec.Header().Get(workflowRolesHeader); got != "ERP,Admin" {
		t.Fatalf("%s = %q, want ERP,Admin", workflowRolesHeader, got)
	}

	var resp domain.APIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Error == nil || resp.Error.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("error response = %+v, want permission denied", resp.Error)
	}
}

func TestWithAccessMetaAllowsMatchingRoleAndAdminOverride(t *testing.T) {
	cases := []struct {
		name  string
		roles string
	}{
		{name: "matching role", roles: "ERP"},
		{name: "admin override", roles: "Admin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(injectRequestActorPlaceholder())
			router.POST("/v1/products/sync/run", withAccessMeta(domain.APIReadinessInternalPlaceholder, domain.RoleERP), func(c *gin.Context) {
				meta, ok := domain.RouteAccessMetaFromContext(c.Request.Context())
				if !ok {
					t.Fatalf("route access meta missing from context")
				}
				c.JSON(http.StatusOK, meta)
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/products/sync/run", nil)
			req.Header.Set(debugActorRolesHeader, tc.roles)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("POST /v1/products/sync/run code = %d, want 200", rec.Code)
			}
			var meta domain.RouteAccessMeta
			if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if meta.AuthMode != domain.AuthModeDebugHeaderRoleEnforced {
				t.Fatalf("route meta auth_mode = %q, want %q", meta.AuthMode, domain.AuthModeDebugHeaderRoleEnforced)
			}
		})
	}
}

func TestWithAccessMetaRejectsDebugActorOnReadyForFrontendRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(nil))
	router.GET("/v1/tasks", withAccessMeta(domain.APIReadinessReadyForFrontend, domain.RoleOps), func(c *gin.Context) {
		t.Fatalf("handler should not run with debug actor on ready route")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks", nil)
	req.Header.Set(debugActorIDHeader, "42")
	req.Header.Set(debugActorRolesHeader, "Ops")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/tasks code = %d, want 401", rec.Code)
	}
}

func TestWithAccessMetaAllowsSessionActorOnReadyForFrontendRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       88,
			Username: "ops",
			Roles:    []domain.Role{domain.RoleOps},
			Source:   domain.RequestActorSourceSessionToken,
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))
	router.GET("/v1/tasks", withAccessMeta(domain.APIReadinessReadyForFrontend, domain.RoleOps), func(c *gin.Context) {
		meta, ok := domain.RouteAccessMetaFromContext(c.Request.Context())
		if !ok {
			t.Fatal("route access meta missing from context")
		}
		c.JSON(http.StatusOK, meta)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks", nil)
	req.Header.Set(authorizationHeader, "Bearer session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/tasks code = %d, want 200", rec.Code)
	}
	var meta domain.RouteAccessMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if meta.AuthMode != domain.AuthModeSessionTokenRoleEnforced {
		t.Fatalf("route meta auth_mode = %q, want %q", meta.AuthMode, domain.AuthModeSessionTokenRoleEnforced)
	}
	if !meta.SessionRequired {
		t.Fatal("route meta session_required = false, want true")
	}
	if meta.DebugCompatible {
		t.Fatal("route meta debug_compatible = true, want false")
	}
}

func TestInjectRequestActorUsesBearerTokenResolver(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       88,
			Username: "ops",
			Roles:    []domain.Role{domain.RoleOps},
			Source:   "session_token",
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))
	router.GET("/whoami", func(c *gin.Context) {
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok {
			t.Fatalf("request actor missing from context")
		}
		c.JSON(http.StatusOK, actor)
	})

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set(authorizationHeader, "Bearer session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /whoami code = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get(workflowAuthModeHeader); got != string(domain.AuthModeSessionTokenRoleEnforced) {
		t.Fatalf("%s = %q, want %q", workflowAuthModeHeader, got, domain.AuthModeSessionTokenRoleEnforced)
	}
}

func TestWithAccessMetaRecordsActorUsernameAndRoutePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logWriter := &permissionLogWriterStub{}
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       88,
			Username: "ops",
			Roles:    []domain.Role{domain.RoleAdmin},
			Source:   domain.RequestActorSourceSessionToken,
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))
	router.GET("/v1/users", withAccessMetaAndLogger(logWriter, domain.APIReadinessReadyForFrontend, domain.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.Header.Set(authorizationHeader, "Bearer session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/users code = %d, want 200", rec.Code)
	}
	if len(logWriter.entries) != 1 {
		t.Fatalf("permission logs len = %d, want 1", len(logWriter.entries))
	}
	entry := logWriter.entries[0]
	if entry.ActorUsername != "ops" {
		t.Fatalf("entry.ActorUsername = %q, want ops", entry.ActorUsername)
	}
	if entry.Readiness != domain.APIReadinessReadyForFrontend {
		t.Fatalf("entry.Readiness = %q", entry.Readiness)
	}
	if !entry.SessionRequired {
		t.Fatal("entry.SessionRequired = false, want true")
	}
	if entry.DebugCompatible {
		t.Fatal("entry.DebugCompatible = true, want false")
	}
	if entry.Reason != "admin override" {
		t.Fatalf("entry.Reason = %q, want admin override", entry.Reason)
	}
}

func TestWithAuthenticatedRejectsSystemFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActorPlaceholder())
	router.GET("/v1/auth/me", withAuthenticated(domain.APIReadinessReadyForFrontend, nil), func(c *gin.Context) {
		t.Fatalf("handler should not run without authenticated actor")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/auth/me code = %d, want 401", rec.Code)
	}
}

func TestWithAuthenticatedRejectsDebugHeaderActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(nil))
	router.GET("/v1/auth/me", withAuthenticated(domain.APIReadinessReadyForFrontend, nil), func(c *gin.Context) {
		t.Fatalf("handler should not run with debug actor only")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set(debugActorIDHeader, "42")
	req.Header.Set(debugActorRolesHeader, "Ops")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/auth/me code = %d, want 401", rec.Code)
	}
}

func TestWithAuthenticatedAllowsSessionActorForAuthMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       200,
			Username: "incident_user",
			Roles: []domain.Role{
				domain.RoleDeptAdmin,
				domain.RoleMember,
				domain.RoleWarehouse,
			},
			Source:   domain.RequestActorSourceSessionToken,
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))
	router.GET("/v1/auth/me", withAuthenticated(domain.APIReadinessReadyForFrontend, nil), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set(authorizationHeader, "Bearer valid-session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/auth/me code = %d, want 200", rec.Code)
	}
}

func TestInjectRequestActorRejectsInvalidBearerTokenForAuthMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{err: domain.ErrUnauthorized}))
	router.GET("/v1/auth/me", withAuthenticated(domain.APIReadinessReadyForFrontend, nil), func(c *gin.Context) {
		t.Fatalf("handler should not run with invalid bearer token")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set(authorizationHeader, "Bearer invalid-session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/auth/me code = %d, want 401", rec.Code)
	}
}

func TestWithUserScopedActorAllowsExplicitDebugActorID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(nil))
	router.GET("/v1/workbench/preferences", withUserScopedActor(domain.APIReadinessReadyForFrontend, nil, true), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/workbench/preferences", nil)
	req.Header.Set(debugActorIDHeader, "42")
	req.Header.Set(debugActorRolesHeader, "Ops")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/workbench/preferences code = %d, want 200", rec.Code)
	}
}

func TestWithUserScopedActorRejectsRoleOnlyDebugActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(nil))
	router.GET("/v1/workbench/preferences", withUserScopedActor(domain.APIReadinessReadyForFrontend, nil, true), func(c *gin.Context) {
		t.Fatalf("handler should not run without an actor id")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/workbench/preferences", nil)
	req.Header.Set(debugActorRolesHeader, "Ops")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/workbench/preferences code = %d, want 401", rec.Code)
	}
}

func TestLegacyRoleGuardConvergenceForOrgAndUserWriteRoutes(t *testing.T) {
	routes := []legacyRoleGuardRouteCase{
		{
			name:          "post departments",
			method:        http.MethodPost,
			pattern:       "/v1/org/departments",
			requestPath:   "/v1/org/departments",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:          "put departments id",
			method:        http.MethodPut,
			pattern:       "/v1/org/departments/:id",
			requestPath:   "/v1/org/departments/7",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:          "post teams",
			method:        http.MethodPost,
			pattern:       "/v1/org/teams",
			requestPath:   "/v1/org/teams",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:          "put teams id",
			method:        http.MethodPut,
			pattern:       "/v1/org/teams/:id",
			requestPath:   "/v1/org/teams/7",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:                   "post users",
			method:                 http.MethodPost,
			pattern:                "/v1/users",
			requestPath:            "/v1/users",
			requiredRoles:          []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin},
			departmentAdminAllowed: true,
		},
		{
			name:                   "patch users id",
			method:                 http.MethodPatch,
			pattern:                "/v1/users/:id",
			requestPath:            "/v1/users/7",
			requiredRoles:          []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin},
			departmentAdminAllowed: true,
		},
		{
			name:                   "put users password",
			method:                 http.MethodPut,
			pattern:                "/v1/users/:id/password",
			requestPath:            "/v1/users/7/password",
			requiredRoles:          []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin},
			departmentAdminAllowed: true,
		},
		{
			name:          "post users roles",
			method:        http.MethodPost,
			pattern:       "/v1/users/:id/roles",
			requestPath:   "/v1/users/7/roles",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:          "put users roles",
			method:        http.MethodPut,
			pattern:       "/v1/users/:id/roles",
			requestPath:   "/v1/users/7/roles",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
		{
			name:          "delete users role",
			method:        http.MethodDelete,
			pattern:       "/v1/users/:id/roles/:role",
			requestPath:   "/v1/users/7/roles/Designer",
			requiredRoles: []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin},
		},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			assertGuardStatus(t, route, []domain.Role{domain.RoleAdmin}, http.StatusForbidden)
			assertGuardStatus(t, route, []domain.Role{domain.RoleOrgAdmin}, http.StatusForbidden)
			assertGuardStatus(t, route, []domain.Role{domain.RoleRoleAdmin}, http.StatusForbidden)
			assertGuardStatus(t, route, []domain.Role{domain.RoleHRAdmin}, http.StatusNoContent)
			assertGuardStatus(t, route, []domain.Role{domain.RoleSuperAdmin}, http.StatusNoContent)
			if route.departmentAdminAllowed {
				assertGuardStatus(t, route, []domain.Role{domain.RoleDeptAdmin}, http.StatusNoContent)
			} else {
				assertGuardStatus(t, route, []domain.Role{domain.RoleDeptAdmin}, http.StatusForbidden)
			}
			assertGuardStatus(t, route, nil, http.StatusForbidden)
		})
	}
}

type tokenResolverStub struct {
	actor *domain.RequestActor
	err   *domain.AppError
}

func (s tokenResolverStub) ResolveRequestActor(_ context.Context, _ string) (*domain.RequestActor, *domain.AppError) {
	return s.actor, s.err
}

type permissionLogWriterStub struct {
	entries []domain.PermissionLog
}

func (s *permissionLogWriterStub) RecordRouteAccess(_ context.Context, entry domain.PermissionLog) {
	s.entries = append(s.entries, entry)
}

func assertGuardStatus(t *testing.T, route legacyRoleGuardRouteCase, roles []domain.Role, want int) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(injectRequestActor(tokenResolverStub{
		actor: &domain.RequestActor{
			ID:       88,
			Username: "guard-test",
			Roles:    roles,
			Source:   domain.RequestActorSourceSessionToken,
			AuthMode: domain.AuthModeSessionTokenRoleEnforced,
		},
	}))

	handlers := []gin.HandlerFunc{
		withLegacyCompatibilityOnlyRoleRejection(nil, domain.APIReadinessReadyForFrontend, route.requiredRoles, domain.RoleAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin),
		withAccessMeta(domain.APIReadinessReadyForFrontend, route.requiredRoles...),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	}
	registerGuardedTestRoute(router, route.method, route.pattern, handlers...)

	req := httptest.NewRequest(route.method, route.requestPath, nil)
	req.Header.Set(authorizationHeader, "Bearer session-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != want {
		t.Fatalf("%s %s roles=%v code=%d want=%d body=%s", route.method, route.requestPath, roles, rec.Code, want, rec.Body.String())
	}
}

func registerGuardedTestRoute(router *gin.Engine, method, path string, handlers ...gin.HandlerFunc) {
	switch method {
	case http.MethodPost:
		router.POST(path, handlers...)
	case http.MethodPut:
		router.PUT(path, handlers...)
	case http.MethodPatch:
		router.PATCH(path, handlers...)
	case http.MethodDelete:
		router.DELETE(path, handlers...)
	default:
		panic("unsupported method in guard test")
	}
}
