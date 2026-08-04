package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestEnsureAdminRoleSafetyProtectsLastSuperAdmin(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[1] = &domain.User{ID: 1, Username: "super", Status: domain.UserStatusActive}
	userRepo.roles[1] = []domain.Role{domain.RoleSuperAdmin}
	svc := &identityService{userRepo: userRepo}

	appErr := svc.ensureAdminRoleSafety(context.Background(), []domain.Role{domain.RoleSuperAdmin}, []domain.Role{domain.RoleMember})
	if appErr == nil {
		t.Fatal("ensureAdminRoleSafety() appErr = nil, want deny")
	}
	if denyCode := appErrorDenyCode(appErr); denyCode != "last_super_admin_removal_denied" {
		t.Fatalf("deny_code = %q, want last_super_admin_removal_denied", denyCode)
	}
}
