package blueprint

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type RuleEngine struct {
	registry        *Registry
	tasks           repo.TaskRepo
	modules         repo.TaskModuleRepo
	events          repo.TaskModuleEventRepo
	notificationGen notificationGenerator
}

type notificationGenerator interface {
	GenerateForEvent(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) error
}

func NewRuleEngine(registry *Registry, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, taskRepos ...repo.TaskRepo) *RuleEngine {
	if registry == nil {
		registry = NewRegistry()
	}
	var tasks repo.TaskRepo
	if len(taskRepos) > 0 {
		tasks = taskRepos[0]
	}
	return &RuleEngine{registry: registry, tasks: tasks, modules: modules, events: events}
}

func (e *RuleEngine) SetNotificationGenerator(gen notificationGenerator) {
	if e != nil {
		e.notificationGen = gen
	}
}

func (e *RuleEngine) InitTask(ctx context.Context, tx repo.Tx, task *domain.Task) error {
	if task == nil {
		return nil
	}
	bp, err := e.registry.MustGet(task.TaskType)
	if err != nil {
		return err
	}
	for i, spec := range bp.Modules {
		state := spec.InitialState
		pool := spec.PoolTeamCode
		if i > 1 {
			switch spec.Key {
			case domain.ModuleKeyAudit:
				state = domain.ModuleStatePending
				pool = nil
			case domain.ModuleKeyWarehouse:
				state = domain.ModuleStatePending
				pool = nil
			}
		}
		m, err := e.modules.Enter(ctx, tx, task.ID, spec.Key, state, pool, json.RawMessage(`{}`))
		if err != nil {
			return err
		}
		to := state
		_, err = e.events.Insert(ctx, tx, &domain.TaskModuleEvent{
			TaskModuleID: m.ID,
			EventType:    domain.ModuleEventEntered,
			ToState:      &to,
			Payload:      payload(map[string]interface{}{"pool_team_code": poolValue(pool), "blueprint_key": bp.Key}),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *RuleEngine) ApplyAfterAction(ctx context.Context, tx repo.Tx, task *domain.Task, moduleKey, action string, actorID *int64, actionEventID int64) error {
	if task == nil {
		return nil
	}
	switch moduleKey + "." + action {
	case domain.ModuleKeyDesign + "." + domain.ModuleActionSubmit:
		return e.enterModule(ctx, tx, task, domain.ModuleKeyAudit, actorID, actionEventID)
	case domain.ModuleKeyRetouch + "." + domain.ModuleActionSubmit:
		return e.completeRetouchTask(ctx, tx, task, actorID, actionEventID)
	case domain.ModuleKeyCustomization + "." + domain.ModuleActionSubmit:
		return e.enterModule(ctx, tx, task, domain.ModuleKeyAudit, actorID, actionEventID)
	case domain.ModuleKeyAudit + "." + domain.ModuleActionApprove:
		if isCustomizationTask(task) {
			if err := e.closeModule(ctx, tx, task.ID, domain.ModuleKeyCustomization, actorID, actionEventID); err != nil {
				return err
			}
		} else if err := e.closeModule(ctx, tx, task.ID, domain.ModuleKeyDesign, actorID, actionEventID); err != nil {
			return err
		}
		return e.enterModule(ctx, tx, task, domain.ModuleKeyWarehouse, actorID, actionEventID)
	case domain.ModuleKeyAudit + "." + domain.ModuleActionReject:
		target := domain.ModuleKeyDesign
		if isCustomizationTask(task) {
			target = domain.ModuleKeyCustomization
		}
		return e.reopenModule(ctx, tx, task.ID, target, actorID, actionEventID)
	}
	return nil
}

func (e *RuleEngine) completeRetouchTask(ctx context.Context, tx repo.Tx, task *domain.Task, actorID *int64, actionEventID int64) error {
	if task == nil {
		return nil
	}
	if err := e.closeModule(ctx, tx, task.ID, domain.ModuleKeyRetouch, actorID, actionEventID); err != nil {
		return err
	}
	if e.tasks == nil {
		return nil
	}
	if err := e.tasks.UpdateStatus(ctx, tx, task.ID, domain.TaskStatusCompleted); err != nil {
		return err
	}
	return e.tasks.UpdateHandler(ctx, tx, task.ID, nil)
}

func (e *RuleEngine) enterModule(ctx context.Context, tx repo.Tx, task *domain.Task, moduleKey string, actorID *int64, triggerEventID int64) error {
	spec, ok := e.specFor(task.TaskType, moduleKey)
	if !ok {
		return fmt.Errorf("module %s not in blueprint for task_type %s", moduleKey, task.TaskType)
	}
	state := domain.ModuleStatePendingClaim
	pool := spec.PoolTeamCode
	if moduleKey == domain.ModuleKeyWarehouse {
		pool = strPtr(domain.TeamWarehouseMain)
	}
	m, err := e.modules.Enter(ctx, tx, task.ID, moduleKey, state, pool, json.RawMessage(`{}`))
	if err != nil {
		return err
	}
	to := state
	event := domain.TaskModuleEvent{
		TaskModuleID: m.ID,
		EventType:    domain.ModuleEventEntered,
		ToState:      &to,
		ActorID:      actorID,
		Payload:      payload(map[string]interface{}{"trigger_event_id": triggerEventID, "entered_at": time.Now().UTC(), "pool_team_code": poolValue(pool)}),
	}
	eventID, err := e.events.Insert(ctx, tx, &event)
	if err != nil {
		return err
	}
	event.ID = eventID
	if e.notificationGen != nil {
		_ = e.notificationGen.GenerateForEvent(ctx, tx, event)
	}
	return nil
}

func (e *RuleEngine) closeModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, actorID *int64, triggerEventID int64) error {
	before, err := e.modules.GetByTaskAndKey(ctx, taskID, moduleKey)
	if err != nil || before == nil {
		return err
	}
	from := before.State
	to := domain.ModuleStateClosed
	if err = e.modules.UpdateState(ctx, tx, taskID, moduleKey, to, true, nil); err != nil {
		return err
	}
	_, err = e.events.Insert(ctx, tx, &domain.TaskModuleEvent{
		TaskModuleID: before.ID,
		EventType:    domain.ModuleEventClosed,
		FromState:    &from,
		ToState:      &to,
		ActorID:      actorID,
		Payload:      payload(map[string]interface{}{"trigger_event_id": triggerEventID}),
	})
	return err
}

func (e *RuleEngine) reopenModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, actorID *int64, triggerEventID int64) error {
	before, err := e.modules.GetByTaskAndKey(ctx, taskID, moduleKey)
	if err != nil || before == nil {
		return err
	}
	from := before.State
	to := domain.ModuleStateInProgress
	if err = e.modules.UpdateState(ctx, tx, taskID, moduleKey, to, false, nil); err != nil {
		return err
	}
	_, err = e.events.Insert(ctx, tx, &domain.TaskModuleEvent{
		TaskModuleID: before.ID,
		EventType:    domain.ModuleEventReopened,
		FromState:    &from,
		ToState:      &to,
		ActorID:      actorID,
		Payload:      payload(map[string]interface{}{"trigger_event_id": triggerEventID}),
	})
	return err
}

func (e *RuleEngine) specFor(taskType domain.TaskType, moduleKey string) (ModuleSpec, bool) {
	bp, ok := e.registry.Get(taskType)
	if !ok {
		return ModuleSpec{}, false
	}
	for _, spec := range bp.Modules {
		if spec.Key == moduleKey {
			return spec, true
		}
	}
	return ModuleSpec{}, false
}

func isCustomizationTask(task *domain.Task) bool {
	return task != nil && (task.TaskType == domain.TaskTypeCustomerCustomization || task.TaskType == domain.TaskTypeRegularCustomization || task.CustomizationRequired)
}

func payload(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func poolValue(pool *string) interface{} {
	if pool == nil {
		return nil
	}
	return *pool
}
