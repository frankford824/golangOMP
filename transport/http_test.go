package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestRegisterOperationalRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	registerOperationalRoutes(router)

	for _, path := range []string{"/health", "/healthz", "/ping"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

func TestWithCompatibilityRouteAddsHeaders(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/compat", withCompatibilityRoute(" /v1/canonical/path ", " candidate_for_v1_0_removal "), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/compat", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want %q", got, "true")
	}
	if got := rec.Header().Get("X-Workflow-API-Status"); got != "compatibility" {
		t.Fatalf("X-Workflow-API-Status = %q, want %q", got, "compatibility")
	}
	if got := rec.Header().Get("X-Workflow-Successor-Path"); got != "/v1/canonical/path" {
		t.Fatalf("X-Workflow-Successor-Path = %q, want %q", got, "/v1/canonical/path")
	}
	if got := rec.Header().Get("X-Workflow-New-Usage-Allowed"); got != "false" {
		t.Fatalf("X-Workflow-New-Usage-Allowed = %q, want %q", got, "false")
	}
	if got := rec.Header().Get("X-Workflow-Target-Removal-Phase"); got != "candidate_for_v1_0_removal" {
		t.Fatalf("X-Workflow-Target-Removal-Phase = %q, want %q", got, "candidate_for_v1_0_removal")
	}
	if got := rec.Header().Get("Link"); got != "</v1/canonical/path>; rel=\"successor-version\"" {
		t.Fatalf("Link = %q, want %q", got, "</v1/canonical/path>; rel=\"successor-version\"")
	}
	if got := rec.Header().Get("Warning"); got != "299 - \"Compatibility-only API; use /v1/canonical/path\"" {
		t.Fatalf("Warning = %q, want %q", got, "299 - \"Compatibility-only API; use /v1/canonical/path\"")
	}
}

func TestWithDeprecatedRouteAddsHeaders(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.POST("/deprecated", withDeprecatedRoute("/v1/canonical/path", "remove_after_frontend_migration"), func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/deprecated", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if got := rec.Header().Get("X-Workflow-API-Status"); got != "deprecated" {
		t.Fatalf("X-Workflow-API-Status = %q, want %q", got, "deprecated")
	}
	if got := rec.Header().Get("X-Workflow-Target-Removal-Phase"); got != "remove_after_frontend_migration" {
		t.Fatalf("X-Workflow-Target-Removal-Phase = %q, want %q", got, "remove_after_frontend_migration")
	}
	if got := rec.Header().Get("Warning"); got != "299 - \"Deprecated API; use /v1/canonical/path\"" {
		t.Fatalf("Warning = %q, want %q", got, "299 - \"Deprecated API; use /v1/canonical/path\"")
	}
}

func TestV1R1_RouteRegistered_All47Paths(t *testing.T) {
	router, _ := newV1R1ContractFreezeTestRouter()

	for _, spec := range v1R1ContractRouteSpecs() {
		if spec.OwnerRound == "R4-SA-A" || spec.OwnerRound == "R4-SA-B" || spec.OwnerRound == "R4-SA-C" || spec.OwnerRound == "R4-SA-D" {
			continue
		}
		req := newV1R1ContractFreezeRequest(spec)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d, want %d", spec.Method, spec.SamplePath, rec.Code, http.StatusNotImplemented)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "not_implemented") {
			t.Fatalf("%s %s body = %q, want not_implemented", spec.Method, spec.SamplePath, body)
		}
		if !strings.Contains(body, spec.OwnerRound) {
			t.Fatalf("%s %s body = %q, want owner round %q", spec.Method, spec.SamplePath, body, spec.OwnerRound)
		}
	}
}

func TestV1R1_RouteAccessCatalog_Shape(t *testing.T) {
	_, catalog := newV1R1ContractFreezeTestRouter()
	rules := catalog.ListRouteAccessRules()
	if got, want := len(rules), len(v1R1ReservedContractRouteSpecsForTest()); got != want {
		t.Fatalf("route access rule count = %d, want %d", got, want)
	}

	for _, spec := range v1R1ReservedContractRouteSpecsForTest() {
		path := joinRoutePath(spec.GroupBase, spec.RelativePath)
		rule, ok := catalog.FindRouteAccessRule(spec.Method, path)
		if !ok {
			t.Fatalf("missing route access rule for %s %s", spec.Method, path)
		}
		if got, want := rule.Path, path; got != want {
			t.Fatalf("rule path = %q, want %q", got, want)
		}
	}

	if _, ok := catalog.FindRouteAccessRule(http.MethodGet, "/v1/me"); ok {
		t.Fatal("/v1/me should be mounted as a live R4-SA-B route, not as a reserved contract stub")
	}
}

func v1R1ReservedContractRouteSpecsForTest() []v1R1RouteSpec {
	specs := v1R1ContractRouteSpecs()
	out := make([]v1R1RouteSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.OwnerRound == "R4-SA-A" || spec.OwnerRound == "R4-SA-B" || spec.OwnerRound == "R4-SA-C" || spec.OwnerRound == "R4-SA-D" {
			continue
		}
		out = append(out, spec)
	}
	return out
}

func TestV1R1_OpenAPI_Lint(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "run", "./cmd/tools/openapi-validate", "docs/api/openapi.yaml")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("openapi validation failed: %v\n%s", err, string(out))
	}
}

func newV1R1ContractFreezeTestRouter() (*gin.Engine, *RouteAccessCatalog) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(injectTraceID())
	router.Use(injectRequestActor(v1R1TestActorResolver{}))
	catalog := NewRouteAccessCatalog()

	v1 := router.Group("/v1")
	ws := router.Group("/ws")
	access := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc {
		catalog.AddRule(domain.NewRouteAccessRule(method, joinRoutePath(group.BasePath(), path), readiness, roles...))
		return withAccessMetaAndLogger(nil, readiness, roles...)
	}

	registerV1R1ReservedRoutes(router, v1, ws, access, true)
	return router, catalog
}

func newV1R1ContractFreezeRequest(spec v1R1RouteSpec) *http.Request {
	var body strings.Reader
	req := httptest.NewRequest(spec.Method, spec.SamplePath, &body)
	if spec.Method == http.MethodPost || spec.Method == http.MethodPatch || spec.Method == http.MethodDelete {
		req = httptest.NewRequest(spec.Method, spec.SamplePath, strings.NewReader(`{"stub":true}`))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer test-token")
	return req
}

type v1R1TestActorResolver struct{}

func (v1R1TestActorResolver) ResolveRequestActor(_ context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError) {
	if strings.TrimSpace(bearerToken) == "" {
		return nil, nil
	}
	return &domain.RequestActor{
		ID:       1,
		Username: "contract-freeze",
		Roles:    []domain.Role{domain.RoleSuperAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}, nil
}

func hasRole(roles []domain.Role, target domain.Role) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller path")
	}
	return filepath.Dir(filepath.Dir(filename))
}
