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
	return &domain.TaskModule{
		ID:           2,
		TaskID:       778,
		ModuleKey:    domain.ModuleKeyCustomization,
		State:        domain.ModuleStatePendingClaim,
		PoolTeamCode: &pool,
	}
}

func customizationProductionTask() *domain.Task {
	return &domain.Task{
		ID:                    778,
		TaskType:              domain.TaskTypeNewProductDevelopment,
		TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
		CustomizationRequired: true,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
	}
}

func TestCustomizationOperatorClaimWritesDesignerAndKeepsProductionStatus(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationProductionTask()}
	modRepo := &scopedModuleRepo{module: customizationPendingClaimModule()}
	svc := NewClaimService(taskRepo, modRepo, &fakeEventRepo{}, fakeTxRunner{})

	actor := domain.RequestActor{
		ID:         213,
		Department: string(domain.DepartmentCustomizationArt),
		Team:       "默认组",
		Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleMember},
	}
	dec := svc.Claim(context.Background(), actor, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if !dec.OK {
		t.Fatalf("claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.TaskStatus != domain.TaskStatusPendingCustomizationProduction {
		t.Fatalf("task_status = %s, want PendingCustomizationProduction", taskRepo.task.TaskStatus)
	}
	if taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != 213 {
		t.Fatalf("designer_id = %+v, want 213", taskRepo.task.DesignerID)
	}
	if taskRepo.task.CurrentHandlerID == nil || *taskRepo.task.CurrentHandlerID != 213 {
		t.Fatalf("current_handler_id = %+v, want 213", taskRepo.task.CurrentHandlerID)
	}
}

func TestPureDesignerCannotClaimCustomizationModule(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationProductionTask()}
	modRepo := &scopedModuleRepo{module: customizationPendingClaimModule()}
	svc := NewClaimService(taskRepo, modRepo, &fakeEventRepo{}, fakeTxRunner{})

	dec := svc.Claim(context.Background(), domain.RequestActor{
		ID:    303,
		Team:  domain.TeamDesignStandard,
		Roles: []domain.Role{domain.RoleDesigner, domain.RoleMember},
	}, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if dec.OK || dec.DenyCode != domain.DenyModuleActionRoleDenied {
		t.Fatalf("claim decision = ok:%t code:%s, want module_action_role_denied", dec.OK, dec.DenyCode)
	}
	if taskRepo.task.DesignerID != nil {
		t.Fatalf("designer_id = %+v, want unchanged nil", taskRepo.task.DesignerID)
	}
}

func TestAdminCanClaimCustomizationWithoutMatchingPoolTeam(t *testing.T) {
	taskRepo := &fakeTaskRepo{task: customizationProductionTask()}
	modRepo := &scopedModuleRepo{module: customizationPendingClaimModule()}
	svc := NewClaimService(taskRepo, modRepo, &fakeEventRepo{}, fakeTxRunner{})

	dec := svc.Claim(context.Background(), domain.RequestActor{
		ID:         1,
		Department: string(domain.DepartmentOperations),
		Team:       "淘系一组",
		Roles:      []domain.Role{domain.RoleAdmin},
	}, 778, domain.ModuleKeyCustomization, domain.TeamCustomizationArt)
	if !dec.OK {
		t.Fatalf("admin claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != 1 {
		t.Fatalf("designer_id = %+v, want 1", taskRepo.task.DesignerID)
	}
}

func TestDesignModuleClaimStillTransitionsPendingAssignToInProgress(t *testing.T) {
	fakeGlobalClaimed.Store(false)
	taskRepo := &fakeTaskRepo{task: &domain.Task{
		ID:         1,
		TaskStatus: domain.TaskStatusPendingAssign,
		TaskType:   domain.TaskTypeOriginalProductDevelopment,
	}}
	svc := NewClaimService(taskRepo, &fakeModuleRepo{}, &fakeEventRepo{}, fakeTxRunner{})

	dec := svc.Claim(context.Background(), domain.RequestActor{
		ID:    10,
		Team:  domain.TeamDesignStandard,
		Roles: []domain.Role{domain.RoleDesigner, domain.RoleMember},
	}, 1, domain.ModuleKeyDesign, domain.TeamDesignStandard)
	if !dec.OK {
		t.Fatalf("design claim failed: code=%s message=%s", dec.DenyCode, dec.Message)
	}
	if taskRepo.task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("task_status = %s, want InProgress", taskRepo.task.TaskStatus)
	}
	if taskRepo.task.DesignerID == nil || *taskRepo.task.DesignerID != 10 {
		t.Fatalf("designer_id = %+v, want 10", taskRepo.task.DesignerID)
	}
}
