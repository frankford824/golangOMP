package domain

// OfficialProductRoles returns the v1.0 product-facing role set.
// New authorization logic must only reference these roles.
func OfficialProductRoles() []Role {
	return []Role{
		RoleMember,
		RoleSuperAdmin,
		RoleHRAdmin,
		RoleDeptAdmin,
		RoleTeamLead,
		RoleOps,
		RoleDesigner,
		RoleCustomizationOperator,
		RoleAuditA,
		RoleAuditB,
		RoleCustomizationReviewer,
		RoleWarehouse,
	}
}

// CompatibilityRoles returns roles preserved only for backward compatibility.
// These must not be used in new authorization paths.
func CompatibilityRoles() []Role {
	return []Role{
		RoleAdmin,
		RoleOrgAdmin,
		RoleRoleAdmin,
		RoleDesignDirector,
		RoleDesignReviewer,
		RoleOutsource,
		RoleERP,
	}
}

// IsOfficialProductRole returns true for v1.0 product-facing roles.
func IsOfficialProductRole(r Role) bool {
	switch r {
	case RoleMember, RoleSuperAdmin, RoleHRAdmin, RoleDeptAdmin, RoleTeamLead,
		RoleOps, RoleDesigner, RoleCustomizationOperator,
		RoleAuditA, RoleAuditB, RoleCustomizationReviewer, RoleWarehouse:
		return true
	}
	return false
}

// IsCompatibilityRole returns true for roles kept only for migration safety.
func IsCompatibilityRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleOrgAdmin, RoleRoleAdmin, RoleDesignDirector,
		RoleDesignReviewer, RoleOutsource, RoleERP:
		return true
	}
	return false
}

// IsNormalAuditRole returns true for normal (design) audit stage roles.
func IsNormalAuditRole(r Role) bool {
	return r == RoleAuditA || r == RoleAuditB
}

// IsCustomizationAuditRole returns true for the customization review role.
func IsCustomizationAuditRole(r Role) bool {
	return r == RoleCustomizationReviewer
}

// IsAnyAuditRole returns true for any audit-related role (normal or customization).
func IsAnyAuditRole(r Role) bool {
	return IsNormalAuditRole(r) || IsCustomizationAuditRole(r)
}

// IsCompanyLevelAdmin returns true for roles with company-wide management authority.
func IsCompanyLevelAdmin(r Role) bool {
	return r == RoleSuperAdmin || r == RoleHRAdmin || r == RoleAdmin
}
