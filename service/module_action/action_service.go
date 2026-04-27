package module_action

import (
	"context"
	"encoding/json"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/blueprint"
	"workflow/service/module"
	"workflow/service/permission"
)

type ActionService struct {
	tasks           repo.TaskRepo
	modules         repo.TaskModuleRepo
	events          repo.TaskModuleEventRepo
	refs            repo.ReferenceFileRefFlatRepo
	txRunner        repo.TxRunner
	authorizer      *permission.Authorizer
	rules           *blueprint.RuleEngine
	notificationGen notificationGenerator
}

type notificationGenerator interface {
	GenerateForEvent(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) error
}

type Option func(*ActionService)

func WithNotificationGenerator(gen notificationGenerator) Option {
	return func(s *ActionService) { s.notificationGen = gen }
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
	dec := s.authorizer.AuthorizeKnownTaskModuleAction(ctx, req.Actor, task, req.ModuleKey, req.Action)
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
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	return permission.Allow()
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
