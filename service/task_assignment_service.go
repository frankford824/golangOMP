package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	taskRepo            repo.TaskRepo
	taskEventRepo       repo.TaskEventRepo
	taskModuleRepo      repo.TaskModuleRepo
	taskModuleEventRepo repo.TaskModuleEventRepo
	txRunner            repo.TxRunner
	dataScopeResolver   DataScopeResolver
	scopeUserRepo       repo.UserRepo
	notifications       taskAssignmentNotificationService
}

type taskAssignmentOperation struct {
	Action          TaskAction
	EventType       string
	LogAction       string
	ResultingStatus domain.TaskStatus
}

var errPendingAssignmentClaimConflict = errors.New("pending assignment claim conflict")

type pendingAssignmentCASUpdater interface {
	ClaimPendingAssignment(ctx context.Context, tx repo.Tx, id int64, designerID int64, resultingStatus domain.TaskStatus) (bool, error)
}

type taskAssignmentNotificationService interface {
	CreateNotification(ctx context.Context, tx repo.Tx, userID int64, ntype domain.NotificationType, payload json.RawMessage) (*domain.Notification, error)
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

func WithTaskAssignmentNotificationService(notifications taskAssignmentNotificationService) TaskAssignmentServiceOption {
	return func(s *taskAssignmentService) {
		s.notifications = notifications
	}
}

func WithTaskAssignmentModuleSync(moduleRepo repo.TaskModuleRepo, moduleEventRepo repo.TaskModuleEventRepo) TaskAssignmentServiceOption {
	return func(s *taskAssignmentService) {
		s.taskModuleRepo = moduleRepo
		s.taskModuleEventRepo = moduleEventRepo
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
	selfClaim, actorID := isTaskAssignmentSelfClaim(ctx, task, p)
	decision := authz.EvaluateTaskActionPolicy(ctx, operation.Action, task, "", "")
	if selfClaim {
		decision.Allowed = true
		decision.DenyCode = ""
		decision.DenyReason = ""
		decision.ActorID = actorID
		decision.MatchedRule = "pending_assign_self_claim"
	}
	authz.logDecision(operation.Action, decision)
	if isActorTakingTaskAlreadyClaimedByOther(task, p, actorID) {
		denied := decision
		denied.Allowed = false
		denied.DenyCode = domain.DenyTaskAlreadyClaimed
		denied.DenyReason = "task is already assigned to another actor"
		denied.ActorID = actorID
		logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, denied, false)
		return nil, taskActionDecisionAppError(operation.Action, denied)
	}
	if !decision.Allowed {
		overrideDecision, overridden, appErr := s.allowDesignManagerAssignmentByTargetScope(ctx, task, p, operation, decision)
		if appErr != nil {
			logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
			return nil, appErr
		}
		if !overridden {
			logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, decision, false)
			return nil, taskActionDecisionAppError(operation.Action, decision)
		}
		decision = overrideDecision
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
		if task.TaskStatus == domain.TaskStatusPendingAssign && task.DesignerID == nil && task.CurrentHandlerID == nil {
			if updater, ok := s.taskRepo.(pendingAssignmentCASUpdater); ok {
				claimed, err := updater.ClaimPendingAssignment(ctx, tx, p.TaskID, *p.DesignerID, operation.ResultingStatus)
				if err != nil {
					return err
				}
				if !claimed {
					return errPendingAssignmentClaimConflict
				}
			} else {
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
			}
		} else {
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
		if err := s.syncDesignModuleAssignment(ctx, tx, task, p, previousDesignerID); err != nil {
			return err
		}
		s.createAssignmentNotification(ctx, tx, task, p, operation, actorID, previousDesignerID, previousHandlerID)
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, errPendingAssignmentClaimConflict) {
			denied := decision
			denied.Allowed = false
			denied.DenyCode = domain.DenyTaskAlreadyClaimed
			denied.DenyReason = "task is already assigned to another actor"
			denied.ActorID = actorID
			logTaskAssignmentDecision(ctx, operation.LogAction, task, p.DesignerID, operation.ResultingStatus, denied, false)
			return nil, taskActionDecisionAppError(operation.Action, denied)
		}
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

func (s *taskAssignmentService) createAssignmentNotification(ctx context.Context, tx repo.Tx, task *domain.Task, p AssignTaskParams, operation taskAssignmentOperation, actorID int64, previousDesignerID, previousHandlerID *int64) {
	if s.notifications == nil || p.DesignerID == nil || *p.DesignerID <= 0 || *p.DesignerID == actorID {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"task_id":              p.TaskID,
		"task_no":              task.TaskNo,
		"task_type":            string(task.TaskType),
		"module_key":           "task",
		"action":               operation.LogAction,
		"assigned_by":          actorID,
		"designer_id":          *p.DesignerID,
		"previous_designer_id": cloneInt64Ptr(previousDesignerID),
		"previous_handler_id":  cloneInt64Ptr(previousHandlerID),
		"remark":               strings.TrimSpace(p.Remark),
		"batch_request_id":     strings.TrimSpace(p.BatchRequestID),
	})
	if err != nil {
		log.Printf("task assignment notification marshal failed task_id=%d user_id=%d err=%v", p.TaskID, *p.DesignerID, err)
		return
	}
	if _, err := s.notifications.CreateNotification(ctx, tx, *p.DesignerID, domain.NotificationTypeTaskAssignedToMe, payload); err != nil {
		log.Printf("task assignment notification create failed task_id=%d user_id=%d err=%v", p.TaskID, *p.DesignerID, err)
	}
}

func (s *taskAssignmentService) syncDesignModuleAssignment(ctx context.Context, tx repo.Tx, task *domain.Task, p AssignTaskParams, previousDesignerID *int64) error {
	if s.taskModuleRepo == nil || p.DesignerID == nil || *p.DesignerID <= 0 || task == nil || !task.TaskType.RequiresDesign() {
		return nil
	}
	moduleKey := assignmentDesignModuleKey(task)
	module, err := s.taskModuleRepo.GetByTaskAndKey(ctx, p.TaskID, moduleKey)
	if err != nil {
		return fmt.Errorf("load design module for task assignment sync: %w", err)
	}
	if module == nil || module.State.Terminal() {
		return nil
	}
	claimedTeam := s.resolveAssignedDesignerTeam(ctx, *p.DesignerID, module)
	if err := s.taskModuleRepo.Reassign(ctx, tx, p.TaskID, moduleKey, *p.DesignerID, claimedTeam, assignmentActorSnapshot(p.AssignedBy)); err != nil {
		return err
	}
	if s.taskModuleEventRepo != nil {
		fromState := module.State
		toState := domain.ModuleStateInProgress
		_, err := s.taskModuleEventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
			TaskModuleID:  module.ID,
			EventType:     assignmentModuleEventType(previousDesignerID),
			FromState:     &fromState,
			ToState:       &toState,
			ActorID:       &p.AssignedBy,
			ActorSnapshot: assignmentActorSnapshot(p.AssignedBy),
			Payload: mustJSON(map[string]interface{}{
				"designer_id":          *p.DesignerID,
				"previous_designer_id": cloneInt64Ptr(previousDesignerID),
				"assigned_by":          p.AssignedBy,
				"source":               "task_assign",
				"remark":               strings.TrimSpace(p.Remark),
			}),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *taskAssignmentService) syncDesignModuleUnassignment(ctx context.Context, tx repo.Tx, task *domain.Task, p AssignTaskParams, previousDesignerID *int64) error {
	if s.taskModuleRepo == nil || task == nil || !task.TaskType.RequiresDesign() {
		return nil
	}
	moduleKey := assignmentDesignModuleKey(task)
	module, err := s.taskModuleRepo.GetByTaskAndKey(ctx, p.TaskID, moduleKey)
	if err != nil {
		return fmt.Errorf("load design module for task unassignment sync: %w", err)
	}
	if module == nil || module.State.Terminal() {
		return nil
	}
	poolTeam := strings.TrimSpace(valueFromStringPtr(module.PoolTeamCode))
	if poolTeam == "" {
		poolTeam = assignmentDefaultPoolTeam(task)
	}
	if err := s.taskModuleRepo.PoolReassign(ctx, tx, p.TaskID, moduleKey, poolTeam); err != nil {
		return err
	}
	if s.taskModuleEventRepo != nil {
		fromState := module.State
		toState := domain.ModuleStatePendingClaim
		_, err := s.taskModuleEventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
			TaskModuleID:  module.ID,
			EventType:     domain.ModuleEventPoolReassignedByAdmin,
			FromState:     &fromState,
			ToState:       &toState,
			ActorID:       &p.AssignedBy,
			ActorSnapshot: assignmentActorSnapshot(p.AssignedBy),
			Payload: mustJSON(map[string]interface{}{
				"previous_designer_id": cloneInt64Ptr(previousDesignerID),
				"assigned_by":          p.AssignedBy,
				"source":               "task_unassign",
				"pool_team_code":       poolTeam,
				"remark":               strings.TrimSpace(p.Remark),
			}),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func assignmentDesignModuleKey(task *domain.Task) string {
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask {
		return domain.ModuleKeyRetouch
	}
	return domain.ModuleKeyDesign
}

func assignmentDefaultPoolTeam(task *domain.Task) string {
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask {
		return domain.TeamDesignRetouch
	}
	return domain.TeamDesignStandard
}

func (s *taskAssignmentService) resolveAssignedDesignerTeam(ctx context.Context, designerID int64, module *domain.TaskModule) string {
	if s.scopeUserRepo != nil && designerID > 0 {
		if user, err := s.scopeUserRepo.GetByID(ctx, designerID); err == nil && user != nil && strings.TrimSpace(string(user.Team)) != "" {
			return strings.TrimSpace(string(user.Team))
		}
	}
	if module != nil {
		if module.ClaimedTeamCode != nil && strings.TrimSpace(*module.ClaimedTeamCode) != "" {
			return strings.TrimSpace(*module.ClaimedTeamCode)
		}
		if module.PoolTeamCode != nil && strings.TrimSpace(*module.PoolTeamCode) != "" {
			return strings.TrimSpace(*module.PoolTeamCode)
		}
	}
	return domain.TeamDesignStandard
}

func assignmentModuleEventType(previousDesignerID *int64) domain.ModuleEventType {
	if previousDesignerID != nil && *previousDesignerID > 0 {
		return domain.ModuleEventReassigned
	}
	return domain.ModuleEventClaimed
}

func assignmentActorSnapshot(actorID int64) json.RawMessage {
	return mustJSON(map[string]interface{}{"actor_id": actorID})
}

func mustJSON(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func valueFromStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
		if err != nil {
			return err
		}
		return s.syncDesignModuleUnassignment(ctx, tx, task, p, previousDesignerID)
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

func (s *taskAssignmentService) allowDesignManagerAssignmentByTargetScope(ctx context.Context, task *domain.Task, p AssignTaskParams, operation taskAssignmentOperation, decision TaskActionDecision) (TaskActionDecision, bool, *domain.AppError) {
	if task == nil || p.DesignerID == nil {
		return decision, false, nil
	}
	if operation.Action != TaskActionAssign && operation.Action != TaskActionReassign {
		return decision, false, nil
	}
	if operation.Action == TaskActionAssign && task.TaskStatus != domain.TaskStatusPendingAssign {
		return decision, false, nil
	}
	if operation.Action == TaskActionReassign && task.TaskStatus != domain.TaskStatusInProgress {
		return decision, false, nil
	}
	if !taskAssignmentScopeDenyCanUseTargetOverride(decision.DenyCode) {
		return decision, false, nil
	}
	if s.scopeUserRepo == nil {
		return decision, false, nil
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !hasAnyRoleValue(actor.Roles, domain.RoleDeptAdmin, domain.RoleDesignDirector, domain.RoleTeamLead) {
		return decision, false, nil
	}
	scopeSource, appErr := s.validateDesignManagerTargetScope(ctx, actor, p.DesignerID)
	if appErr != nil {
		return decision, false, appErr
	}
	decision.Allowed = true
	decision.DenyCode = ""
	decision.DenyReason = ""
	decision.StatusReason = ""
	decision.ScopeSource = string(scopeSource)
	decision.MatchedRule = "design_manager_target_scope"
	return decision, true, nil
}

func taskAssignmentScopeDenyCanUseTargetOverride(denyCode string) bool {
	switch strings.TrimSpace(denyCode) {
	case "task_out_of_department_scope", "task_out_of_team_scope", "task_out_of_scope", "task_not_assigned_to_actor":
		return true
	default:
		return false
	}
}

func (s *taskAssignmentService) validateDesignManagerTargetScope(ctx context.Context, actor domain.RequestActor, designerID *int64) (TaskActionScopeSource, *domain.AppError) {
	if designerID == nil {
		return "", nil
	}
	if hasAnyRoleValue(actor.Roles, domain.RoleDeptAdmin, domain.RoleDesignDirector) {
		managedDepartments := normalizeTaskDepartmentCodes(actor.ManagedDepartments)
		if len(managedDepartments) == 0 {
			managedDepartments = normalizeTaskDepartmentCodes(actor.FrontendAccess.ManagedDepartments)
		}
		if len(managedDepartments) == 0 {
			return "", domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed departments", map[string]interface{}{
				"deny_code":           "reassign_target_out_of_managed_department",
				"target_user_id":      *designerID,
				"actor_id":            actor.ID,
				"managed_departments": managedDepartments,
			})
		}
		target, appErr := s.loadTaskAssignmentTarget(ctx, designerID)
		if appErr != nil {
			return "", appErr
		}
		targetDepartment := normalizeTaskDepartmentCode(string(target.Department))
		for _, department := range managedDepartments {
			if strings.EqualFold(department, targetDepartment) {
				return TaskActionScopeManagedDepartment, nil
			}
		}
		return "", domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed departments", map[string]interface{}{
			"deny_code":           "reassign_target_out_of_managed_department",
			"target_user_id":      *designerID,
			"target_department":   targetDepartment,
			"actor_id":            actor.ID,
			"managed_departments": managedDepartments,
		})
	}
	if hasRoleValue(actor.Roles, domain.RoleTeamLead) {
		managedTeams := normalizeTaskAssignmentTeamCodes(actor.ManagedTeams)
		if len(managedTeams) == 0 {
			managedTeams = normalizeTaskAssignmentTeamCodes(actor.FrontendAccess.ManagedTeams)
		}
		if len(managedTeams) == 0 {
			managedTeams = normalizeTaskAssignmentTeamCodes([]string{actor.Team, actor.FrontendAccess.Team})
		}
		if len(managedTeams) == 0 {
			return "", domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed teams", map[string]interface{}{
				"deny_code":      "reassign_target_out_of_managed_team",
				"target_user_id": *designerID,
				"actor_id":       actor.ID,
				"managed_teams":  managedTeams,
			})
		}
		target, appErr := s.loadTaskAssignmentTarget(ctx, designerID)
		if appErr != nil {
			return "", appErr
		}
		targetTeam := strings.TrimSpace(target.Team)
		for _, team := range managedTeams {
			if strings.EqualFold(team, targetTeam) {
				return TaskActionScopeManagedTeam, nil
			}
		}
		return "", domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed teams", map[string]interface{}{
			"deny_code":      "reassign_target_out_of_managed_team",
			"target_user_id": *designerID,
			"target_team":    targetTeam,
			"actor_id":       actor.ID,
			"managed_teams":  managedTeams,
		})
	}
	return "", domain.NewAppError(domain.ErrCodePermissionDenied, "target assignee is outside actor managed scope", map[string]interface{}{
		"deny_code":      "reassign_target_out_of_managed_scope",
		"target_user_id": *designerID,
		"actor_id":       actor.ID,
	})
}

func (s *taskAssignmentService) loadTaskAssignmentTarget(ctx context.Context, designerID *int64) (*domain.User, *domain.AppError) {
	if s.scopeUserRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "target assignee validation is not configured", nil)
	}
	target, err := s.scopeUserRepo.GetByID(ctx, *designerID)
	if err != nil {
		return nil, infraError("load target assignee", err)
	}
	if target == nil {
		return nil, domain.ErrNotFound
	}
	return target, nil
}

func normalizeTaskAssignmentTeamCodes(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func isTaskAssignmentSelfClaim(ctx context.Context, task *domain.Task, p AssignTaskParams) (bool, int64) {
	actor, ok := domain.RequestActorFromContext(ctx)
	actorID := p.AssignedBy
	if ok && actor.ID > 0 {
		actorID = actor.ID
	}
	if task == nil || task.TaskStatus != domain.TaskStatusPendingAssign || p.DesignerID == nil || *p.DesignerID <= 0 || actorID <= 0 || *p.DesignerID != actorID {
		return false, actorID
	}
	if task.DesignerID != nil || task.CurrentHandlerID != nil {
		return false, actorID
	}
	if !ok {
		return true, actorID
	}
	return hasAnyRoleValue(actor.Roles,
		domain.RoleDesigner,
		domain.RoleOps,
		domain.RoleCustomizationOperator,
		domain.RoleOutsource,
		domain.RoleTeamLead,
		domain.RoleDeptAdmin,
		domain.RoleDesignDirector,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
	), actorID
}

func taskAlreadyClaimedByOther(task *domain.Task, actorID int64) bool {
	if task == nil || actorID <= 0 {
		return false
	}
	if task.CurrentHandlerID != nil && *task.CurrentHandlerID > 0 && *task.CurrentHandlerID != actorID {
		return true
	}
	if task.DesignerID != nil && *task.DesignerID > 0 && *task.DesignerID != actorID {
		return true
	}
	return false
}

func isActorTakingTaskAlreadyClaimedByOther(task *domain.Task, p AssignTaskParams, actorID int64) bool {
	if p.DesignerID == nil || actorID <= 0 || *p.DesignerID != actorID {
		return false
	}
	return taskAlreadyClaimedByOther(task, actorID)
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
