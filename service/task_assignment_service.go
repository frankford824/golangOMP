package service

import (
	"context"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type AssignTaskParams struct {
	TaskID         int64
	DesignerID     *int64
	AssignedBy     int64
	Remark         string
	BatchRequestID string
}

type TaskAssignmentService interface {
	Assign(ctx context.Context, p AssignTaskParams) (*domain.Task, *domain.AppError)
	BatchAssign(ctx context.Context, p BatchAssignTasksParams) (*BatchTaskActionResult, *domain.AppError)
	BatchRemind(ctx context.Context, p BatchRemindTasksParams) (*BatchTaskActionResult, *domain.AppError)
}

type BatchAssignTasksParams struct {
	TaskIDs        []int64
	DesignerID     int64
	AssignedBy     int64
	Remark         string
	BatchRequestID string
}

type BatchRemindTasksParams struct {
	TaskIDs        []int64
	ActorID        int64
	Reason         string
	RemindChannel  string
	BatchRequestID string
}

type BatchTaskActionItemResult struct {
	TaskID       int64  `json:"task_id"`
	Success      bool   `json:"success"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type BatchTaskActionResult struct {
	BatchRequestID string                      `json:"batch_request_id"`
	Total          int                         `json:"total"`
	Succeeded      int                         `json:"succeeded"`
	Failed         int                         `json:"failed"`
	Items          []BatchTaskActionItemResult `json:"items"`
}

type taskAssignmentService struct {
	taskRepo          repo.TaskRepo
	taskEventRepo     repo.TaskEventRepo
	txRunner          repo.TxRunner
	dataScopeResolver DataScopeResolver
	scopeUserRepo     repo.UserRepo
}

type taskAssignmentOperation struct {
	Action          TaskAction
	EventType       string
	LogAction       string
	ResultingStatus domain.TaskStatus
}

type TaskAssignmentServiceOption func(*taskAssignmentService)

func WithTaskAssignmentDataScopeResolver(resolver DataScopeResolver) TaskAssignmentServiceOption {
	return func(s *taskAssignmentService) {
		s.dataScopeResolver = resolver
	}
}

func WithTaskAssignmentScopeUserRepo(userRepo repo.UserRepo) TaskAssignmentServiceOption {
	return func(s *taskAssignmentService) {
		s.scopeUserRepo = userRepo
	}
}

func NewTaskAssignmentService(taskRepo repo.TaskRepo, taskEventRepo repo.TaskEventRepo, txRunner repo.TxRunner, opts ...TaskAssignmentServiceOption) TaskAssignmentService {
	svc := &taskAssignmentService{
		taskRepo:      taskRepo,
		taskEventRepo: taskEventRepo,
		txRunner:      txRunner,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *taskAssignmentService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *taskAssignmentService) Assign(ctx context.Context, p AssignTaskParams) (*domain.Task, *domain.AppError) {
	if p.DesignerID != nil && *p.DesignerID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid_designer_id", map[string]interface{}{"deny_code": "invalid_designer_id"})
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for assign", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	operation := resolveTaskAssignmentOperation(task)
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicy(ctx, operation.Action, task, "", "")
	authz.logDecision(operation.Action, decision)
	if !decision.Allowed {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
		return nil, taskActionDecisionAppError(operation.Action, decision)
	}
	if task.TaskStatus != domain.TaskStatusPendingAssign && !taskActionDecisionHasElevatedScope(decision) {
		denied := decision
		denied.Allowed = false
		denied.DenyCode = "task_reassign_requires_manager_scope"
		denied.DenyReason = "task reassign requires manager scope"
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, denied, false)
		return nil, taskActionDecisionAppError(operation.Action, denied)
	}
	if task.TaskStatus == domain.TaskStatusPendingAssign && task.TaskType == domain.TaskTypePurchaseTask {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"purchase_task does not support designer assignment",
			nil,
		)
	}
	if task.TaskStatus == domain.TaskStatusInProgress && task.TaskType == domain.TaskTypePurchaseTask {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"purchase_task does not support designer reassignment",
			nil,
		)
	}
	if p.DesignerID == nil {
		return s.clearAssignment(ctx, task, p, operation, decision)
	}
	if appErr := s.validateManagedDepartmentTarget(ctx, p.DesignerID); appErr != nil {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
		return nil, appErr
	}
	if task.TaskStatus == domain.TaskStatusInProgress &&
		task.DesignerID != nil && *task.DesignerID == *p.DesignerID &&
		task.CurrentHandlerID != nil && *task.CurrentHandlerID == *p.DesignerID {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, true)
		return task, nil
	}

	previousDesignerID := cloneInt64Ptr(task.DesignerID)
	previousHandlerID := cloneInt64Ptr(task.CurrentHandlerID)
	previousStatus := task.TaskStatus
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateDesigner(ctx, tx, p.TaskID, p.DesignerID); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, p.DesignerID); err != nil {
			return err
		}
		if operation.ResultingStatus != task.TaskStatus {
			if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, operation.ResultingStatus); err != nil {
				return err
			}
		}
		payload := taskTransitionEventPayload(task, previousStatus, operation.ResultingStatus, previousHandlerID, p.DesignerID, map[string]interface{}{
			"action":               operation.LogAction,
			"designer_id":          cloneInt64Ptr(p.DesignerID),
			"assigned_by":          p.AssignedBy,
			"remark":               p.Remark,
			"batch_request_id":     strings.TrimSpace(p.BatchRequestID),
			"previous_designer_id": cloneInt64Ptr(previousDesignerID),
			"previous_handler_id":  cloneInt64Ptr(previousHandlerID),
		})
		if operation.Action == TaskActionReassign {
			payload["reassigned_by"] = p.AssignedBy
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, operation.EventType, &p.AssignedBy, payload)
		if err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
		return nil, infraError("assign task tx", txErr)
	}

	updated, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read assigned task", err)
	}
	logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, true)
	return updated, nil
}

func (s *taskAssignmentService) clearAssignment(ctx context.Context, task *domain.Task, p AssignTaskParams, operation taskAssignmentOperation, decision TaskActionDecision) (*domain.Task, *domain.AppError) {
	if task.TaskStatus == domain.TaskStatusPendingAssign && task.DesignerID == nil && task.CurrentHandlerID == nil {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, nil, task.TaskStatus, decision, true)
		return task, nil
	}
	previousDesignerID := cloneInt64Ptr(task.DesignerID)
	previousHandlerID := cloneInt64Ptr(task.CurrentHandlerID)
	previousStatus := task.TaskStatus
	resultingStatus := task.TaskStatus
	if task.TaskStatus == domain.TaskStatusInProgress {
		resultingStatus = domain.TaskStatusPendingAssign
	}
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateDesigner(ctx, tx, p.TaskID, nil); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nil); err != nil {
			return err
		}
		if resultingStatus != task.TaskStatus {
			if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, resultingStatus); err != nil {
				return err
			}
		}
		payload := taskTransitionEventPayload(task, previousStatus, resultingStatus, previousHandlerID, nil, map[string]interface{}{
			"action":               operation.LogAction,
			"designer_id":          nil,
			"assigned_by":          p.AssignedBy,
			"remark":               p.Remark,
			"batch_request_id":     strings.TrimSpace(p.BatchRequestID),
			"previous_designer_id": cloneInt64Ptr(previousDesignerID),
			"previous_handler_id":  cloneInt64Ptr(previousHandlerID),
		})
		if operation.Action == TaskActionReassign {
			payload["reassigned_by"] = p.AssignedBy
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, operation.EventType, &p.AssignedBy, payload)
		return err
	})
	if txErr != nil {
		logTaskAssignmentDecision(ctx, operation.LogAction, task, nil, resultingStatus, decision, false)
		return nil, infraError("clear task assignment tx", txErr)
	}
	updated, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read cleared task assignment", err)
	}
	logTaskAssignmentDecision(ctx, operation.LogAction, task, nil, resultingStatus, decision, true)
	return updated, nil
}

func (s *taskAssignmentService) validateManagedDepartmentTarget(ctx context.Context, designerID *int64) *domain.AppError {
	if designerID == nil {
		return nil
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !hasAnyRoleValue(actor.Roles, domain.RoleDeptAdmin, domain.RoleDesignDirector) {
		return nil
	}
	managedDepartments := normalizeTaskDepartmentCodes(actor.ManagedDepartments)
	if len(managedDepartments) == 0 {
		managedDepartments = normalizeTaskDepartmentCodes(actor.FrontendAccess.ManagedDepartments)
	}
	if len(managedDepartments) == 0 {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed departments", map[string]interface{}{
			"deny_code":           "reassign_target_out_of_managed_department",
			"target_user_id":      *designerID,
			"actor_id":            actor.ID,
			"managed_departments": managedDepartments,
		})
	}
	if s.scopeUserRepo == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "target assignee validation is not configured", nil)
	}
	target, err := s.scopeUserRepo.GetByID(ctx, *designerID)
	if err != nil {
		return infraError("load target assignee", err)
	}
	if target == nil {
		return domain.ErrNotFound
	}
	targetDepartment := normalizeTaskDepartmentCode(string(target.Department))
	for _, department := range managedDepartments {
		if department == targetDepartment {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed departments", map[string]interface{}{
		"deny_code":           "reassign_target_out_of_managed_department",
		"target_user_id":      *designerID,
		"target_department":   targetDepartment,
		"actor_id":            actor.ID,
		"managed_departments": managedDepartments,
	})
}

func (s *taskAssignmentService) BatchAssign(ctx context.Context, p BatchAssignTasksParams) (*BatchTaskActionResult, *domain.AppError) {
	taskIDs := normalizeBatchTaskIDs(p.TaskIDs)
	if len(taskIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_ids must not be empty", nil)
	}
	batchRequestID := strings.TrimSpace(p.BatchRequestID)
	result := &BatchTaskActionResult{
		BatchRequestID: batchRequestID,
		Total:          len(taskIDs),
		Items:          make([]BatchTaskActionItemResult, 0, len(taskIDs)),
	}
	for _, taskID := range taskIDs {
		_, appErr := s.Assign(ctx, AssignTaskParams{
			TaskID:         taskID,
			DesignerID:     &p.DesignerID,
			AssignedBy:     p.AssignedBy,
			Remark:         p.Remark,
			BatchRequestID: batchRequestID,
		})
		item := BatchTaskActionItemResult{
			TaskID:  taskID,
			Success: appErr == nil,
		}
		if appErr != nil {
			item.ErrorCode = appErr.Code
			item.ErrorMessage = appErr.Message
			result.Failed++
		} else {
			result.Succeeded++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *taskAssignmentService) BatchRemind(ctx context.Context, p BatchRemindTasksParams) (*BatchTaskActionResult, *domain.AppError) {
	taskIDs := normalizeBatchTaskIDs(p.TaskIDs)
	if len(taskIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_ids must not be empty", nil)
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required", nil)
	}
	remindChannel := strings.TrimSpace(p.RemindChannel)
	if remindChannel == "" {
		remindChannel = "in_app"
	}
	batchRequestID := strings.TrimSpace(p.BatchRequestID)
	result := &BatchTaskActionResult{
		BatchRequestID: batchRequestID,
		Total:          len(taskIDs),
		Items:          make([]BatchTaskActionItemResult, 0, len(taskIDs)),
	}
	for _, taskID := range taskIDs {
		task, err := s.taskRepo.GetByID(ctx, taskID)
		if err != nil {
			result.Items = append(result.Items, BatchTaskActionItemResult{
				TaskID:       taskID,
				Success:      false,
				ErrorCode:    domain.ErrCodeInternalError,
				ErrorMessage: "failed to load task",
			})
			result.Failed++
			continue
		}
		if task == nil {
			result.Items = append(result.Items, BatchTaskActionItemResult{
				TaskID:       taskID,
				Success:      false,
				ErrorCode:    domain.ErrCodeNotFound,
				ErrorMessage: "task not found",
			})
			result.Failed++
			continue
		}
		eventErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			_, appendErr := s.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventReminded, &p.ActorID, map[string]interface{}{
				"actor_id":         p.ActorID,
				"task_id":          taskID,
				"target_user_id":   cloneInt64Ptr(task.CurrentHandlerID),
				"reason":           reason,
				"event_type":       domain.TaskEventReminded,
				"remind_channel":   remindChannel,
				"batch_request_id": batchRequestID,
				"created_at":       time.Now().UTC(),
			})
			return appendErr
		})
		item := BatchTaskActionItemResult{TaskID: taskID, Success: eventErr == nil}
		if eventErr != nil {
			item.ErrorCode = domain.ErrCodeInternalError
			item.ErrorMessage = "failed to append remind event"
			result.Failed++
		} else {
			result.Succeeded++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func normalizeBatchTaskIDs(taskIDs []int64) []int64 {
	out := make([]int64, 0, len(taskIDs))
	seen := map[int64]struct{}{}
	for _, taskID := range taskIDs {
		if taskID <= 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		out = append(out, taskID)
	}
	return out
}

func resolveTaskAssignmentOperation(task *domain.Task) taskAssignmentOperation {
	if task != nil && task.TaskStatus == domain.TaskStatusPendingAssign {
		return taskAssignmentOperation{
			Action:          TaskActionAssign,
			EventType:       domain.TaskEventAssigned,
			LogAction:       "assign",
			ResultingStatus: domain.TaskStatusInProgress,
		}
	}
	return taskAssignmentOperation{
		Action:          TaskActionReassign,
		EventType:       domain.TaskEventReassigned,
		LogAction:       "reassign",
		ResultingStatus: domain.TaskStatusInProgress,
	}
}

func logTaskAssignmentDecision(
	ctx context.Context,
	action string,
	task *domain.Task,
	newDesignerID *int64,
	resultingStatus domain.TaskStatus,
	decision TaskActionDecision,
	allowed bool,
) {
	if task == nil {
		return
	}
	var previousDesignerID int64
	if task.DesignerID != nil {
		previousDesignerID = *task.DesignerID
	}
	log.Printf(
		"task_assignment trace_id=%s task_id=%d action=%s actor_id=%d actor_roles=%s owner_department=%s owner_org_team=%s previous_designer_id=%d new_designer_id=%d previous_status=%s resulting_status=%s allow=%t deny_reason=%s",
		domain.TraceIDFromContext(ctx),
		task.ID,
		action,
		decision.ActorID,
		domain.JoinRoles(decision.ActorRoles),
		task.OwnerDepartment,
		task.OwnerOrgTeam,
		previousDesignerID,
		int64PtrLogValue(newDesignerID),
		task.TaskStatus,
		resultingStatus,
		allowed,
		decision.DenyCode,
	)
}

func int64PtrLogValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
