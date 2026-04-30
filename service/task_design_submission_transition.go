package service

import "workflow/domain"

type designSubmissionTransition struct {
	TaskStatus     domain.TaskStatus
	ModuleKey      string
	ModuleState    domain.ModuleState
	ModuleTerminal bool
}

func designSubmissionTransitionForTask(task *domain.Task) designSubmissionTransition {
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask {
		return designSubmissionTransition{
			TaskStatus:     domain.TaskStatusCompleted,
			ModuleKey:      domain.ModuleKeyRetouch,
			ModuleState:    domain.ModuleStateCompleted,
			ModuleTerminal: true,
		}
	}
	return designSubmissionTransition{
		TaskStatus:     domain.TaskStatusPendingAuditA,
		ModuleKey:      domain.ModuleKeyDesign,
		ModuleState:    domain.ModuleStateSubmitted,
		ModuleTerminal: false,
	}
}

func designAssetSourceModuleKeyForTask(task *domain.Task, assetType domain.TaskAssetType) string {
	assetType = domain.NormalizeTaskAssetType(assetType)
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask &&
		(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
		return domain.ModuleKeyRetouch
	}
	return domain.ModuleKeyDesign
}
