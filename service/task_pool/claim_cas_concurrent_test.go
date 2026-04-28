package task_pool

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/permission"
)

func TestClaimCAS_100Concurrent(t *testing.T) {
	fakeGlobalClaimed.Store(false)
	svc := NewClaimService(&fakeTaskRepo{}, &fakeModuleRepo{}, &fakeEventRepo{}, fakeTxRunner{})
	actor := domain.RequestActor{ID: 10, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}
	var successCount int64
	var conflictCount int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := actor
			a.ID = int64(i + 1)
			dec := svc.Claim(context.Background(), a, 1, domain.ModuleKeyDesign, domain.TeamDesignStandard)
			if dec.OK {
				atomic.AddInt64(&successCount, 1)
			}
			if dec.DenyCode == permission.DenyModuleClaimConflict {
				atomic.AddInt64(&conflictCount, 1)
			}
		}(i)
	}
	wg.Wait()
	if successCount != 1 || conflictCount != 99 {
		t.Fatalf("success/conflict = %d/%d, want 1/99", successCount, conflictCount)
	}
}

type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(fakeTx{})
}

type fakeTx struct{}

func (fakeTx) IsTx() {}

type fakeTaskRepo struct {
	task *domain.Task
}

func (r *fakeTaskRepo) GetByID(context.Context, int64) (*domain.Task, error) {
	if r.task != nil {
		return r.task, nil
	}
	return &domain.Task{ID: 1, TaskType: domain.TaskTypeOriginalProductDevelopment, CreatorID: 2}, nil
}

func (r *fakeTaskRepo) UpdateDesigner(_ context.Context, _ repo.Tx, _ int64, designerID *int64) error {
	if r.task != nil {
		r.task.DesignerID = designerID
	}
	return nil
}

func (r *fakeTaskRepo) UpdateHandler(_ context.Context, _ repo.Tx, _ int64, handlerID *int64) error {
	if r.task != nil {
		r.task.CurrentHandlerID = handlerID
	}
	return nil
}

func (r *fakeTaskRepo) UpdateStatus(_ context.Context, _ repo.Tx, _ int64, status domain.TaskStatus) error {
	if r.task != nil {
		r.task.TaskStatus = status
	}
	return nil
}

type fakeModuleRepo struct{ claimed atomic.Bool }

var fakeGlobalClaimed atomic.Bool

func (r *fakeModuleRepo) GetByTaskAndKey(context.Context, int64, string) (*domain.TaskModule, error) {
	pool := domain.TeamDesignStandard
	return &domain.TaskModule{ID: 1, TaskID: 1, ModuleKey: domain.ModuleKeyDesign, State: domain.ModuleStatePendingClaim, PoolTeamCode: &pool}, nil
}
func (r *fakeModuleRepo) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *fakeModuleRepo) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return fakeGlobalClaimed.CompareAndSwap(false, true), nil
}
func (r *fakeModuleRepo) Enter(context.Context, repo.Tx, int64, string, domain.ModuleState, *string, json.RawMessage) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *fakeModuleRepo) UpdateState(context.Context, repo.Tx, int64, string, domain.ModuleState, bool, json.RawMessage) error {
	return nil
}
func (r *fakeModuleRepo) Reassign(context.Context, repo.Tx, int64, string, int64, string, json.RawMessage) error {
	return nil
}
func (r *fakeModuleRepo) PoolReassign(context.Context, repo.Tx, int64, string, string) error {
	return nil
}
func (r *fakeModuleRepo) CloseOpenModules(context.Context, repo.Tx, int64, domain.ModuleState) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *fakeModuleRepo) InsertPlaceholder(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}

type fakeEventRepo struct{}

func (r *fakeEventRepo) Insert(context.Context, repo.Tx, *domain.TaskModuleEvent) (int64, error) {
	return 1, nil
}
func (r *fakeEventRepo) ListByTaskModule(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return nil, nil
}
func (r *fakeEventRepo) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return nil, nil
}

func TestClaimCASDeniesTaskAssignedToOther(t *testing.T) {
	otherID := int64(99)
	taskRepo := &fakeTaskRepo{task: &domain.Task{ID: 1, TaskStatus: domain.TaskStatusInProgress, DesignerID: &otherID, CurrentHandlerID: &otherID}}
	svc := NewClaimService(taskRepo, &fakeModuleRepo{}, &fakeEventRepo{}, fakeTxRunner{})

	dec := svc.Claim(context.Background(), domain.RequestActor{ID: 10, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}, 1, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if dec.OK || dec.DenyCode != domain.DenyTaskAlreadyClaimed {
		t.Fatalf("claim decision = ok:%t code:%s, want task_already_claimed", dec.OK, dec.DenyCode)
	}
}

func TestClaimCASAssignsUnassignedTaskToActor(t *testing.T) {
	fakeGlobalClaimed.Store(false)
	taskRepo := &fakeTaskRepo{task: &domain.Task{ID: 1, TaskStatus: domain.TaskStatusPendingAssign, TaskType: domain.TaskTypeOriginalProductDevelopment}}
	svc := NewClaimService(taskRepo, &fakeModuleRepo{}, &fakeEventRepo{}, fakeTxRunner{})

	dec := svc.Claim(context.Background(), domain.RequestActor{ID: 10, Team: domain.TeamDesignStandard, Roles: []domain.Role{domain.RoleMember}}, 1, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if !dec.OK {
		t.Fatalf("claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("task status = %s, want InProgress", taskRepo.task.TaskStatus)
	}
	if taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != 10 {
		t.Fatalf("designer_id = %+v, want 10", taskRepo.task.DesignerID)
	}
	if taskRepo.task.CurrentHandlerID == nil || *taskRepo.task.CurrentHandlerID != 10 {
		t.Fatalf("current_handler_id = %+v, want 10", taskRepo.task.CurrentHandlerID)
	}
}
