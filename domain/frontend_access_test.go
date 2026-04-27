package domain

import "testing"

func TestBuildFrontendAccessDepartmentOnlyAddsScopeNotBusinessMenus(t *testing.T) {
	settings := FrontendAccessSettings{
		Defaults: FrontendAccessDefaults{
			AllAuthenticated: FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home", "profile_me"},
				Actions: []string{"auth.me.read", "profile.view"},
			},
		},
		Departments: map[string]DepartmentAccessEntry{
			string(DepartmentOperations): {
				Code: "operations",
				FrontendAccessSpec: FrontendAccessSpec{
					Scopes:  []string{"department_operations"},
					Menus:   []string{"task_board", "task_list"},
					Pages:   []string{"task_board", "task_list"},
					Actions: []string{"task.list"},
				},
			},
		},
		Roles: map[string]FrontendAccessSpec{},
	}

	view := BuildFrontendAccess(&User{
		ID:         7,
		Username:   "member_ops_scope_only",
		Department: DepartmentOperations,
		Team:       "运营三组",
		Roles:      []Role{RoleMember},
	}, settings)

	if !containsStringValue(view.Scopes, "department_operations") {
		t.Fatalf("department scope missing: %+v", view.Scopes)
	}
	if containsStringValue(view.Menus, "task_board") || containsStringValue(view.Menus, "task_list") {
		t.Fatalf("department-only user should not receive task menus: %+v", view.Menus)
	}
	if containsStringValue(view.Pages, "task_board") || containsStringValue(view.Pages, "task_list") {
		t.Fatalf("department-only user should not receive task pages: %+v", view.Pages)
	}
	if containsStringValue(view.Actions, "task.list") {
		t.Fatalf("department-only user should not receive task actions: %+v", view.Actions)
	}
}

func TestBuildFrontendAccessRoleStillAddsBusinessMenus(t *testing.T) {
	settings := FrontendAccessSettings{
		Defaults: FrontendAccessDefaults{
			AllAuthenticated: FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home", "profile_me"},
				Actions: []string{"auth.me.read", "profile.view"},
			},
		},
		Departments: map[string]DepartmentAccessEntry{
			string(DepartmentOperations): {
				Code: "operations",
				FrontendAccessSpec: FrontendAccessSpec{
					Scopes: []string{"department_operations"},
				},
			},
		},
		Roles: map[string]FrontendAccessSpec{
			string(RoleOps): {
				Roles:   []string{"ops"},
				Scopes:  []string{"workflow_ops"},
				Menus:   []string{"task_board", "task_list"},
				Pages:   []string{"task_board", "task_list"},
				Actions: []string{"task.list"},
			},
		},
	}

	view := BuildFrontendAccess(&User{
		ID:         8,
		Username:   "ops_user",
		Department: DepartmentOperations,
		Team:       "运营三组",
		Roles:      []Role{RoleMember, RoleOps},
	}, settings)

	if !containsStringValue(view.Scopes, "department_operations") || !containsStringValue(view.Scopes, "workflow_ops") {
		t.Fatalf("scopes missing: %+v", view.Scopes)
	}
	if !containsStringValue(view.Menus, "task_board") || !containsStringValue(view.Menus, "task_list") {
		t.Fatalf("ops role menus missing: %+v", view.Menus)
	}
	if !containsStringValue(view.Pages, "task_board") || !containsStringValue(view.Pages, "task_list") {
		t.Fatalf("ops role pages missing: %+v", view.Pages)
	}
	if !containsStringValue(view.Actions, "task.list") {
		t.Fatalf("ops role actions missing: %+v", view.Actions)
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

func TestBuildFrontendAccessIncludesCustomizationAndResourceVisibility(t *testing.T) {
	settings := FrontendAccessSettings{
		Defaults: FrontendAccessDefaults{
			AllAuthenticated: FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home"},
				Actions: []string{"auth.me.read"},
			},
		},
		Departments: map[string]DepartmentAccessEntry{},
		Roles:       map[string]FrontendAccessSpec{},
	}

	view := BuildFrontendAccess(&User{
		ID:    88,
		Roles: []Role{RoleOps, RoleCustomizationReviewer},
	}, settings)

	if !containsStringValue(view.Menus, "resource_management") || !containsStringValue(view.Menus, "customization_management") {
		t.Fatalf("menus missing resource/customization visibility: %+v", view.Menus)
	}
	if !containsStringValue(view.Pages, "assets_index") || !containsStringValue(view.Pages, "customization_jobs") || !containsStringValue(view.Pages, "customization_job_detail") {
		t.Fatalf("pages missing customization/resource entries: %+v", view.Pages)
	}
}

func TestBuildFrontendAccessCustomizationReviewerRoleAddsAuditQueueAndAssets(t *testing.T) {
	settings := FrontendAccessSettings{
		Defaults: FrontendAccessDefaults{
			AllAuthenticated: FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home"},
				Actions: []string{"auth.me.read"},
			},
		},
		Departments: map[string]DepartmentAccessEntry{},
		Roles:       map[string]FrontendAccessSpec{},
	}

	view := BuildFrontendAccess(&User{
		ID:    99,
		Roles: []Role{RoleMember, RoleCustomizationReviewer},
	}, settings)

	if !containsStringValue(view.Roles, "customization_reviewer") {
		t.Fatalf("roles missing customization reviewer: %+v", view.Roles)
	}
	if !containsStringValue(view.Menus, "customization_management") || !containsStringValue(view.Menus, "resource_management") || !containsStringValue(view.Menus, "audit_queue") {
		t.Fatalf("menus missing customization/audit/resource visibility: %+v", view.Menus)
	}
	if containsStringValue(view.Menus, "task_list") {
		t.Fatalf("customization reviewer should not receive task_list top-level menu: %+v", view.Menus)
	}
	if !containsStringValue(view.Pages, "customization_jobs") || !containsStringValue(view.Pages, "assets_index") || !containsStringValue(view.Pages, "audit_workspace") {
		t.Fatalf("pages missing customization/resource entries: %+v", view.Pages)
	}
}

func TestBuildFrontendAccessCustomizationOperatorRoleUsesDesignWorkspaceBaseMenus(t *testing.T) {
	settings := FrontendAccessSettings{
		Defaults: FrontendAccessDefaults{
			AllAuthenticated: FrontendAccessSpec{
				Roles:   []string{"member"},
				Scopes:  []string{"frontend_ready", "self_only"},
				Menus:   []string{"dashboard"},
				Pages:   []string{"dashboard_home"},
				Actions: []string{"auth.me.read"},
			},
		},
		Departments: map[string]DepartmentAccessEntry{},
		Roles:       map[string]FrontendAccessSpec{},
	}

	view := BuildFrontendAccess(&User{
		ID:    100,
		Roles: []Role{RoleMember, RoleCustomizationOperator},
	}, settings)

	if !containsStringValue(view.Roles, "customization_operator") {
		t.Fatalf("roles missing customization operator: %+v", view.Roles)
	}
	if !containsStringValue(view.Menus, "design_workspace") || !containsStringValue(view.Menus, "resource_management") {
		t.Fatalf("menus missing customization operator base workspace: %+v", view.Menus)
	}
	if containsStringValue(view.Menus, "customization_management") || containsStringValue(view.Menus, "task_list") {
		t.Fatalf("customization operator member should not receive dept-admin customization menus: %+v", view.Menus)
	}
	if !containsStringValue(view.Actions, "task.customization.submit") || !containsStringValue(view.Actions, "task.customization.transfer") {
		t.Fatalf("actions missing customization operator abilities: %+v", view.Actions)
	}
}
