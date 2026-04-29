package task_cancel

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestCancelForceKeepsTaskStatusCancelled(t *testing.T) {
	taskID := int64(607)
	taskRepo := &cancelTaskRepoStub{
		task: &domain.Task{
			ID:         taskID,
			CreatorID:  1,
			TaskStatus: domain.TaskStatusPendingClose,
		},
	}
	moduleRepo := &cancelModuleRepoStub{
		modules: []*domain.TaskModule{{
			ID:        10,
			TaskID:    taskID,
			ModuleKey: domain.ModuleKeyWarehouse,
			State:     domain.ModuleStateActive,
		}},
	}
	eventRepo := &cancelModuleEventRepoStub{}
	svc := NewService(taskRepo, moduleRepo, eventRepo, cancelTxRunnerStub{})

	decision := svc.Cancel(context.Background(), Request{
		Actor:  domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleSuperAdmin}},
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
func (r *cancelTaskRepoStub) ListBoardCandidates(context.Context, repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	return nil, nil
}
func (r *cancelTaskRepoStub) UpdateDetailBusinessInfo(context.Context, repo.Tx, *domain.TaskDetail) error {
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
