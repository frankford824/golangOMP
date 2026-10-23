package assetworkbench

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestExplicitWorkbenchAssignmentsReplaceStaleLegacyRoles(t *testing.T) {
	actor := domain.RequestActor{ID: 332, Roles: []domain.Role{domain.RoleAssetManager}, EffectiveAccess: &domain.EffectiveAccess{
		Permissions: []domain.PermissionCode{domain.PermissionAssetWorkbenchUse, domain.PermissionAssetWorkbenchSettlement},
		Assignments: []domain.AccessAssignment{{RoleCode: "asset_settlement"}},
	}}
	if actorHasAny(actor, domain.RoleAssetManager) {
		t.Fatal("revoked legacy manager still authorizes")
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement) {
		t.Fatal("explicit settlement grant is not recognized")
	}
	capabilities := assetWorkbenchCapabilities(actor)
	if containsString(capabilities, "asset.workbench.manage") || containsString(capabilities, "asset.workbench.profile.manage") {
		t.Fatalf("unexpected capability: %v", capabilities)
	}
	if !containsString(capabilities, "asset.workbench.settlement") {
		t.Fatal("settlement UI did not follow explicit grant")
	}
	if got := assetRolesFromActor(actor); len(got) != 1 || got[0] != domain.RoleAssetSettlement {
		t.Fatalf("display roles = %v", got)
	}
}

func TestActiveMembershipCannotBypassRevokedUsePermission(t *testing.T) {
	svc := NewService(Config{}, WithRepository(&bootstrapAccessRepo{}, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 303, Roles: []domain.Role{domain.RoleAssetManager}, EffectiveAccess: &domain.EffectiveAccess{}}
	access, appErr := svc.ResolveAssetWorkbenchAccess(context.Background(), actor)
	if appErr != nil || access.IsEnabled || access.DeniedReason == "" {
		t.Fatalf("access=%+v err=%v", access, appErr)
	}
}

func TestMemberRoleWriteRequiresSharedRevision(t *testing.T) {
	svc := NewService(Config{}, WithRepository(&memberRoleUpdateRepo{}, assetWorkbenchTestTxRunner{}), WithUserRepository(&memberRoleUpdateUserRepo{}))
	_, appErr := svc.UpdateMemberRoles(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleSuperAdmin}}, 302, UpdateMemberRolesParams{Roles: []domain.Role{domain.RoleAssetManager}})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error=%v", appErr)
	}
}

func TestInheritedMemberRolesAreReadOnlyInWorkbench(t *testing.T) {
	member := &domain.AssetWorkbenchMember{Status: domain.AppMembershipStatusActive, RolesManagedExternally: true}
	decorateMemberForActor(member, domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleSuperAdmin}})
	if member.CanEditRoles {
		t.Fatal("inherited policy offered as direct role editor")
	}
}
