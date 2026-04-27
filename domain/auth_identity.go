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

func DefaultDepartments() []Department {
	return []Department{
		DepartmentHR,
		DepartmentDesignRD,
		DepartmentCustomizationArt,
		DepartmentAudit,
		DepartmentOperations,
		DepartmentCloudWarehouse,
		DepartmentUnassigned,
		// Compatibility values.
		DepartmentDesign,
		DepartmentProcurement,
		DepartmentWarehouse,
		DepartmentBakeryWH,
	}
}

func DefaultOrgDepartmentTeams() map[string][]string {
	return map[string][]string{
		string(DepartmentHR):               {"默认组", "人事管理组"},
		string(DepartmentDesignRD):         {"默认组", "研发默认组"},
		string(DepartmentCustomizationArt): {"默认组", "定制默认组"},
		string(DepartmentAudit):            {"普通审核组", "定制美工审核组", "常规审核组", "定制审核组"},
		string(DepartmentOperations):       {"淘系一组", "淘系二组", "天猫一组", "天猫二组", "拼多多南京组", "拼多多池州组", "运营一组", "运营二组", "运营三组", "运营四组", "运营五组", "运营六组", "运营七组"},
		string(DepartmentCloudWarehouse):   {"默认组", "云仓默认组"},
		string(DepartmentUnassigned):       {"未分配池"},

		// Compatibility departments.
		string(DepartmentDesign):      {"设计组", "定制美工组", "设计审核组"},
		string(DepartmentProcurement): {"采购组"},
		string(DepartmentWarehouse):   {"仓储组"},
		string(DepartmentBakeryWH):    {"烘焙仓储组"},
	}
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
		return []Role{RoleDesigner, RoleDesignReviewer}
	case DepartmentCustomizationArt:
		return []Role{RoleCustomizationOperator}
	case DepartmentAudit:
		return []Role{RoleAuditA, RoleAuditB, RoleCustomizationReviewer}
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
	Account            string             `db:"-"                      json:"account"`
	DisplayName        string             `db:"display_name"           json:"display_name"`
	Name               string             `db:"-"                      json:"name"`
	Department         Department         `db:"department"             json:"department"`
	Team               string             `db:"team"                   json:"team,omitempty"`
	Group              string             `db:"-"                      json:"group,omitempty"`
	ManagedDepartments []string           `db:"-"                      json:"managed_departments,omitempty"`
	ManagedTeams       []string           `db:"-"                      json:"managed_teams,omitempty"`
	Mobile             string             `db:"mobile"                 json:"mobile"`
	Phone              string             `db:"-"                      json:"phone"`
	Email              string             `db:"email"                  json:"email,omitempty"`
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
	Enabled   bool            `json:"enabled,omitempty"`
}

type RegistrationOptions struct {
	Departments []DepartmentOption `json:"departments"`
}

type RoleCatalogEntry struct {
	Role         Role     `json:"role"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities,omitempty"`
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
	return []RoleCatalogEntry{
		{
			Role:         RoleSuperAdmin,
			Name:         "Super Admin",
			Description:  "Global authority for user, role, organization, and business runtime administration.",
			Capabilities: []string{"user.manage", "role.assign", "org.manage", "permission_logs.read", "task.full_access"},
		},
		{
			Role:         RoleHRAdmin,
			Name:         "HR Admin",
			Description:  "Manage people records, unassigned pool dispatch, and personnel-facing organization administration.",
			Capabilities: []string{"user.manage", "org.assign", "permission_logs.read"},
		},
		{
			Role:         RoleOrgAdmin,
			Name:         "Org Admin",
			Description:  "Legacy compatibility role for historical organization-affiliation flows; not a future-facing product role.",
			Capabilities: []string{"org.manage", "user.org.assign"},
		},
		{
			Role:         RoleRoleAdmin,
			Name:         "Role Admin",
			Description:  "Legacy compatibility role for historical role-assignment flows; not a future-facing product role.",
			Capabilities: []string{"role.assign", "role.remove", "role.read"},
		},
		{
			Role:         RoleAdmin,
			Name:         "Admin",
			Description:  "Legacy compatibility management role kept for existing route guards during v1.0 convergence.",
			Capabilities: []string{"user.manage", "role.assign", "permission_logs.read", "operation_logs.read", "task.full_access"},
		},
		{
			Role:         RoleDeptAdmin,
			Name:         "Department Admin",
			Description:  "Formal department-scoped management role for user creation, disable/reset, intra-department team moves, and department task coordination.",
			Capabilities: []string{"department.manage", "department.users.read", "department.scope", "task.reassign.department"},
		},
		{
			Role:         RoleTeamLead,
			Name:         "Team Lead",
			Description:  "Formal own-team management role with own-team task reassignment and department-wide task visibility, but no account admin powers.",
			Capabilities: []string{"team.manage", "team.users.read", "task.reassign.team"},
		},
		{
			Role:         RoleDesignDirector,
			Name:         "Design Director",
			Description:  "Legacy compatibility role for historical design review coordination.",
			Capabilities: []string{"design.department.manage", "design.review.read"},
		},
		{
			Role:         RoleDesignReviewer,
			Name:         "Design Reviewer",
			Description:  "Legacy compatibility role for historical design review flows.",
			Capabilities: []string{"design.review", "design.audit.view"},
		},
		{
			Role:         RoleMember,
			Name:         "Member",
			Description:  "Minimal authenticated member role with self-only default access.",
			Capabilities: []string{"profile.view"},
		},
		{
			Role:         RoleOps,
			Name:         "Ops",
			Description:  "Create tasks, maintain business info, and drive the mainline workflow.",
			Capabilities: []string{"task.create", "task.business_info", "warehouse.prepare", "task.close"},
		},
		{
			Role:         RoleDesigner,
			Name:         "Designer",
			Description:  "Handle design upload and design submission for design-required tasks.",
			Capabilities: []string{"task.design_submit", "task.asset_upload"},
		},
		{
			Role:         RoleCustomizationOperator,
			Name:         "Customization Operator",
			Description:  "Handle customization-only production uploads, effect routing, and production transfer without reusing the normal design lane as the authority.",
			Capabilities: []string{"task.customization.submit", "task.customization.transfer", "task.asset_upload"},
		},
		{
			Role:         RoleAuditA,
			Name:         "Normal Audit (Lane A)",
			Description:  "Technical compatibility role used to implement the product grouping 'Normal Audit' in the primary audit lane.",
			Capabilities: []string{"task.audit.claim", "task.audit.review"},
		},
		{
			Role:         RoleAuditB,
			Name:         "Normal Audit (Lane B)",
			Description:  "Technical compatibility role used to implement the product grouping 'Normal Audit' for takeover and secondary audit handling.",
			Capabilities: []string{"task.audit.claim", "task.audit.review", "task.audit.takeover"},
		},
		{
			Role:         RoleWarehouse,
			Name:         "Warehouse",
			Description:  "Receive, reject, and complete warehouse flow for eligible tasks.",
			Capabilities: []string{"warehouse.receive", "warehouse.reject", "warehouse.complete"},
		},
		{
			Role:         RoleOutsource,
			Name:         "Outsource",
			Description:  "Handle outsource task creation and follow-up.",
			Capabilities: []string{"outsource.manage"},
		},
		{
			Role:         RoleCustomizationReviewer,
			Name:         "Customization Audit",
			Description:  "Technical compatibility role used to implement the product grouping 'Customization Audit' for review, revision, replacement, and return-upstream handling.",
			Capabilities: []string{"task.customization.review", "task.customization.effect_review"},
		},
		{
			Role:         RoleERP,
			Name:         "ERP",
			Description:  "Run internal ERP sync and ERP-facing placeholder routes.",
			Capabilities: []string{"erp.sync"},
		},
	}
}
