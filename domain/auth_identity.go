package domain

import "time"

type UserStatus string
type Department string

const (
	DepartmentHR               Department = "人事部"
	DepartmentDesignRD         Department = "设计研发部"
	DepartmentCustomizationArt Department = "定制美工部"
	DepartmentAudit            Department = "审核部"
	DepartmentOperations       Department = "运营部"
	DepartmentCloudWarehouse   Department = "云仓部"
	DepartmentUnassigned       Department = "未分配"

	// Compatibility departments kept for existing persisted data.
	DepartmentDesign      Department = "设计部"
	DepartmentProcurement Department = "采购部"
	DepartmentWarehouse   Department = "仓储部"
	DepartmentBakeryWH    Department = "烘焙仓储部"
)

// DefaultDepartments returns the v1.0 official org baseline only. Legacy
// compatibility departments (设计部 / 采购部 / 仓储部 / 烘焙仓储部) are intentionally
// excluded so that org-master seeding can never resurrect retired rows; use
// CompatibilityDepartments when validating historical config/data inputs.
func DefaultDepartments() []Department {
	return []Department{
		DepartmentHR,
		DepartmentDesignRD,
		DepartmentCustomizationArt,
		DepartmentAudit,
		DepartmentOperations,
		DepartmentCloudWarehouse,
		DepartmentUnassigned,
	}
}

// CompatibilityDepartments lists retired department names that may still
// appear in persisted user rows, historical task snapshots, or existing
// auth_identity.json files. They are accepted as historical inputs but are
// never seeded into org master again.
func CompatibilityDepartments() []Department {
	return []Department{
		DepartmentDesign,
		DepartmentProcurement,
		DepartmentWarehouse,
		DepartmentBakeryWH,
	}
}

// DefaultOrgDepartmentTeams returns the v1.0 official baseline team layout.
// Legacy operations groups (运营一组~运营七组) and compatibility departments are
// intentionally excluded: they only exist as historical rows in org master
// and must not be re-created by startup seeding.
func DefaultOrgDepartmentTeams() map[string][]string {
	return map[string][]string{
		string(DepartmentHR):               {"人事管理组"},
		string(DepartmentDesignRD):         {"默认组"},
		string(DepartmentCustomizationArt): {"默认组"},
		string(DepartmentAudit):            {"普通审核组", "定制审核组"},
		string(DepartmentOperations):       {"淘系一组", "淘系二组", "天猫一组", "天猫二组", "拼多多南京组", "拼多多池州组"},
		string(DepartmentCloudWarehouse):   {"默认组"},
		string(DepartmentUnassigned):       {"未分配池"},
	}
}

// CompatibilityOrgDepartmentTeams lists retired org-team rows (keyed by
// department name) that may still exist in persisted users, historical task
// snapshots, or legacy org-master rows. They stay accepted for validation and
// task owner_team compatibility mapping, but are never seeded again.
func CompatibilityOrgDepartmentTeams() map[string][]string {
	return map[string][]string{
		string(DepartmentHR):               {"默认组"},
		string(DepartmentDesignRD):         {"研发默认组"},
		string(DepartmentCustomizationArt): {"定制默认组"},
		string(DepartmentAudit):            {"定制美工审核组", "常规审核组"},
		string(DepartmentOperations):       {"运营一组", "运营二组", "运营三组", "运营四组", "运营五组", "运营六组", "运营七组"},
		string(DepartmentCloudWarehouse):   {"云仓默认组"},

		// Compatibility departments.
		string(DepartmentDesign):      {"设计组", "定制美工组", "设计审核组"},
		string(DepartmentProcurement): {"采购组"},
		string(DepartmentWarehouse):   {"仓储组"},
		string(DepartmentBakeryWH):    {"烘焙仓储组"},
	}
}

// MergedOrgDepartmentTeamsWithCompatibility returns the baseline layout plus
// every compatibility entry. Use it wherever historical inputs must still be
// recognized (runtime validation fallback, task owner_team compat mapping);
// never use it as a seeding source.
func MergedOrgDepartmentTeamsWithCompatibility() map[string][]string {
	merged := DefaultOrgDepartmentTeams()
	for department, teams := range CompatibilityOrgDepartmentTeams() {
		merged[department] = append(merged[department], teams...)
	}
	return merged
}

// DefaultDepartmentTeams is kept as the task owner_team compatibility source.
// Task create / query / read-model logic still depends on these values, so
// account-org teams must not reuse this function.
func DefaultDepartmentTeams() map[string][]string {
	return map[string][]string{
		"人力行政中心": {"人力行政组"},
		"设计部":    {"设计组"},
		"内贸运营部":  {"内贸运营组"},
		"采购仓储部":  {"采购仓储组"},
		"总经办":    {"总经办组"},
	}
}

func DefaultTaskTeamMappings() map[string][]string {
	return map[string][]string{
		string(DepartmentDesign):      {"设计组"},
		string(DepartmentOperations):  {"内贸运营组"},
		string(DepartmentProcurement): {"采购仓储组"},
		string(DepartmentWarehouse):   {"采购仓储组"},
		string(DepartmentBakeryWH):    {"采购仓储组"},
	}
}

// DepartmentDefaultBusinessRoles returns the default business role bundle for
// department-leader account registration. It intentionally excludes broad
// cross-domain roles and keeps unassigned/non-business departments empty.
func DepartmentDefaultBusinessRoles(department Department) []Role {
	switch department {
	case DepartmentOperations, DepartmentProcurement:
		return []Role{RoleOps}
	case DepartmentCloudWarehouse, DepartmentWarehouse, DepartmentBakeryWH:
		return []Role{RoleWarehouse}
	case DepartmentDesignRD:
		return []Role{RoleDesigner}
	case DepartmentDesign:
		return []Role{RoleDesigner}
	case DepartmentCustomizationArt:
		return []Role{RoleCustomizationOperator}
	case DepartmentAudit:
		return []Role{RoleAuditA, RoleCustomizationReviewer}
	default:
		return []Role{}
	}
}

func (d Department) Valid() bool {
	for _, candidate := range DefaultDepartments() {
		if d == candidate {
			return true
		}
	}
	return false
}

func ValidTeam(team string) bool {
	for _, teams := range DefaultDepartmentTeams() {
		for _, t := range teams {
			if t == team {
				return true
			}
		}
	}
	return false
}

func AllValidTeams() []string {
	var all []string
	for _, teams := range DefaultDepartmentTeams() {
		all = append(all, teams...)
	}
	return all
}

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusDeleted  UserStatus = "deleted"
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusDisabled:
		return true
	default:
		return false
	}
}

type EmploymentType string

const (
	EmploymentTypeFullTime EmploymentType = "full_time"
	EmploymentTypePartTime EmploymentType = "part_time"
)

func (t EmploymentType) Valid() bool {
	switch t {
	case EmploymentTypeFullTime, EmploymentTypePartTime:
		return true
	default:
		return false
	}
}

type User struct {
	ID                 int64              `db:"id"                     json:"id"`
	Username           string             `db:"username"               json:"username"`
	EmployeeNo         *int               `db:"employee_no"            json:"employee_no,omitempty"`
	Account            string             `db:"-"                      json:"account"`
	DisplayName        string             `db:"display_name"           json:"display_name"`
	Name               string             `db:"-"                      json:"name"`
	RealName           string             `db:"-"                      json:"real_name,omitempty"`
	Department         Department         `db:"department"             json:"department"`
	Team               string             `db:"team"                   json:"team,omitempty"`
	Group              string             `db:"-"                      json:"group,omitempty"`
	ManagedDepartments []string           `db:"-"                      json:"managed_departments,omitempty"`
	ManagedTeams       []string           `db:"-"                      json:"managed_teams,omitempty"`
	Mobile             string             `db:"mobile"                 json:"mobile"`
	Phone              string             `db:"-"                      json:"phone"`
	Email              string             `db:"email"                  json:"email,omitempty"`
	AvatarURL          string             `db:"avatar_url"             json:"avatar_url,omitempty"`
	Avatar             string             `db:"-"                      json:"avatar,omitempty"`
	PasswordHash       string             `db:"password_hash"          json:"-"`
	Status             UserStatus         `db:"status"                 json:"status"`
	EmploymentType     EmploymentType     `db:"employment_type"        json:"employment_type"`
	IsConfigSuperAdmin bool               `db:"is_config_super_admin"  json:"-"`
	Roles              []Role             `db:"-"                      json:"roles,omitempty"`
	FrontendAccess     FrontendAccessView `db:"-"                      json:"frontend_access"`
	LastLoginAt        *time.Time         `db:"last_login_at"            json:"last_login_at,omitempty"`
	CreatedAt          time.Time          `db:"created_at"               json:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at"               json:"updated_at"`
	JstUID             *int64             `db:"jst_u_id"                 json:"jst_u_id,omitempty"`
	JstRawSnapshotJSON string             `db:"jst_raw_snapshot_json"    json:"-"`
}

type UserSession struct {
	SessionID  string     `db:"session_id"   json:"session_id"`
	UserID     int64      `db:"user_id"      json:"user_id"`
	TokenHash  string     `db:"token_hash"   json:"-"`
	ExpiresAt  time.Time  `db:"expires_at"   json:"expires_at"`
	LastSeenAt *time.Time `db:"last_seen_at" json:"last_seen_at,omitempty"`
	RevokedAt  *time.Time `db:"revoked_at"   json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at"   json:"created_at"`
}

type AuthSession struct {
	SessionID string    `json:"session_id"`
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AuthResult struct {
	User    *User        `json:"user"`
	Session *AuthSession `json:"session"`
}

type PermissionLog struct {
	ID              int64        `db:"id"                  json:"id"`
	ActorID         *int64       `db:"actor_id"            json:"actor_id,omitempty"`
	ActorUsername   string       `db:"actor_username"      json:"actor_username,omitempty"`
	ActorSource     string       `db:"actor_source"        json:"actor_source"`
	AuthMode        AuthMode     `db:"auth_mode"           json:"auth_mode"`
	Readiness       APIReadiness `db:"readiness"           json:"readiness"`
	SessionRequired bool         `db:"session_required"    json:"session_required"`
	DebugCompatible bool         `db:"debug_compatible"    json:"debug_compatible"`
	ActionType      string       `db:"action_type"         json:"action_type"`
	ActorRoles      []Role       `db:"-"                   json:"actor_roles,omitempty"`
	TargetUserID    *int64       `db:"target_user_id"      json:"target_user_id,omitempty"`
	TargetUsername  string       `db:"target_username"     json:"target_username,omitempty"`
	TargetRoles     []Role       `db:"-"                   json:"target_roles,omitempty"`
	Method          string       `db:"method"              json:"method"`
	RoutePath       string       `db:"route_path"          json:"route_path"`
	RequiredRoles   []Role       `db:"-"                   json:"required_roles,omitempty"`
	Granted         bool         `db:"granted"             json:"granted"`
	Reason          string       `db:"reason"              json:"reason,omitempty"`
	CreatedAt       time.Time    `db:"created_at"          json:"created_at"`
}

type DepartmentOption struct {
	ID        int64           `json:"id,omitempty"`
	Name      string          `json:"name"`
	Teams     []string        `json:"teams,omitempty"`
	TeamItems []OrgTeamOption `json:"team_items,omitempty"`
	Enabled   bool            `json:"enabled"`
	// MemberCount is the number of users currently assigned to this
	// department (any team, active or disabled accounts). Always emitted so
	// zero-member legacy departments are visible to org-maintenance clients.
	MemberCount int `json:"member_count"`
}

type RegistrationOptions struct {
	Departments []DepartmentOption `json:"departments"`
}

type RoleCatalogEntry struct {
	Role                     Role     `json:"role"`
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	Capabilities             []string `json:"capabilities,omitempty"`
	Category                 string   `json:"category,omitempty"`
	Assignable               bool     `json:"assignable"`
	AssignableByCurrentActor bool     `json:"assignable_by_current_actor"`
	Deprecated               bool     `json:"deprecated"`
	HiddenByDefault          bool     `json:"hidden_by_default"`
	AssignmentNote           string   `json:"assignment_note,omitempty"`
}

type ConfiguredSuperAdmin struct {
	Username           string         `json:"username"`
	DisplayName        string         `json:"display_name"`
	Department         Department     `json:"department"`
	Team               string         `json:"team,omitempty"`
	Mobile             string         `json:"mobile"`
	Email              string         `json:"email,omitempty"`
	Password           string         `json:"password"`
	Roles              []Role         `json:"roles,omitempty"`
	ManagedDepartments []string       `json:"managed_departments,omitempty"`
	ManagedTeams       []string       `json:"managed_teams,omitempty"`
	Status             UserStatus     `json:"status,omitempty"`
	EmploymentType     EmploymentType `json:"employment_type,omitempty"`
}

type ConfiguredUserAssignment struct {
	Username           string     `json:"username,omitempty"`
	DisplayName        string     `json:"display_name,omitempty"`
	Department         Department `json:"department"`
	Team               string     `json:"team"`
	Roles              []Role     `json:"roles,omitempty"`
	ManagedDepartments []string   `json:"managed_departments,omitempty"`
	ManagedTeams       []string   `json:"managed_teams,omitempty"`
	Status             UserStatus `json:"status,omitempty"`
}

type AuthSettings struct {
	Departments           []Department               `json:"departments"`
	DepartmentTeams       map[string][]string        `json:"department_teams"`
	PhoneUnique           bool                       `json:"phone_unique"`
	DepartmentAdminKeys   map[string][]string        `json:"department_admin_keys"`
	SuperAdmins           []ConfiguredSuperAdmin     `json:"super_admins"`
	UnassignedPoolEnabled bool                       `json:"unassigned_pool_enabled"`
	ConfiguredAssignments []ConfiguredUserAssignment `json:"configured_user_assignments,omitempty"`
	TaskTeamMappings      map[string][]string        `json:"task_team_mappings,omitempty"`
}

type OrgOptions struct {
	Departments           []DepartmentOption         `json:"departments"`
	TeamsByDepartment     map[string][]string        `json:"teams_by_department"`
	RoleCatalogSummary    []RoleCatalogEntry         `json:"role_catalog_summary"`
	UnassignedPoolEnabled bool                       `json:"unassigned_pool_enabled"`
	ConfiguredAssignments []ConfiguredUserAssignment `json:"configured_assignments,omitempty"`
}

func DefaultRoleCatalog() []RoleCatalogEntry {
	entries := []RoleCatalogEntry{
		{
			Role:         RoleSuperAdmin,
			Name:         "超级管理员",
			Description:  "拥有全局用户、组织、角色、任务与系统管理权限。",
			Capabilities: []string{"user.manage", "role.assign", "org.manage", "permission_logs.read", "task.full_access"},
		},
		{
			Role:         RoleHRAdmin,
			Name:         "人事管理员",
			Description:  "负责人账号、未分配池调度与组织归属管理。",
			Capabilities: []string{"user.manage", "org.assign", "permission_logs.read"},
		},
		{
			Role:         RoleOrgAdmin,
			Name:         "组织管理员",
			Description:  "历史保留角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"org.manage", "user.org.assign"},
		},
		{
			Role:         RoleRoleAdmin,
			Name:         "角色管理员",
			Description:  "历史保留角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"role.read"},
		},
		{
			Role:         RoleAdmin,
			Name:         "管理员",
			Description:  "历史保留管理角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"user.manage", "permission_logs.read", "operation_logs.read", "task.full_access"},
		},
		{
			Role:         RoleDeptAdmin,
			Name:         "部门管理员",
			Description:  "管理本部门用户、小组归属与部门任务协同。",
			Capabilities: []string{"department.manage", "department.users.read", "department.scope", "task.reassign.department"},
		},
		{
			Role:         RoleTeamLead,
			Name:         "组长",
			Description:  "查看本部门任务并管理本组任务协同，不具备账号管理权限。",
			Capabilities: []string{"team.manage", "team.users.read", "task.reassign.team"},
		},
		{
			Role:         RoleDesignDirector,
			Name:         "设计总监",
			Description:  "历史保留角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"design.department.manage", "design.review.read"},
		},
		{
			Role:         RoleDesignReviewer,
			Name:         "设计审核",
			Description:  "历史保留角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"design.review", "design.audit.view"},
		},
		{
			Role:         RoleMember,
			Name:         "成员",
			Description:  "基础成员角色，可查看个人相关信息。",
			Capabilities: []string{"profile.view"},
		},
		{
			Role:         RoleOps,
			Name:         "运营",
			Description:  "创建任务、维护业务信息并推进主流程。",
			Capabilities: []string{"task.create", "task.business_info", "warehouse.prepare", "task.close"},
		},
		{
			Role:         RoleDesigner,
			Name:         "设计师",
			Description:  "处理设计任务并提交设计文件。",
			Capabilities: []string{"task.design_submit", "task.asset_upload"},
		},
		{
			Role:         RoleCustomizationOperator,
			Name:         "定制美工",
			Description:  "处理定制生产上传、效果流转与生产交接。",
			Capabilities: []string{"task.customization.submit", "task.customization.transfer", "task.asset_upload"},
		},
		{
			Role:         RoleAuditA,
			Name:         "常规审核",
			Description:  "处理常规任务审核、交接复核与打回。",
			Capabilities: []string{"task.audit.claim", "task.audit.review", "task.audit.takeover"},
		},
		{
			Role:           RoleAuditB,
			Name:           "历史兼容：常规审核旧编码",
			Description:    "历史保留角色，仅兼容既有账号；常规审核新授权请使用 Audit_A。",
			AssignmentNote: "常规审核旧编码，不允许新增分配",
			Capabilities:   []string{"task.audit.claim", "task.audit.review", "task.audit.takeover"},
		},
		{
			Role:         RoleWarehouse,
			Name:         "仓库人员",
			Description:  "处理仓库接收、退回与完成。",
			Capabilities: []string{"warehouse.receive", "warehouse.reject", "warehouse.complete"},
		},
		{
			Role:         RoleAssetSubmitter,
			Name:         "素材工作台-交付人员",
			Description:  "在素材工作台提交计件交付内容。",
			Capabilities: []string{"asset.workbench.submit", "asset.workbench.profile", "asset.workbench.material.download"},
		},
		{
			Role:         RoleAssetManager,
			Name:         "素材工作台-作品管理",
			Description:  "管理素材工作台提交内容、审核作品并查询素材来源。",
			Capabilities: []string{"asset.workbench.manage", "asset.workbench.system_search"},
		},
		{
			Role:         RoleAssetTemplateAdmin,
			Name:         "素材工作台-计价配置",
			Description:  "维护素材工作台计价、扣款、福利与活动价配置。",
			Capabilities: []string{"asset.workbench.cost_center.manage"},
		},
		{
			Role:         RoleAssetSettlement,
			Name:         "素材工作台-结算财务",
			Description:  "处理素材工作台结算预览、生成、确认、取消与调整。",
			Capabilities: []string{"asset.workbench.settlement", "asset.workbench.export"},
		},
		{
			Role:         RoleOutsource,
			Name:         "外协",
			Description:  "历史保留角色，仅用于兼容既有账号，不再作为新增授权入口。",
			Capabilities: []string{"outsource.manage"},
		},
		{
			Role:         RoleCustomizationReviewer,
			Name:         "定制审核",
			Description:  "处理定制初审、效果审核、修改源文件上传与退回。",
			Capabilities: []string{"task.customization.review", "task.customization.effect_review", "task.customization.review.asset_upload"},
		},
		{
			Role:         RoleERP,
			Name:         "企管ERP",
			Description:  "历史保留角色，仅用于兼容既有系统同步账号。",
			Capabilities: []string{"erp.sync"},
		},
	}
	for i := range entries {
		HydrateRoleCatalogEntryDefaults(&entries[i])
	}
	return entries
}

func HydrateRoleCatalogEntryDefaults(entry *RoleCatalogEntry) {
	if entry == nil {
		return
	}
	entry.Assignable = true
	switch entry.Role {
	case RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin, RoleTeamLead:
		entry.Category = "management"
	case RoleAssetSubmitter, RoleAssetManager, RoleAssetTemplateAdmin, RoleAssetSettlement:
		entry.Category = "asset_workbench"
	case RoleAuditB, RoleAdmin, RoleOrgAdmin, RoleRoleAdmin, RoleDesignDirector, RoleDesignReviewer, RoleOutsource, RoleERP:
		entry.Category = "compatibility"
		entry.Assignable = false
		entry.Deprecated = true
		entry.HiddenByDefault = true
		if entry.AssignmentNote == "" {
			entry.AssignmentNote = "历史兼容角色，不允许新增分配"
		}
	default:
		entry.Category = "business"
	}
	if entry.Role == RoleMember {
		entry.Category = "business"
	}
}
