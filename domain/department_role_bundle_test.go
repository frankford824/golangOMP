package domain

import (
	"slices"
	"testing"
)

func TestDepartmentDefaultBusinessRoles(t *testing.T) {
	cases := []struct {
		name       string
		department Department
		want       []Role
	}{
		{
			name:       "operations gets ops bundle",
			department: DepartmentOperations,
			want:       []Role{RoleOps},
		},
		{
			name:       "design compatibility gets design bundle",
			department: DepartmentDesign,
			want:       []Role{RoleDesigner, RoleDesignReviewer},
		},
		{
			name:       "design rd gets official designer bundle",
			department: DepartmentDesignRD,
			want:       []Role{RoleDesigner},
		},
		{
			name:       "customization art gets customization bundle",
			department: DepartmentCustomizationArt,
			want:       []Role{RoleCustomizationOperator},
		},
		{
			name:       "audit gets audit and customization review bundle",
			department: DepartmentAudit,
			want:       []Role{RoleAuditA, RoleAuditB, RoleCustomizationReviewer},
		},
		{
			name:       "cloud warehouse gets warehouse bundle",
			department: DepartmentCloudWarehouse,
			want:       []Role{RoleWarehouse},
		},
		{
			name:       "procurement gets ops bundle",
			department: DepartmentProcurement,
			want:       []Role{RoleOps},
		},
		{
			name:       "warehouse gets warehouse bundle",
			department: DepartmentWarehouse,
			want:       []Role{RoleWarehouse},
		},
		{
			name:       "bakery warehouse gets warehouse bundle",
			department: DepartmentBakeryWH,
			want:       []Role{RoleWarehouse},
		},
		{
			name:       "hr has no default business roles",
			department: DepartmentHR,
			want:       []Role{},
		},
		{
			name:       "unassigned has no default business roles",
			department: DepartmentUnassigned,
			want:       []Role{},
		},
		{
			name:       "unknown department has no default business roles",
			department: Department("未知部门"),
			want:       []Role{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := DepartmentDefaultBusinessRoles(tc.department)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("DepartmentDefaultBusinessRoles(%q) mismatch, got=%v want=%v", tc.department, got, tc.want)
			}
		})
	}
}
