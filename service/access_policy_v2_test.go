package service

import (
	"context"
	"testing"

	"workflow/domain"
)

type accessPolicySecurityRepo struct {
	AccessPolicyRepository
	effective   map[int64]*domain.EffectiveAccess
	roles       map[int64]*domain.AccessRole
	permissions []domain.AccessPermission
}

func (r *accessPolicySecurityRepo) EffectiveAccess(_ context.Context, userID int64) (*domain.EffectiveAccess, error) {
	if item := r.effective[userID]; item != nil {
		copy := *item
		return &copy, nil
	}
	return &domain.EffectiveAccess{UserID: userID, Permissions: []domain.PermissionCode{}, Assignments: []domain.AccessAssignment{}, Sources: []domain.EffectiveAccessNote{}}, nil
}

func (r *accessPolicySecurityRepo) GetRole(_ context.Context, id int64) (*domain.AccessRole, error) {
	if role := r.roles[id]; role != nil {
		copy := *role
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}

func (r *accessPolicySecurityRepo) ListPermissions(context.Context) ([]domain.AccessPermission, error) {
	return append([]domain.AccessPermission(nil), r.permissions...), nil
}

func TestAccessPolicyLegacyAdminRoleDoesNotBypassManageCapability(t *testing.T) {
	repository := &accessPolicySecurityRepo{effective: map[int64]*domain.EffectiveAccess{}, roles: map[int64]*domain.AccessRole{}}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin}}, AccessRoleCreateRequest{
		Code: "reviewer", Name: "审核员", Reason: "test", ExpectedPolicyRevision: 1,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateRole appErr = %#v, want permission denied", appErr)
	}
}

func TestAccessPolicyManageRequiredToGrantOperations(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionERPManage},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "erp_operator", ScopeMode: domain.AccessScopeGlobal}},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionERPManage, RoleID: 2, RoleCode: "erp_operator", ScopeMode: domain.AccessScopeGlobal},
		},
	}
	repository := &accessPolicySecurityRepo{
		effective:   map[int64]*domain.EffectiveAccess{7: effective},
		roles:       map[int64]*domain.AccessRole{2: {ID: 2, Code: "erp_operator"}},
		permissions: []domain.AccessPermission{{Code: domain.PermissionERPManage, RiskLevel: "high", Enabled: true}},
	}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin}, EffectiveAccess: effective}, AccessRoleCreateRequest{
		Code: "erp_operator_2", Name: "ERP 管理", Permissions: []domain.AccessRolePermission{{Code: domain.PermissionERPManage}}, Reason: "test", ExpectedPolicyRevision: 1,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateRole appErr = %#v, want access.manage required", appErr)
	}
}

func TestAccessPolicyManagerCannotGrantHighRiskPermission(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessManage, domain.PermissionERPManage},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionAccessManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal},
			{Permission: domain.PermissionERPManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal},
		},
	}
	repository := &accessPolicySecurityRepo{
		effective:   map[int64]*domain.EffectiveAccess{7: effective},
		roles:       map[int64]*domain.AccessRole{2: {ID: 2, Code: "access_admin"}},
		permissions: []domain.AccessPermission{{Code: domain.PermissionERPManage, RiskLevel: "high", Enabled: true}},
	}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, EffectiveAccess: effective}, AccessRoleCreateRequest{
		Code: "erp_operator_2", Name: "ERP 管理", Permissions: []domain.AccessRolePermission{{Code: domain.PermissionERPManage}}, Reason: "test", ExpectedPolicyRevision: 1,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateRole appErr = %#v, want high-risk grant denied", appErr)
	}
}

func TestAccessPolicyManagerCannotGrantPermissionItDoesNotHold(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessManage},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAccessManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
	}
	repository := &accessPolicySecurityRepo{
		effective:   map[int64]*domain.EffectiveAccess{7: effective},
		roles:       map[int64]*domain.AccessRole{2: {ID: 2, Code: "access_admin"}},
		permissions: []domain.AccessPermission{{Code: domain.PermissionAssetView, RiskLevel: "normal", Enabled: true}},
	}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, EffectiveAccess: effective}, AccessRoleCreateRequest{
		Code: "asset_viewer", Name: "资产查看", Permissions: []domain.AccessRolePermission{{Code: domain.PermissionAssetView}}, Reason: "test", ExpectedPolicyRevision: 1,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateRole appErr = %#v, want non-held permission denied", appErr)
	}
}

func TestAccessPolicyRejectsInvalidTaskTypeRestriction(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessManage, domain.PermissionTaskCreate},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAccessManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
	}
	svc := NewAccessPolicyService(&accessPolicySecurityRepo{effective: map[int64]*domain.EffectiveAccess{7: effective}}, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, EffectiveAccess: effective}, AccessRoleCreateRequest{
		Code: "creator", Name: "Creator", Reason: "test", ExpectedPolicyRevision: 1,
		Permissions: []domain.AccessRolePermission{{Code: domain.PermissionTaskCreate, TaskTypes: []string{"not_a_task_type"}}},
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CreateRole appErr = %#v, want invalid request", appErr)
	}
}

func TestAccessPolicyLegacyAdminCannotAssignSelfProtectedSuperAdmin(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessManage},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAccessManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
	}
	repository := &accessPolicySecurityRepo{
		effective: map[int64]*domain.EffectiveAccess{7: effective},
		roles: map[int64]*domain.AccessRole{
			1: {ID: 1, Code: "super_admin", SystemProtected: true, Permissions: []domain.AccessRolePermission{}},
			2: {ID: 2, Code: "access_admin"},
		},
	}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.ReplaceUserAssignments(context.Background(), domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin}, EffectiveAccess: effective}, 7, domain.ReplaceAccessAssignmentsRequest{
		ExpectedPolicyRevision: 1,
		Reason:                 "self escalation attempt",
		Assignments:            []domain.AccessAssignment{{RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("ReplaceUserAssignments appErr = %#v, want self-escalation denied", appErr)
	}
}
