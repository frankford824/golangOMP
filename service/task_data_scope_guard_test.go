package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestCanViewTaskWithScopeStageVisibilityHonorsLane(t *testing.T) {
	scope, err := NewRoleBasedDataScopeResolver().Resolve(context.Background(), &domain.User{
		ID:    101,
		Roles: []domain.Role{domain.RoleAuditA},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	normalLaneTask := &domain.Task{
		ID:                    1,
		CreatorID:             9001,
		OwnerDepartment:       string(domain.DepartmentOperations),
		TaskStatus:            domain.TaskStatusPendingAuditA,
		CustomizationRequired: false,
	}
	if !canViewTaskWithScope(normalLaneTask, scope) {
		t.Fatal("canViewTaskWithScope() = false, want true for normal-lane PendingAuditA")
	}

	customizationLaneTask := &domain.Task{
		ID:                    2,
		CreatorID:             9002,
		OwnerDepartment:       string(domain.DepartmentOperations),
		TaskStatus:            domain.TaskStatusPendingAuditA,
		CustomizationRequired: true,
	}
	if canViewTaskWithScope(customizationLaneTask, scope) {
		t.Fatal("canViewTaskWithScope() = true, want false for customization-lane PendingAuditA")
	}
}

func TestCanViewTaskWithScope_ManagedDepartment_ByDesigner(t *testing.T) {
	task := &domain.Task{
		ID:              1,
		CreatorID:       100,
		OwnerDepartment: string(domain.DepartmentOperations),
		DesignerID:      authzInt64Ptr(200),
	}
	scope := &DataScope{
		ManagedDepartmentCodes: []string{string(domain.DepartmentDesignRD)},
	}
	actorOrg := &taskActorOrgSnapshot{
		Departments: []string{string(domain.DepartmentDesignRD)},
	}

	if !canViewTaskWithScopeAndActorOrg(task, scope, actorOrg) {
		t.Fatal("canViewTaskWithScopeAndActorOrg() = false, want true for managed designer department match")
	}
}

func TestCanViewTaskWithScope_ManagedDepartment_ByCreator(t *testing.T) {
	task := &domain.Task{
		ID:              2,
		CreatorID:       300,
		OwnerDepartment: string(domain.DepartmentOperations),
	}
	scope := &DataScope{
		ManagedDepartmentCodes: []string{string(domain.DepartmentDesignRD)},
	}
	actorOrg := &taskActorOrgSnapshot{
		Departments: []string{string(domain.DepartmentDesignRD)},
	}

	if !canViewTaskWithScopeAndActorOrg(task, scope, actorOrg) {
		t.Fatal("canViewTaskWithScopeAndActorOrg() = false, want true for managed creator department match")
	}
}

func TestCanViewTaskWithScope_ManagedDepartment_NoMatch(t *testing.T) {
	task := &domain.Task{
		ID:               3,
		CreatorID:        400,
		OwnerDepartment:  string(domain.DepartmentOperations),
		DesignerID:       authzInt64Ptr(401),
		CurrentHandlerID: authzInt64Ptr(402),
	}
	scope := &DataScope{
		ManagedDepartmentCodes: []string{string(domain.DepartmentDesignRD)},
	}
	actorOrg := &taskActorOrgSnapshot{
		Departments: []string{string(domain.DepartmentAudit)},
	}

	if canViewTaskWithScopeAndActorOrg(task, scope, actorOrg) {
		t.Fatal("canViewTaskWithScopeAndActorOrg() = true, want false when all actor departments are outside managed scope")
	}
}

func TestCanViewTaskWithScope_ManagedScope_Empty_PreservesOldBehavior(t *testing.T) {
	task := &domain.Task{
		ID:                    4,
		CreatorID:             500,
		OwnerDepartment:       string(domain.DepartmentOperations),
		TaskStatus:            domain.TaskStatusPendingAuditA,
		CustomizationRequired: false,
	}
	scope, err := NewRoleBasedDataScopeResolver().Resolve(context.Background(), &domain.User{
		ID:    501,
		Roles: []domain.Role{domain.RoleAuditA},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	oldResult := canViewTaskWithScope(task, scope)
	newResult := canViewTaskWithScopeAndActorOrg(task, scope, &taskActorOrgSnapshot{
		Departments: []string{string(domain.DepartmentDesignRD)},
		Teams:       []string{"A组"},
	})
	if newResult != oldResult {
		t.Fatalf("canViewTaskWithScopeAndActorOrg() = %v, want %v when managed scope is empty", newResult, oldResult)
	}
}
