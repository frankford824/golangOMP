package domain

import "testing"

func TestOrgDepartmentAliasesMatchCurrentAndHistoricalNames(t *testing.T) {
	if !OrgDepartmentsEquivalent("视觉研创部", string(DepartmentDesignRD)) {
		t.Fatal("视觉研创部 should match 设计研发部")
	}
	if !OrgDepartmentsEquivalent(string(DepartmentProcurement), string(DepartmentCloudWarehouse)) {
		t.Fatal("采购部 should match 云仓部")
	}
	if !OrgDepartmentsEquivalent("定制中心", string(DepartmentCustomizationArt)) {
		t.Fatal("定制中心 should match 定制美工部")
	}
	if OrgDepartmentsEquivalent(string(DepartmentAudit), string(DepartmentOperations)) {
		t.Fatal("审核部 should not match 运营部")
	}
}

func TestOrgTeamAliasesMatchRenamedOperationTeams(t *testing.T) {
	if !OrgTeamsEquivalent("淘系三组", "淘系运营三部") {
		t.Fatal("淘系三组 should match 淘系运营三部")
	}
	if !OrgTeamsEquivalent("拼多多南京组", "拼多多运营部（南京）") {
		t.Fatal("拼多多南京组 should match 拼多多运营部（南京）")
	}
	if !OrgTeamsEquivalent("拼多多池州组", "天猫运营一部（池州)") {
		t.Fatal("拼多多池州组 should match 天猫运营一部（池州)")
	}
	if OrgTeamsEquivalent("淘系二组", "淘系运营三部") {
		t.Fatal("淘系二组 should not match 淘系运营三部")
	}
}

func TestExpandOrgAliasesKeepsOriginalAndAddsKnownAliases(t *testing.T) {
	departments := ExpandOrgDepartmentAliases([]string{"视觉研创部"})
	if !containsOrgAliasValue(departments, "视觉研创部") || !containsOrgAliasValue(departments, string(DepartmentDesignRD)) {
		t.Fatalf("ExpandOrgDepartmentAliases() = %v, want visual and design-rd aliases", departments)
	}

	teams := ExpandOrgTeamAliases([]string{"淘系运营三部"})
	if !containsOrgAliasValue(teams, "淘系运营三部") || !containsOrgAliasValue(teams, "淘系三组") {
		t.Fatalf("ExpandOrgTeamAliases() = %v, want current and historical team aliases", teams)
	}
}

func containsOrgAliasValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
