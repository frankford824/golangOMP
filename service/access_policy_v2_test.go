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

func TestAccessPolicyLegacyAdminWithExplicitManageCannotGrantHighRiskPermission(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessPolicyAdmin, domain.PermissionERPManage},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionAccessPolicyAdmin, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal},
			{Permission: domain.PermissionERPManage, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal},
		},
	}
	repository := &accessPolicySecurityRepo{
		effective:   map[int64]*domain.EffectiveAccess{7: effective},
		roles:       map[int64]*domain.AccessRole{2: {ID: 2, Code: "access_admin"}},
		permissions: []domain.AccessPermission{{Code: domain.PermissionERPManage, RiskLevel: "high", Enabled: true}},
	}
	svc := NewAccessPolicyService(repository, nil, nil)
	_, appErr := svc.CreateRole(context.Background(), domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin}, EffectiveAccess: effective}, AccessRoleCreateRequest{
		Code: "erp_operator", Name: "ERP 管理", Permissions: []domain.PermissionCode{domain.PermissionERPManage}, Reason: "test", ExpectedPolicyRevision: 1,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateRole appErr = %#v, want high-risk permission denied", appErr)
	}
}

func TestAccessPolicyLegacyAdminCannotAssignSelfProtectedSuperAdmin(t *testing.T) {
	effective := &domain.EffectiveAccess{
		UserID:      7,
		Permissions: []domain.PermissionCode{domain.PermissionAccessPolicyAdmin},
		Assignments: []domain.AccessAssignment{{RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionAccessPolicyAdmin, RoleID: 2, RoleCode: "access_admin", ScopeMode: domain.AccessScopeGlobal}},
	}
	repository := &accessPolicySecurityRepo{
		effective: map[int64]*domain.EffectiveAccess{7: effective},
		roles: map[int64]*domain.AccessRole{
			1: {ID: 1, Code: "super_admin", SystemProtected: true, Permissions: []domain.PermissionCode{}},
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
