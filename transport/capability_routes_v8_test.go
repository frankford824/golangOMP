package transport

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"workflow/domain"
)

func TestV8BusinessRoutePermissionsCoverActiveTaskAssetAndERPSurfaces(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   domain.PermissionCode
	}{
		{http.MethodGet, "/v1/tasks/8/product-info", domain.PermissionTaskView},
		{http.MethodPatch, "/v1/tasks/8/product-info", domain.PermissionCatalogManage},
		{http.MethodPatch, "/v1/tasks/8/sku-items/4", domain.PermissionCatalogManage},
		{http.MethodPatch, "/v1/tasks/8/sku-items/4", domain.PermissionTaskCreate},
		{http.MethodPost, "/v1/tasks/8/modules/design/claim", domain.PermissionTaskUploadSource},
		{http.MethodGet, "/v1/tasks/audit/handover-candidates", domain.PermissionTaskAuditHandover},
		{http.MethodPost, "/v1/tasks/audit/handover-batch", domain.PermissionTaskAuditHandover},
		{http.MethodPost, "/v1/tasks/8/audit/handover", domain.PermissionTaskAuditHandover},
		{http.MethodGet, "/v1/tasks/8/audit/handovers", domain.PermissionTaskView},
		{http.MethodPost, "/v1/tasks/8/audit/takeover", domain.PermissionTaskAuditHandover},
		{http.MethodPost, "/v1/tasks/8/filing/retry", domain.PermissionERPManage},
		{http.MethodGet, "/v1/tasks/8/assets", domain.PermissionAssetView},
		{http.MethodGet, "/v1/tasks/8/assets/7/download", domain.PermissionAssetDownload},
		{http.MethodPost, "/v1/tasks/8/assets/upload-sessions", domain.PermissionTaskUploadSource},
		{http.MethodGet, "/v1/assets/search", domain.PermissionAssetView},
		{http.MethodDelete, "/v1/assets/77", domain.PermissionAssetManage},
		{http.MethodPost, "/v1/assets/batch-download", domain.PermissionAssetView},
		{http.MethodPost, "/v1/assets/excel-package/preview", domain.PermissionAssetView},
		{http.MethodPost, "/v1/assets/excel-package/preview-file", domain.PermissionAssetView},
		{http.MethodPost, "/v1/assets/excel-package/jobs", domain.PermissionAssetView},
		{http.MethodPost, "/v1/assets/excel-package/jobs/file", domain.PermissionAssetView},
		{http.MethodGet, "/v1/assets/excel-package/jobs/pkg-1", domain.PermissionAssetView},
		{http.MethodGet, "/v1/task-board/overview", domain.PermissionTaskView},
		{http.MethodGet, "/v1/erp/products", domain.PermissionCatalogView},
		{http.MethodGet, "/v1/erp/iids", domain.PermissionCatalogView},
		{http.MethodGet, "/v1/erp/iids", domain.PermissionTaskCreate},
		{http.MethodGet, "/v1/erp/iids", domain.PermissionPlanningSKUCreate},
		{http.MethodPost, "/v1/erp/products/upsert", domain.PermissionERPManage},
		{http.MethodGet, "/v1/cost-rules", domain.PermissionCatalogView},
		{http.MethodPatch, "/v1/cost-rules/3", domain.PermissionCatalogManage},
		{http.MethodGet, "/v1/asset-workbench/bootstrap", domain.PermissionAssetWorkbenchUse},
		{http.MethodPatch, "/v1/asset-workbench/items/9", domain.PermissionAssetWorkbenchQC},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			permissions, governed := v8BusinessRoutePermissions(tt.method, tt.path)
			if !governed {
				t.Fatalf("route is not governed by the v8 capability catalog")
			}
			if !permissionListContains(permissions, tt.want) {
				t.Fatalf("permissions = %v, want %q", permissions, tt.want)
			}
			rule := domain.NewCapabilityRouteAccessRule(tt.method, tt.path, domain.APIReadinessReadyForFrontend, permissions...)
			if len(rule.RequiredRoles) != 0 {
				t.Fatalf("active rule retained role-only authorization: %v", rule.RequiredRoles)
			}
			if len(rule.RequiredPermissions) == 0 {
				t.Fatal("active rule has no explicit capability")
			}
		})
	}
}

func TestProductionPackageRoutesAreAvailableToAssetViewers(t *testing.T) {
	tests := []struct {
		method       string
		path         string
		registration string
	}{
		{method: http.MethodPost, path: "/v1/assets/batch-download", registration: `assetGroup.POST("/batch-download"`},
		{method: http.MethodPost, path: "/v1/assets/excel-package/preview", registration: `assetGroup.POST("/excel-package/preview"`},
		{method: http.MethodPost, path: "/v1/assets/excel-package/preview-file", registration: `assetGroup.POST("/excel-package/preview-file"`},
		{method: http.MethodPost, path: "/v1/assets/excel-package/jobs", registration: `assetGroup.POST("/excel-package/jobs"`},
		{method: http.MethodPost, path: "/v1/assets/excel-package/jobs/file", registration: `assetGroup.POST("/excel-package/jobs/file"`},
		{method: http.MethodGet, path: "/v1/assets/excel-package/jobs/:job_id", registration: `assetGroup.GET("/excel-package/jobs/:job_id"`},
	}

	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read http.go: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			permissions, governed := v8BusinessRoutePermissions(tt.method, tt.path)
			if !governed {
				t.Fatal("production package route is not governed")
			}
			if !permissionListContains(permissions, domain.PermissionAssetView) {
				t.Fatalf("permissions = %v, want asset.view", permissions)
			}
			if permissionListContains(permissions, domain.PermissionAssetDownload) {
				t.Fatalf("permissions = %v, production package must not require asset.download", permissions)
			}

			var registration string
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.Contains(line, tt.registration) {
					registration = strings.TrimSpace(line)
					break
				}
			}
			if registration == "" {
				t.Fatalf("registration not found for %s", tt.path)
			}
			if !strings.Contains(registration, "domain.PermissionAssetView") {
				t.Fatalf("registration missing asset.view: %s", registration)
			}
			if strings.Contains(registration, "domain.PermissionAssetDownload") {
				t.Fatalf("registration still requires asset.download: %s", registration)
			}
		})
	}
}

func TestV8AssetDeleteRegistrationUsesExplicitAssetManageCapability(t *testing.T) {
	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read http.go: %v", err)
	}
	var registration string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `assetGroup.DELETE("/:asset_id"`) {
			registration = strings.TrimSpace(line)
			break
		}
	}
	if registration == "" {
		t.Fatal("DELETE /v1/assets/:asset_id registration not found")
	}
	if !strings.Contains(registration, "capabilityAccess(") || !strings.Contains(registration, "domain.PermissionAssetManage") {
		t.Fatalf("asset delete registration is not explicit asset.manage capability access: %s", registration)
	}
	if strings.Contains(registration, "v1R1AllLoggedInRoles") || strings.Contains(registration, "domain.Role") {
		t.Fatalf("asset delete registration retained legacy role authorization: %s", registration)
	}
}

func TestResourceGroupRevisionRegistrationMatchesAssetViewDetailContract(t *testing.T) {
	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read http.go: %v", err)
	}
	var registration string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `resourceGroups.GET("/:id/revisions"`) {
			registration = strings.TrimSpace(line)
			break
		}
	}
	if registration == "" {
		t.Fatal("GET /v1/resource-groups/:id/revisions registration not found")
	}
	if !strings.Contains(registration, "domain.PermissionAssetView") {
		t.Fatalf("revision registration missing asset.view: %s", registration)
	}
	if strings.Contains(registration, "domain.PermissionTaskView") {
		t.Fatalf("revision registration widens the resource detail contract to task.view: %s", registration)
	}
}

func TestV8AssetWorkbenchUsesNarrowCapabilitiesForHighRiskSurfaces(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   domain.PermissionCode
	}{
		{http.MethodPost, "/v1/asset-workbench/access/disable", domain.PermissionSystemManage},
		{http.MethodPatch, "/v1/asset-workbench/members/7/roles", domain.PermissionSystemManage},
		{http.MethodPost, "/v1/asset-workbench/accounts/merge", domain.PermissionSystemManage},
		{http.MethodGet, "/v1/asset-workbench/profiles", domain.PermissionAssetWorkbenchProfiles},
		{http.MethodPost, "/v1/asset-workbench/difficulty-classes", domain.PermissionAssetWorkbenchTemplates},
		{http.MethodPatch, "/v1/asset-workbench/difficulty-classes/A", domain.PermissionAssetWorkbenchTemplates},
		{http.MethodPost, "/v1/asset-workbench/price-matrix", domain.PermissionAssetWorkbenchTemplates},
		{http.MethodPatch, "/v1/asset-workbench/items/9/qc", domain.PermissionAssetWorkbenchQC},
		{http.MethodPost, "/v1/asset-workbench/files/batch-move", domain.PermissionAssetWorkbenchQC},
		{http.MethodPost, "/v1/asset-workbench/settlement/batches/4/confirm", domain.PermissionAssetWorkbenchSettlement},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			permissions, governed := v8BusinessRoutePermissions(tt.method, tt.path)
			if !governed || !permissionListContains(permissions, tt.want) {
				t.Fatalf("permissions = %v governed=%v, want only dedicated %q", permissions, governed, tt.want)
			}
			for _, forbidden := range []domain.PermissionCode{domain.PermissionAssetView, domain.PermissionAssetManage} {
				if permissionListContains(permissions, forbidden) {
					t.Fatalf("high-risk route leaked through broad %q permission: %v", forbidden, permissions)
				}
			}
		})
	}
}

func TestV8AssetWorkbenchLegacyRegistrationsHaveCompleteCapabilityMapping(t *testing.T) {
	raw, err := os.ReadFile("routes_asset_workbench.go")
	if err != nil {
		t.Fatalf("read routes: %v", err)
	}
	re := regexp.MustCompile(`access\(group, http\.(MethodGet|MethodPost|MethodPatch|MethodPut|MethodDelete), "([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		t.Fatal("no asset-workbench access registrations found")
	}
	for _, match := range matches {
		method := strings.TrimPrefix(match[1], "Method")
		path := "/v1/asset-workbench" + match[2]
		if permissions, governed := v8BusinessRoutePermissions(method, path); !governed || len(permissions) == 0 {
			t.Errorf("%s %s can silently fall back to legacy role middleware", method, path)
		}
	}
}

func TestV8TaskLegacyRegistrationsHaveCompleteCapabilityMapping(t *testing.T) {
	raw, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("read routes: %v", err)
	}
	re := regexp.MustCompile(`access\(taskGroup, http\.(MethodGet|MethodPost|MethodPatch|MethodPut|MethodDelete), "([^"]+)", domain\.APIReadinessReadyForFrontend`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	for _, match := range matches {
		method := strings.TrimPrefix(match[1], "Method")
		path := "/v1/tasks" + match[2]
		t.Errorf("%s %s still uses retired role middleware instead of capabilityAccess", method, path)
	}
}

func TestV8UnmappedTaskWriteFailsClosed(t *testing.T) {
	permissions, governed := v8BusinessRoutePermissions(http.MethodPatch, "/v1/tasks/8/unmapped-write")
	if !governed || len(permissions) != 0 {
		t.Fatalf("unmapped task write permissions=%v governed=%v, want governed empty sentinel input", permissions, governed)
	}
	if unmappedV8RoutePermission == "" {
		t.Fatal("unmapped route sentinel must not be empty")
	}
}

func TestV8PlanningSKURetryAndResyncUseSeparateCapabilities(t *testing.T) {
	retry, _ := v8BusinessRoutePermissions(http.MethodPost, "/v1/tasks/7/planning-skus/erp-retry")
	resync, _ := v8BusinessRoutePermissions(http.MethodPost, "/v1/tasks/7/planning-skus/erp-resync")
	if !permissionListContains(retry, domain.PermissionPlanningSKURetry) || permissionListContains(retry, domain.PermissionPlanningSKUSync) {
		t.Fatalf("retry permissions = %v", retry)
	}
	if !permissionListContains(resync, domain.PermissionPlanningSKUSync) || permissionListContains(resync, domain.PermissionPlanningSKURetry) {
		t.Fatalf("resync permissions = %v", resync)
	}
}

func TestV8TaskAssetUploadSessionAliasesShareCanonicalCapabilityContract(t *testing.T) {
	writes := []string{
		"/v1/assets/upload-sessions",
		"/v1/assets/upload-sessions/session-1/complete",
		"/v1/assets/upload-sessions/session-1/cancel",
		"/v1/tasks/8/assets/upload-sessions",
		"/v1/tasks/8/assets/upload-sessions/session-1/complete",
		"/v1/tasks/8/assets/upload-sessions/session-1/abort",
		"/v1/tasks/8/asset-center/upload-sessions",
		"/v1/tasks/8/asset-center/upload-sessions/small",
		"/v1/tasks/8/asset-center/upload-sessions/multipart",
		"/v1/tasks/8/asset-center/upload-sessions/session-1/complete",
		"/v1/tasks/8/asset-center/upload-sessions/session-1/cancel",
		"/v1/tasks/8/asset-center/upload-sessions/session-1/abort",
	}
	want := []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
	for _, path := range writes {
		permissions, governed := v8BusinessRoutePermissions(http.MethodPost, path)
		if !governed {
			t.Fatalf("POST %s is not capability governed", path)
		}
		for _, permission := range want {
			if !permissionListContains(permissions, permission) {
				t.Errorf("POST %s permissions=%v, missing %s", path, permissions, permission)
			}
		}
	}

	reads := []string{
		"/v1/assets/upload-sessions/session-1",
		"/v1/tasks/8/assets/upload-sessions/session-1",
		"/v1/tasks/8/asset-center/upload-sessions/session-1",
	}
	for _, path := range reads {
		permissions, governed := v8BusinessRoutePermissions(http.MethodGet, path)
		if !governed || !permissionListContains(permissions, domain.PermissionTaskView) || !permissionListContains(permissions, domain.PermissionAssetView) {
			t.Errorf("GET %s permissions=%v governed=%v", path, permissions, governed)
		}
	}
}

func TestV8BusinessRoutePermissionsKeepNonBusinessAndMachineRoutesOutsideCatalog(t *testing.T) {
	for _, path := range []string{
		"/health",
		"/v1/auth/login",
		"/v1/agent/heartbeat",
		"/v1/integration/external-assets/events",
		"/v1/integration/asset-sync/finalized/manifest",
		"/v1/integration/asset-sync/finalized/download-tickets",
	} {
		if permissions, governed := v8BusinessRoutePermissions(http.MethodGet, path); governed {
			t.Fatalf("%s unexpectedly governed with %v", path, permissions)
		}
	}
}

func permissionListContains(items []domain.PermissionCode, want domain.PermissionCode) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
