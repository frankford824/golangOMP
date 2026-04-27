package asset_lifecycle

import (
	"testing"

	"workflow/domain"
)

func TestStateMachineGuards(t *testing.T) {
	tests := []struct {
		state      domain.AssetLifecycleState
		canArchive bool
		canRestore bool
		canDelete  bool
	}{
		{domain.AssetLifecycleStateActive, true, false, true},
		{domain.AssetLifecycleStateClosedRetained, true, false, true},
		{domain.AssetLifecycleStateArchived, false, true, true},
		{domain.AssetLifecycleStateAutoCleaned, false, false, false},
		{domain.AssetLifecycleStateDeleted, false, false, false},
	}
	for _, tt := range tests {
		if got := CanArchive(tt.state); got != tt.canArchive {
			t.Fatalf("CanArchive(%s) = %t, want %t", tt.state, got, tt.canArchive)
		}
		if got := CanRestore(tt.state); got != tt.canRestore {
			t.Fatalf("CanRestore(%s) = %t, want %t", tt.state, got, tt.canRestore)
		}
		if got := CanDelete(tt.state); got != tt.canDelete {
			t.Fatalf("CanDelete(%s) = %t, want %t", tt.state, got, tt.canDelete)
		}
	}
}

func TestSuperAdminExactRole(t *testing.T) {
	if !isSuperAdmin(domain.RequestActor{Roles: []domain.Role{domain.RoleSuperAdmin}}) {
		t.Fatalf("SuperAdmin role not accepted")
	}
	if isSuperAdmin(domain.RequestActor{Roles: []domain.Role{domain.RoleAdmin}}) {
		t.Fatalf("Admin must not satisfy SA-A exact SuperAdmin gate")
	}
}
