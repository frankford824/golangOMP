package service

import (
	"testing"

	"workflow/domain"
)

func TestValidConfiguredTaskDepartmentUsesConfiguredCatalog(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{
		DepartmentTeams: map[string][]string{
			string(domain.DepartmentOperations): {"\u6dd8\u7cfb\u4e00\u7ec4"},
		},
	})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	if !validConfiguredTaskDepartment(string(domain.DepartmentOperations)) {
		t.Fatalf("validConfiguredTaskDepartment(%q) = false, want true", domain.DepartmentOperations)
	}
}
