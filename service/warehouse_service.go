package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type WarehouseFilter struct {
	TaskID       *int64
	Status       string
	WorkflowLane string
	ReceiverID   *int64
	Page         int
	PageSize     int
}

type ReceiveWarehouseParams struct {
	TaskID     int64
	ReceiverID int64
	Remark     string
}

type RejectWarehouseParams struct {
	TaskID         int64
	ReceiverID     int64
	RejectReason   string
	RejectCategory string
	Remark         string
}

type CompleteWarehouseParams struct {
	TaskID     int64
	ReceiverID int64
	Remark     string
}

// WarehouseService manages warehouse receive/reject/complete lifecycle.
type WarehouseService interface {
	List(ctx context.Context, filter WarehouseFilter) ([]*domain.WarehouseReceipt, domain.PaginationMeta, *domain.AppError)
	Receive(ctx context.Context, p ReceiveWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError)
	Reject(ctx context.Context, p RejectWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError)
	Complete(ctx context.Context, p CompleteWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError)
}

type warehouseService struct {
	taskRepo             repo.TaskRepo
	taskAssetRepo        repo.TaskAssetRepo
	warehouseRepo        repo.WarehouseRepo
	taskEventRepo        repo.TaskEventRepo
	customizationJobRepo repo.CustomizationJobRepo
	txRunner             repo.TxRunner
	filingTrigger        warehouseTaskFilingTrigger
	dataScopeResolver    DataScopeResolver
	scopeUserRepo        repo.UserRepo
	nowFn                func() time.Time
}

type warehouseTaskFilingTrigger interface {
	TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
}

type WarehouseServiceOption func(*warehouseService)

func WithWarehouseFilingTrigger(trigger warehouseTaskFilingTrigger) WarehouseServiceOption {
	return func(s *warehouseService) {
		s.filingTrigger = trigger
	}
}

func WithWarehouseCustomizationJobRepo(customizationJobRepo repo.CustomizationJobRepo) WarehouseServiceOption {
	return func(s *warehouseService) {
		s.customizationJobRepo = customizationJobRepo
	}
}

func WithWarehouseDataScopeResolver(resolver DataScopeResolver) WarehouseServiceOption {
	return func(s *warehouseService) {
		s.dataScopeResolver = resolver
	}
}

func WithWarehouseScopeUserRepo(userRepo repo.UserRepo) WarehouseServiceOption {
	return func(s *warehouseService) {
		s.scopeUserRepo = userRepo
	}
}

func NewWarehouseService(
	taskRepo repo.TaskRepo,
	taskAssetRepo repo.TaskAssetRepo,
	warehouseRepo repo.WarehouseRepo,
	taskEventRepo repo.TaskEventRepo,
	txRunner repo.TxRunner,
	opts ...WarehouseServiceOption,
) WarehouseService {
	svc := &warehouseService{
		taskRepo:      taskRepo,
		taskAssetRepo: taskAssetRepo,
		warehouseRepo: warehouseRepo,
		taskEventRepo: taskEventRepo,
		txRunner:      txRunner,
		nowFn:         time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *warehouseService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *warehouseService) List(ctx context.Context, filter WarehouseFilter) ([]*domain.WarehouseReceipt, domain.PaginationMeta, *domain.AppError) {
	f := repo.WarehouseListFilter{
		TaskID:     filter.TaskID,
		ReceiverID: filter.ReceiverID,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	}
	if filter.Status != "" {
		status := domain.WarehouseReceiptStatus(filter.Status)
		f.Status = &status
	}
	if strings.TrimSpace(filter.WorkflowLane) != "" {
		lane := domain.WorkflowLane(strings.TrimSpace(filter.WorkflowLane))
		f.WorkflowLane = &lane
	}

	receipts, total, err := s.warehouseRepo.List(ctx, f)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list warehouse receipts", err)
	}
	if receipts == nil {
		receipts = []*domain.WarehouseReceipt{}
	}
	for _, receipt := range receipts {
		if err := s.hydrateWarehouseReadyVersion(ctx, receipt); err != nil {
			return nil, domain.PaginationMeta{}, infraError("hydrate warehouse ready version", err)
		}
	}
	return receipts, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *warehouseService) Receive(ctx context.Context, p ReceiveWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError) {
	task, appErr := s.getWarehouseTask(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionWarehouseReceive, task); appErr != nil {
		return nil, appErr
	}

	existing, err := s.warehouseRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get warehouse receipt before receive", err)
	}
	if existing != nil && existing.Status != domain.WarehouseReceiptStatusRejected {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d already has warehouse receipt status %q", p.TaskID, existing.Status),
			nil,
		)
	}

	now := s.nowFn().UTC()
	receiverID := p.ReceiverID
	receipt := existing
	if receipt == nil {
		receipt = &domain.WarehouseReceipt{
			TaskID:       p.TaskID,
			ReceiptNo:    buildWarehouseReceiptNo(p.TaskID, now),
			Status:       domain.WarehouseReceiptStatusReceived,
			ReceiverID:   &receiverID,
			ReceivedAt:   &now,
			RejectReason: "",
			Remark:       p.Remark,
		}
	}
	fromReceiptStatus := warehouseReceiptStatusValue(existing)
	nextStatus, nextHandlerID := s.resolveWarehouseReceiveTaskState(task, receiverID)
	receipt.Status = domain.WarehouseReceiptStatusReceived
	receipt.ReceiverID = &receiverID
	receipt.ReceivedAt = &now
	receipt.CompletedAt = nil
	receipt.RejectReason = ""
	receipt.Remark = p.Remark

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if existing == nil {
			if _, err := s.warehouseRepo.Create(ctx, tx, receipt); err != nil {
				return fmt.Errorf("create warehouse receipt: %w", err)
			}
		} else {
			if err := s.warehouseRepo.Update(ctx, tx, receipt); err != nil {
				return err
			}
		}
		if err := s.advanceWarehouseTask(ctx, tx, task, TaskActionWarehouseReceive, nextStatus, nextHandlerID); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventWarehouseReceived, &receiverID,
			taskTransitionEventPayload(task, task.TaskStatus, nextStatus, task.CurrentHandlerID, nextHandlerID, map[string]interface{}{
				"receiver_id":         receiverID,
				"receipt_no":          receipt.ReceiptNo,
				"remark":              p.Remark,
				"from_receipt_status": fromReceiptStatus,
				"to_receipt_status":   string(receipt.Status),
			}),
		)
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("warehouse receive tx", txErr)
	}

	return s.loadReceiptByTask(ctx, p.TaskID, "re-read warehouse receipt after receive")
}

func (s *warehouseService) Reject(ctx context.Context, p RejectWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError) {
	if p.RejectReason == "" && p.Remark == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reject_reason or remark is required", nil)
	}

	task, appErr := s.getWarehouseTask(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionWarehouseReject, task, "", "")
	authz.logDecision(TaskActionWarehouseReject, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionWarehouseReject, decision)
	}

	existing, err := s.warehouseRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get warehouse receipt before reject", err)
	}
	now := s.nowFn().UTC()
	receiverID := p.ReceiverID
	if task.CustomizationRequired &&
		task.TaskStatus == domain.TaskStatusPendingWarehouseQC &&
		task.LastCustomizationOperatorID == nil &&
		s.customizationJobRepo != nil {
		job, jobErr := s.customizationJobRepo.GetLatestByTaskID(ctx, p.TaskID)
		if jobErr != nil {
			return nil, infraError("get latest customization job before warehouse reject", jobErr)
		}
		if job != nil && job.LastOperatorID != nil {
			task.LastCustomizationOperatorID = cloneInt64Ptr(job.LastOperatorID)
		}
	}
	nextStatus, nextHandlerID := s.resolveWarehouseRejectTaskState(task)
	fromReceiptStatus := warehouseReceiptStatusValue(existing)

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if existing == nil {
			existing = &domain.WarehouseReceipt{
				TaskID:       p.TaskID,
				ReceiptNo:    buildWarehouseReceiptNo(p.TaskID, now),
				Status:       domain.WarehouseReceiptStatusRejected,
				ReceiverID:   &receiverID,
				ReceivedAt:   &now,
				RejectReason: p.RejectReason,
				Remark:       p.Remark,
			}
			if _, err := s.warehouseRepo.Create(ctx, tx, existing); err != nil {
				return fmt.Errorf("create warehouse receipt on reject: %w", err)
			}
		} else {
			if existing.Status == domain.WarehouseReceiptStatusCompleted {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
					fmt.Sprintf("task %d warehouse receipt already completed", p.TaskID), nil)
			}
			existing.Status = domain.WarehouseReceiptStatusRejected
			if existing.ReceiverID == nil {
				existing.ReceiverID = &receiverID
			}
			if existing.ReceivedAt == nil {
				existing.ReceivedAt = &now
			}
			existing.CompletedAt = nil
			existing.RejectReason = p.RejectReason
			existing.Remark = p.Remark
			if err := s.warehouseRepo.Update(ctx, tx, existing); err != nil {
				return err
			}
		}

		if err := s.advanceWarehouseTask(ctx, tx, task, TaskActionWarehouseReject, nextStatus, nextHandlerID); err != nil {
			return err
		}
		if err := s.applyWarehouseRejectToCustomization(ctx, tx, task, p.RejectReason, strings.TrimSpace(p.RejectCategory)); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventWarehouseRejected, &receiverID,
			taskTransitionEventPayload(task, task.TaskStatus, nextStatus, task.CurrentHandlerID, nextHandlerID, map[string]interface{}{
				"receiver_id":            receiverID,
				"receipt_no":             existing.ReceiptNo,
				"from_receipt_status":    fromReceiptStatus,
				"to_receipt_status":      string(domain.WarehouseReceiptStatusRejected),
				"reject_reason":          p.RejectReason,
				"reject_category":        strings.TrimSpace(p.RejectCategory),
				"remark":                 p.Remark,
				"resolution_task_status": string(nextStatus),
				"designer_id":            cloneInt64Ptr(task.DesignerID),
			}),
		)
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("warehouse reject tx", txErr)
	}

	return s.loadReceiptByTask(ctx, p.TaskID, "re-read warehouse receipt after reject")
}

func (s *warehouseService) Complete(ctx context.Context, p CompleteWarehouseParams) (*domain.WarehouseReceipt, *domain.AppError) {
	task, appErr := s.getWarehouseTask(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionWarehouseComplete, task, "", "")
	authz.logDecision(TaskActionWarehouseComplete, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionWarehouseComplete, decision)
	}
	if task.SKUCode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task without sku_code cannot be completed by warehouse", nil)
	}

	existing, err := s.warehouseRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get warehouse receipt before complete", err)
	}
	if existing == nil || existing.Status != domain.WarehouseReceiptStatusReceived {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d must be received before warehouse complete", p.TaskID),
			nil,
		)
	}
	if s.filingTrigger != nil {
		_, filingErr := s.filingTrigger.TriggerFiling(ctx, TriggerTaskFilingParams{
			TaskID:     p.TaskID,
			OperatorID: p.ReceiverID,
			Remark:     p.Remark,
			Source:     TaskFilingTriggerSourceWarehouseCompletePrechk,
			Force:      false,
		})
		if filingErr != nil {
			log.Printf("warehouse_complete_precheck_filing_trigger_failed task_id=%d err=%s", p.TaskID, filingErr.Message)
		}
	}
	now := s.nowFn().UTC()
	receiverID := p.ReceiverID
	existing.Status = domain.WarehouseReceiptStatusCompleted
	existing.CompletedAt = &now
	existing.RejectReason = ""
	existing.Remark = p.Remark
	if existing.ReceiverID == nil {
		existing.ReceiverID = &receiverID
	}
	nextStatus, nextHandlerID := s.resolveWarehouseCompleteTaskState(task)

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.warehouseRepo.Update(ctx, tx, existing); err != nil {
			return err
		}
		if err := s.advanceWarehouseTask(ctx, tx, task, TaskActionWarehouseComplete, nextStatus, nextHandlerID); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventWarehouseCompleted, &receiverID,
			taskTransitionEventPayload(task, task.TaskStatus, nextStatus, task.CurrentHandlerID, nextHandlerID, map[string]interface{}{
				"receiver_id":         receiverID,
				"receipt_no":          existing.ReceiptNo,
				"remark":              p.Remark,
				"from_receipt_status": string(domain.WarehouseReceiptStatusReceived),
				"to_receipt_status":   string(existing.Status),
			}),
		)
		return err
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, infraError("warehouse complete tx", txErr)
	}

	return s.loadReceiptByTask(ctx, p.TaskID, "re-read warehouse receipt after complete")
}

func (s *warehouseService) getWarehouseTask(ctx context.Context, taskID int64) (*domain.Task, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for warehouse", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

func (s *warehouseService) loadReceiptByTask(ctx context.Context, taskID int64, op string) (*domain.WarehouseReceipt, *domain.AppError) {
	receipt, err := s.warehouseRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError(op, err)
	}
	if receipt == nil {
		return nil, domain.ErrNotFound
	}
	if err := s.hydrateWarehouseReadyVersion(ctx, receipt); err != nil {
		return nil, infraError(op, err)
	}
	return receipt, nil
}

func buildWarehouseReceiptNo(taskID int64, now time.Time) string {
	return fmt.Sprintf("WR-%d-%s", taskID, now.Format("20060102150405"))
}

func (s *warehouseService) resolveWarehouseReceiveTaskState(task *domain.Task, receiverID int64) (domain.TaskStatus, *int64) {
	return domain.TaskStatusPendingProductionTransfer, &receiverID
}

func (s *warehouseService) resolveWarehouseRejectTaskState(task *domain.Task) (domain.TaskStatus, *int64) {
	if task == nil {
		return domain.TaskStatusBlocked, nil
	}
	if task.CustomizationRequired && task.TaskStatus == domain.TaskStatusPendingWarehouseQC {
		return domain.TaskStatusRejectedByWarehouse, cloneInt64Ptr(task.LastCustomizationOperatorID)
	}
	if task.TaskType == domain.TaskTypePurchaseTask {
		return domain.TaskStatusRejectedByWarehouse, nil
	}
	return domain.TaskStatusRejectedByWarehouse, cloneInt64Ptr(task.DesignerID)
}

func (s *warehouseService) resolveWarehouseCompleteTaskState(task *domain.Task) (domain.TaskStatus, *int64) {
	return domain.TaskStatusPendingClose, nil
}

func (s *warehouseService) advanceWarehouseTask(
	ctx context.Context,
	tx repo.Tx,
	task *domain.Task,
	action TaskAction,
	nextStatus domain.TaskStatus,
	nextHandlerID *int64,
) error {
	if task == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "warehouse task transition requires task context", nil)
	}
	if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, nextStatus); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, fmt.Sprintf("task %d %s transition failed", task.ID, action), map[string]interface{}{
			"task_id":     task.ID,
			"action":      string(action),
			"from_status": string(task.TaskStatus),
			"to_status":   string(nextStatus),
			"cause":       err.Error(),
		})
	}
	if err := s.taskRepo.UpdateHandler(ctx, tx, task.ID, nextHandlerID); err != nil {
		return fmt.Errorf("update task handler during %s: %w", action, err)
	}
	return nil
}

func (s *warehouseService) applyWarehouseRejectToCustomization(ctx context.Context, tx repo.Tx, task *domain.Task, rejectReason, rejectCategory string) error {
	if task == nil {
		return nil
	}
	if err := s.taskRepo.UpdateCustomizationState(ctx, tx, task.ID, task.LastCustomizationOperatorID, rejectReason, rejectCategory); err != nil {
		return err
	}
	if !task.CustomizationRequired {
		return nil
	}
	if s.customizationJobRepo == nil {
		return nil
	}
	job, err := s.customizationJobRepo.GetLatestByTaskID(ctx, task.ID)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}
	job.Status = domain.CustomizationJobStatusRejectedByWarehouse
	job.WarehouseRejectReason = rejectReason
	job.WarehouseRejectCategory = rejectCategory
	return s.customizationJobRepo.Update(ctx, tx, job)
}

func (s *warehouseService) hydrateWarehouseReadyVersion(ctx context.Context, receipt *domain.WarehouseReceipt) error {
	if receipt == nil {
		return nil
	}
	task, err := s.taskRepo.GetByID(ctx, receipt.TaskID)
	if err != nil || task == nil {
		return err
	}
	receipt.WorkflowLane = task.WorkflowLane()
	receipt.SourceDepartment = taskSourceDepartment(task)
	receipt.TaskType = string(task.TaskType)
	records, err := s.taskAssetRepo.ListByTaskID(ctx, receipt.TaskID)
	if err != nil {
		return err
	}
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		if record == nil || !domain.NormalizeTaskAssetType(record.AssetType).IsDelivery() {
			continue
		}
		version := domain.BuildDesignAssetVersion(record)
		if version == nil {
			continue
		}
		version.TaskNo = task.TaskNo
		version.AssetType = domain.NormalizeTaskAssetType(version.AssetType)
		version.IsDeliveryFile = true
		version.AccessPolicy = domain.DesignAssetAccessPolicyDeliveryFlow
		version.PreviewAvailable = designAssetPreviewAvailable(version)
		version.PreviewPublicAllowed = version.PreviewAvailable
		version.ApprovedForFlow = taskHasApprovedDelivery(task)
		version.WarehouseReady = taskHasApprovedDelivery(task)
		version.CurrentVersionRole = "warehouse_ready_version"
		version.AccessHint = buildDesignAssetAccessHint(version)
		version.Notes = buildDesignAssetNotes(version)
		receipt.WarehouseReadyVersion = version
		return nil
	}
	return nil
}
