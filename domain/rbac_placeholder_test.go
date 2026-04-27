package domain

import "testing"

func TestNormalizeRoleValuesIncludesCustomizationReviewer(t *testing.T) {
	roles := NormalizeRoleValues([]Role{RoleCustomizationReviewer, RoleMember})
	if len(roles) != 2 {
		t.Fatalf("NormalizeRoleValues() roles = %+v, want customization reviewer preserved", roles)
	}
	if roles[0] != RoleCustomizationReviewer {
		t.Fatalf("NormalizeRoleValues() first role = %s, want %s", roles[0], RoleCustomizationReviewer)
	}
}
