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
	PermissionActionRouteAccess                  = "route_access"
	PermissionActionRegister                     = "register"
	PermissionActionLogin                        = "login"
	PermissionActionLoginFailed                  = "login_failed"
	PermissionActionUserCreated                  = "user_created"
	PermissionActionRoleAssigned                 = "role_assigned"
	PermissionActionRoleRemoved                  = "role_removed"
	PermissionActionPasswordChanged              = "password_changed"
	PermissionActionPasswordReset                = "password_reset"
	PermissionActionUserUpdated                  = "user_updated"
	PermissionActionUserStatusChanged            = "user_status_changed"
	PermissionActionUserActivated                = "user_activated"
	PermissionActionUserDeactivated              = "user_deactivated"
	PermissionActionUserOrgChanged               = "user_org_changed"
	PermissionActionUserDepartmentChangedByAdmin = "user_department_changed_by_admin"
	PermissionActionUserScopeChanged             = "user_scope_changed"
	PermissionActionPoolAssigned                 = "user_pool_assigned"
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

	view.Department = string(user.Department)
	view.Team = user.Team

	roleNames.Add(frontendRoleName(RoleMember))
	scopes.Add("authenticated")

	applyFrontendSpec(roleNames, scopes, menus, pages, actions, modules, settings.Defaults.AllAuthenticated)

	for _, role := range roleValues {
		roleNames.Add(frontendRoleName(role))
		if role == RoleDeptAdmin {
			view.IsDepartmentAdmin = true
		}
	}

	view.ManagedDepartments = nil
	view.ManagedTeams = nil
	view.DepartmentCodes = nil
	view.TeamCodes = nil
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

// MergeEffectiveAccessIntoFrontendAccess derives active menus and actions only
// from the explicit v8 capability model. Legacy role names remain presentation
// metadata and never grant a business action.
func MergeEffectiveAccessIntoFrontendAccess(view FrontendAccessView, effective *EffectiveAccess) FrontendAccessView {
	if effective == nil {
		return view
	}
	actions := newStringSet()
	actions.AddAll(view.Actions...)
	menus := newStringSet()
	menus.AddAll(view.Menus...)
	pages := newStringSet()
	pages.AddAll(view.Pages...)
	for _, permission := range effective.Permissions {
		code := string(permission)
		actions.Add(code)
		for _, alias := range frontendActionAliasesForPermission(permission) {
			actions.Add(alias)
		}
		switch permission {
		case PermissionAccessView, PermissionAccessManage:
			menus.Add("user_admin")
			pages.Add("user_admin")
		case PermissionTaskView, PermissionTaskCreate, PermissionTaskAssign, PermissionTaskReassign, PermissionTaskTerminate, PermissionTaskUploadSource, PermissionTaskAudit, PermissionTaskAuditHandover, PermissionTaskReopen,
			PermissionPlanningSKUView, PermissionPlanningSKUCreate, PermissionPlanningSKUEdit, PermissionPlanningSKUExport, PermissionPlanningSKUSync, PermissionPlanningSKURetry:
			menus.Add("task_list")
			pages.Add("task_list")
		case PermissionAssetView, PermissionAssetDownload, PermissionAssetExport, PermissionAssetPublish, PermissionAssetManage:
			menus.Add("resource_management")
			pages.Add("resource_management")
		case PermissionCatalogView, PermissionCatalogManage:
			menus.Add("cost_rules")
			pages.Add("cost_rules")
		case PermissionAssetWorkbenchUse, PermissionAssetWorkbenchSubmit, PermissionAssetWorkbenchMembers,
			PermissionAssetWorkbenchProfiles, PermissionAssetWorkbenchGroups, PermissionAssetWorkbenchDrive,
			PermissionAssetWorkbenchBatch, PermissionAssetWorkbenchTemplates, PermissionAssetWorkbenchQC,
			PermissionAssetWorkbenchSettlement, PermissionAssetWorkbenchAuditView:
			menus.Add("asset_workbench")
			pages.Add("asset_workbench")
		case PermissionReportView:
			menus.Add("report_center")
			pages.Add("data_center")
		}
	}
	for _, source := range effective.Sources {
		if source.RoleCode == "super_admin" {
			view.IsSuperAdmin = true
			view.ViewAll = true
			break
		}
	}
	view.Actions = actions.SortedValues()
	view.Menus = menus.SortedValues()
	view.Pages = pages.SortedValues()
	view.PermissionFlags = append([]string{}, view.Actions...)
	view.MenuKeys = append([]string{}, view.Menus...)
	view.PageKeys = append([]string{}, view.Pages...)
	return view
}

func frontendActionAliasesForPermission(code PermissionCode) []string {
	switch code {
	case PermissionTaskUploadSource:
		return []string{"task.design.submit"}
	case PermissionTaskAudit:
		return []string{"task.audit.decision", "task.audit"}
	case PermissionTaskAuditHandover:
		return []string{"task.audit.handover"}
	case PermissionTaskAssign, PermissionTaskReassign, PermissionTaskTerminate:
		return []string{"task.manage", "task.assign"}
	case PermissionAccessManage:
		return []string{"access_policy.manage"}
	case PermissionAccessView:
		return []string{"access_policy.view"}
	default:
		return nil
	}
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
	case RoleAssetSubmitter:
		return "asset_submitter"
	case RoleAssetManager:
		return "asset_manager"
	case RoleAssetTemplateAdmin:
		return "asset_template_admin"
	case RoleAssetSettlement:
		return "asset_settlement"
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
