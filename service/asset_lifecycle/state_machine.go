package asset_lifecycle

import (
	"workflow/domain"
	"workflow/repo"
)

func CanArchive(state domain.AssetLifecycleState) bool {
	return state == domain.AssetLifecycleStateActive || state == domain.AssetLifecycleStateClosedRetained
}

func CanRestore(state domain.AssetLifecycleState) bool {
	return state == domain.AssetLifecycleStateArchived
}

func CanDelete(state domain.AssetLifecycleState) bool {
	switch state {
	case domain.AssetLifecycleStateActive, domain.AssetLifecycleStateClosedRetained, domain.AssetLifecycleStateArchived:
		return true
	default:
		return false
	}
}

func isSuperAdmin(actor domain.RequestActor) bool {
	for _, role := range domain.NormalizeRoleValues(actor.Roles) {
		if role == domain.RoleSuperAdmin {
			return true
		}
	}
	return false
}

func canDeleteCompletedTaskAsset(actor domain.RequestActor, row *repo.TaskAssetSearchRow) bool {
	if isSuperAdmin(actor) {
		return true
	}
	if row == nil || row.Asset == nil || row.Task == nil || row.Task.TaskStatus != domain.TaskStatusCompleted {
		return false
	}
	assetType := domain.NormalizeTaskAssetType(row.Asset.AssetType)
	if assetType != domain.TaskAssetTypeReference && assetType != domain.TaskAssetTypeSource && assetType != domain.TaskAssetTypeDelivery {
		return false
	}
	for _, role := range domain.NormalizeRoleValues(actor.Roles) {
		switch role {
		case domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAssetManager:
			return true
		}
	}
	return false
}
