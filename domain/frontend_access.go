package domain

import (
	"encoding/json"
	"sort"
)

// ScopesFlex is []string with flexible JSON: array or object (keys with true).
type ScopesFlex []string

// UnmarshalJSON accepts []string or object {"all":true,"department":true,...}; object keys with true become scope tokens.
func (s *ScopesFlex) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	var obj map[string]bool
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	out := make([]string, 0, len(obj))
	for k, v := range obj {
		if v {
			out = append(out, k)
		}
	}
	*s = out
	return nil
}

const (
	PermissionActionRouteAccess                     = "route_access"
	PermissionActionRegister                        = "register"
	PermissionActionLogin                           = "login"
	PermissionActionLoginFailed                     = "login_failed"
	PermissionActionUserCreated                     = "user_created"
	PermissionActionRoleAssigned                    = "role_assigned"
	PermissionActionRoleRemoved                     = "role_removed"
	PermissionActionPasswordChanged                 = "password_changed"
	PermissionActionPasswordReset                   = "password_reset"
	PermissionActionUserUpdated                     = "user_updated"
	PermissionActionUserStatusChanged               = "user_status_changed"
	PermissionActionUserActivated                   = "user_activated"
	PermissionActionUserDeactivated                 = "user_deactivated"
	PermissionActionUserDeleted                     = "user_deleted"
	PermissionActionUserOrgChanged                  = "user_org_changed"
	PermissionActionUserDepartmentChangedByAdmin    = "user_department_changed_by_admin"
	PermissionActionUserDepartmentChangedViaOrgMove = "user_department_changed_via_org_move"
	PermissionActionUserScopeChanged                = "user_scope_changed"
	PermissionActionPoolAssigned                    = "user_pool_assigned"
	PermissionActionOrgMoveRequested                = "org_move_requested"
	PermissionActionOrgMoveApproved                 = "org_move_approved"
	PermissionActionOrgMoveRejected                 = "org_move_rejected"
)

type FrontendAccessView struct {
	IsSuperAdmin      bool     `json:"is_super_admin"`
	IsDepartmentAdmin bool     `json:"is_department_admin"`
	ViewAll           bool     `json:"view_all"`
	Department        string   `json:"department,omitempty"`
	Team              string   `json:"team,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	Menus             []string `json:"menus,omitempty"`
	Pages             []string `json:"pages,omitempty"`
	Actions           []string `json:"actions,omitempty"`
	Modules           []string `json:"modules,omitempty"`

	ManagedDepartments []string `json:"managed_departments,omitempty"`
	ManagedTeams       []string `json:"managed_teams,omitempty"`
	DepartmentCodes    []string `json:"department_codes,omitempty"`
	TeamCodes          []string `json:"team_codes,omitempty"`

	// Compatibility aliases kept for the existing frontend/auth tests.
	AccessScopes    []string `json:"access_scopes,omitempty"`
	MenuKeys        []string `json:"menu_keys,omitempty"`
	PageKeys        []string `json:"page_keys,omitempty"`
	PermissionFlags []string `json:"permission_flags,omitempty"`
	ModuleKeys      []string `json:"module_keys,omitempty"`
}

type FrontendAccessSpec struct {
	Roles   []string   `json:"roles,omitempty"`
	Scopes  ScopesFlex `json:"scopes,omitempty"`
	Menus   []string   `json:"menus,omitempty"`
	Pages   []string   `json:"pages,omitempty"`
	Actions []string   `json:"actions,omitempty"`
	Modules []string   `json:"modules,omitempty"`

	// Compatibility aliases for older config files.
	AccessScopes    []string `json:"access_scopes,omitempty"`
	MenuKeys        []string `json:"menu_keys,omitempty"`
	PageKeys        []string `json:"page_keys,omitempty"`
	PermissionFlags []string `json:"permission_flags,omitempty"`
	ModuleKeys      []string `json:"module_keys,omitempty"`
}

type FrontendAccessDefaults struct {
	AllAuthenticated FrontendAccessSpec `json:"all_authenticated"`
}

// UnmarshalJSON accepts both {"all_authenticated":{...}} and flat {"menus":[...],"pages":[...],"actions":[...],"scopes":{...}}.
func (d *FrontendAccessDefaults) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if aa, ok := raw["all_authenticated"]; ok {
		return json.Unmarshal(aa, &d.AllAuthenticated)
	}
	spec := &FrontendAccessSpec{}
	if m, ok := raw["menus"]; ok {
		_ = json.Unmarshal(m, &spec.Menus)
	}
	if p, ok := raw["pages"]; ok {
		_ = json.Unmarshal(p, &spec.Pages)
	}
	if a, ok := raw["actions"]; ok {
		_ = json.Unmarshal(a, &spec.Actions)
	}
	if s, ok := raw["scopes"]; ok {
		_ = json.Unmarshal(s, &spec.Scopes)
	}
	d.AllAuthenticated = *spec
	return nil
}

type DepartmentAccessEntry struct {
	Code string `json:"code"`
	FrontendAccessSpec
}

// UnmarshalJSON accepts both flat department frontend-access fields and the
// nested {"frontend_access": {...}} shape used by config/frontend_access.json.
func (d *DepartmentAccessEntry) UnmarshalJSON(data []byte) error {
	type rawDepartmentAccessEntry struct {
		Code           string             `json:"code"`
		FrontendAccess FrontendAccessSpec `json:"frontend_access"`
	}
	var raw rawDepartmentAccessEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Code = raw.Code
	d.FrontendAccessSpec = raw.FrontendAccess

	type flatDepartmentAccessEntry struct {
		Code string `json:"code"`
		FrontendAccessSpec
	}
	var flat flatDepartmentAccessEntry
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	if len(d.FrontendAccessSpec.normalizedRoles()) == 0 &&
		len(d.FrontendAccessSpec.normalizedScopes()) == 0 &&
		len(d.FrontendAccessSpec.normalizedMenus()) == 0 &&
		len(d.FrontendAccessSpec.normalizedPages()) == 0 &&
		len(d.FrontendAccessSpec.normalizedActions()) == 0 &&
		len(d.FrontendAccessSpec.normalizedModules()) == 0 {
		d.FrontendAccessSpec = flat.FrontendAccessSpec
	}
	return nil
}

type TeamEntry struct {
	Department string `json:"department"`
	Code       string `json:"code,omitempty"`
}

type MenuCatalogEntry struct {
	Label     string `json:"label"`
	Icon      string `json:"icon,omitempty"`
	SortOrder int    `json:"sort_order"`
	Parent    string `json:"parent,omitempty"`
}

type FrontendAccessSettings struct {
	Version     string                           `json:"version"`
	Defaults    FrontendAccessDefaults           `json:"defaults"`
	Departments map[string]DepartmentAccessEntry `json:"departments"`
	Teams       map[string]TeamEntry             `json:"teams"`
	Roles       map[string]FrontendAccessSpec    `json:"roles"`
	Identities  map[string]FrontendAccessSpec    `json:"identities"`
	MenuCatalog map[string]MenuCatalogEntry      `json:"menu_catalog"`
}

func BuildFrontendAccess(user *User, settings FrontendAccessSettings) FrontendAccessView {
	view := FrontendAccessView{}
	if user == nil {
		return view
	}

	roleValues := NormalizeRoleValues(user.Roles)
	roleNames := newStringSet()
	scopes := newStringSet()
	menus := newStringSet()
	pages := newStringSet()
	actions := newStringSet()
	modules := newStringSet()
	managedDepartments := newStringSet()
	managedTeams := newStringSet()
	departmentCodes := newStringSet()
	teamCodes := newStringSet()

	view.Department = string(user.Department)
	view.Team = user.Team

	roleNames.Add(frontendRoleName(RoleMember))
	scopes.Add("authenticated")
	if user.Department != "" {
		scopes.Add("department:" + string(user.Department))
		departmentCodes.Add(string(user.Department))
	}
	if user.Team != "" {
		scopes.Add("team:" + user.Team)
		teamCodes.Add(user.Team)
	}
	managedDepartments.AddAll(user.ManagedDepartments...)
	managedTeams.AddAll(user.ManagedTeams...)

	applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, settings.Defaults.AllAuthenticated)
	applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, derivedFrontendSpec(RoleMember))

	for _, role := range roleValues {
		roleNames.Add(frontendRoleName(role))
		if role == RoleAdmin || role == RoleSuperAdmin || role == RoleHRAdmin {
			view.IsSuperAdmin = true
			view.ViewAll = true
			roleNames.Add(frontendRoleName(RoleSuperAdmin))
		}
		if role == RoleDeptAdmin {
			view.IsDepartmentAdmin = true
		}
		applyManagedScope(role, user, managedDepartments, managedTeams)
		applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, derivedFrontendSpec(role))
		applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, settings.Roles[string(role)])
	}

	if user.Department != "" {
		dept := settings.Departments[string(user.Department)]
		scopes.AddAll(dept.FrontendAccessSpec.normalizedScopes()...)
		if view.IsDepartmentAdmin {
			applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, dept.FrontendAccessSpec)
		}
	}

	if view.IsDepartmentAdmin {
		applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, settings.Identities["department_admin"])
	}

	if view.IsSuperAdmin {
		applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, collectAllFrontendAccess(settings))
		applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, settings.Identities["super_admin"])
		scopes.Add("super_admin")
	}

	for _, department := range managedDepartments.SortedValues() {
		scopes.Add("managed_department:" + department)
		departmentCodes.Add(department)
	}
	for _, team := range managedTeams.SortedValues() {
		scopes.Add("managed_team:" + team)
		teamCodes.Add(team)
	}

	view.ManagedDepartments = managedDepartments.SortedValues()
	view.ManagedTeams = managedTeams.SortedValues()
	view.DepartmentCodes = departmentCodes.SortedValues()
	view.TeamCodes = teamCodes.SortedValues()
	view.Roles = roleNames.SortedValues()
	view.Scopes = scopes.SortedValues()
	view.Menus = menus.SortedValues()
	view.Pages = pages.SortedValues()
	view.Actions = actions.SortedValues()
	view.Modules = modules.SortedValues()

	view.AccessScopes = append([]string{}, view.Scopes...)
	view.MenuKeys = append([]string{}, view.Menus...)
	view.PageKeys = append([]string{}, view.Pages...)
	view.PermissionFlags = append([]string{}, view.Actions...)
	view.ModuleKeys = append([]string{}, view.Modules...)
	return view
}

func frontendRoleName(role Role) string {
	switch role {
	case RoleAdmin:
		return "admin"
	case RoleSuperAdmin:
		return "super_admin"
	case RoleHRAdmin:
		return "hr_admin"
	case RoleOrgAdmin:
		return "org_admin"
	case RoleRoleAdmin:
		return "role_admin"
	case RoleDeptAdmin:
		return "department_admin"
	case RoleTeamLead:
		return "team_lead"
	case RoleDesignDirector:
		return "design_director"
	case RoleDesignReviewer:
		return "design_reviewer"
	case RoleMember:
		return "member"
	case RoleOps:
		return "ops"
	case RoleDesigner:
		return "designer"
	case RoleCustomizationOperator:
		return "customization_operator"
	case RoleAuditA:
		return "audit_a"
	case RoleAuditB:
		return "audit_b"
	case RoleWarehouse:
		return "warehouse"
	case RoleOutsource:
		return "outsource"
	case RoleCustomizationReviewer:
		return "customization_reviewer"
	case RoleERP:
		return "erp"
	default:
		return string(role)
	}
}

func derivedFrontendSpec(role Role) FrontendAccessSpec {
	switch role {
	case RoleSuperAdmin:
		return FrontendAccessSpec{
			Roles:   []string{"super_admin"},
			Scopes:  []string{"view_all", "identity_admin", "organization_admin", "role_admin"},
			Menus:   []string{"user_admin", "org_admin", "role_admin", "logs_center"},
			Pages:   []string{"admin_users", "admin_roles", "admin_permission_logs", "admin_operation_logs", "org_options"},
			Actions: []string{"user.manage", "org.manage", "role.assign", "permission_logs.read"},
		}
	case RoleHRAdmin:
		return FrontendAccessSpec{
			Roles:   []string{"hr_admin"},
			Scopes:  []string{"view_all", "hr_admin", "unassigned_pool.manage"},
			Menus:   []string{"user_admin", "org_admin", "role_admin", "logs_center"},
			Pages:   []string{"admin_users", "admin_roles", "admin_permission_logs", "admin_operation_logs", "org_options"},
			Actions: []string{"user.manage", "org.assign", "org.manage", "role.assign", "role.read", "permission_logs.read", "operation_logs.read"},
		}
	case RoleOrgAdmin:
		return FrontendAccessSpec{
			Roles:   []string{"org_admin"},
			Scopes:  []string{"org_admin"},
			Menus:   []string{"org_admin", "user_admin"},
			Pages:   []string{"admin_users", "org_options"},
			Actions: []string{"org.manage", "user.org.assign"},
		}
	case RoleRoleAdmin:
		return FrontendAccessSpec{
			Roles:   []string{"role_admin"},
			Scopes:  []string{"role_admin"},
			Menus:   []string{"role_admin", "user_admin"},
			Pages:   []string{"admin_users", "admin_roles"},
			Actions: []string{"role.assign", "role.remove", "role.read"},
		}
	case RoleDeptAdmin:
		// Round B convergence: DepartmentAdmin is department-scoped and must not
		// surface the org-wide "组织与权限" menu (`org_admin`) nor the
		// org-options page. Org-wide menus belong to HRAdmin and SuperAdmin only.
		return FrontendAccessSpec{
			Roles:  []string{"department_admin"},
			Scopes: []string{"department_scope"},
			Menus:  []string{"user_admin"},
			Pages:  []string{"department_users"},
			Actions: []string{
				"department.manage",
				"department.users.read",
				"department.users.create",
				"department.users.move_team",
				"department.users.disable",
				"department.users.reset_password",
				"department.users.assign_from_unassigned",
				"task.reassign.department",
			},
		}
	case RoleTeamLead:
		// Round C convergence: TeamLead must surface a usable workbench that
		// matches SOT capabilities ("可看本部门全部任务", "只能操作本组任务",
		// "管理本组成员"). `task_list` menu covers department-scoped task
		// visibility, `user_admin` + `team_users` covers own-team member
		// management, and `task.reassign.team` / `team.manage` backs the
		// own-team reassignment policy.
		return FrontendAccessSpec{
			Roles:   []string{"team_lead"},
			Scopes:  []string{"team_scope"},
			Menus:   []string{"task_list", "user_admin"},
			Pages:   []string{"team_users", "task_list", "my_tasks"},
			Actions: []string{"team.users.read", "team.manage", "task.reassign.team", "task.list"},
		}
	case RoleDesignDirector:
		return FrontendAccessSpec{
			Roles:   []string{"design_director"},
			Scopes:  []string{"design_department_scope"},
			Menus:   []string{"design_workspace", "task_list", "customization_management", "warehouse_receive", "warehouse_processing", "resource_management", "user_admin"},
			Pages:   []string{"design_workspace", "task_list", "department_users", "task_assets", "asset_detail", "assets_index", "customization_jobs", "customization_job_detail", "warehouse_receive", "warehouse_processing"},
			Actions: []string{"design.review.read", "department.users.read", "task.list"},
		}
	case RoleDesignReviewer:
		return FrontendAccessSpec{
			Roles:   []string{"design_reviewer"},
			Scopes:  []string{"design_review_scope"},
			Menus:   []string{"design_workspace"},
			Pages:   []string{"design_workspace", "audit_workspace"},
			Actions: []string{"design.review", "task.audit.review"},
		}
	case RoleCustomizationReviewer:
		return FrontendAccessSpec{
			Roles:   []string{"customization_reviewer"},
			Scopes:  []string{"customization_review_scope", "department:审核部"},
			Menus:   []string{"customization_management", "audit_queue", "resource_management"},
			Pages:   []string{"customization_jobs", "customization_job_detail", "task_assets", "asset_detail", "assets_index", "audit_workspace"},
			Actions: []string{"task.customization.review", "task.customization.effect_review", "task.list", "warehouse_lane_filter"},
		}
	case RoleCustomizationOperator:
		return FrontendAccessSpec{
			Roles:   []string{"customization_operator"},
			Scopes:  []string{"customization_workspace", "department:定制美工部"},
			Menus:   []string{"design_workspace", "task_list", "resource_management"},
			Pages:   []string{"design_workspace", "task_list", "my_tasks", "task_assets", "asset_detail", "assets_index"},
			Actions: []string{"task.customization.submit", "task.customization.transfer", "task.asset_upload", "task.list", "warehouse_lane_filter"},
		}
	case RoleMember:
		return FrontendAccessSpec{
			Roles:   []string{"member"},
			Scopes:  []string{"self_only"},
			Menus:   []string{"dashboard"},
			Pages:   []string{"dashboard"},
			Actions: []string{"profile.view"},
		}
	// Round C convergence: explicit defense-in-depth branches for the eight
	// business roles. Each branch mirrors the corresponding entry in
	// config/frontend_access.json so that even when the JSON fails to load,
	// role coverage is preserved. Content must be kept in sync with the JSON.
	case RoleAdmin:
		// Legacy Admin compatibility role. Mirrors config Admin entry. Note:
		// BuildFrontendAccess also promotes Admin to IsSuperAdmin which pulls
		// in the full settings-derived union via collectAllFrontendAccess. The
		// explicit branch below is the fallback when JSON is missing.
		return FrontendAccessSpec{
			Roles:   []string{"admin"},
			Scopes:  []string{"view_all", "identity_admin"},
			Menus:   []string{"user_admin", "logs_center"},
			Pages:   []string{"admin_users", "admin_permission_logs", "admin_operation_logs"},
			Actions: []string{"user.manage", "role.assign", "role.remove", "permission_logs.read", "operation_logs.read", "organization.manage"},
		}
	case RoleOps:
		return FrontendAccessSpec{
			Roles:   []string{"ops"},
			Scopes:  []string{"workflow_ops"},
			Menus:   []string{"task_create", "business_info", "task_board", "task_list", "warehouse_receive", "warehouse_processing", "resource_management", "customization_management"},
			Pages:   []string{"task_board", "task_list", "task_create", "assets_index", "task_assets", "asset_detail", "customization_jobs", "customization_job_detail"},
			Actions: []string{"task.create", "task.business_info", "task.list", "warehouse.prepare", "task.close"},
		}
	case RoleDesigner:
		return FrontendAccessSpec{
			Roles:   []string{"designer"},
			Scopes:  []string{"design_workspace"},
			Menus:   []string{"design_workspace", "task_list", "resource_management"},
			Pages:   []string{"design_workspace", "task_list", "my_tasks", "design_submit", "design_rework", "assets_index", "task_assets", "asset_detail"},
			Actions: []string{"task.design_submit", "task.asset_upload", "task.list"},
		}
	case RoleAuditA:
		return FrontendAccessSpec{
			Roles:   []string{"audit_a"},
			Scopes:  []string{"audit_workspace"},
			Menus:   []string{"audit_queue", "task_board", "task_list", "resource_management"},
			Pages:   []string{"task_board", "task_list", "audit_workspace", "assets_index", "task_assets", "asset_detail"},
			Actions: []string{"task.audit.claim", "task.audit.review", "task.asset_upload", "task.list"},
		}
	case RoleAuditB:
		return FrontendAccessSpec{
			Roles:   []string{"audit_b"},
			Scopes:  []string{"audit_workspace"},
			Menus:   []string{"audit_queue", "task_board", "task_list", "resource_management"},
			Pages:   []string{"task_board", "task_list", "audit_workspace", "assets_index", "task_assets", "asset_detail"},
			Actions: []string{"task.audit.claim", "task.audit.review", "task.audit.takeover", "task.asset_upload", "task.list"},
		}
	case RoleWarehouse:
		return FrontendAccessSpec{
			Roles:   []string{"warehouse"},
			Scopes:  []string{"warehouse_workspace"},
			Menus:   []string{"warehouse_receive", "warehouse_processing", "resource_management", "export_center"},
			Pages:   []string{"warehouse_receive", "warehouse_processing", "task_list", "task_board", "export_jobs", "assets_index", "task_assets", "asset_detail"},
			Actions: []string{"warehouse.receive", "warehouse.reject", "warehouse.complete", "task.list"},
		}
	case RoleOutsource:
		return FrontendAccessSpec{
			Roles:   []string{"outsource"},
			Scopes:  []string{"outsource_workspace"},
			Menus:   []string{"task_list"},
			Pages:   []string{"outsource_orders", "task_list"},
			Actions: []string{"outsource.manage", "task.list"},
		}
	case RoleERP:
		return FrontendAccessSpec{
			Roles:   []string{"erp"},
			Scopes:  []string{"erp_internal"},
			Menus:   []string{"integration_center"},
			Pages:   []string{"erp_sync_console"},
			Actions: []string{"erp.sync"},
		}
	default:
		return FrontendAccessSpec{}
	}
}

func applyManagedScope(role Role, user *User, managedDepartments, managedTeams stringSet) {
	if user == nil {
		return
	}
	switch role {
	case RoleDeptAdmin, RoleDesignDirector:
		if len(user.ManagedDepartments) == 0 && user.Department != "" {
			managedDepartments.Add(string(user.Department))
		}
	case RoleTeamLead:
		if len(user.ManagedTeams) == 0 && user.Team != "" {
			managedTeams.Add(user.Team)
		}
	}
}

func collectAllFrontendAccess(settings FrontendAccessSettings) FrontendAccessSpec {
	all := FrontendAccessSpec{}
	appendSpec := func(spec FrontendAccessSpec) {
		all.Roles = append(all.Roles, spec.normalizedRoles()...)
		all.Scopes = append(all.Scopes, spec.normalizedScopes()...)
		all.Menus = append(all.Menus, spec.normalizedMenus()...)
		all.Pages = append(all.Pages, spec.normalizedPages()...)
		all.Actions = append(all.Actions, spec.normalizedActions()...)
		all.Modules = append(all.Modules, spec.normalizedModules()...)
	}

	appendSpec(settings.Defaults.AllAuthenticated)
	for _, spec := range settings.Roles {
		appendSpec(spec)
	}
	for _, entry := range settings.Departments {
		appendSpec(entry.FrontendAccessSpec)
	}
	appendSpec(settings.Identities["department_admin"])
	return all
}

func applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules stringSet, spec FrontendAccessSpec) {
	roleNames.AddAll(spec.normalizedRoles()...)
	scopes.AddAll(spec.normalizedScopes()...)
	menus.AddAll(spec.normalizedMenus()...)
	pages.AddAll(spec.normalizedPages()...)
	actions.AddAll(spec.normalizedActions()...)
	modules.AddAll(spec.normalizedModules()...)
}

func (s FrontendAccessSpec) normalizedRoles() []string {
	return dedupeStrings(s.Roles)
}

func (s FrontendAccessSpec) normalizedScopes() []string {
	return dedupeStrings(append(append([]string{}, s.Scopes...), s.AccessScopes...))
}

func (s FrontendAccessSpec) normalizedMenus() []string {
	return dedupeStrings(append(append([]string{}, s.Menus...), s.MenuKeys...))
}

func (s FrontendAccessSpec) normalizedPages() []string {
	return dedupeStrings(append(append([]string{}, s.Pages...), s.PageKeys...))
}

func (s FrontendAccessSpec) normalizedActions() []string {
	return dedupeStrings(append(append([]string{}, s.Actions...), s.PermissionFlags...))
}

func (s FrontendAccessSpec) normalizedModules() []string {
	return dedupeStrings(append(append([]string{}, s.Modules...), s.ModuleKeys...))
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

type stringSet map[string]struct{}

func newStringSet() stringSet {
	return stringSet{}
}

func (s stringSet) Add(value string) {
	if value == "" {
		return
	}
	s[value] = struct{}{}
}

func (s stringSet) AddAll(values ...string) {
	for _, value := range values {
		s.Add(value)
	}
}

func (s stringSet) SortedValues() []string {
	out := make([]string, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
