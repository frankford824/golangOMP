package service

import (
	"context"
	"reflect"
	"testing"

	"workflow/domain"
)

func TestRoleBasedDataScopeResolverStageVisibilities(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	tests := []struct {
		name string
		user *domain.User
		want []stageVisibilitySnapshot
	}{
		{
			name: "audit_a gets normal lane audit a stages",
			user: &domain.User{
				ID:    1,
				Roles: []domain.Role{domain.RoleAuditA},
			},
			want: []stageVisibilitySnapshot{{
				Lane: domain.WorkflowLaneNormal,
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingAuditA,
					domain.TaskStatusRejectedByAuditA,
				},
			}},
		},
		{
			name: "audit_b gets normal lane audit b stages",
			user: &domain.User{
				ID:    2,
				Roles: []domain.Role{domain.RoleAuditB},
			},
			want: []stageVisibilitySnapshot{{
				Lane: domain.WorkflowLaneNormal,
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingAuditB,
					domain.TaskStatusRejectedByAuditB,
				},
			}},
		},
		{
			name: "warehouse gets unrestricted warehouse stages",
			user: &domain.User{
				ID:    3,
				Roles: []domain.Role{domain.RoleWarehouse},
			},
			want: []stageVisibilitySnapshot{{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingWarehouseQC,
					domain.TaskStatusPendingWarehouseReceive,
					domain.TaskStatusRejectedByWarehouse,
					domain.TaskStatusPendingProductionTransfer,
					domain.TaskStatusPendingClose,
				},
			}},
		},
		{
			name: "outsource gets unrestricted outsource stages",
			user: &domain.User{
				ID:    4,
				Roles: []domain.Role{domain.RoleOutsource},
			},
			want: []stageVisibilitySnapshot{{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingOutsource,
					domain.TaskStatusOutsourcing,
					domain.TaskStatusPendingOutsourceReview,
				},
			}},
		},
		{
			name: "customization operator gets customization lane stages",
			user: &domain.User{
				ID:    5,
				Roles: []domain.Role{domain.RoleCustomizationOperator},
			},
			want: []stageVisibilitySnapshot{{
				Lane: domain.WorkflowLaneCustomization,
				Statuses: []domain.TaskStatus{
					domain.TaskStatusInProgress,
					domain.TaskStatusPendingCustomizationProduction,
					domain.TaskStatusRejectedByAuditA,
					domain.TaskStatusRejectedByAuditB,
				},
			}},
		},
		{
			name: "customization reviewer gets customization review stages",
			user: &domain.User{
				ID:    6,
				Roles: []domain.Role{domain.RoleCustomizationReviewer},
			},
			want: []stageVisibilitySnapshot{{
				Lane: domain.WorkflowLaneCustomization,
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingCustomizationReview,
					domain.TaskStatusPendingEffectReview,
					domain.TaskStatusPendingEffectRevision,
				},
			}},
		},
		{
			name: "audit department admin gets audit and customization review union without duplicates",
			user: &domain.User{
				ID:         7,
				Department: domain.DepartmentAudit,
				Roles: []domain.Role{
					domain.RoleDeptAdmin,
					domain.RoleAuditA,
					domain.RoleAuditB,
					domain.RoleCustomizationReviewer,
				},
			},
			want: []stageVisibilitySnapshot{
				{
					Lane: domain.WorkflowLaneNormal,
					Statuses: []domain.TaskStatus{
						domain.TaskStatusPendingAuditA,
						domain.TaskStatusRejectedByAuditA,
						domain.TaskStatusPendingAuditB,
						domain.TaskStatusRejectedByAuditB,
					},
				},
				{
					Lane: domain.WorkflowLaneCustomization,
					Statuses: []domain.TaskStatus{
						domain.TaskStatusPendingCustomizationReview,
						domain.TaskStatusPendingEffectReview,
						domain.TaskStatusPendingEffectRevision,
					},
				},
			},
		},
		{
			name: "cloud warehouse department admin gets warehouse union",
			user: &domain.User{
				ID:         8,
				Department: domain.DepartmentCloudWarehouse,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
			},
			want: []stageVisibilitySnapshot{{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingWarehouseQC,
					domain.TaskStatusPendingWarehouseReceive,
					domain.TaskStatusRejectedByWarehouse,
					domain.TaskStatusPendingProductionTransfer,
					domain.TaskStatusPendingClose,
				},
			}},
		},
		{
			name: "customization art department admin gets operator and reviewer union on customization lane",
			user: &domain.User{
				ID:         9,
				Department: domain.DepartmentCustomizationArt,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
			},
			want: []stageVisibilitySnapshot{{
				Lane: domain.WorkflowLaneCustomization,
				Statuses: []domain.TaskStatus{
					domain.TaskStatusInProgress,
					domain.TaskStatusPendingCustomizationProduction,
					domain.TaskStatusRejectedByAuditA,
					domain.TaskStatusRejectedByAuditB,
					domain.TaskStatusPendingCustomizationReview,
					domain.TaskStatusPendingEffectReview,
					domain.TaskStatusPendingEffectRevision,
				},
			}},
		},
		{
			name: "operations department admin keeps owner department scope only",
			user: &domain.User{
				ID:         10,
				Department: domain.DepartmentOperations,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := resolver.Resolve(context.Background(), tc.user)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got := snapshotStageVisibilities(scope.StageVisibilities); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("StageVisibilities = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDataScope_DeptAdmin_EmptyManagedDepartments_FallsBackToUserDepartment(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	scope, err := resolver.Resolve(context.Background(), &domain.User{
		ID:           101,
		Department:   domain.DepartmentOperations,
		Roles:        []domain.Role{domain.RoleDeptAdmin},
		ManagedTeams: []string{},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !containsString(scope.DepartmentCodes, string(domain.DepartmentOperations)) {
		t.Fatalf("DepartmentCodes = %+v, want %q", scope.DepartmentCodes, domain.DepartmentOperations)
	}
	if len(scope.ManagedDepartmentCodes) != 0 {
		t.Fatalf("ManagedDepartmentCodes = %+v, want empty fallback-free scope", scope.ManagedDepartmentCodes)
	}
}

func TestDataScope_DeptAdmin_CarriesManagedDepartmentsAndTeams(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	scope, err := resolver.Resolve(context.Background(), &domain.User{
		ID:                 104,
		Department:         domain.DepartmentDesignRD,
		Team:               "默认组",
		Roles:              []domain.Role{domain.RoleDeptAdmin},
		ManagedDepartments: []string{string(domain.DepartmentDesignRD)},
		ManagedTeams:       []string{"默认组"},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !containsString(scope.DepartmentCodes, string(domain.DepartmentDesignRD)) {
		t.Fatalf("DepartmentCodes = %+v, want %q", scope.DepartmentCodes, domain.DepartmentDesignRD)
	}
	if !containsString(scope.ManagedDepartmentCodes, string(domain.DepartmentDesignRD)) {
		t.Fatalf("ManagedDepartmentCodes = %+v, want %q", scope.ManagedDepartmentCodes, domain.DepartmentDesignRD)
	}
	if !containsString(scope.ManagedTeamCodes, "默认组") {
		t.Fatalf("ManagedTeamCodes = %+v, want 默认组", scope.ManagedTeamCodes)
	}
}

func TestDataScope_DeptAdmin_EmptyManagedDepartments_EmptyDepartment_NoFallback(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	scope, err := resolver.Resolve(context.Background(), &domain.User{
		ID:    102,
		Roles: []domain.Role{domain.RoleDeptAdmin},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(scope.DepartmentCodes) != 0 {
		t.Fatalf("DepartmentCodes = %+v, want empty", scope.DepartmentCodes)
	}
}

func TestDataScope_Member_DoesNotBenefitFromFallback(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	scope, err := resolver.Resolve(context.Background(), &domain.User{
		ID:         103,
		Department: domain.DepartmentOperations,
		Roles:      []domain.Role{domain.RoleMember},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(scope.DepartmentCodes) != 0 {
		t.Fatalf("DepartmentCodes = %+v, want empty", scope.DepartmentCodes)
	}
}

type stageVisibilitySnapshot struct {
	Lane     domain.WorkflowLane
	Statuses []domain.TaskStatus
}

func snapshotStageVisibilities(visibilities []StageVisibility) []stageVisibilitySnapshot {
	if len(visibilities) == 0 {
		return nil
	}
	out := make([]stageVisibilitySnapshot, 0, len(visibilities))
	for _, visibility := range visibilities {
		snapshot := stageVisibilitySnapshot{
			Statuses: append([]domain.TaskStatus(nil), visibility.Statuses...),
		}
		if visibility.Lane != nil {
			snapshot.Lane = *visibility.Lane
		}
		out = append(out, snapshot)
	}
	return out
}
