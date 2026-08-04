package asset_lifecycle

import "workflow/domain"

func CanDelete(state domain.AssetLifecycleState) bool {
	switch state {
	case domain.AssetLifecycleStateActive, domain.AssetLifecycleStateClosedRetained, domain.AssetLifecycleStateArchived:
		return true
	default:
		return false
	}
}
