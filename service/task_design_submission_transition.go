package service

import "workflow/domain"

func designAssetSourceModuleKeyForTask(task *domain.Task, assetType domain.TaskAssetType) string {
	assetType = domain.NormalizeTaskAssetType(assetType)
	if assetType.IsReference() {
		return domain.ModuleKeyBasicInfo
	}
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask &&
		(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
		return domain.ModuleKeyRetouch
	}
	if task != nil && task.TaskStatus == domain.TaskStatusPendingAudit &&
		(assetType.IsSource() || assetType.IsDelivery()) {
		return domain.ModuleKeyAudit
	}
	if task != nil && task.CustomizationRequired &&
		(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
		return domain.ModuleKeyCustomization
	}
	return domain.ModuleKeyDesign
}

func submittedCustomizationOperatorID(task *domain.Task, actorID int64) *int64 {
	if task == nil || !task.CustomizationRequired {
		return nil
	}
	return &actorID
}
