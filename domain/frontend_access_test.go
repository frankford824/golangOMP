package domain

import "testing"

func TestBuildFrontendAccessDoesNotGrantBusinessAccessFromLegacyRoles(t *testing.T) {
	settings := FrontendAccessSettings{Defaults: FrontendAccessDefaults{AllAuthenticated: FrontendAccessSpec{
		Menus: []string{"dashboard"}, Pages: []string{"dashboard"}, Actions: []string{"account.use"},
	}}}
	view := BuildFrontendAccess(&User{
		ID: 7, Department: DepartmentOperations, Team: "运营一组",
		Roles: []Role{RoleAdmin, RoleOps, RoleAuditA, RoleWarehouse},
	}, settings)
	if len(view.Menus) != 1 || view.Menus[0] != "dashboard" {
		t.Fatalf("legacy roles granted menus: %+v", view.Menus)
	}
	if len(view.Actions) != 1 || view.Actions[0] != "account.use" {
		t.Fatalf("legacy roles granted actions: %+v", view.Actions)
	}
	for _, stale := range []string{"warehouse_receive", "export_center", "audit_queue", "logs_center"} {
		if containsStringValue(view.Menus, stale) || containsStringValue(view.Pages, stale) {
			t.Fatalf("stale frontend access %q leaked: %+v", stale, view)
		}
	}
	if containsStringValue(view.Scopes, "department:运营部") || containsStringValue(view.Scopes, "team:运营一组") {
		t.Fatalf("organization names must not become authorization scopes: %+v", view.Scopes)
	}
}

func TestMergeEffectiveAccessMapsOnlyCurrentMainOpsSurfaces(t *testing.T) {
	view := MergeEffectiveAccessIntoFrontendAccess(FrontendAccessView{Menus: []string{"dashboard"}}, &EffectiveAccess{
		Permissions: []PermissionCode{
			PermissionTaskView, PermissionAssetView, PermissionCatalogView,
			PermissionReportView, PermissionAccessView,
		},
	})
	for _, menu := range []string{"dashboard", "task_list", "resource_management", "cost_rules", "report_center", "user_admin"} {
		if !containsStringValue(view.Menus, menu) {
			t.Fatalf("current menu %q missing: %+v", menu, view.Menus)
		}
	}
	for _, stale := range []string{"warehouse_receive", "export_center", "audit_queue", "logs_center", "product_management"} {
		if containsStringValue(view.Menus, stale) {
			t.Fatalf("stale menu %q leaked: %+v", stale, view.Menus)
		}
	}
}

func TestMergeEffectiveAccessMarksOnlyProtectedSuperAdminSource(t *testing.T) {
	normal := MergeEffectiveAccessIntoFrontendAccess(FrontendAccessView{}, &EffectiveAccess{Sources: []EffectiveAccessNote{{RoleCode: "admin"}}})
	if normal.IsSuperAdmin || normal.ViewAll {
		t.Fatalf("legacy admin elevated: %+v", normal)
	}
	super := MergeEffectiveAccessIntoFrontendAccess(FrontendAccessView{}, &EffectiveAccess{Sources: []EffectiveAccessNote{{RoleCode: "super_admin"}}})
	if !super.IsSuperAdmin || !super.ViewAll {
		t.Fatalf("protected super admin source not reflected: %+v", super)
	}
}

func containsStringValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
