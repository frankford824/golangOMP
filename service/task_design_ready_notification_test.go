package service

import (
	"context"
	"encoding/json"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type designReadyNotifierStub struct {
	calls     int
	recipient int64
	kind      domain.NotificationType
	payload   json.RawMessage
}

func (n *designReadyNotifierStub) CreateNotification(_ context.Context, _ repo.Tx, id int64, kind domain.NotificationType, payload json.RawMessage) (*domain.Notification, error) {
	n.calls++
	n.recipient = id
	n.kind = kind
	n.payload = payload
	return &domain.Notification{}, nil
}

type designReadyWorkflowRepo struct {
	*bindingRollbackRepo
	replay json.RawMessage
}

func (r *designReadyWorkflowRepo) ListStagedAssetsForUpdate(_ context.Context, _ repo.Tx, ids []int64) (map[int64]domain.StagedTaskAssetBinding, error) {
	out := map[int64]domain.StagedTaskAssetBinding{}
	for _, id := range ids {
		if item, ok := r.staged[id]; ok {
			out[id] = item
		}
	}
	return out, nil
}

func (r *designReadyWorkflowRepo) GetWorkflow(context.Context, int64) (*domain.TaskWorkflowLock, error) {
	return r.task, nil
}
func (r *designReadyWorkflowRepo) ListByTaskID(context.Context, int64) ([]domain.TaskAssetGroup, error) {
	return []domain.TaskAssetGroup{r.group}, nil
}
func (r *designReadyWorkflowRepo) ExpectedResourceGroupCount(context.Context, int64, domain.TaskType) (int64, error) {
	return 1, nil
}
func (r *designReadyWorkflowRepo) CASTaskStatus(_ context.Context, _ repo.Tx, _ int64, revision int64, _, next domain.TaskStatus, _ bool) (int64, error) {
	r.task.Status = next
	r.task.WorkflowRevision = revision + 1
	return revision + 1, nil
}
func (r *designReadyWorkflowRepo) FinalizeGroup(context.Context, repo.Tx, int64, int64, int64, int64) error {
	return nil
}
func (r *designReadyWorkflowRepo) CompleteModules(context.Context, repo.Tx, int64) error { return nil }
func (r *designReadyWorkflowRepo) EnqueueTaskFinalized(context.Context, repo.Tx, int64, int64, bool, bool) error {
	return nil
}
func (r *designReadyWorkflowRepo) StoreIdempotency(context.Context, repo.Tx, int64, int64, string, string, string, interface{}) (bool, json.RawMessage, error) {
	return len(r.replay) == 0, r.replay, nil
}
func (r *designReadyWorkflowRepo) CompleteIdempotency(_ context.Context, _ repo.Tx, _, _ int64, _, _ string, response interface{}) error {
	r.replay, _ = json.Marshal(response)
	return nil
}

func TestDesignReadyNotifiesCreatorOnceAndDoesNotRequireDimensions(t *testing.T) {
	for _, kind := range []domain.TaskType{domain.TaskTypeOriginalProductDevelopment, domain.TaskTypeNewProductDevelopment, domain.TaskTypeRetouchTask} {
		t.Run(string(kind), func(t *testing.T) {
			designer, source, final := int64(9), int64(1), int64(2)
			r := &designReadyWorkflowRepo{bindingRollbackRepo: &bindingRollbackRepo{task: &domain.TaskWorkflowLock{TaskID: 10, TaskType: kind, Status: domain.TaskStatusInProgress, WorkflowRevision: 3, CreatorID: 7, DesignerID: &designer, CurrentHandlerID: &designer}, group: domain.TaskAssetGroup{ID: 20, TaskID: 10, ScopeKind: domain.TaskAssetGroupScopeTask}, staged: map[int64]domain.StagedTaskAssetBinding{source: {TaskAssetID: source, TaskID: 10, BindingState: "staged", StagedRole: "source", StagedBy: &designer}, final: {TaskAssetID: final, TaskID: 10, BindingState: "staged", StagedRole: "final", StagedBy: &designer}}, revision: &domain.TaskAssetGroupRevision{ID: 100, Status: domain.TaskAssetGroupRevisionSubmitted, Mode: domain.TaskAssetGroupModeSingle, Items: []domain.TaskAssetGroupRevisionItem{{TaskAssetID: final}}}}}
			notifier := &designReadyNotifierStub{}
			events := &finalizerEventRepo{}
			svc := NewTaskResourceWorkflowService(r, noopTxRunner{}, events, WithTaskResourceWorkflowNotifications(notifier))
			group := domain.SubmitResourceGroupInput{GroupID: 20, Mode: domain.TaskAssetGroupModeSingle, SourceTaskAssetID: &source}
			if kind == domain.TaskTypeRetouchTask {
				group.FinalTaskAssetIDs = []int64{final}
			}
			request := domain.SubmitDesignV2Request{ExpectedWorkflowRevision: 3, IdempotencyKey: "design-ready", Groups: []domain.SubmitResourceGroupInput{group}}
			actor := specificationActor(designer, domain.PermissionTaskDesignSubmit, domain.PermissionTaskView)
			for i := 0; i < 2; i++ {
				if _, appErr := svc.SubmitDesign(context.Background(), 10, actor, request); appErr != nil {
					t.Fatal(appErr)
				}
			}
			if notifier.calls != 1 || notifier.recipient != 7 || notifier.kind != domain.NotificationTypeTaskDesignReady {
				t.Fatalf("notification duplicated or sent to designer: %+v", notifier)
			}
			var payload map[string]interface{}
			_ = json.Unmarshal(notifier.payload, &payload)
			if payload["task_id"] != float64(10) || payload["workflow_revision"] != float64(4) {
				t.Fatal("missing stable routing/revision fields")
			}
		})
	}
}
