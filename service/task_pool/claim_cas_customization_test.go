package task_pool

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type scopedModuleRepo struct {
	module  *domain.TaskModule
	claimed atomic.Bool
}

func (r *scopedModuleRepo) GetByTaskAndKey(context.Context, int64, string) (*domain.TaskModule, error) {
	return r.module, nil
}
func (r *scopedModuleRepo) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *scopedModuleRepo) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return r.claimed.CompareAndSwap(false, true), nil
}
func (r *scopedModuleRepo) Enter(context.Context, repo.Tx, int64, string, domain.ModuleState, *string, json.RawMessage) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *scopedModuleRepo) UpdateState(context.Context, repo.Tx, int64, string, domain.ModuleState, bool, json.RawMessage) error {
	return nil
}
func (r *scopedModuleRepo) Reassign(context.Context, repo.Tx, int64, string, int64, string, json.RawMessage) error {
	return nil
}
func (r *scopedModuleRepo) PoolReassign(context.Context, repo.Tx, int64, string, string) error {
	return nil
}
func (r *scopedModuleRepo) CloseOpenModules(context.Context, repo.Tx, int64, domain.ModuleState) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *scopedModuleRepo) InsertPlaceholder(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}

func customizationPendingClaimModule() *domain.TaskModule {
	pool := domain.TeamCustomizationArt
	return &domain.TaskModule{ID: 2, TaskID: 778, ModuleKey: domain.ModuleKeyCustomization, State: domain.ModuleStatePendingClaim, PoolTeamCode: &pool}
}

func customizationInProgressTask() *domain.Task {
	return &domain.Task{
		ID: 778, TaskType: domain.TaskTypeNewProductDevelopment, TaskStatus: domain.TaskStatusInProgress,
		CustomizationRequired: true, BusinessLane: domain.TaskBusinessLaneCustomization,
	}
}

func TestCustomizationClaimUsesExplicitDesignCapability(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationInProgressTask()}
	svc := NewClaimService(taskRepo, &scopedModuleRepo{module: customizationPendingClaimModule()}, &fakeEventRepo{}, fakeTxRunner{})
	actor := claimCapabilityActor(213, domain.PermissionTaskDesignSubmit, domain.AccessScopeGlobal)

	dec := svc.Claim(context.Background(), actor, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if !dec.OK {
		t.Fatalf("claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.TaskStatus != domain.TaskStatusInProgress || taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != actor.ID || taskRepo.task.CurrentHandlerID == nil || *taskRepo.task.CurrentHandlerID != actor.ID {
		t.Fatalf("claimed task = %+v", taskRepo.task)
	}
}

func TestCustomizationClaimRejectsLegacyRoleWithoutCapability(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationInProgressTask()}
	svc := NewClaimService(taskRepo, &scopedModuleRepo{module: customizationPendingClaimModule()}, &fakeEventRepo{}, fakeTxRunner{})
	actor := domain.RequestActor{ID: 303, Roles: []domain.Role{domain.RoleCustomizationOperator, domain.RoleAdmin}}

	dec := svc.Claim(context.Background(), actor, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if dec.OK || dec.DenyCode != domain.ErrCodePermissionDenied {
		t.Fatalf("claim decision = ok:%t code:%s", dec.OK, dec.DenyCode)
	}
}

func TestCustomizationClaimAllowsProspectiveSelfScope(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationInProgressTask()}
	svc := NewClaimService(taskRepo, &scopedModuleRepo{module: customizationPendingClaimModule()}, &fakeEventRepo{}, fakeTxRunner{})
	actor := claimCapabilityActor(304, domain.PermissionTaskDesignSubmit, domain.AccessScopeSelf)

	dec := svc.Claim(context.Background(), actor, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if !dec.OK {
		t.Fatalf("self-scope claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
}

func TestDesignModuleClaimTransitionsPendingAssignToInProgress(t *testing.T) {
	fakeGlobalClaimed.Store(false)
	taskRepo := &fakeTaskRepo{task: &domain.Task{ID: 1, TaskStatus: domain.TaskStatusPendingAssign, TaskType: domain.TaskTypeOriginalProductDevelopment}}
	svc := NewClaimService(taskRepo, &fakeModuleRepo{}, &fakeEventRepo{}, fakeTxRunner{})
	actor := claimCapabilityActor(10, domain.PermissionTaskDesignSubmit, domain.AccessScopeGlobal)

	dec := svc.Claim(context.Background(), actor, 1, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if !dec.OK {
		t.Fatalf("design claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.TaskStatus != domain.TaskStatusInProgress || taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != actor.ID {
		t.Fatalf("claimed task = %+v", taskRepo.task)
	}
}
