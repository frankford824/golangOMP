package module_action

import (
	"context"
	"encoding/json"
	"errors"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/blueprint"
	"workflow/service/module"
	"workflow/service/permission"
)

type ActionService struct {
	tasks             repo.TaskRepo
	modules           repo.TaskModuleRepo
	events            repo.TaskModuleEventRepo
	refs              repo.ReferenceFileRefFlatRepo
	txRunner          repo.TxRunner
	authorizer        *permission.Authorizer
	rules             *blueprint.RuleEngine
	customizationJobs repo.CustomizationJobRepo
	notificationGen   notificationGenerator
}

type notificationGenerator interface {
	GenerateForEvent(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) error
}

type Option func(*ActionService)

func WithNotificationGenerator(gen notificationGenerator) Option {
	return func(s *ActionService) { s.notificationGen = gen }
}

func WithCustomizationJobRepo(jobs repo.CustomizationJobRepo) Option {
	return func(s *ActionService) { s.customizationJobs = jobs }
}

func NewActionService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, refs repo.ReferenceFileRefFlatRepo, txRunner repo.TxRunner, rules *blueprint.RuleEngine, opts ...Option) *ActionService {
	s := &ActionService{tasks: tasks, modules: modules, events: events, refs: refs, txRunner: txRunner, authorizer: permission.NewAuthorizer(tasks, modules), rules: rules}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type ActionRequest struct {
	Actor     domain.RequestActor
	TaskID    int64
	ModuleKey string
	Action    string
	Payload   json.RawMessage
}

func (s *ActionService) Apply(ctx context.Context, req ActionRequest) permission.Decision {
	task, err := s.tasks.GetByID(ctx, req.TaskID)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if task == nil {
		return permission.Deny("task_not_found", "task not found")
	}
	isCustomizationSubmit := req.ModuleKey == domain.ModuleKeyCustomization && req.Action == domain.ModuleActionSubmit
	dec := permission.Allow()
	if isCustomizationSubmit {
		if task.TaskStatus != domain.TaskStatusInProgress || !task.CustomizationRequired {
			return permission.Deny(domain.ErrCodeInvalidStateTransition, "定制任务当前不可提交")
		}
		if !domain.EffectiveAccessAllowsTask(req.Actor, domain.PermissionTaskDesignSubmit, task.AccessSubject()) {
			return permission.Deny(domain.ErrCodePermissionDenied, "task.design.submit is required in the task scope")
		}
	} else {
		dec = s.authorizer.AuthorizeKnownTaskModuleAction(ctx, req.Actor, task, req.ModuleKey, req.Action)
	}
	if !dec.OK {
		return dec
	}
	tm, err := s.modules.GetByTaskAndKey(ctx, req.TaskID, req.ModuleKey)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	next, denyCode, ok := module.NextState(req.ModuleKey, tm.State, req.Action)
	if !ok {
		return permission.Deny(denyCode, "module state does not allow action")
	}
	snapshot := mustJSON(map[string]interface{}{"user_id": req.Actor.ID, "username": req.Actor.Username, "team": req.Actor.Team, "roles": req.Actor.Roles})
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if isCustomizationSubmit {
			return s.markCustomizationReadyForSubmit(ctx, tx, req, snapshot)
		}
		if next != tm.State {
			if err := s.modules.UpdateState(ctx, tx, req.TaskID, req.ModuleKey, next, next.Terminal(), nil); err != nil {
				return err
			}
		}
		from := tm.State
		to := next
		event := domain.TaskModuleEvent{
			TaskID:        req.TaskID,
			TaskModuleID:  tm.ID,
			ModuleKey:     req.ModuleKey,
			EventType:     eventTypeForAction(req.Action),
			FromState:     &from,
			ToState:       &to,
			ActorID:       &req.Actor.ID,
			ActorSnapshot: snapshot,
			Payload:       payloadOrObject(req.Payload),
		}
		eventID, err := s.events.Insert(ctx, tx, &event)
		if err != nil {
			return err
		}
		event.ID = eventID
		if s.notificationGen != nil {
			_ = s.notificationGen.GenerateForEvent(ctx, tx, event)
		}
		if s.rules != nil {
			return s.rules.ApplyAfterAction(ctx, tx, task, req.ModuleKey, req.Action, &req.Actor.ID, eventID)
		}
		return nil
	})
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return permission.Deny(appErr.Code, appErr.Message)
		}
		if errors.Is(err, repo.ErrConflict) {
			return permission.Deny(domain.DenyModuleStateMismatch, "module state changed; refresh and retry")
		}
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	return permission.Allow()
}

type taskForUpdateRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.Task, error)
}

type taskModuleForUpdateRepo interface {
	GetByTaskAndKeyForUpdate(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error)
}

func (s *ActionService) markCustomizationReadyForSubmit(ctx context.Context, tx repo.Tx, req ActionRequest, snapshot json.RawMessage) error {
	taskLocker, ok := s.tasks.(taskForUpdateRepo)
	if !ok || s.customizationJobs == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "customization readiness repository is not configured", nil)
	}
	moduleLocker, ok := s.modules.(taskModuleForUpdateRepo)
	if !ok {
		return domain.NewAppError(domain.ErrCodeInternalError, "module locking repository is not configured", nil)
	}
	lockedTask, err := taskLocker.GetByIDForUpdate(ctx, tx, req.TaskID)
	if err != nil {
		return err
	}
	if lockedTask == nil || lockedTask.TaskStatus != domain.TaskStatusInProgress || !lockedTask.CustomizationRequired {
		return repo.ErrConflict
	}
	if !domain.EffectiveAccessAllowsTask(req.Actor, domain.PermissionTaskDesignSubmit, lockedTask.AccessSubject()) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "task is outside the effective data scope", nil)
	}
	lockedModule, err := moduleLocker.GetByTaskAndKeyForUpdate(ctx, tx, req.TaskID, domain.ModuleKeyCustomization)
	if err != nil {
		return err
	}
	if lockedModule == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "customization module is missing", nil)
	}
	next, denyCode, allowed := module.NextState(domain.ModuleKeyCustomization, lockedModule.State, domain.ModuleActionSubmit)
	if !allowed || next != domain.ModuleStateSubmitted {
		return domain.NewAppError(denyCode, "customization module state does not allow submit", nil)
	}
	job, err := s.customizationJobs.GetLatestByTaskIDForUpdate(ctx, tx, req.TaskID)
	if err != nil {
		return err
	}
	if job == nil || job.Status != domain.CustomizationJobStatusInProgress {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "定制任务尚未处于可完成的设计状态", map[string]interface{}{"job_status": customizationJobStatus(job)})
	}
	if err := s.modules.UpdateState(ctx, tx, req.TaskID, domain.ModuleKeyCustomization, next, false, nil); err != nil {
		return err
	}
	job.Status = domain.CustomizationJobStatusReadyForSubmit
	job.LastOperatorID = &req.Actor.ID
	if err := s.customizationJobs.Update(ctx, tx, job); err != nil {
		return err
	}
	from := lockedModule.State
	event := domain.TaskModuleEvent{
		TaskID: req.TaskID, TaskModuleID: lockedModule.ID, ModuleKey: domain.ModuleKeyCustomization,
		EventType: domain.ModuleEventSubmitted, FromState: &from, ToState: &next, ActorID: &req.Actor.ID,
		ActorSnapshot: snapshot, Payload: payloadOrObject(req.Payload),
	}
	eventID, err := s.events.Insert(ctx, tx, &event)
	if err != nil {
		return err
	}
	event.ID = eventID
	if s.notificationGen != nil {
		_ = s.notificationGen.GenerateForEvent(ctx, tx, event)
	}
	return nil
}

func customizationJobStatus(job *domain.CustomizationJob) domain.CustomizationJobStatus {
	if job == nil {
		return ""
	}
	return job.Status
}

func eventTypeForAction(action string) domain.ModuleEventType {
	switch action {
	case domain.ModuleActionSubmit:
		return domain.ModuleEventSubmitted
	case domain.ModuleActionApprove:
		return domain.ModuleEventApproved
	case domain.ModuleActionReject:
		return domain.ModuleEventRejected
	case domain.ModuleActionUpdateReferenceFiles:
		return domain.ModuleEventReferenceFilesUpdated
	case "receive":
		return domain.ModuleEventReceived
	case "complete":
		return domain.ModuleEventCompleted
	default:
		return domain.ModuleEventType(action)
	}
}

func payloadOrObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func mustJSON(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
