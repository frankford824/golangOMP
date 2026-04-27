package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func (s *taskService) SubmitCustomizationReview(ctx context.Context, p SubmitCustomizationReviewParams) (*domain.CustomizationJob, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "customization job repo is not configured", nil)
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for customization review", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionCustomizationReview, task); appErr != nil {
		return nil, appErr
	}
	if !p.Decision.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "customization_review_decision must be approved/return_to_designer/reviewer_fixed", nil)
	}

	levelCode := strings.TrimSpace(p.CustomizationLevelCode)
	levelName := strings.TrimSpace(p.CustomizationLevelName)
	if levelCode == "" && levelName != "" {
		levelCode = levelName
	}
	if p.Decision != domain.CustomizationReviewDecisionReturnToDesigner && levelCode == "" && levelName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "customization_level_code or customization_level_name is required for non-return decisions", nil)
	}

	currentJob, err := s.customizationJobRepo.GetLatestByTaskID(ctx, task.ID)
	if err != nil {
		return nil, infraError("get latest customization job for review", err)
	}
	if currentJob != nil && currentJob.Status != domain.CustomizationJobStatusPendingCustomizationReview {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "customization review requires pending_customization_review job status", map[string]interface{}{
			"task_id":     task.ID,
			"job_id":      currentJob.ID,
			"job_status":  currentJob.Status,
			"task_status": task.TaskStatus,
		})
	}

	if currentJob == nil {
		currentJob = &domain.CustomizationJob{
			TaskID:       task.ID,
			DecisionType: domain.CustomizationJobDecisionTypeFinal,
			Status:       domain.CustomizationJobStatusPendingCustomizationReview,
		}
	}
	currentJob.SourceAssetID = p.SourceAssetID
	previousAssetID := cloneInt64Ptr(currentJob.CurrentAssetID)
	currentJob.CurrentAssetID = p.SourceAssetID
	currentJob.CustomizationLevelCode = levelCode
	currentJob.CustomizationLevelName = levelName
	currentJob.ReviewReferenceUnitPrice = cloneFloat64Ptr(p.CustomizationPrice)
	currentJob.ReviewReferenceWeightFactor = cloneFloat64Ptr(p.CustomizationWeight)
	currentJob.Note = strings.TrimSpace(p.CustomizationNote)
	currentJob.ReviewDecision = p.Decision
	currentJob.DecisionType = domain.CustomizationJobDecisionTypeFinal

	nextStatus := domain.TaskStatusPendingCustomizationReview
	nextHandler := cloneInt64Ptr(task.DesignerID)
	nextJobStatus := domain.CustomizationJobStatusPendingCustomizationReview
	if p.Decision != domain.CustomizationReviewDecisionReturnToDesigner {
		nextStatus = domain.TaskStatusPendingCustomizationProduction
		nextHandler = nil
		nextJobStatus = domain.CustomizationJobStatusPendingCustomizationProduction
	}
	currentJob.Status = nextJobStatus

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if currentJob.ID == 0 {
			id, err := s.customizationJobRepo.Create(ctx, tx, currentJob)
			if err != nil {
				return err
			}
			currentJob.ID = id
		} else if err := s.customizationJobRepo.Update(ctx, tx, currentJob); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, nextStatus); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, task.ID, nextHandler); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, "task.customization.reviewed", &p.ReviewerID, mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
			"customization_review_decision":  p.Decision,
			"customization_level_code":       levelCode,
			"customization_level_name":       levelName,
			"review_reference_unit_price":    p.CustomizationPrice,
			"review_reference_weight_factor": p.CustomizationWeight,
			"customization_note":             p.CustomizationNote,
			"customization_job_id":           currentJob.ID,
			"previous_asset_id":              previousAssetID,
			"current_asset_id":               currentJob.CurrentAssetID,
			"replacement_actor_id":           p.ReviewerID,
			"from_task_status":               task.TaskStatus,
			"to_task_status":                 nextStatus,
			"to_job_status":                  nextJobStatus,
		}))
		return err
	})
	if txErr != nil {
		return nil, infraError("customization review tx", txErr)
	}
	return s.GetCustomizationJob(ctx, currentJob.ID)
}

func (s *taskService) SubmitCustomizationEffectPreview(ctx context.Context, p SubmitCustomizationEffectPreviewParams) (*domain.CustomizationJob, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "customization job repo is not configured", nil)
	}
	job, task, appErr := s.loadCustomizationJobAndTask(ctx, p.JobID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionCustomizationEffectPreview, task); appErr != nil {
		return nil, appErr
	}
	switch job.Status {
	case domain.CustomizationJobStatusPendingCustomizationProduction,
		domain.CustomizationJobStatusPendingEffectRevision,
		domain.CustomizationJobStatusRejectedByWarehouse:
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "customization job is not actionable for effect preview", map[string]interface{}{
			"job_id":     job.ID,
			"job_status": job.Status,
		})
	}
	decisionType := p.DecisionType
	if !decisionType.Valid() {
		decisionType = domain.CustomizationJobDecisionTypeEffectPreview
	}
	currentAssetID, appErr := resolveCustomizationAssetID(job.CurrentAssetID, p.CurrentAssetID)
	if appErr != nil {
		return nil, appErr
	}
	nextTaskStatus := domain.TaskStatusPendingEffectReview
	nextJobStatus := domain.CustomizationJobStatusPendingEffectReview
	nextHandler := (*int64)(nil)
	if decisionType == domain.CustomizationJobDecisionTypeFinal {
		nextTaskStatus = domain.TaskStatusPendingProductionTransfer
		nextJobStatus = domain.CustomizationJobStatusPendingProductionTransfer
		nextHandler = &p.OperatorID
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if job.AssignedOperatorID == nil {
			job.AssignedOperatorID = &p.OperatorID
		}
		job.LastOperatorID = &p.OperatorID
		if !job.PricingWorkerType.Valid() {
			pricingWorkerType, unitPrice, weightFactor, appErr := s.resolveCustomizationPricingSnapshot(ctx, p.OperatorID, job.CustomizationLevelCode)
			if appErr != nil {
				return appErr
			}
			job.PricingWorkerType = pricingWorkerType
			job.UnitPrice = unitPrice
			job.WeightFactor = weightFactor
		}
		job.OrderNo = strings.TrimSpace(p.OrderNo)
		job.CurrentAssetID = currentAssetID
		job.DecisionType = decisionType
		job.Note = strings.TrimSpace(p.Note)
		job.Status = nextJobStatus
		if err := s.customizationJobRepo.Update(ctx, tx, job); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, nextTaskStatus); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, task.ID, nextHandler); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateCustomizationState(ctx, tx, task.ID, &p.OperatorID, "", ""); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, "task.customization.effect_preview_submitted", &p.OperatorID, mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
			"customization_job_id":           job.ID,
			"order_no":                       job.OrderNo,
			"current_asset_id":               currentAssetID,
			"decision_type":                  decisionType,
			"note":                           p.Note,
			"to_task_status":                 nextTaskStatus,
			"to_job_status":                  nextJobStatus,
			"pricing_worker_type":            job.PricingWorkerType,
			"review_reference_unit_price":    job.ReviewReferenceUnitPrice,
			"review_reference_weight_factor": job.ReviewReferenceWeightFactor,
			"unit_price":                     job.UnitPrice,
			"weight_factor":                  job.WeightFactor,
		}))
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("customization effect preview tx", txErr)
	}
	return s.GetCustomizationJob(ctx, p.JobID)
}

func (s *taskService) ReviewCustomizationEffect(ctx context.Context, p ReviewCustomizationEffectParams) (*domain.CustomizationJob, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "customization job repo is not configured", nil)
	}
	job, task, appErr := s.loadCustomizationJobAndTask(ctx, p.JobID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionCustomizationEffectReview, task); appErr != nil {
		return nil, appErr
	}
	if job.Status != domain.CustomizationJobStatusPendingEffectReview {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "customization effect review requires pending_effect_review job status", map[string]interface{}{
			"job_id":     job.ID,
			"job_status": job.Status,
		})
	}
	if !p.Decision.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "customization_review_decision must be approved/return_to_designer/reviewer_fixed", nil)
	}

	nextStatus := domain.TaskStatusPendingProductionTransfer
	nextJobStatus := domain.CustomizationJobStatusPendingProductionTransfer
	nextHandler := job.LastOperatorID
	if p.Decision == domain.CustomizationReviewDecisionReturnToDesigner {
		nextStatus = domain.TaskStatusPendingEffectRevision
		nextJobStatus = domain.CustomizationJobStatusPendingEffectRevision
		nextHandler = job.LastOperatorID
	}
	previousAssetID := cloneInt64Ptr(job.CurrentAssetID)
	currentAssetID, appErr := resolveCustomizationAssetID(job.CurrentAssetID, p.CurrentAssetID)
	if appErr != nil {
		return nil, appErr
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		job.ReviewDecision = p.Decision
		if strings.TrimSpace(p.CustomizationLevelCode) != "" {
			job.CustomizationLevelCode = strings.TrimSpace(p.CustomizationLevelCode)
		}
		if strings.TrimSpace(p.CustomizationLevelName) != "" {
			job.CustomizationLevelName = strings.TrimSpace(p.CustomizationLevelName)
		}
		if p.CustomizationPrice != nil {
			job.ReviewReferenceUnitPrice = cloneFloat64Ptr(p.CustomizationPrice)
		}
		if p.CustomizationWeight != nil {
			job.ReviewReferenceWeightFactor = cloneFloat64Ptr(p.CustomizationWeight)
		}
		if strings.TrimSpace(p.CustomizationNote) != "" {
			job.Note = strings.TrimSpace(p.CustomizationNote)
		}
		if currentAssetID != nil {
			job.CurrentAssetID = currentAssetID
		}
		job.Status = nextJobStatus
		if err := s.customizationJobRepo.Update(ctx, tx, job); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, nextStatus); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, task.ID, nextHandler); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, "task.customization.effect_reviewed", &p.ReviewerID, mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
			"customization_job_id":           job.ID,
			"customization_review_decision":  p.Decision,
			"current_asset_id":               currentAssetID,
			"previous_asset_id":              previousAssetID,
			"replacement_actor_id":           p.ReviewerID,
			"review_reference_unit_price":    job.ReviewReferenceUnitPrice,
			"review_reference_weight_factor": job.ReviewReferenceWeightFactor,
			"reviewer_fixed":                 p.Decision == domain.CustomizationReviewDecisionReviewerFixed,
			"to_task_status":                 nextStatus,
			"to_job_status":                  nextJobStatus,
		}))
		return err
	})
	if txErr != nil {
		return nil, infraError("customization effect review tx", txErr)
	}
	return s.GetCustomizationJob(ctx, p.JobID)
}

func (s *taskService) TransferCustomizationProduction(ctx context.Context, p TransferCustomizationProductionParams) (*domain.CustomizationJob, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "customization job repo is not configured", nil)
	}
	job, task, appErr := s.loadCustomizationJobAndTask(ctx, p.JobID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionCustomizationTransfer, task); appErr != nil {
		return nil, appErr
	}
	if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "customization transfer requires pending_production_transfer job status", map[string]interface{}{
			"job_id":     job.ID,
			"job_status": job.Status,
		})
	}
	currentAssetID, appErr := resolveCustomizationAssetID(job.CurrentAssetID, p.CurrentAssetID)
	if appErr != nil {
		return nil, appErr
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		job.Status = domain.CustomizationJobStatusPendingWarehouseQC
		job.DecisionType = domain.CustomizationJobDecisionTypeFinal
		job.LastOperatorID = &p.OperatorID
		job.CurrentAssetID = currentAssetID
		job.Note = strings.TrimSpace(p.Note)
		if err := s.customizationJobRepo.Update(ctx, tx, job); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, domain.TaskStatusPendingWarehouseQC); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, task.ID, nil); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateCustomizationState(ctx, tx, task.ID, &p.OperatorID, "", ""); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, "task.customization.production_transferred", &p.OperatorID, mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
			"customization_job_id": job.ID,
			"order_no":             job.OrderNo,
			"current_asset_id":     currentAssetID,
			"transfer_channel":     strings.TrimSpace(p.TransferChannel),
			"transfer_reference":   strings.TrimSpace(p.TransferReference),
			"note":                 p.Note,
		}))
		return err
	})
	if txErr != nil {
		return nil, infraError("customization transfer tx", txErr)
	}
	return s.GetCustomizationJob(ctx, p.JobID)
}

func (s *taskService) ListCustomizationJobs(ctx context.Context, filter CustomizationJobFilter) ([]*domain.CustomizationJob, domain.PaginationMeta, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return []*domain.CustomizationJob{}, domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0}, nil
	}
	items, total, err := s.customizationJobRepo.List(ctx, repo.CustomizationJobListFilter{
		TaskID:     filter.TaskID,
		Status:     filter.Status,
		OperatorID: filter.OperatorID,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list customization jobs", err)
	}
	if items == nil {
		items = []*domain.CustomizationJob{}
	}
	return items, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *taskService) GetCustomizationJob(ctx context.Context, id int64) (*domain.CustomizationJob, *domain.AppError) {
	if s.customizationJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "customization job repo is not configured", nil)
	}
	item, err := s.customizationJobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get customization job", err)
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *taskService) loadCustomizationJobAndTask(ctx context.Context, jobID int64) (*domain.CustomizationJob, *domain.Task, *domain.AppError) {
	job, err := s.customizationJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, nil, infraError("get customization job", err)
	}
	if job == nil {
		return nil, nil, domain.ErrNotFound
	}
	task, err := s.taskRepo.GetByID(ctx, job.TaskID)
	if err != nil {
		return nil, nil, infraError("get task for customization job", err)
	}
	if task == nil {
		return nil, nil, domain.ErrNotFound
	}
	return job, task, nil
}

func (s *taskService) resolveCustomizationPricingSnapshot(ctx context.Context, operatorID int64, levelCode string) (domain.EmploymentType, *float64, *float64, *domain.AppError) {
	if s.customizationPricingUserRepo == nil {
		return "", nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "customization pricing user repo is not configured", nil)
	}
	user, err := s.customizationPricingUserRepo.GetByID(ctx, operatorID)
	if err != nil {
		return "", nil, nil, infraError("get customization operator for pricing", err)
	}
	if user == nil {
		return "", nil, nil, domain.ErrNotFound
	}
	employmentType := user.EmploymentType
	if !employmentType.Valid() {
		employmentType = domain.EmploymentTypeFullTime
	}
	if s.customizationPricingRuleRepo == nil {
		return "", nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "customization pricing rule repo is not configured", nil)
	}
	rule, err := s.customizationPricingRuleRepo.GetActiveByLevelAndEmploymentType(ctx, levelCode, employmentType)
	if err != nil {
		return "", nil, nil, infraError("get customization pricing rule", err)
	}
	if rule == nil {
		return "", nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "customization pricing rule missing for employment_type and customization_level_code", map[string]interface{}{
			"operator_id":              operatorID,
			"employment_type":          employmentType,
			"customization_level_code": strings.TrimSpace(levelCode),
		})
	}
	return employmentType, cloneFloat64Ptr(&rule.UnitPrice), cloneFloat64Ptr(&rule.WeightFactor), nil
}

func resolveCustomizationAssetID(existing, requested *int64) (*int64, *domain.AppError) {
	if requested != nil {
		return requested, nil
	}
	if existing != nil {
		return cloneInt64Ptr(existing), nil
	}
	return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "current_asset_id is required before customization delivery can advance", nil)
}

func (s *taskService) applyWarehouseRejectToCustomization(ctx context.Context, tx repo.Tx, task *domain.Task, rejectReason, rejectCategory string) error {
	if task == nil || !task.CustomizationRequired {
		return nil
	}
	if s.customizationJobRepo != nil {
		job, err := s.customizationJobRepo.GetLatestByTaskID(ctx, task.ID)
		if err != nil {
			return err
		}
		if job != nil {
			job.Status = domain.CustomizationJobStatusRejectedByWarehouse
			job.WarehouseRejectReason = rejectReason
			job.WarehouseRejectCategory = rejectCategory
			if err := s.customizationJobRepo.Update(ctx, tx, job); err != nil {
				return err
			}
		}
	}
	return s.taskRepo.UpdateCustomizationState(ctx, tx, task.ID, task.LastCustomizationOperatorID, rejectReason, rejectCategory)
}

func (s *taskService) restoreCustomizationAfterWarehouseReject(ctx context.Context, tx repo.Tx, task *domain.Task) error {
	if task == nil || !task.CustomizationRequired || s.customizationJobRepo == nil {
		return nil
	}
	job, err := s.customizationJobRepo.GetLatestByTaskID(ctx, task.ID)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}
	job.Status = domain.CustomizationJobStatusPendingEffectRevision
	if err := s.customizationJobRepo.Update(ctx, tx, job); err != nil {
		return err
	}
	return nil
}

func (s *taskService) resolveCustomizationRejectStatus(task *domain.Task) (domain.TaskStatus, *int64) {
	if task == nil || !task.CustomizationRequired {
		return domain.TaskStatusRejectedByAuditB, cloneInt64Ptr(task.DesignerID)
	}
	return domain.TaskStatusRejectedByWarehouse, cloneInt64Ptr(task.LastCustomizationOperatorID)
}

func (s *taskService) resolveWarehouseReceiveStatus(task *domain.Task) domain.TaskStatus {
	if task != nil && task.CustomizationRequired && task.TaskStatus == domain.TaskStatusPendingWarehouseQC {
		return domain.TaskStatusPendingWarehouseQC
	}
	return domain.TaskStatusPendingWarehouseReceive
}
