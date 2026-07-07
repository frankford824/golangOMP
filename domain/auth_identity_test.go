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
		RoleAuditA:                "常规审核",
		RoleAuditB:                "历史兼容：常规审核旧编码",
		RoleCustomizationReviewer: "定制审核",
		RoleWarehouse:             "仓库人员",
		RoleAssetSubmitter:        "素材工作台-交付人员",
		RoleAssetManager:          "素材工作台-作品管理",
		RoleAssetTemplateAdmin:    "素材工作台-计价配置",
		RoleAssetSettlement:       "素材工作台-结算财务",
	}

	for role, want := range tests {
		if got := names[role]; got != want {
			t.Fatalf("DefaultRoleCatalog()[%s].Name = %q, want %q", role, got, want)
		}
	}
}

func TestDefaultRoleCatalogMarksAuditBCompatibilityOnly(t *testing.T) {
	catalog := DefaultRoleCatalog()
	for _, entry := range catalog {
		if entry.Role != RoleAuditB {
			continue
		}
		if entry.Assignable {
			t.Fatal("Audit_B assignable = true, want false")
		}
		if !entry.Deprecated || !entry.HiddenByDefault || entry.Category != "compatibility" {
			t.Fatalf("Audit_B compatibility flags = category:%q deprecated:%v hidden:%v", entry.Category, entry.Deprecated, entry.HiddenByDefault)
		}
		return
	}
	t.Fatal("Audit_B role catalog entry not found")
}
