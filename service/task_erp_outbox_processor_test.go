package service

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type taskERPOutboxFilingStub struct {
	params TriggerTaskFilingParams
	view   *domain.TaskFilingStatusView
	err    *domain.AppError
}

func (s *taskERPOutboxFilingStub) TriggerFiling(_ context.Context, params TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	s.params = params
	return s.view, s.err
}

type taskERPOutboxImageStub struct {
	taskID  int64
	actorID int64
	err     *domain.AppError
}

func (s *taskERPOutboxImageStub) AutoSyncImagesAfterTaskClosed(_ context.Context, taskID, actorID int64) *domain.AppError {
	s.taskID = taskID
	s.actorID = actorID
	return s.err
}

type taskERPOutboxERPStub struct {
	ERPBridgeService
	payloads []domain.ERPProductUpsertPayload
	err      *domain.AppError
}

func (s *taskERPOutboxERPStub) UpsertProduct(_ context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	s.payloads = append(s.payloads, payload)
	return &domain.ERPProductUpsertResult{SKUCode: payload.SKUCode}, s.err
}

func TestTaskERPOutboxProcessorDispatchesFinalizedTaskJobs(t *testing.T) {
	filing := &taskERPOutboxFilingStub{view: &domain.TaskFilingStatusView{CanRetry: false}}
	images := &taskERPOutboxImageStub{}
	processor := NewTaskERPOutboxProcessor(filing, images, nil, nil, nil)

	if err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{ID: 1, TaskID: 42, JobType: "task_filing"}); err != nil {
		t.Fatalf("ProcessTaskERPOutbox(task_filing) error = %v", err)
	}
	if filing.params.TaskID != 42 || filing.params.Source != TaskFilingTriggerSourceAuditFinalApproved || !filing.params.Force {
		t.Fatalf("filing params = %+v", filing.params)
	}
	if err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{ID: 2, TaskID: 42, JobType: "task_image_sync"}); err != nil {
		t.Fatalf("ProcessTaskERPOutbox(task_image_sync) error = %v", err)
	}
	if images.taskID != 42 || images.actorID != 0 {
		t.Fatalf("image sync task/actor = %d/%d", images.taskID, images.actorID)
	}
}

func TestTaskERPOutboxProcessorUsesPersistedTaskFilingSource(t *testing.T) {
	filing := &taskERPOutboxFilingStub{view: &domain.TaskFilingStatusView{CanRetry: false}}
	processor := NewTaskERPOutboxProcessor(filing, nil, nil, nil, nil)
	payload := []byte(`{"task_id":42,"operator_id":9,"source":"task_create"}`)

	if err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{
		ID: 2, TaskID: 42, JobType: "task_filing", Payload: payload,
	}); err != nil {
		t.Fatalf("ProcessTaskERPOutbox(task_create) error = %v", err)
	}
	if filing.params.TaskID != 42 || filing.params.OperatorID != 9 ||
		filing.params.Source != TaskFilingTriggerSourceCreate || !filing.params.Force {
		t.Fatalf("filing params = %+v", filing.params)
	}
}

func TestTaskERPOutboxProcessorMapsRecoveryToManualRetry(t *testing.T) {
	filing := &taskERPOutboxFilingStub{view: &domain.TaskFilingStatusView{CanRetry: false}}
	processor := NewTaskERPOutboxProcessor(filing, nil, nil, nil, nil)
	payload := []byte(`{"task_id":42,"operator_id":1,"source":"task_sku_sync_recovery"}`)

	if err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{
		ID: 3, TaskID: 42, JobType: "task_filing", Payload: payload,
	}); err != nil {
		t.Fatalf("ProcessTaskERPOutbox(recovery) error = %v", err)
	}
	if filing.params.Source != TaskFilingTriggerSourceManualRetry || filing.params.OperatorID != 1 {
		t.Fatalf("filing params = %+v", filing.params)
	}
}

func TestTaskERPOutboxProcessorRejectsMismatchedTaskFilingPayload(t *testing.T) {
	processor := NewTaskERPOutboxProcessor(&taskERPOutboxFilingStub{}, nil, nil, nil, nil)
	err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{
		ID: 4, TaskID: 42, JobType: "task_filing", Payload: []byte(`{"task_id":99,"source":"create"}`),
	})
	if err == nil {
		t.Fatal("identity mismatch error = nil")
	}
}

func TestTaskERPOutboxProcessorRetriesUnfinishedFiling(t *testing.T) {
	processor := NewTaskERPOutboxProcessor(&taskERPOutboxFilingStub{
		view: &domain.TaskFilingStatusView{CanRetry: true, FilingErrorMessage: "ERP unavailable"},
	}, nil, nil, nil, nil)
	if err := processor.ProcessTaskERPOutbox(context.Background(), repo.TaskERPOutboxItem{ID: 1, TaskID: 42, JobType: "task_filing"}); err == nil {
		t.Fatal("ProcessTaskERPOutbox() error = nil, want retryable failure")
	}
}

func TestTaskERPOutboxProcessorPlanningSKUUsesOutboxIdentity(t *testing.T) {
	erp := &taskERPOutboxERPStub{}
	processor := NewTaskERPOutboxProcessor(nil, nil, erp, nil, nil)
	skuItemID := int64(7)
	payload, err := json.Marshal(planningSKUOutboxPayload{
		TaskID: 42, TaskSKUItemID: skuItemID, SKUCode: "PLAN-001", RevisionID: 9,
		ERPProductIID: "IID-1", ERPProductName: "策划商品",
	})
	if err != nil {
		t.Fatal(err)
	}
	item := repo.TaskERPOutboxItem{ID: 3, TaskID: 42, TaskSKUItemID: &skuItemID, JobType: "planning_sku_sync", Payload: payload}
	if err := processor.ProcessTaskERPOutbox(context.Background(), item); err != nil {
		t.Fatalf("ProcessTaskERPOutbox(planning_sku_sync) error = %v", err)
	}
	if len(erp.payloads) != 1 || erp.payloads[0].SKUCode != "PLAN-001" || erp.payloads[0].IID != "IID-1" {
		t.Fatalf("ERP payloads = %+v", erp.payloads)
	}

	wrongItemID := int64(8)
	item.TaskSKUItemID = &wrongItemID
	if err := processor.ProcessTaskERPOutbox(context.Background(), item); err == nil {
		t.Fatal("identity mismatch error = nil")
	}
}
