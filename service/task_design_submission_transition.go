package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

type designSubmissionTransition struct {
	TaskStatus     domain.TaskStatus
	ModuleKey      string
	ModuleState    domain.ModuleState
	ModuleTerminal bool
}

type designSubmissionWorkflowEngine interface {
	ApplyAfterAction(ctx context.Context, tx repo.Tx, task *domain.Task, moduleKey, action string, actorID *int64, actionEventID int64) error
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

func applyDesignSubmissionWorkflow(ctx context.Context, tx repo.Tx, rules designSubmissionWorkflowEngine, task *domain.Task, transition designSubmissionTransition, actorID int64) error {
	if rules == nil || task == nil || transition.TaskStatus != domain.TaskStatusPendingAuditA {
		return nil
	}
	return rules.ApplyAfterAction(ctx, tx, task, transition.ModuleKey, domain.ModuleActionSubmit, &actorID, 0)
}
