package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type WarehouseAutoReleaseCandidateRepo interface {
	ListWarehouseAutoReleaseCandidates(ctx context.Context, cutoff time.Time, limit int) ([]int64, error)
}

type taskStatusCASRepo interface {
	CASUpdateStatus(ctx context.Context, tx repo.Tx, id int64, expected, next domain.TaskStatus) (bool, error)
}

type WarehouseAutoReleaseJob struct {
	candidatesRepo  WarehouseAutoReleaseCandidateRepo
	taskRepo        repo.TaskRepo
	warehouseRepo   repo.WarehouseRepo
	taskEventRepo   repo.TaskEventRepo
	taskModuleRepo  repo.TaskModuleRepo
	moduleEventRepo repo.TaskModuleEventRepo
	closeSyncer     ProductManagementCloseSyncer
	notifications   taskNotificationService
	txRunner        repo.TxRunner
	now             func() time.Time
	logger          *log.Logger
}

type WarehouseAutoReleaseOptions struct {
	DryRun        bool
	Limit         int
	GracePeriod   time.Duration
	SystemActorID int64
}

type WarehouseAutoReleaseResult struct {
	DryRun   bool      `json:"dry_run"`
	Scanned  int       `json:"scanned"`
	Released int       `json:"released"`
	Cutoff   time.Time `json:"cutoff"`
	Skipped  int       `json:"skipped"`
}

type WarehouseAutoReleaseJobOption func(*WarehouseAutoReleaseJob)

func WithWarehouseAutoReleaseModuleRepos(moduleRepo repo.TaskModuleRepo, moduleEventRepo repo.TaskModuleEventRepo) WarehouseAutoReleaseJobOption {
	return func(j *WarehouseAutoReleaseJob) {
		j.taskModuleRepo = moduleRepo
		j.moduleEventRepo = moduleEventRepo
	}
}

func WithWarehouseAutoReleaseProductManagementCloseSyncer(syncer ProductManagementCloseSyncer) WarehouseAutoReleaseJobOption {
	return func(j *WarehouseAutoReleaseJob) {
		j.closeSyncer = syncer
	}
}

func WithWarehouseAutoReleaseNotificationService(notifications taskNotificationService) WarehouseAutoReleaseJobOption {
	return func(j *WarehouseAutoReleaseJob) {
		j.notifications = notifications
	}
}

func NewWarehouseAutoReleaseJob(candidateRepo WarehouseAutoReleaseCandidateRepo, taskRepo repo.TaskRepo, warehouseRepo repo.WarehouseRepo, taskEventRepo repo.TaskEventRepo, txRunner repo.TxRunner, logger *log.Logger, opts ...WarehouseAutoReleaseJobOption) *WarehouseAutoReleaseJob {
	j := &WarehouseAutoReleaseJob{
		candidatesRepo: candidateRepo,
		taskRepo:       taskRepo,
		warehouseRepo:  warehouseRepo,
		taskEventRepo:  taskEventRepo,
		txRunner:       txRunner,
		now:            time.Now,
		logger:         logger,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(j)
		}
	}
	return j
}

func (j *WarehouseAutoReleaseJob) Run(ctx context.Context, opts WarehouseAutoReleaseOptions) (*WarehouseAutoReleaseResult, *domain.AppError) {
	if j == nil || j.candidatesRepo == nil || j.taskRepo == nil || j.warehouseRepo == nil || j.taskEventRepo == nil || j.txRunner == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "warehouse auto release job is not configured", nil)
	}
	if opts.Limit <= 0 || opts.Limit > 1000 {
		opts.Limit = 100
	}
	if opts.GracePeriod <= 0 {
		opts.GracePeriod = 30 * time.Minute
	}
	cutoff := j.now().UTC().Add(-opts.GracePeriod)
	ids, err := j.candidatesRepo.ListWarehouseAutoReleaseCandidates(ctx, cutoff, opts.Limit)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	result := &WarehouseAutoReleaseResult{DryRun: opts.DryRun, Scanned: len(ids), Cutoff: cutoff}
	if opts.DryRun {
		return result, nil
	}
	for _, taskID := range ids {
		released, err := j.releaseOne(ctx, taskID, opts.SystemActorID)
		if err != nil {
			if j.logger != nil {
				j.logger.Printf("[WAREHOUSE-AUTO-RELEASE] task_id=%d err=%v", taskID, err)
			}
			result.Skipped++
			continue
		}
		if released {
			result.Released++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

func (j *WarehouseAutoReleaseJob) releaseOne(ctx context.Context, taskID int64, systemActorID int64) (bool, error) {
	task, err := j.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return false, err
	}
	if task == nil {
		return false, nil
	}
	now := j.now().UTC()
	var actorPtr *int64
	if systemActorID > 0 {
		actorID := systemActorID
		actorPtr = &actorID
	}
	casRepo, ok := j.taskRepo.(taskStatusCASRepo)
	if !ok {
		return false, fmt.Errorf("task repo does not support CASUpdateStatus")
	}
	var atomicModuleRepo taskModuleAtomicStateRepo
	if j.taskModuleRepo != nil || j.moduleEventRepo != nil {
		if j.taskModuleRepo == nil || j.moduleEventRepo == nil {
			return false, fmt.Errorf("warehouse auto release module synchronization is partially configured")
		}
		atomicModuleRepo, ok = j.taskModuleRepo.(taskModuleAtomicStateRepo)
		if !ok {
			return false, fmt.Errorf("task module repo does not support atomic state synchronization")
		}
	}
	if err := j.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var warehouseModule *domain.TaskModule
		var warehouseModuleState domain.ModuleState
		if atomicModuleRepo != nil {
			warehouseModule, err = atomicModuleRepo.GetByTaskAndKeyForUpdate(ctx, tx, taskID, domain.ModuleKeyWarehouse)
			if err != nil {
				return err
			}
			if warehouseModule == nil {
				return fmt.Errorf("warehouse module is missing")
			}
			warehouseModuleState = warehouseModule.State
		}
		current := task.TaskStatus
		receipt, err := j.warehouseRepo.GetByTaskID(ctx, taskID)
		if err != nil {
			return err
		}
		if receipt != nil && receipt.Status == domain.WarehouseReceiptStatusRejected {
			return fmt.Errorf("warehouse receipt is rejected")
		}
		if current == domain.TaskStatusPendingWarehouseReceive {
			if receipt == nil {
				receipt = &domain.WarehouseReceipt{
					TaskID:       taskID,
					ReceiptNo:    buildWarehouseReceiptNo(taskID, now),
					Status:       domain.WarehouseReceiptStatusReceived,
					ReceiverID:   actorPtr,
					ReceivedAt:   &now,
					RejectReason: "",
					Remark:       "系统自动放行：仓库 30 分钟未处理",
				}
				if _, err := j.warehouseRepo.Create(ctx, tx, receipt); err != nil {
					return err
				}
			} else {
				receipt.Status = domain.WarehouseReceiptStatusReceived
				receipt.ReceiverID = actorPtr
				receipt.ReceivedAt = &now
				receipt.CompletedAt = nil
				receipt.RejectReason = ""
				receipt.Remark = "系统自动放行：仓库 30 分钟未处理"
				if err := j.warehouseRepo.Update(ctx, tx, receipt); err != nil {
					return err
				}
			}
			updated, err := casRepo.CASUpdateStatus(ctx, tx, taskID, domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingProductionTransfer)
			if err != nil {
				return err
			}
			if !updated {
				return fmt.Errorf("task status changed before auto receive")
			}
			_, err = j.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventWarehouseReceived, actorPtr,
				taskTransitionEventPayload(task, domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingProductionTransfer, task.CurrentHandlerID, actorPtr, map[string]interface{}{
					"auto_release": true,
					"receipt_no":   receipt.ReceiptNo,
					"remark":       "系统自动接收：仓库 30 分钟未处理",
				}))
			if err != nil {
				return err
			}
			if err := j.updateWarehouseModuleState(ctx, tx, warehouseModule, &warehouseModuleState, domain.ModuleStateReceived, domain.ModuleEventReceived, actorPtr, map[string]interface{}{
				"auto_release": true,
				"receipt_no":   receipt.ReceiptNo,
				"remark":       "系统自动接收：仓库 30 分钟未处理",
			}); err != nil {
				return err
			}
			current = domain.TaskStatusPendingProductionTransfer
		}
		if receipt == nil {
			return fmt.Errorf("warehouse receipt missing before auto complete")
		}
		if current == domain.TaskStatusPendingProductionTransfer {
			receipt.Status = domain.WarehouseReceiptStatusCompleted
			receipt.CompletedAt = &now
			receipt.RejectReason = ""
			receipt.Remark = "系统自动放行：仓库 30 分钟未处理"
			if receipt.ReceiverID == nil {
				receipt.ReceiverID = actorPtr
			}
			if receipt.ReceivedAt == nil {
				receipt.ReceivedAt = &now
			}
			if err := j.warehouseRepo.Update(ctx, tx, receipt); err != nil {
				return err
			}
			updated, err := casRepo.CASUpdateStatus(ctx, tx, taskID, domain.TaskStatusPendingProductionTransfer, domain.TaskStatusPendingClose)
			if err != nil {
				return err
			}
			if !updated {
				return fmt.Errorf("task status changed before auto complete")
			}
			_, err = j.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventWarehouseCompleted, actorPtr,
				taskTransitionEventPayload(task, domain.TaskStatusPendingProductionTransfer, domain.TaskStatusPendingClose, task.CurrentHandlerID, nil, map[string]interface{}{
					"auto_release": true,
					"receipt_no":   receipt.ReceiptNo,
					"remark":       "系统自动完成仓库环节：仓库 30 分钟未处理",
				}))
			if err != nil {
				return err
			}
			if err := j.updateWarehouseModuleState(ctx, tx, warehouseModule, &warehouseModuleState, domain.ModuleStateCompleted, domain.ModuleEventCompleted, actorPtr, map[string]interface{}{
				"auto_release": true,
				"receipt_no":   receipt.ReceiptNo,
				"remark":       "系统自动完成仓库环节：仓库 30 分钟未处理",
			}); err != nil {
				return err
			}
			current = domain.TaskStatusPendingClose
		}
		if current != domain.TaskStatusPendingClose {
			return fmt.Errorf("task status %s is not auto releasable", current)
		}
		if err := j.updateWarehouseModuleState(ctx, tx, warehouseModule, &warehouseModuleState, domain.ModuleStateCompleted, domain.ModuleEventCompleted, actorPtr, map[string]interface{}{
			"auto_release": true,
			"receipt_no":   receipt.ReceiptNo,
			"remark":       "系统自动补齐仓库模块完成状态：仓库 30 分钟未处理",
		}); err != nil {
			return err
		}
		updated, err := casRepo.CASUpdateStatus(ctx, tx, taskID, domain.TaskStatusPendingClose, domain.TaskStatusCompleted)
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("task status changed before auto close")
		}
		if _, err := j.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventClosed, actorPtr,
			taskTransitionEventPayload(task, domain.TaskStatusPendingClose, domain.TaskStatusCompleted, task.CurrentHandlerID, nil, map[string]interface{}{
				"auto_release":     true,
				"warehouse_status": string(domain.WarehouseReceiptStatusCompleted),
				"remark":           "系统自动结单：仓库 30 分钟未处理",
			})); err != nil {
			return err
		}
		if j.notifications != nil && task.CreatorID > 0 {
			_, err := j.notifications.CreateNotification(ctx, tx, task.CreatorID, domain.NotificationTypeTaskClosed, mustJSON(map[string]interface{}{
				"task_id":          task.ID,
				"task_no":          task.TaskNo,
				"creator_id":       task.CreatorID,
				"designer_id":      task.DesignerID,
				"closed_by":        systemActorID,
				"auto_release":     true,
				"warehouse_status": string(domain.WarehouseReceiptStatusCompleted),
				"remark":           "系统自动结单：仓库 30 分钟未处理",
			}))
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return false, err
	}
	if j.closeSyncer != nil {
		_ = j.closeSyncer.AutoSyncImagesAfterTaskClosed(ctx, taskID, systemActorID)
	}
	return true, nil
}

func (j *WarehouseAutoReleaseJob) updateWarehouseModuleState(ctx context.Context, tx repo.Tx, module *domain.TaskModule, current *domain.ModuleState, next domain.ModuleState, eventType domain.ModuleEventType, actorID *int64, payload map[string]interface{}) error {
	if module == nil || current == nil || j.taskModuleRepo == nil || j.moduleEventRepo == nil {
		return nil
	}
	if *current == next {
		return nil
	}
	from := *current
	if !warehouseModuleTransitionAllowed(from, next) {
		return fmt.Errorf("warehouse module transition %s -> %s is not allowed", from, next)
	}
	atomicRepo, ok := j.taskModuleRepo.(taskModuleAtomicStateRepo)
	if !ok {
		return fmt.Errorf("task module repo does not support atomic state synchronization")
	}
	updated, err := atomicRepo.UpdateStateCAS(ctx, tx, module.TaskID, domain.ModuleKeyWarehouse, from, next, next.Terminal(), nil)
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("warehouse module state changed before auto release")
	}
	to := next
	_, err = j.moduleEventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
		TaskID:       module.TaskID,
		TaskModuleID: module.ID,
		ModuleKey:    domain.ModuleKeyWarehouse,
		EventType:    eventType,
		FromState:    &from,
		ToState:      &to,
		ActorID:      actorID,
		Payload:      warehouseAutoReleaseJSON(payload),
	})
	if err != nil {
		return err
	}
	*current = next
	return nil
}

func warehouseAutoReleaseJSON(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
