package module_action

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type actionV8Tx struct{}

func (actionV8Tx) IsTx() {}

type actionV8TxRunner struct {
	committed  bool
	rolledBack bool
}

func (r *actionV8TxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	err := fn(actionV8Tx{})
	if err != nil {
		r.rolledBack = true
		return err
	}
	r.committed = true
	return nil
}

type actionV8TaskRepo struct {
	repo.TaskRepo
	task *domain.Task
}

func (r *actionV8TaskRepo) GetByID(context.Context, int64) (*domain.Task, error) { return r.task, nil }
func (r *actionV8TaskRepo) GetByIDForUpdate(context.Context, repo.Tx, int64) (*domain.Task, error) {
	copyTask := *r.task
	return &copyTask, nil
}

type actionV8ModuleRepo struct {
	repo.TaskModuleRepo
	module  *domain.TaskModule
	updates []domain.ModuleState
}

func (r *actionV8ModuleRepo) GetByTaskAndKey(context.Context, int64, string) (*domain.TaskModule, error) {
	copyModule := *r.module
	return &copyModule, nil
}
func (r *actionV8ModuleRepo) GetByTaskAndKeyForUpdate(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	copyModule := *r.module
	return &copyModule, nil
}
func (r *actionV8ModuleRepo) UpdateState(_ context.Context, _ repo.Tx, _ int64, _ string, state domain.ModuleState, _ bool, _ json.RawMessage) error {
	r.updates = append(r.updates, state)
	return nil
}

type actionV8EventRepo struct {
	repo.TaskModuleEventRepo
	events []domain.TaskModuleEvent
}

func (r *actionV8EventRepo) Insert(_ context.Context, _ repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	r.events = append(r.events, *event)
	return int64(len(r.events)), nil
}

type actionV8CustomizationRepo struct {
	repo.CustomizationJobRepo
	job     *domain.CustomizationJob
	updates int
}

func (r *actionV8CustomizationRepo) GetLatestByTaskIDForUpdate(context.Context, repo.Tx, int64) (*domain.CustomizationJob, error) {
	if r.job == nil {
		return nil, nil
	}
	copyJob := *r.job
	return &copyJob, nil
}
func (r *actionV8CustomizationRepo) Update(_ context.Context, _ repo.Tx, job *domain.CustomizationJob) error {
	r.updates++
	copyJob := *job
	r.job = &copyJob
	return nil
}

func TestCustomizationSubmitMarksReadyWithoutAdvancingTask(t *testing.T) {
	actorID := int64(19)
	tasks := &actionV8TaskRepo{task: &domain.Task{ID: 41, TaskStatus: domain.TaskStatusInProgress, CustomizationRequired: true, CreatorID: actorID}}
	modules := &actionV8ModuleRepo{module: &domain.TaskModule{ID: 51, TaskID: 41, ModuleKey: domain.ModuleKeyCustomization, State: domain.ModuleStateInProgress, ClaimedBy: &actorID}}
	events := &actionV8EventRepo{}
	jobs := &actionV8CustomizationRepo{job: &domain.CustomizationJob{ID: 61, TaskID: 41, Status: domain.CustomizationJobStatusInProgress}}
	runner := &actionV8TxRunner{}
	svc := NewActionService(tasks, modules, events, nil, runner, nil, WithCustomizationJobRepo(jobs))

	decision := svc.Apply(context.Background(), ActionRequest{
		Actor: actionV8Actor(actorID, domain.PermissionTaskDesignSubmit), TaskID: 41,
		ModuleKey: domain.ModuleKeyCustomization, Action: domain.ModuleActionSubmit,
	})
	if !decision.OK {
		t.Fatalf("Apply() denied: %+v", decision)
	}
	if !runner.committed || runner.rolledBack {
		t.Fatalf("transaction committed/rolledBack = %v/%v", runner.committed, runner.rolledBack)
	}
	if len(modules.updates) != 1 || modules.updates[0] != domain.ModuleStateSubmitted {
		t.Fatalf("module updates = %+v", modules.updates)
	}
	if jobs.updates != 1 || jobs.job.Status != domain.CustomizationJobStatusReadyForSubmit || jobs.job.LastOperatorID == nil || *jobs.job.LastOperatorID != actorID {
		t.Fatalf("customization job = %+v; updates=%d", jobs.job, jobs.updates)
	}
	if len(events.events) != 1 || events.events[0].EventType != domain.ModuleEventSubmitted {
		t.Fatalf("events = %+v", events.events)
	}
	if tasks.task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("task status advanced outside submit-design: %s", tasks.task.TaskStatus)
	}
}

func TestCustomizationSubmitRequiresExplicitScopedCapability(t *testing.T) {
	tasks := &actionV8TaskRepo{task: &domain.Task{ID: 41, TaskStatus: domain.TaskStatusInProgress, CustomizationRequired: true, CreatorID: 19}}
	modules := &actionV8ModuleRepo{module: &domain.TaskModule{ID: 51, TaskID: 41, ModuleKey: domain.ModuleKeyCustomization, State: domain.ModuleStateInProgress}}
	runner := &actionV8TxRunner{}
	svc := NewActionService(tasks, modules, &actionV8EventRepo{}, nil, runner, nil, WithCustomizationJobRepo(&actionV8CustomizationRepo{}))

	decision := svc.Apply(context.Background(), ActionRequest{
		Actor: domain.RequestActor{ID: 19, Roles: []domain.Role{domain.RoleDesigner}}, TaskID: 41,
		ModuleKey: domain.ModuleKeyCustomization, Action: domain.ModuleActionSubmit,
	})
	if decision.OK || decision.DenyCode != domain.ErrCodePermissionDenied {
		t.Fatalf("legacy-role-only decision = %+v", decision)
	}
	if runner.committed || runner.rolledBack {
		t.Fatal("authorization denial unexpectedly opened a transaction")
	}
}

func actionV8Actor(id int64, permission domain.PermissionCode) domain.RequestActor {
	assignment := domain.AccessAssignment{ID: 1, UserID: id, RoleID: 2, ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
	effective := &domain.EffectiveAccess{
		UserID: id, Permissions: []domain.PermissionCode{permission}, Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{Permission: permission, RoleID: assignment.RoleID, SourceType: assignment.SourceType, ScopeMode: assignment.ScopeMode}},
	}
	return domain.RequestActor{ID: id, Permissions: effective.Permissions, EffectiveAccess: effective}
}
