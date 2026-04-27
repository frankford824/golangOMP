package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type warehouseTestSnapshotter interface {
	snapshotState() any
	restoreState(any)
}

type warehouseTestTxRunner struct {
	snapshotters []warehouseTestSnapshotter
}

func (r warehouseTestTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	snapshots := make([]any, len(r.snapshotters))
	for i, snapshotter := range r.snapshotters {
		snapshots[i] = snapshotter.snapshotState()
	}
	if err := fn(step04Tx{}); err != nil {
		for i, snapshotter := range r.snapshotters {
			snapshotter.restoreState(snapshots[i])
		}
		return err
	}
	return nil
}

type warehouseTestTaskRepo struct {
	*step04TaskRepo
	failUpdateStatus error
}

func newWarehouseTestTaskRepo(tasks ...*domain.Task) *warehouseTestTaskRepo {
	return &warehouseTestTaskRepo{step04TaskRepo: newStep04TaskRepo(tasks...)}
}

func (r *warehouseTestTaskRepo) UpdateStatus(ctx context.Context, tx repo.Tx, id int64, status domain.TaskStatus) error {
	if r.failUpdateStatus != nil {
		return r.failUpdateStatus
	}
	return r.step04TaskRepo.UpdateStatus(ctx, tx, id, status)
}

func (r *warehouseTestTaskRepo) snapshotState() any {
	return cloneTaskMap(r.tasks)
}

func (r *warehouseTestTaskRepo) restoreState(snapshot any) {
	r.tasks = cloneTaskMap(snapshot.(map[int64]*domain.Task))
}

type warehouseTestReceiptRepo struct {
	nextID   int64
	receipts map[int64]*domain.WarehouseReceipt
}

func newWarehouseTestReceiptRepo(receipts ...*domain.WarehouseReceipt) *warehouseTestReceiptRepo {
	store := make(map[int64]*domain.WarehouseReceipt, len(receipts))
	var maxID int64
	for _, receipt := range receipts {
		if receipt == nil {
			continue
		}
		copied := cloneWarehouseReceipt(receipt)
		store[copied.TaskID] = copied
		if copied.ID > maxID {
			maxID = copied.ID
		}
	}
	return &warehouseTestReceiptRepo{nextID: maxID + 1, receipts: store}
}

func (r *warehouseTestReceiptRepo) Create(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) (int64, error) {
	if r.receipts == nil {
		r.receipts = map[int64]*domain.WarehouseReceipt{}
	}
	copied := cloneWarehouseReceipt(receipt)
	if copied.ID == 0 {
		if r.nextID == 0 {
			r.nextID = 1
		}
		copied.ID = r.nextID
		r.nextID++
	}
	if copied.CreatedAt.IsZero() {
		if copied.ReceivedAt != nil {
			copied.CreatedAt = copied.ReceivedAt.UTC()
		} else {
			copied.CreatedAt = time.Now().UTC()
		}
	}
	if copied.UpdatedAt.IsZero() {
		copied.UpdatedAt = copied.CreatedAt
	}
	r.receipts[copied.TaskID] = copied
	receipt.ID = copied.ID
	receipt.CreatedAt = copied.CreatedAt
	receipt.UpdatedAt = copied.UpdatedAt
	return copied.ID, nil
}

func (r *warehouseTestReceiptRepo) GetByID(_ context.Context, id int64) (*domain.WarehouseReceipt, error) {
	for _, receipt := range r.receipts {
		if receipt != nil && receipt.ID == id {
			return cloneWarehouseReceipt(receipt), nil
		}
	}
	return nil, nil
}

func (r *warehouseTestReceiptRepo) GetByTaskID(_ context.Context, taskID int64) (*domain.WarehouseReceipt, error) {
	if r.receipts == nil {
		return nil, nil
	}
	return cloneWarehouseReceipt(r.receipts[taskID]), nil
}

func (r *warehouseTestReceiptRepo) List(_ context.Context, _ repo.WarehouseListFilter) ([]*domain.WarehouseReceipt, int64, error) {
	items := make([]*domain.WarehouseReceipt, 0, len(r.receipts))
	for _, receipt := range r.receipts {
		if receipt == nil {
			continue
		}
		items = append(items, cloneWarehouseReceipt(receipt))
	}
	return items, int64(len(items)), nil
}

func (r *warehouseTestReceiptRepo) Update(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) error {
	if r.receipts == nil {
		r.receipts = map[int64]*domain.WarehouseReceipt{}
	}
	copied := cloneWarehouseReceipt(receipt)
	current := r.receipts[copied.TaskID]
	if current != nil && copied.CreatedAt.IsZero() {
		copied.CreatedAt = current.CreatedAt
	}
	if copied.UpdatedAt.IsZero() {
		if copied.CompletedAt != nil {
			copied.UpdatedAt = copied.CompletedAt.UTC()
		} else if copied.ReceivedAt != nil {
			copied.UpdatedAt = copied.ReceivedAt.UTC()
		} else if current != nil {
			copied.UpdatedAt = current.UpdatedAt
		}
	}
	r.receipts[copied.TaskID] = copied
	return nil
}

func (r *warehouseTestReceiptRepo) snapshotState() any {
	return warehouseReceiptSnapshot{
		nextID:   r.nextID,
		receipts: cloneReceiptMap(r.receipts),
	}
}

func (r *warehouseTestReceiptRepo) restoreState(snapshot any) {
	state := snapshot.(warehouseReceiptSnapshot)
	r.nextID = state.nextID
	r.receipts = cloneReceiptMap(state.receipts)
}

type warehouseReceiptSnapshot struct {
	nextID   int64
	receipts map[int64]*domain.WarehouseReceipt
}

func TestWarehouseReceiveAdvancesTaskStatus(t *testing.T) {
	const (
		taskID     = int64(453)
		receiverID = int64(200)
	)
	taskRepo := newWarehouseTestTaskRepo(&domain.Task{
		ID:              taskID,
		TaskNo:          "T-453",
		TaskType:        domain.TaskTypePurchaseTask,
		TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "ops-team",
	})
	warehouseRepo := newWarehouseTestReceiptRepo()
	eventRepo := &step04TaskEventRepo{}
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, eventRepo, warehouseTestTxRunner{
		snapshotters: []warehouseTestSnapshotter{taskRepo, warehouseRepo},
	}).(*warehouseService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 4, 18, 10, 45, 40, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         receiverID,
		Roles:      []domain.Role{domain.RoleWarehouse},
		Department: string(domain.DepartmentCloudWarehouse),
		Team:       "warehouse-team",
	})

	receipt, appErr := svc.Receive(ctx, ReceiveWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     "received in UAT",
	})
	if appErr != nil {
		t.Fatalf("Receive() appErr = %+v", appErr)
	}
	if receipt == nil {
		t.Fatal("Receive() receipt is nil")
	}
	if receipt.Status != domain.WarehouseReceiptStatusReceived {
		t.Fatalf("receipt status = %s, want received", receipt.Status)
	}
	if receipt.ReceiverID == nil || *receipt.ReceiverID != receiverID {
		t.Fatalf("receipt receiver_id = %+v, want %d", receipt.ReceiverID, receiverID)
	}
	if taskRepo.tasks[taskID].TaskStatus != domain.TaskStatusPendingProductionTransfer {
		t.Fatalf("task_status = %s, want PendingProductionTransfer", taskRepo.tasks[taskID].TaskStatus)
	}
	storedReceipt := warehouseRepo.receipts[taskID]
	if storedReceipt == nil {
		t.Fatal("warehouse receipt was not persisted")
	}
	if storedReceipt.Status != domain.WarehouseReceiptStatusReceived {
		t.Fatalf("stored warehouse status = %s, want received", storedReceipt.Status)
	}

	detail := &domain.TaskDetail{
		TaskID:       taskID,
		Category:     "bags",
		SpecText:     "spec",
		CostPrice:    warehouseFloat64Ptr(10),
		FilingStatus: domain.FilingStatusFiled,
		FiledAt:      warehouseTimePtr(time.Date(2026, 4, 18, 10, 45, 39, 0, time.UTC)),
	}
	procurement := &domain.ProcurementRecord{
		TaskID:             taskID,
		Status:             domain.ProcurementStatusCompleted,
		ProcurementPrice:   warehouseFloat64Ptr(10),
		Quantity:           int64Ptr(1),
		ExpectedDeliveryAt: warehouseTimePtr(time.Date(2026, 4, 18, 10, 45, 39, 0, time.UTC)),
	}
	workflow := buildTaskWorkflowSnapshot(taskRepo.tasks[taskID], detail, procurement, false, storedReceipt)
	if hasWorkflowReason(workflow.WarehouseBlockingReasons, domain.WorkflowReasonTaskAlreadyPendingWH) {
		t.Fatalf("warehouse_blocking_reasons unexpectedly contains %s", domain.WorkflowReasonTaskAlreadyPendingWH)
	}
	if !hasWorkflowReason(workflow.WarehouseBlockingReasons, domain.WorkflowReasonWarehouseAlreadyReceived) {
		t.Fatalf("warehouse_blocking_reasons missing %s", domain.WorkflowReasonWarehouseAlreadyReceived)
	}
}

func TestWarehouseReceiveRollbackOnTransitionFailure(t *testing.T) {
	const (
		taskID     = int64(443)
		receiverID = int64(200)
	)
	taskRepo := newWarehouseTestTaskRepo(&domain.Task{
		ID:              taskID,
		TaskNo:          "T-443",
		TaskType:        domain.TaskTypePurchaseTask,
		TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "ops-team",
	})
	taskRepo.failUpdateStatus = errors.New("transition refused")
	warehouseRepo := newWarehouseTestReceiptRepo()
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, &step04TaskEventRepo{}, warehouseTestTxRunner{
		snapshotters: []warehouseTestSnapshotter{taskRepo, warehouseRepo},
	}).(*warehouseService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 4, 18, 10, 45, 41, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         receiverID,
		Roles:      []domain.Role{domain.RoleWarehouse},
		Department: string(domain.DepartmentCloudWarehouse),
		Team:       "warehouse-team",
	})

	receipt, appErr := svc.Receive(ctx, ReceiveWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     "should rollback",
	})
	if appErr == nil {
		t.Fatal("Receive() expected transition error")
	}
	if appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("Receive() code = %s, want %s", appErr.Code, domain.ErrCodeInvalidStateTransition)
	}
	if receipt != nil {
		t.Fatalf("Receive() receipt = %+v, want nil", receipt)
	}
	if warehouseRepo.receipts[taskID] != nil {
		t.Fatalf("warehouse receipt persisted despite rollback: %+v", warehouseRepo.receipts[taskID])
	}
	if taskRepo.tasks[taskID].TaskStatus != domain.TaskStatusPendingWarehouseReceive {
		t.Fatalf("task_status = %s, want PendingWarehouseReceive after rollback", taskRepo.tasks[taskID].TaskStatus)
	}
}

func TestWarehouseRejectAdvancesTaskStatus(t *testing.T) {
	const (
		taskID     = int64(454)
		receiverID = int64(200)
	)
	taskRepo := newWarehouseTestTaskRepo(&domain.Task{
		ID:               taskID,
		TaskNo:           "T-454",
		TaskType:         domain.TaskTypePurchaseTask,
		TaskStatus:       domain.TaskStatusPendingWarehouseReceive,
		CurrentHandlerID: int64Ptr(receiverID),
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "ops-team",
	})
	warehouseRepo := newWarehouseTestReceiptRepo()
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, &step04TaskEventRepo{}, warehouseTestTxRunner{
		snapshotters: []warehouseTestSnapshotter{taskRepo, warehouseRepo},
	}).(*warehouseService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 4, 18, 10, 46, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         receiverID,
		Roles:      []domain.Role{domain.RoleWarehouse},
		Department: string(domain.DepartmentCloudWarehouse),
		Team:       "warehouse-team",
	})

	receipt, appErr := svc.Reject(ctx, RejectWarehouseParams{
		TaskID:       taskID,
		ReceiverID:   receiverID,
		RejectReason: "damaged package",
		Remark:       "reject at intake",
	})
	if appErr != nil {
		t.Fatalf("Reject() appErr = %+v", appErr)
	}
	if taskRepo.tasks[taskID].TaskStatus != domain.TaskStatusRejectedByWarehouse {
		t.Fatalf("task_status = %s, want RejectedByWarehouse", taskRepo.tasks[taskID].TaskStatus)
	}
	if receipt == nil || receipt.Status != domain.WarehouseReceiptStatusRejected {
		t.Fatalf("receipt = %+v, want rejected receipt", receipt)
	}
	if receipt.RejectReason != "damaged package" {
		t.Fatalf("reject_reason = %q, want damaged package", receipt.RejectReason)
	}
}

func TestWarehouseCompleteAdvancesTaskStatus(t *testing.T) {
	const (
		taskID     = int64(455)
		receiverID = int64(200)
	)
	receivedAt := time.Date(2026, 4, 18, 10, 46, 10, 0, time.UTC)
	taskRepo := newWarehouseTestTaskRepo(&domain.Task{
		ID:               taskID,
		TaskNo:           "T-455",
		TaskType:         domain.TaskTypePurchaseTask,
		TaskStatus:       domain.TaskStatusPendingProductionTransfer,
		SKUCode:          "SKU-455",
		CurrentHandlerID: int64Ptr(receiverID),
		OwnerDepartment:  string(domain.DepartmentOperations),
		OwnerOrgTeam:     "ops-team",
	})
	warehouseRepo := newWarehouseTestReceiptRepo(&domain.WarehouseReceipt{
		ID:         1,
		TaskID:     taskID,
		ReceiptNo:  "WR-455",
		Status:     domain.WarehouseReceiptStatusReceived,
		ReceiverID: int64Ptr(receiverID),
		ReceivedAt: &receivedAt,
		CreatedAt:  receivedAt,
		UpdatedAt:  receivedAt,
	})
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, &step04TaskEventRepo{}, warehouseTestTxRunner{
		snapshotters: []warehouseTestSnapshotter{taskRepo, warehouseRepo},
	}).(*warehouseService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 4, 18, 10, 46, 20, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         receiverID,
		Roles:      []domain.Role{domain.RoleWarehouse},
		Department: string(domain.DepartmentCloudWarehouse),
		Team:       "warehouse-team",
	})

	receipt, appErr := svc.Complete(ctx, CompleteWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     "done",
	})
	if appErr != nil {
		t.Fatalf("Complete() appErr = %+v", appErr)
	}
	if taskRepo.tasks[taskID].TaskStatus != domain.TaskStatusPendingClose {
		t.Fatalf("task_status = %s, want PendingClose", taskRepo.tasks[taskID].TaskStatus)
	}
	if receipt == nil || receipt.Status != domain.WarehouseReceiptStatusCompleted {
		t.Fatalf("receipt = %+v, want completed receipt", receipt)
	}
	if receipt.CompletedAt == nil {
		t.Fatalf("completed_at is nil: %+v", receipt)
	}
}

func TestWarehouseReceiptReceivedAtMatchesCreatedAt(t *testing.T) {
	const (
		taskID     = int64(456)
		receiverID = int64(200)
	)
	taskRepo := newWarehouseTestTaskRepo(&domain.Task{
		ID:              taskID,
		TaskNo:          "T-456",
		TaskType:        domain.TaskTypePurchaseTask,
		TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "ops-team",
	})
	warehouseRepo := newWarehouseTestReceiptRepo()
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, &step04TaskEventRepo{}, warehouseTestTxRunner{
		snapshotters: []warehouseTestSnapshotter{taskRepo, warehouseRepo},
	}).(*warehouseService)
	svc.nowFn = func() time.Time {
		return time.Date(2026, 4, 18, 10, 47, 0, 0, time.UTC)
	}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         receiverID,
		Roles:      []domain.Role{domain.RoleWarehouse},
		Department: string(domain.DepartmentCloudWarehouse),
		Team:       "warehouse-team",
	})

	receipt, appErr := svc.Receive(ctx, ReceiveWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
	})
	if appErr != nil {
		t.Fatalf("Receive() appErr = %+v", appErr)
	}
	if receipt == nil || receipt.ReceivedAt == nil {
		t.Fatalf("receipt = %+v, want received_at", receipt)
	}
	if diff := receipt.ReceivedAt.Sub(receipt.CreatedAt); diff < -time.Second || diff > time.Second {
		t.Fatalf("received_at - created_at = %s, want < 1s", diff)
	}
}

func cloneTaskMap(src map[int64]*domain.Task) map[int64]*domain.Task {
	if src == nil {
		return nil
	}
	out := make(map[int64]*domain.Task, len(src))
	for id, task := range src {
		if task == nil {
			out[id] = nil
			continue
		}
		copied := *task
		out[id] = &copied
	}
	return out
}

func cloneReceiptMap(src map[int64]*domain.WarehouseReceipt) map[int64]*domain.WarehouseReceipt {
	if src == nil {
		return nil
	}
	out := make(map[int64]*domain.WarehouseReceipt, len(src))
	for taskID, receipt := range src {
		out[taskID] = cloneWarehouseReceipt(receipt)
	}
	return out
}

func cloneWarehouseReceipt(src *domain.WarehouseReceipt) *domain.WarehouseReceipt {
	if src == nil {
		return nil
	}
	copied := *src
	copied.ReceiverID = cloneInt64Ptr(src.ReceiverID)
	copied.ReceivedAt = cloneTimePtr(src.ReceivedAt)
	copied.CompletedAt = cloneTimePtr(src.CompletedAt)
	return &copied
}

func hasWorkflowReason(reasons []domain.WorkflowReason, code domain.WorkflowReasonCode) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func warehouseFloat64Ptr(v float64) *float64 {
	return &v
}

func warehouseTimePtr(v time.Time) *time.Time {
	return &v
}
