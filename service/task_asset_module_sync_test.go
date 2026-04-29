package service

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskAssetServicesMarkDesignModuleSubmitted(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		run  func(*taskModuleStateRecorder) error
	}{
		{
			name: "asset center upload completion",
			run: func(modules *taskModuleStateRecorder) error {
				svc := &taskAssetCenterService{taskModuleRepo: modules}
				return svc.markDesignModuleSubmitted(ctx, nil, 629)
			},
		},
		{
			name: "legacy submit design",
			run: func(modules *taskModuleStateRecorder) error {
				svc := &taskAssetService{taskModuleRepo: modules}
				return svc.markDesignModuleSubmitted(ctx, nil, 629)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modules := &taskModuleStateRecorder{}
			if err := tc.run(modules); err != nil {
				t.Fatalf("markDesignModuleSubmitted() error = %v", err)
			}
			if modules.taskID != 629 || modules.moduleKey != domain.ModuleKeyDesign || modules.state != domain.ModuleStateSubmitted {
				t.Fatalf("module update = task:%d key:%s state:%s", modules.taskID, modules.moduleKey, modules.state)
			}
		})
	}
}

type taskModuleStateRecorder struct {
	taskID    int64
	moduleKey string
	state     domain.ModuleState
}

func (r *taskModuleStateRecorder) GetByTaskAndKey(context.Context, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *taskModuleStateRecorder) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *taskModuleStateRecorder) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return false, nil
}
func (r *taskModuleStateRecorder) Enter(context.Context, repo.Tx, int64, string, domain.ModuleState, *string, json.RawMessage) (*domain.TaskModule, error) {
	return nil, nil
}
func (r *taskModuleStateRecorder) UpdateState(_ context.Context, _ repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, _ bool, _ json.RawMessage) error {
	r.taskID = taskID
	r.moduleKey = moduleKey
	r.state = state
	return nil
}
func (r *taskModuleStateRecorder) Reassign(context.Context, repo.Tx, int64, string, int64, string, json.RawMessage) error {
	return nil
}
func (r *taskModuleStateRecorder) PoolReassign(context.Context, repo.Tx, int64, string, string) error {
	return nil
}
func (r *taskModuleStateRecorder) CloseOpenModules(context.Context, repo.Tx, int64, domain.ModuleState) ([]*domain.TaskModule, error) {
	return nil, nil
}
func (r *taskModuleStateRecorder) InsertPlaceholder(context.Context, repo.Tx, int64, string) (*domain.TaskModule, error) {
	return nil, nil
}
