package asset_lifecycle

import (
	"testing"

	"workflow/domain"
	"workflow/repo"
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

func TestCompletedTaskAssetDeleteRoles(t *testing.T) {
	asset := &domain.TaskAsset{AssetType: domain.TaskAssetTypeSource}
	task := &domain.Task{TaskStatus: domain.TaskStatusCompleted}
	row := &repo.TaskAssetSearchRow{Asset: asset, Task: task}

	for _, role := range []domain.Role{
		domain.RoleCustomizationReviewer,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleAssetManager,
		domain.RoleSuperAdmin,
	} {
		if !canDeleteCompletedTaskAsset(domain.RequestActor{Roles: []domain.Role{role}}, row) {
			t.Fatalf("role %s should delete a completed current asset", role)
		}
	}

	task.TaskStatus = domain.TaskStatusPendingAuditA
	if canDeleteCompletedTaskAsset(domain.RequestActor{Roles: []domain.Role{domain.RoleAuditA}}, row) {
		t.Fatalf("audit role must not delete an active-task asset")
	}
	if !canDeleteCompletedTaskAsset(domain.RequestActor{Roles: []domain.Role{domain.RoleSuperAdmin}}, row) {
		t.Fatalf("SuperAdmin should retain lifecycle delete authority")
	}

	task.TaskStatus = domain.TaskStatusCompleted
	asset.AssetType = domain.TaskAssetTypePreview
	if canDeleteCompletedTaskAsset(domain.RequestActor{Roles: []domain.Role{domain.RoleCustomizationReviewer}}, row) {
		t.Fatalf("review role must not delete derived preview assets")
	}
	if canDeleteCompletedTaskAsset(domain.RequestActor{Roles: []domain.Role{domain.RoleDesigner}}, row) {
		t.Fatalf("designer role must not receive completed asset deletion authority")
	}
}
