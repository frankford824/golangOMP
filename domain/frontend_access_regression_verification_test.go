package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFrontendAccessRegressionScenarioMatrix(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)
	superAdminMenus := fullMenuUnionMenus(settings)

	scenarios := []struct {
		name        string
		user        *User
		expectMenus []string
	}{
		{
			name: "Row01_OpsDeptAdmin",
			user: &User{
				ID:         2001,
				Username:   "roundk_ops_dept_admin",
				Department: DepartmentOperations,
				Team:       "运营一组",
				Roles:      []Role{RoleMember, RoleDeptAdmin, RoleOps},
			},
			expectMenus: []string{"dashboard", "task_create", "business_info", "task_board", "task_list", "customization_management", "warehouse_receive", "warehouse_processing", "resource_management", "user_admin", "task_center"},
		},
		{
			name: "Row02_OpsMember",
			user: &User{
				ID:         2002,
				Username:   "roundk_ops_member",
				Department: DepartmentOperations,
				Team:       "运营二组",
				Roles:      []Role{RoleMember, RoleOps},
			},
			expectMenus: []string{"dashboard", "task_create", "business_info", "task_board", "task_list", "customization_management", "warehouse_receive", "warehouse_processing", "resource_management"},
		},
		{
			name: "Row03_DesignDirectorDeptAdmin",
			user: &User{
				ID:         2003,
				Username:   "roundk_design_director_admin",
				Department: DepartmentDesign,
				Team:       "设计审核组",
				Roles:      []Role{RoleMember, RoleDeptAdmin, RoleDesignDirector},
			},
			expectMenus: []string{"dashboard", "design_workspace", "customization_management", "warehouse_receive", "warehouse_processing", "resource_management", "user_admin", "task_center", "task_list"},
		},
		{
			name: "Row04_DesignerMember",
			user: &User{
				ID:         2004,
				Username:   "roundk_designer_member",
				Department: DepartmentDesignRD,
				Team:       "研发默认组",
				Roles:      []Role{RoleMember, RoleDesigner},
			},
			expectMenus: []string{"dashboard", "design_workspace", "resource_management", "task_list"},
		},
		{
			name: "Row05_AuditAMember",
			user: &User{
				ID:         2005,
				Username:   "roundk_audit_a_member",
				Department: DepartmentAudit,
				Team:       "常规审核组",
				Roles:      []Role{RoleMember, RoleAuditA},
			},
			expectMenus: []string{"dashboard", "task_board", "task_list", "audit_queue", "resource_management"},
		},
		{
			name: "Row05b_AuditBMember",
			user: &User{
				ID:         2006,
				Username:   "roundk_audit_b_member",
				Department: DepartmentAudit,
				Team:       "普通审核组",
				Roles:      []Role{RoleMember, RoleAuditB},
			},
			expectMenus: []string{"dashboard", "task_board", "task_list", "audit_queue", "resource_management"},
		},
		{
			name: "Row06_CustomizationReviewerMember",
			user: &User{
				ID:         2007,
				Username:   "roundk_customization_reviewer",
				Department: DepartmentAudit,
				Team:       "定制审核组",
				Roles:      []Role{RoleMember, RoleCustomizationReviewer},
			},
			expectMenus: []string{"dashboard", "customization_management", "audit_queue", "resource_management"},
		},
		{
			name: "Row07_AuditDeptAdmin",
			user: &User{
				ID:         2008,
				Username:   "roundk_audit_dept_admin",
				Department: DepartmentAudit,
				Team:       "普通审核组",
				Roles:      []Role{RoleMember, RoleDeptAdmin, RoleAuditA, RoleAuditB, RoleCustomizationReviewer},
			},
			expectMenus: []string{"dashboard", "task_board", "task_list", "audit_queue", "customization_management", "resource_management", "user_admin", "task_center"},
		},
		{
			name: "Row08_CustomizationDeptAdmin",
			user: &User{
				ID:         2009,
				Username:   "roundk_customization_dept_admin",
				Department: DepartmentCustomizationArt,
				Team:       "定制默认组",
				Roles:      []Role{RoleMember, RoleDeptAdmin, RoleCustomizationOperator},
			},
			expectMenus: []string{"dashboard", "design_workspace", "customization_management", "resource_management", "user_admin", "task_center", "task_list"},
		},
		{
			name: "Row09_CustomizationOperatorMember",
			user: &User{
				ID:         2010,
				Username:   "roundk_customization_operator",
				Department: DepartmentCustomizationArt,
				Team:       "定制默认组",
				Roles:      []Role{RoleMember, RoleCustomizationOperator},
			},
			expectMenus: []string{"dashboard", "design_workspace", "resource_management", "task_list"},
		},
		{
			name: "Row10_WarehouseDeptAdmin",
			user: &User{
				ID:         2011,
				Username:   "roundk_warehouse_admin",
				Department: DepartmentCloudWarehouse,
				Team:       "云仓默认组",
				Roles:      []Role{RoleMember, RoleDeptAdmin, RoleWarehouse},
			},
			expectMenus: []string{"dashboard", "warehouse_receive", "warehouse_processing", "resource_management", "export_center", "user_admin", "task_center", "task_list"},
		},
		{
			name: "Row11_WarehouseMember",
			user: &User{
				ID:         2012,
				Username:   "roundk_warehouse_member",
				Department: DepartmentCloudWarehouse,
				Team:       "云仓默认组",
				Roles:      []Role{RoleMember, RoleWarehouse},
			},
			expectMenus: []string{"dashboard", "warehouse_receive", "warehouse_processing", "resource_management", "export_center"},
		},
		{
			name: "Row12_BareMember",
			user: &User{
				ID:         2013,
				Username:   "roundk_bare_member",
				Department: DepartmentUnassigned,
				Team:       "未分配池",
				Roles:      []Role{RoleMember},
			},
			expectMenus: []string{"dashboard"},
		},
		{
			name: "Row13_HRAdmin",
			user: &User{
				ID:         2014,
				Username:   "roundk_hr_admin",
				Department: DepartmentHR,
				Team:       "人事管理组",
				Roles:      []Role{RoleHRAdmin},
			},
			expectMenus: superAdminMenus,
		},
		{
			name: "Row14_SuperAdmin",
			user: &User{
				ID:         2015,
				Username:   "roundk_super_admin",
				Department: DepartmentUnassigned,
				Team:       "未分配池",
				Roles:      []Role{RoleSuperAdmin},
			},
			expectMenus: superAdminMenus,
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			view := BuildFrontendAccess(scenario.user, settings)
			assertStringSetEqual(t, view.Menus, scenario.expectMenus, "menus")
			assertCompatibilityMirrors(t, view)
		})
	}
}

func TestCustomizationArtDeptAdminSeesCustomizationManagementMenu(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)

	view := BuildFrontendAccess(&User{
		ID:         2101,
		Username:   "customization_art_admin",
		Department: DepartmentCustomizationArt,
		Team:       "定制默认组",
		Roles:      []Role{RoleMember, RoleDeptAdmin, RoleCustomizationOperator},
	}, settings)

	assertContainsAll(t, view.Menus, []string{"dashboard", "design_workspace", "customization_management", "resource_management", "user_admin"}, "menus")
}

func TestHRAdminGetsFullMenuUnion(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)

	hrView := BuildFrontendAccess(&User{
		ID:         2102,
		Username:   "hr_admin_union",
		Department: DepartmentHR,
		Team:       "人事管理组",
		Roles:      []Role{RoleHRAdmin},
	}, settings)
	superView := BuildFrontendAccess(&User{
		ID:         2103,
		Username:   "super_admin_union",
		Department: DepartmentUnassigned,
		Team:       "未分配池",
		Roles:      []Role{RoleSuperAdmin},
	}, settings)

	assertStringSetEqual(t, hrView.Menus, superView.Menus, "menus")
}

func TestCustomizationOperatorMemberHidesCustomizationManagement(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)

	view := BuildFrontendAccess(&User{
		ID:         2104,
		Username:   "customization_operator_member",
		Department: DepartmentCustomizationArt,
		Team:       "定制默认组",
		Roles:      []Role{RoleMember, RoleCustomizationOperator},
	}, settings)

	assertStringSetEqual(t, view.Menus, []string{"dashboard", "design_workspace", "resource_management", "task_list"}, "menus")
	assertNotContainsAny(t, view.Menus, []string{"customization_management"}, "menus")
}

func TestBareMemberLandsOnDashboardOnly(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)

	view := BuildFrontendAccess(&User{
		ID:         2105,
		Username:   "bare_member_dashboard",
		Department: DepartmentUnassigned,
		Team:       "未分配池",
		Roles:      []Role{RoleMember},
	}, settings)

	assertStringSetEqual(t, view.Menus, []string{"dashboard"}, "menus")
}

func TestFrontendAccessRoundKConfigSync(t *testing.T) {
	settings := mustLoadFrontendAccessSettingsForRegression(t)

	roleKeys := map[Role]string{
		RoleMember:                "Member",
		RoleOps:                   "Ops",
		RoleDesignDirector:        "DesignDirector",
		RoleDesigner:              "Designer",
		RoleCustomizationOperator: "CustomizationOperator",
		RoleAuditA:                "Audit_A",
		RoleAuditB:                "Audit_B",
		RoleWarehouse:             "Warehouse",
		RoleCustomizationReviewer: "CustomizationReviewer",
	}

	for role, key := range roleKeys {
		role := role
		key := key
		t.Run("Role_"+key, func(t *testing.T) {
			want := derivedFrontendSpec(role)
			got, ok := settings.Roles[key]
			if !ok {
				t.Fatalf("settings.Roles missing %q", key)
			}
			assertStringSetEqual(t, got.normalizedMenus(), want.normalizedMenus(), "menus")
			assertStringSetEqual(t, got.normalizedPages(), want.normalizedPages(), "pages")
			assertStringSetEqual(t, got.normalizedActions(), want.normalizedActions(), "actions")
		})
	}

	dept, ok := settings.Departments[string(DepartmentCustomizationArt)]
	if !ok {
		t.Fatalf("settings.Departments missing %q", DepartmentCustomizationArt)
	}
	assertStringSetEqual(t, dept.FrontendAccessSpec.normalizedMenus(), []string{"customization_management"}, "department menus")
	assertStringSetEqual(t, dept.FrontendAccessSpec.normalizedPages(), []string{"customization_jobs", "customization_job_detail"}, "department pages")
}

func mustLoadFrontendAccessSettingsForRegression(t *testing.T) FrontendAccessSettings {
	t.Helper()
	path := filepath.Join("..", "config", "frontend_access.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read frontend_access config failed: %v", err)
	}
	var settings FrontendAccessSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("unmarshal frontend_access config failed: %v", err)
	}
	return settings
}

func fullMenuUnionMenus(settings FrontendAccessSettings) []string {
	menus := newStringSet()
	menus.AddAll(settings.Defaults.AllAuthenticated.normalizedMenus()...)
	menus.AddAll(derivedFrontendSpec(RoleMember).normalizedMenus()...)
	menus.AddAll(collectAllFrontendAccess(settings).normalizedMenus()...)
	menus.AddAll(settings.Identities["super_admin"].normalizedMenus()...)
	return menus.SortedValues()
}

func assertContainsAll(t *testing.T, actual []string, expected []string, label string) {
	t.Helper()
	for _, item := range expected {
		if !slices.Contains(actual, item) {
			t.Fatalf("%s missing %q in %v", label, item, actual)
		}
	}
}

func assertNotContainsAny(t *testing.T, actual []string, denied []string, label string) {
	t.Helper()
	for _, item := range denied {
		if slices.Contains(actual, item) {
			t.Fatalf("%s unexpectedly contains %q in %v", label, item, actual)
		}
	}
}

func assertStringSetEqual(t *testing.T, actual []string, expected []string, label string) {
	t.Helper()
	actualSorted := slices.Clone(actual)
	expectedSorted := slices.Clone(expected)
	slices.Sort(actualSorted)
	slices.Sort(expectedSorted)
	if !slices.Equal(actualSorted, expectedSorted) {
		t.Fatalf("%s mismatch, got=%v want=%v", label, actualSorted, expectedSorted)
	}
}

func assertCompatibilityMirrors(t *testing.T, view FrontendAccessView) {
	t.Helper()
	if !slices.Equal(view.AccessScopes, view.Scopes) {
		t.Fatalf("access_scopes mismatch, scopes=%v access_scopes=%v", view.Scopes, view.AccessScopes)
	}
	if !slices.Equal(view.MenuKeys, view.Menus) {
		t.Fatalf("menu_keys mismatch, menus=%v menu_keys=%v", view.Menus, view.MenuKeys)
	}
	if !slices.Equal(view.PageKeys, view.Pages) {
		t.Fatalf("page_keys mismatch, pages=%v page_keys=%v", view.Pages, view.PageKeys)
	}
	if !slices.Equal(view.PermissionFlags, view.Actions) {
		t.Fatalf("permission_flags mismatch, actions=%v permission_flags=%v", view.Actions, view.PermissionFlags)
	}
	if !slices.Equal(view.ModuleKeys, view.Modules) {
		t.Fatalf("module_keys mismatch, modules=%v module_keys=%v", view.Modules, view.ModuleKeys)
	}
}
