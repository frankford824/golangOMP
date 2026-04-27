package service

import (
	"reflect"
	"testing"
	"time"

	"workflow/domain"
)

func TestAvailableActionsForTask(t *testing.T) {
	tests := []struct {
		name             string
		taskType         domain.TaskType
		taskStatus       domain.TaskStatus
		warehouseReceipt *domain.WarehouseReceipt
		procurement      *domain.ProcurementRecord
		detail           *domain.TaskDetail
		assets           []*domain.TaskAsset
		want             []domain.AvailableAction
	}{
		{
			name:       "pending assign",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingAssign,
			want:       []domain.AvailableAction{domain.AvailableActionAssign},
		},
		{
			name:       "in progress",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusInProgress,
			want:       []domain.AvailableAction{domain.AvailableActionSubmitDesign},
		},
		{
			name:       "pending audit a",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingAuditA,
			want: []domain.AvailableAction{
				domain.AvailableActionClaimAudit,
				domain.AvailableActionApproveAudit,
				domain.AvailableActionRejectAudit,
				domain.AvailableActionHandover,
			},
		},
		{
			name:       "pending outsource",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingOutsource,
			want:       []domain.AvailableAction{domain.AvailableActionCreateOutsource},
		},
		{
			name:       "pending warehouse without receipt",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingWarehouseReceive,
			want: []domain.AvailableAction{
				domain.AvailableActionWarehouseReceive,
				domain.AvailableActionWarehouseReject,
			},
		},
		{
			name:       "pending warehouse after receive",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingWarehouseReceive,
			warehouseReceipt: &domain.WarehouseReceipt{
				Status: domain.WarehouseReceiptStatusReceived,
			},
			want: []domain.AvailableAction{
				domain.AvailableActionWarehouseReject,
				domain.AvailableActionWarehouseComplete,
			},
		},
		{
			name:       "pending warehouse after reject allows receive again",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusPendingWarehouseReceive,
			warehouseReceipt: &domain.WarehouseReceipt{
				Status: domain.WarehouseReceiptStatusRejected,
			},
			want: []domain.AvailableAction{
				domain.AvailableActionWarehouseReceive,
				domain.AvailableActionWarehouseReject,
			},
		},
		{
			name:       "purchase task can prepare warehouse after procurement complete",
			taskType:   domain.TaskTypePurchaseTask,
			taskStatus: domain.TaskStatusPendingAssign,
			detail: &domain.TaskDetail{
				Category:  "Accessory",
				SpecText:  "Spec-A",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
			},
			procurement: &domain.ProcurementRecord{
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(8.8),
				Quantity:         int64Ptr(10),
			},
			want: []domain.AvailableAction{
				domain.AvailableActionPrepareWarehouse,
			},
		},
		{
			name:       "purchase task awaiting arrival cannot prepare warehouse",
			taskType:   domain.TaskTypePurchaseTask,
			taskStatus: domain.TaskStatusPendingAssign,
			detail: &domain.TaskDetail{
				Category:  "Accessory",
				SpecText:  "Spec-A",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
			},
			procurement: &domain.ProcurementRecord{
				Status:           domain.ProcurementStatusInProgress,
				ProcurementPrice: float64Ptr(8.8),
				Quantity:         int64Ptr(10),
			},
			want: []domain.AvailableAction{},
		},
		{
			name:       "purchase task draft procurement has no design actions at entry",
			taskType:   domain.TaskTypePurchaseTask,
			taskStatus: domain.TaskStatusPendingAssign,
			procurement: &domain.ProcurementRecord{
				Status: domain.ProcurementStatusDraft,
			},
			want: []domain.AvailableAction{},
		},
		{
			name:       "pending close exposes close action",
			taskType:   domain.TaskTypePurchaseTask,
			taskStatus: domain.TaskStatusPendingClose,
			detail: &domain.TaskDetail{
				Category:  "Accessory",
				SpecText:  "Spec-B",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
			},
			procurement: &domain.ProcurementRecord{
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(8.8),
				Quantity:         int64Ptr(10),
			},
			warehouseReceipt: &domain.WarehouseReceipt{
				Status: domain.WarehouseReceiptStatusCompleted,
			},
			want: []domain.AvailableAction{
				domain.AvailableActionClose,
			},
		},
		{
			name:       "rejected by audit b can resubmit design",
			taskType:   domain.TaskTypeOriginalProductDevelopment,
			taskStatus: domain.TaskStatusRejectedByAuditB,
			want:       []domain.AvailableAction{domain.AvailableActionSubmitDesign},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := availableActionsForTask(&domain.Task{
				TaskType:   tt.taskType,
				TaskStatus: tt.taskStatus,
				TaskNo:     "RW-TEST",
				SKUCode:    "SKU-TEST",
			}, tt.detail, tt.procurement, tt.assets, tt.warehouseReceipt)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("availableActionsForTask() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func timePtr() *time.Time {
	now := time.Now()
	return &now
}
