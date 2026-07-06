package domain

import "testing"

func TestDefaultRoleCatalogUsesBusinessDisplayNames(t *testing.T) {
	catalog := DefaultRoleCatalog()
	names := make(map[Role]string, len(catalog))
	for _, entry := range catalog {
		names[entry.Role] = entry.Name
	}

	tests := map[Role]string{
		RoleSuperAdmin:            "超级管理员",
		RoleHRAdmin:               "人事管理员",
		RoleDeptAdmin:             "部门管理员",
		RoleTeamLead:              "组长",
		RoleAuditA:                "普通审核A",
		RoleAuditB:                "普通审核B",
		RoleCustomizationReviewer: "定制审核",
		RoleAssetSubmitter:        "交付人员",
		RoleAssetManager:          "作品管理",
		RoleAssetTemplateAdmin:    "计价配置",
		RoleAssetSettlement:       "结算财务",
	}

	for role, want := range tests {
		if got := names[role]; got != want {
			t.Fatalf("DefaultRoleCatalog()[%s].Name = %q, want %q", role, got, want)
		}
	}
}
