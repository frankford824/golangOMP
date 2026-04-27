package domain

type AssetLifecycleState string

const (
	AssetLifecycleStateActive         AssetLifecycleState = "active"
	AssetLifecycleStateClosedRetained AssetLifecycleState = "closed_retained"
	AssetLifecycleStateArchived       AssetLifecycleState = "archived"
	AssetLifecycleStateAutoCleaned    AssetLifecycleState = "auto_cleaned"
	AssetLifecycleStateDeleted        AssetLifecycleState = "deleted"
)

func DeriveLifecycleState(asset TaskAsset, task Task) AssetLifecycleState {
	if asset.DeletedAt != nil {
		return AssetLifecycleStateDeleted
	}
	if asset.CleanedAt != nil {
		return AssetLifecycleStateAutoCleaned
	}
	if asset.IsArchived {
		return AssetLifecycleStateArchived
	}
	if TaskAssetLifecycleTaskIsTerminal(task) {
		return AssetLifecycleStateClosedRetained
	}
	return AssetLifecycleStateActive
}

func TaskAssetLifecycleTaskIsTerminal(task Task) bool {
	switch task.TaskStatus {
	case TaskStatusCompleted, TaskStatusCancelled, TaskStatusArchived:
		return true
	default:
		return false
	}
}
