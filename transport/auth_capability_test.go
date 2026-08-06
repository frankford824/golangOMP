package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	searchsvc "workflow/service/search"
	"workflow/transport/handler"
)

func TestTaskAssetUploadSessionRoutesAcceptCapabilityOnlyAndRejectLegacyRoleOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := []struct {
		name    string
		pattern string
		path    string
	}{
		{name: "canonical create", pattern: "/v1/assets/upload-sessions", path: "/v1/assets/upload-sessions"},
		{name: "canonical complete", pattern: "/v1/assets/upload-sessions/:session_id/complete", path: "/v1/assets/upload-sessions/session-1/complete"},
		{name: "task alias create", pattern: "/v1/tasks/:id/assets/upload-sessions", path: "/v1/tasks/8/assets/upload-sessions"},
		{name: "asset-center alias cancel", pattern: "/v1/tasks/:id/asset-center/upload-sessions/:session_id/cancel", path: "/v1/tasks/8/asset-center/upload-sessions/session-1/cancel"},
	}
	required := []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				view        *domain.EffectiveAccess
				wantStatus  int
				wantReached bool
			}{
				{name: "capability only", view: &domain.EffectiveAccess{UserID: 7, Permissions: []domain.PermissionCode{domain.PermissionTaskDesignSubmit}}, wantStatus: http.StatusNoContent, wantReached: true},
				{name: "legacy role only", view: &domain.EffectiveAccess{UserID: 7}, wantStatus: http.StatusForbidden, wantReached: false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					reached := false
					router := gin.New()
					router.Use(func(c *gin.Context) {
						actor := domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin, domain.RoleAuditA}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
						c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
						c.Next()
					})
					router.POST(route.pattern, withCapabilityAccess(nil, effectiveAccessResolverStub{view: tc.view}, domain.APIReadinessReadyForFrontend, required...), func(c *gin.Context) {
						reached = true
						c.Status(http.StatusNoContent)
					})
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route.path, nil))
					if recorder.Code != tc.wantStatus || reached != tc.wantReached {
						t.Fatalf("status=%d reached=%v, want status=%d reached=%v", recorder.Code, reached, tc.wantStatus, tc.wantReached)
					}
				})
			}
			rule := domain.NewCapabilityRouteAccessRule(http.MethodPost, route.path, domain.APIReadinessReadyForFrontend, required...)
			if len(rule.RequiredRoles) != 0 || len(rule.RequiredPermissions) != len(required) {
				t.Fatalf("route access rule = %+v", rule)
			}
		})
	}
}

func TestERPBridgeInternalAccessRequiresMatchingTokenAndLoopbackPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ERP_BRIDGE_INTERNAL_TOKEN", "bridge-secret")

	for _, tc := range []struct {
		name       string
		token      string
		remoteAddr string
		wantStatus int
	}{
		{name: "matching loopback service credential", token: "bridge-secret", remoteAddr: "127.0.0.1:41234", wantStatus: http.StatusNoContent},
		{name: "wrong credential", token: "wrong", remoteAddr: "127.0.0.1:41234", wantStatus: http.StatusUnauthorized},
		{name: "matching credential from non-loopback peer", token: "bridge-secret", remoteAddr: "192.0.2.10:41234", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/v1/erp/products/upsert",
				withERPBridgeInternalOrCapabilityAccess(nil, effectiveAccessResolverStub{}, domain.APIReadinessReadyForFrontend, domain.PermissionERPManage),
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)
			request := httptest.NewRequest(http.MethodPost, "/v1/erp/products/upsert", nil)
			request.RemoteAddr = tc.remoteAddr
			request.Header.Set(erpBridgeInternalTokenHeader, tc.token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestTaskAssetUploadSessionHTTPRegistrationsDoNotUseLegacyRoleAccess(t *testing.T) {
	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read http.go: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "upload-sessions") || (!strings.Contains(line, "taskGroup.") && !strings.Contains(line, "assetGroup.")) {
			continue
		}
		if strings.Contains(line, "access(") && !strings.Contains(line, "capabilityAccess(") {
			t.Errorf("upload-session route retained legacy role access: %s", strings.TrimSpace(line))
		}
	}
}

func TestCostRulePreviewUsesAuthenticatedAccountCapability(t *testing.T) {
	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read http.go: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, `costRuleGroup.POST("/preview"`) {
			continue
		}
		if !strings.Contains(line, "domain.PermissionAccountUse") {
			t.Fatalf("cost rule preview must be available to authenticated accounts: %s", strings.TrimSpace(line))
		}
		if strings.Contains(line, "domain.PermissionCatalogManage") {
			t.Fatalf("read-only cost rule preview must not require catalog management: %s", strings.TrimSpace(line))
		}
		return
	}
	t.Fatal("cost rule preview registration not found")
}

type effectiveAccessResolverStub struct {
	view  *domain.EffectiveAccess
	calls *int
}

func (s effectiveAccessResolverStub) EffectiveAccess(context.Context, int64) (*domain.EffectiveAccess, *domain.AppError) {
	if s.calls != nil {
		(*s.calls)++
	}
	return s.view, nil
}

type searchCapabilityRepoStub struct {
	calls  int
	access domain.ResourceGroupAccessFilter
}

func (*searchCapabilityRepoStub) SearchTasks(context.Context, string, int) ([]domain.SearchTask, error) {
	return nil, nil
}
func (*searchCapabilityRepoStub) SearchAssets(context.Context, string, int) ([]domain.SearchAsset, error) {
	return nil, nil
}
func (*searchCapabilityRepoStub) SearchProducts(context.Context, string, int) ([]domain.SearchProduct, error) {
	return nil, nil
}
func (*searchCapabilityRepoStub) SearchUsers(context.Context, string, int) ([]domain.SearchUser, error) {
	return nil, nil
}
func (s *searchCapabilityRepoStub) SearchResourceGroups(_ context.Context, _ string, _ int, _ bool, access domain.ResourceGroupAccessFilter) ([]domain.SearchAsset, error) {
	s.calls++
	s.access = access
	return []domain.SearchAsset{{AssetID: 10, ResourceGroupID: 10, SourceType: "task_resource_group", FileName: "final.png"}}, nil
}

func TestGlobalSearchRouteHydratesEffectiveAccessBeforeApplyingAssetScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name             string
		permissions      []domain.PermissionCode
		wantProviderCall int
	}{
		{name: "account and asset view", permissions: []domain.PermissionCode{domain.PermissionAccountUse, domain.PermissionAssetView}, wantProviderCall: 1},
		{name: "account only", permissions: []domain.PermissionCode{domain.PermissionAccountUse}, wantProviderCall: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &searchCapabilityRepoStub{}
			service := searchsvc.NewService(provider)
			searchHandler := handler.NewSearchHandler(service)
			resolverCalls := 0
			view := &domain.EffectiveAccess{
				UserID: 22, Permissions: tc.permissions,
				Assignments: []domain.AccessAssignment{{RoleID: 4, ScopeMode: domain.AccessScopeSelf}},
				Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAssetView, RoleID: 4}},
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				actor := domain.RequestActor{ID: 22, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
				c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
				c.Next()
			})
			router.GET("/v1/search", withCapabilityAccess(nil, effectiveAccessResolverStub{view: view, calls: &resolverCalls}, domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), searchHandler.Search)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/search?q=final&scope=assets", nil))
			if recorder.Code != http.StatusOK || resolverCalls != 1 {
				t.Fatalf("status=%d resolver_calls=%d body=%s", recorder.Code, resolverCalls, recorder.Body.String())
			}
			if provider.calls != tc.wantProviderCall {
				t.Fatalf("provider calls=%d want=%d", provider.calls, tc.wantProviderCall)
			}
			if provider.calls == 1 && (!provider.access.Self || provider.access.ActorID != 22) {
				t.Fatalf("search scope not hydrated: %+v", provider.access)
			}
		})
	}
}

func TestCapabilityAccessIgnoresLegacyRoleAndHydratesMigratedAssignment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	legacyActor := domain.RequestActor{
		ID: 7, Roles: []domain.Role{domain.RoleAssetManager},
		Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}

	t.Run("legacy role alone is denied", func(t *testing.T) {
		view := &domain.EffectiveAccess{UserID: legacyActor.ID, Permissions: []domain.PermissionCode{}, Assignments: []domain.AccessAssignment{}, Sources: []domain.EffectiveAccessNote{}}
		status, _ := exerciseCapabilityMiddleware(t, legacyActor, view)
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", status, http.StatusForbidden)
		}
	})

	t.Run("migrated effective assignment is hydrated", func(t *testing.T) {
		assignment := domain.AccessAssignment{ID: 10, UserID: legacyActor.ID, RoleID: 20, RoleCode: "asset_manager", ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
		view := &domain.EffectiveAccess{
			UserID: legacyActor.ID, Permissions: []domain.PermissionCode{domain.PermissionAssetView},
			Assignments: []domain.AccessAssignment{assignment},
			Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAssetView, RoleID: assignment.RoleID, RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: assignment.ScopeMode}},
		}
		status, hydrated := exerciseCapabilityMiddleware(t, legacyActor, view)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if hydrated.EffectiveAccess == nil || !domain.ActorHasPermission(hydrated, domain.PermissionAssetView) {
			t.Fatalf("actor was not hydrated from effective assignment: %+v", hydrated)
		}
	})
}

func TestAssetWorkbenchHighRiskCapabilityDoesNotAcceptAssetViewOrAssetManage(t *testing.T) {
	actor := domain.RequestActor{
		ID: 9, Roles: []domain.Role{domain.RoleAssetManager},
		Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
	for _, granted := range []domain.PermissionCode{domain.PermissionAssetView, domain.PermissionAssetManage} {
		view := &domain.EffectiveAccess{UserID: actor.ID, Permissions: []domain.PermissionCode{granted}}
		status, _ := exerciseCapabilityMiddlewareFor(t, actor, view, domain.PermissionSystemManage)
		if status != http.StatusForbidden {
			t.Fatalf("%q actor status = %d, want %d", granted, status, http.StatusForbidden)
		}
	}
	view := &domain.EffectiveAccess{UserID: actor.ID, Permissions: []domain.PermissionCode{domain.PermissionSystemManage}}
	status, hydrated := exerciseCapabilityMiddlewareFor(t, actor, view, domain.PermissionSystemManage)
	if status != http.StatusOK || !domain.ActorHasPermission(hydrated, domain.PermissionSystemManage) {
		t.Fatalf("dedicated system manager status=%d actor=%+v", status, hydrated)
	}
}

func exerciseCapabilityMiddleware(t *testing.T, actor domain.RequestActor, view *domain.EffectiveAccess) (int, domain.RequestActor) {
	return exerciseCapabilityMiddlewareFor(t, actor, view, domain.PermissionAssetView)
}

func exerciseCapabilityMiddlewareFor(t *testing.T, actor domain.RequestActor, view *domain.EffectiveAccess, required ...domain.PermissionCode) (int, domain.RequestActor) {
	t.Helper()
	router := gin.New()
	var hydrated domain.RequestActor
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
		c.Next()
	})
	router.Use(withCapabilityAccess(nil, effectiveAccessResolverStub{view: view}, domain.APIReadinessReadyForFrontend, required...))
	router.GET("/asset", func(c *gin.Context) {
		hydrated, _ = domain.RequestActorFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/asset", nil)
	router.ServeHTTP(recorder, request)
	return recorder.Code, hydrated
}

func TestAssetWorkbenchMigratedRoleCapabilityParity(t *testing.T) {
	roles := map[string][]domain.PermissionCode{
		"asset_manager": {
			domain.PermissionAssetView, domain.PermissionAssetManage, domain.PermissionAssetWorkbenchUse,
			domain.PermissionAssetWorkbenchSubmit, domain.PermissionAssetWorkbenchMembers,
			domain.PermissionAssetWorkbenchGroups, domain.PermissionAssetWorkbenchDrive,
			domain.PermissionAssetWorkbenchBatch, domain.PermissionAssetWorkbenchAuditView,
		},
		"asset_template_admin": {
			domain.PermissionAssetWorkbenchUse, domain.PermissionAssetWorkbenchGroups,
			domain.PermissionAssetWorkbenchTemplates, domain.PermissionAssetWorkbenchAuditView,
		},
		"asset_settlement": {
			domain.PermissionAssetWorkbenchUse, domain.PermissionAssetWorkbenchQC,
			domain.PermissionAssetWorkbenchSettlement, domain.PermissionAssetWorkbenchAuditView,
		},
		"asset_submitter": {domain.PermissionAssetWorkbenchUse, domain.PermissionAssetWorkbenchSubmit},
	}
	tests := []struct {
		name   string
		role   string
		method string
		path   string
		allow  bool
	}{
		{"manager directory admin", "asset_manager", http.MethodGet, "/v1/asset-workbench/upload-directories/admin", true},
		{"submitter directory admin denied", "asset_submitter", http.MethodGet, "/v1/asset-workbench/upload-directories/admin", false},
		{"manager batch jobs", "asset_manager", http.MethodGet, "/v1/asset-workbench/batch-jobs", true},
		{"template admin batch jobs denied", "asset_template_admin", http.MethodGet, "/v1/asset-workbench/batch-jobs", false},
		{"settlement reads price rules", "asset_settlement", http.MethodGet, "/v1/asset-workbench/price-matrix", true},
		{"use only cannot read price rules", "asset_submitter", http.MethodGet, "/v1/asset-workbench/price-matrix", false},
		{"settlement reads profiles", "asset_settlement", http.MethodGet, "/v1/asset-workbench/profiles", true},
		{"settlement creates supplement upload session", "asset_settlement", http.MethodPost, "/v1/asset-workbench/upload-sessions", true},
		{"submitter profiles denied", "asset_submitter", http.MethodGet, "/v1/asset-workbench/profiles", false},
		{"submitter creates own supplement", "asset_submitter", http.MethodPost, "/v1/asset-workbench/settlement/supplements", true},
		{"submitter deletes own supplement route", "asset_submitter", http.MethodDelete, "/v1/asset-workbench/settlement/supplements/3", true},
		{"submitter batch deletes own supplements route", "asset_submitter", http.MethodPost, "/v1/asset-workbench/settlement/supplements/batch-delete", true},
		{"submitter settlement confirm denied", "asset_submitter", http.MethodPost, "/v1/asset-workbench/settlement/batches/3/confirm", false},
		{"submitter batch delete", "asset_submitter", http.MethodPost, "/v1/asset-workbench/files/batch-delete", true},
		{"submitter batch move denied", "asset_submitter", http.MethodPost, "/v1/asset-workbench/files/batch-move", false},
		{"manager groups", "asset_manager", http.MethodGet, "/v1/asset-workbench/groups", true},
		{"template admin groups", "asset_template_admin", http.MethodGet, "/v1/asset-workbench/groups", true},
		{"manager audit", "asset_manager", http.MethodGet, "/v1/asset-workbench/events", true},
		{"template audit", "asset_template_admin", http.MethodGet, "/v1/asset-workbench/events", true},
		{"settlement audit", "asset_settlement", http.MethodGet, "/v1/asset-workbench/events", true},
		{"submitter audit denied", "asset_submitter", http.MethodGet, "/v1/asset-workbench/events", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required, governed := v8BusinessRoutePermissions(tt.method, tt.path)
			if !governed || len(required) == 0 {
				t.Fatalf("route has no explicit mapping: %s %s", tt.method, tt.path)
			}
			actor := domain.RequestActor{ID: 17, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}
			view := &domain.EffectiveAccess{UserID: actor.ID, Permissions: roles[tt.role]}
			status, _ := exerciseCapabilityMiddlewareFor(t, actor, view, required...)
			if tt.allow && status != http.StatusOK {
				t.Fatalf("status = %d, want allowed; required=%v granted=%v", status, required, roles[tt.role])
			}
			if !tt.allow && status != http.StatusForbidden {
				t.Fatalf("status = %d, want forbidden; required=%v granted=%v", status, required, roles[tt.role])
			}
		})
	}
}
