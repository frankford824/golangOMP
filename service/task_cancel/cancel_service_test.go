package task_cancel

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestCancelExplicitCapabilityCanCancelUnclaimedTask(t *testing.T) {
	taskID := int64(1001)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{ID: taskID, CreatorID: 42, TaskStatus: domain.TaskStatusPendingAssign},
	}
	moduleRepo := &cancelModuleRepoStub{
		modules: []*domain.TaskModule{{
			ID:        1,
			TaskID:    taskID,
			ModuleKey: domain.ModuleKeyDesign,
			State:     domain.ModuleStatePending,
		}},
	}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  terminateActor(42, domain.AccessScopeGlobal),
		TaskID: taskID,
		Reason: "creator cancel",
		Force:  false,
	})
	if !decision.OK {
		t.Fatalf("creator cancel denied: %s %s", decision.DenyCode, decision.Message)
	}
	if taskRepo.updatedStatus != domain.TaskStatusCancelled {
		t.Fatalf("updatedStatus = %s, want %s", taskRepo.updatedStatus, domain.TaskStatusCancelled)
	}
	if moduleRepo.closedState != domain.ModuleStateForciblyClosed {
		t.Fatalf("closedState = %s, want %s", moduleRepo.closedState, domain.ModuleStateForciblyClosed)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.ModuleEventTaskCancelled {
		t.Fatalf("events = %+v, want one task_cancelled event", eventRepo.events)
	}
}

func TestCancelNonForceCannotCancelClaimedTask(t *testing.T) {
	taskID := int64(1002)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{ID: taskID, CreatorID: 42, TaskStatus: domain.TaskStatusInProgress},
	}
	moduleRepo := &cancelModuleRepoStub{
		modules: []*domain.TaskModule{{
			ID:        2,
			TaskID:    taskID,
			ModuleKey: domain.ModuleKeyDesign,
			State:     domain.ModuleStateActive,
		}},
	}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  terminateActor(42, domain.AccessScopeGlobal),
		TaskID: taskID,
		Reason: "creator cancel claimed",
		Force:  false,
	})
	if decision.OK || decision.DenyCode != "task_already_claimed" {
		t.Fatalf("claimed task decision = %+v, want task_already_claimed", decision)
	}
	if taskRepo.updatedStatus != "" || len(eventRepo.events) != 0 {
		t.Fatalf("denied cancellation mutated task or events: status=%s events=%+v", taskRepo.updatedStatus, eventRepo.events)
	}
}

func TestCancelNonCreatorMemberCannotCancelOthersTask(t *testing.T) {
	taskID := int64(1003)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{ID: taskID, CreatorID: 42, TaskStatus: domain.TaskStatusPendingAssign},
	}
	moduleRepo := &cancelModuleRepoStub{}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  domain.RequestActor{ID: 88, Roles: []domain.Role{domain.RoleMember}},
		TaskID: taskID,
		Reason: "member cancel other",
		Force:  false,
	})
	if decision.OK {
		t.Fatalf("expected deny for non-creator member")
	}
	if decision.DenyCode != domain.DenyModuleActionRoleDenied {
		t.Fatalf("denyCode = %s, want %s", decision.DenyCode, domain.DenyModuleActionRoleDenied)
	}
}

func TestCancelForceKeepsTaskStatusCancelled(t *testing.T) {
	taskID := int64(607)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{
			ID:         taskID,
			CreatorID:  1,
			TaskStatus: domain.TaskStatusInProgress,
		},
	}
	moduleRepo := &cancelModuleRepoStub{
		modules: []*domain.TaskModule{{
			ID:        10,
			TaskID:    taskID,
			ModuleKey: domain.ModuleKeyDesign,
			State:     domain.ModuleStateActive,
		}},
	}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  terminateActor(99, domain.AccessScopeGlobal),
		TaskID: taskID,
		Reason: "force terminate",
		Force:  true,
	})
	if !decision.OK {
		t.Fatalf("Cancel(force) denied: %s %s", decision.DenyCode, decision.Message)
	}
	if taskRepo.updatedStatus != domain.TaskStatusCancelled {
		t.Fatalf("updatedStatus = %s, want %s", taskRepo.updatedStatus, domain.TaskStatusCancelled)
	}
	if moduleRepo.closedState != domain.ModuleStateClosedByAdmin {
		t.Fatalf("closedState = %s, want %s", moduleRepo.closedState, domain.ModuleStateClosedByAdmin)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.ModuleEventForciblyClosed {
		t.Fatalf("events = %+v, want one forcibly_closed event", eventRepo.events)
	}
}

func TestCancelExplicitDepartmentScopeKeepsForceBehavior(t *testing.T) {
	taskID := int64(1004)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{ID: taskID, CreatorID: 1, TaskStatus: domain.TaskStatusInProgress, OwnerDepartmentID: int64Ptr(7)},
	}
	moduleRepo := &cancelModuleRepoStub{
		modules: []*domain.TaskModule{{
			ID:        11,
			TaskID:    taskID,
			ModuleKey: domain.ModuleKeyDesign,
			State:     domain.ModuleStateActive,
		}},
	}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  terminateActorWithSubject(120, domain.AccessScopeSelectedOrg, domain.AccessScopeSubject{SubjectType: domain.AccessSubjectDepartment, SubjectID: 7}),
		TaskID: taskID,
		Reason: "dept admin force terminate",
		Force:  true,
	})
	if !decision.OK {
		t.Fatalf("department-scoped force denied: %s %s", decision.DenyCode, decision.Message)
	}
	if moduleRepo.closedState != domain.ModuleStateClosedByAdmin {
		t.Fatalf("closedState = %s, want %s", moduleRepo.closedState, domain.ModuleStateClosedByAdmin)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.ModuleEventForciblyClosed {
		t.Fatalf("events = %+v, want one forcibly_closed event", eventRepo.events)
	}
}

func TestCancelLegacyRoleWithoutExplicitCapabilityCannotForce(t *testing.T) {
	taskID := int64(1005)
	svc := NewService(
		&cancelTaskRepoStub{task: &domain.Task{ID: taskID, CreatorID: 1, TaskStatus: domain.TaskStatusInProgress}},
		&cancelModuleRepoStub{},
		&cancelModuleEventRepoStub{},
		cancelTxRunnerStub{},
	)

	decision := svc.Cancel(context.Background(), Request{
		Actor:  domain.RequestActor{ID: 121, Roles: []domain.Role{domain.RoleSuperAdmin}},
		TaskID: taskID,
		Reason: "legacy role must not authorize",
		Force:  true,
	})
	if decision.OK || decision.DenyCode != domain.DenyModuleActionRoleDenied {
		t.Fatalf("legacy-role-only decision = %+v, want explicit capability denial", decision)
	}
}

func terminateActor(id int64, scope domain.AccessScopeMode) domain.RequestActor {
	return terminateActorWithSubject(id, scope)
}

func terminateActorWithSubject(id int64, scope domain.AccessScopeMode, subjects ...domain.AccessScopeSubject) domain.RequestActor {
	return domain.RequestActor{
		ID: id,
		EffectiveAccess: &domain.EffectiveAccess{
			UserID:      id,
			Permissions: []domain.PermissionCode{domain.PermissionTaskTerminate},
			Assignments: []domain.AccessAssignment{{
				UserID: id, RoleID: 1, RoleCode: "task_terminator", ScopeMode: scope, Subjects: subjects,
			}},
			Sources: []domain.EffectiveAccessNote{{
				Permission: domain.PermissionTaskTerminate,
				RoleID:     1,
				RoleCode:   "task_terminator",
				SourceType: "direct",
				ScopeMode:  scope,
			}},
		},
	}
}

func int64Ptr(value int64) *int64 { return &value }

type cancelTxRunnerStub struct{}

func (cancelTxRunnerStub) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(nil)
}

type cancelTaskRepoStub struct {
	task          *domain.Task
	updatedStatus domain.TaskStatus
}

func (r *cancelTaskRepoStub) Create(context.Context, repo.Tx, *domain.Task, *domain.TaskDetail) (int64, error) {
	return 0, nil
}
func (r *cancelTaskRepoStub) CreateSKUItems(context.Context, repo.Tx, []*domain.TaskSKUItem) error {
	return nil
}
func (r *cancelTaskRepoStub) GetByID(context.Context, int64) (*domain.Task, error) {
	return r.task, nil
}
func (r *cancelTaskRepoStub) GetDetailByTaskID(context.Context, int64) (*domain.TaskDetail, error) {
	return nil, nil
}
func (r *cancelTaskRepoStub) GetSKUItemBySKUCode(context.Context, string) (*domain.TaskSKUItem, error) {
	return nil, nil
}
func (r *cancelTaskRepoStub) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	return nil, nil
}
func (r *cancelTaskRepoStub) List(context.Context, repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return nil, 0, nil
}
func (r *cancelTaskRepoStub) UpdateDetailBusinessInfo(context.Context, repo.Tx, *domain.TaskDetail) error {
	return nil
}
func (r *cancelTaskRepoStub) UpdatePriority(_ context.Context, _ repo.Tx, _ int64, priority domain.TaskPriority) error {
	if r.task != nil {
		r.task.Priority = priority
	}
	return nil
}
func (r *cancelTaskRepoStub) UpdateProductBinding(context.Context, repo.Tx, *domain.Task) error {
	return nil
}
func (r *cancelTaskRepoStub) UpdateStatus(_ context.Context, _ repo.Tx, _ int64, status domain.TaskStatus) error {
	r.updatedStatus = status
	if r.task != nil {
		r.task.TaskStatus = status
	}
	return nil
}
func (r *cancelTaskRepoStub) UpdateDesigner(context.Context, repo.Tx, int64, *int64) error {
	return nil
}
func (r *cancelTaskRepoStub) UpdateHandler(context.Context, repo.Tx, int64, *int64) error {
	return nil
}
func (r *cancelTaskRepoStub) UpdateCustomizationState(context.Context, repo.Tx, int64, *int64, string, string) error {
	return nil
}

type cancelModuleRepoStub struct {
	modules     []*domain.TaskModule
	closedState domain.ModuleState
}

func (r *cancelModuleRepoStub) GetByTaskAndKey(context.Context, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *cancelModuleRepoStub) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	return r.modules, nil
}
func (r *cancelModuleRepoStub) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return false, nil
}
func (r *cancelModuleRepoStub) Enter(context.Context, repo.Tx, int64, string, domain.ModuleState, *string, json.RawMessage) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *cancelModuleRepoStub) UpdateState(context.Context, repo.Tx, int64, string, domain.ModuleState, bool, json.RawMessage) error {
	return nil
}
func (r *cancelModuleRepoStub) Reassign(context.Context, repo.Tx, int64, string, int64, string, json.RawMessage) error {
	return nil
}
func (r *cancelModuleRepoStub) PoolReassign(context.Context, repo.Tx, int64, string, string) error {
	return nil
}
func (r *cancelModuleRepoStub) CloseOpenModules(_ context.Context, _ repo.Tx, _ int64, state domain.ModuleState) ([]*domain.TaskModule, error) {
	r.closedState = state
	return r.modules, nil
}
func (r *cancelModuleRepoStub) InsertPlaceholder(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}

type cancelModuleEventRepoStub struct {
	events []*domain.TaskModuleEvent
}

func (r *cancelModuleEventRepoStub) Insert(_ context.Context, _ repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	r.events = append(r.events, event)
	return int64(len(r.events)), nil
}
func (r *cancelModuleEventRepoStub) ListByTaskModule(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return nil, nil
}
func (r *cancelModuleEventRepoStub) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return nil, nil
}
