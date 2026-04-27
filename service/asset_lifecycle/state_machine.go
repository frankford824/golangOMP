package asset_lifecycle

import "workflow/domain"

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
