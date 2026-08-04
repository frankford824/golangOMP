package domain

// Role for RBAC (spec §3.1).
type Role string

// --- Official product roles (v1.0) ---
const (
	RoleMember                Role = "Member"
	RoleSuperAdmin            Role = "SuperAdmin"
	RoleHRAdmin               Role = "HRAdmin"
	RoleDeptAdmin             Role = "DepartmentAdmin"
	RoleTeamLead              Role = "TeamLead"
	RoleOps                   Role = "Ops"
	RoleDesigner              Role = "Designer"
	RoleCustomizationOperator Role = "CustomizationOperator"
	RoleAuditA                Role = "Audit_A"
	RoleAuditB                Role = "Audit_B"
	RoleCustomizationReviewer Role = "CustomizationReviewer"
	RoleWarehouse             Role = "Warehouse"
	RoleAssetSubmitter        Role = "AssetSubmitter"
	RoleAssetManager          Role = "AssetManager"
	RoleAssetTemplateAdmin    Role = "AssetTemplateAdmin"
	RoleAssetSettlement       Role = "AssetSettlement"
)

// --- Compatibility-only roles: do NOT use in new logic ---
const (
	RoleAdmin          Role = "Admin"          // compatibility: treated as SuperAdmin equivalent
	RoleOrgAdmin       Role = "OrgAdmin"       // compatibility: limited org-scope management
	RoleRoleAdmin      Role = "RoleAdmin"      // compatibility: role assignment only
	RoleDesignDirector Role = "DesignDirector" // compatibility: design department scope
	RoleDesignReviewer Role = "DesignReviewer" // compatibility: design review scope
	RoleOutsource      Role = "Outsource"      // compatibility: outsource workflow
	RoleERP            Role = "ERP"            // compatibility: ERP integration
)
