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
	if task != nil && task.CustomizationRequired {
		return designSubmissionTransition{
			TaskStatus:     domain.TaskStatusPendingCustomizationReview,
			ModuleKey:      domain.ModuleKeyCustomization,
			ModuleState:    domain.ModuleStateSubmitted,
			ModuleTerminal: false,
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
	if task != nil && task.CustomizationRequired &&
		(assetType.IsSource() || assetType.IsDelivery() || assetType.IsPreview() || assetType.IsDesignThumb()) {
		return domain.ModuleKeyCustomization
	}
	return domain.ModuleKeyDesign
}

func applyDesignSubmissionWorkflow(ctx context.Context, tx repo.Tx, rules designSubmissionWorkflowEngine, task *domain.Task, transition designSubmissionTransition, actorID int64) error {
	if rules == nil || task == nil || transition.ModuleKey == "" {
		return nil
	}
	switch transition.TaskStatus {
	case domain.TaskStatusPendingAuditA:
		// design.submit -> audit (audit_standard pool)
	case domain.TaskStatusPendingCustomizationReview:
		// customization.submit -> audit (audit_customization pool)
	default:
		return nil
	}
	return rules.ApplyAfterAction(ctx, tx, task, transition.ModuleKey, domain.ModuleActionSubmit, &actorID, 0)
}

func syncCustomizationDesignSubmission(
	ctx context.Context,
	tx repo.Tx,
	taskRepo repo.TaskRepo,
	customizationJobRepo repo.CustomizationJobRepo,
	task *domain.Task,
	operatorID int64,
) error {
	if task == nil || !task.CustomizationRequired {
		return nil
	}
	lastOperatorID := operatorID
	if err := taskRepo.UpdateCustomizationState(ctx, tx, task.ID, &lastOperatorID, "", ""); err != nil {
		return err
	}
	if customizationJobRepo == nil {
		return nil
	}
	job, err := customizationJobRepo.GetLatestByTaskID(ctx, task.ID)
	if err != nil {
		return err
	}
	if job == nil {
		job = &domain.CustomizationJob{
			TaskID:             task.ID,
			DecisionType:       domain.CustomizationJobDecisionTypeFinal,
			AssignedOperatorID: &lastOperatorID,
			LastOperatorID:     &lastOperatorID,
			Status:             domain.CustomizationJobStatusPendingCustomizationReview,
		}
		_, err = customizationJobRepo.Create(ctx, tx, job)
		return err
	}
	if job.AssignedOperatorID == nil {
		job.AssignedOperatorID = &lastOperatorID
	}
	job.LastOperatorID = &lastOperatorID
	job.DecisionType = domain.CustomizationJobDecisionTypeFinal
	job.Status = domain.CustomizationJobStatusPendingCustomizationReview
	job.WarehouseRejectReason = ""
	job.WarehouseRejectCategory = ""
	return customizationJobRepo.Update(ctx, tx, job)
}

func submittedCustomizationOperatorID(task *domain.Task, actorID int64) *int64 {
	if task == nil || !task.CustomizationRequired {
		return nil
	}
	return &actorID
}
