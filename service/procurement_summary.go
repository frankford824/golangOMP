package service

import (
	"time"

	"workflow/domain"
)

func buildProcurementSummary(task *domain.Task, detail *domain.TaskDetail, record *domain.ProcurementRecord, warehouse *domain.WarehouseReceipt, workflow domain.TaskWorkflowSnapshot, matchedRuleGovernance *domain.TaskMatchedRuleGovernance, overrideSummary *domain.TaskCostOverrideSummary, governanceAuditSummary *domain.TaskGovernanceAuditSummary, overrideBoundary *domain.TaskCostOverrideGovernanceBoundary) *domain.ProcurementSummary {
	if task == nil || task.TaskType != domain.TaskTypePurchaseTask || record == nil {
		return nil
	}
	summary := &domain.ProcurementSummary{
		Status:                   record.Status,
		CoordinationStatus:       deriveProcurementCoordinationStatus(task, record, warehouse),
		CoordinationLabel:        deriveProcurementCoordinationLabel(task, record, warehouse),
		WarehouseStatus:          warehouseStatusValue(warehouse),
		WarehousePrepareReady:    workflow.CanPrepareWarehouse,
		WarehouseReceiveReady:    canReceiveInWarehouse(task, warehouse),
		CategoryCode:             procurementCategoryCode(detail),
		CategoryName:             procurementCategoryName(detail),
		ProcurementPrice:         record.ProcurementPrice,
		CostPrice:                procurementCostPrice(detail),
		EstimatedCost:            procurementEstimatedCost(detail),
		CostRuleID:               procurementCostRuleID(detail),
		CostRuleName:             procurementCostRuleName(detail),
		CostRuleSource:           procurementCostRuleSource(detail),
		MatchedRuleVersion:       procurementMatchedRuleVersion(detail),
		PrefillSource:            procurementPrefillSource(detail),
		PrefillAt:                procurementPrefillAt(detail),
		RequiresManualReview:     procurementRequiresManualReview(detail),
		ManualCostOverride:       procurementManualCostOverride(detail),
		ManualCostOverrideReason: procurementManualCostOverrideReason(detail),
		OverrideActor:            procurementOverrideActor(detail),
		OverrideAt:               procurementOverrideAt(detail),
		Quantity:                 record.Quantity,
		SupplierName:             record.SupplierName,
		ExpectedDeliveryAt:       record.ExpectedDeliveryAt,
		ProductSelection:         buildTaskProductSelectionSummaryFromTask(task, detail),
		MatchedRuleGovernance:    matchedRuleGovernance,
		OverrideSummary:          overrideSummary,
		GovernanceAuditSummary:   governanceAuditSummary,
		OverrideBoundary:         overrideBoundary,
	}
	domain.HydrateProcurementSummaryPolicy(summary)
	return summary
}

func buildProcurementSummaryFromListItem(item *domain.TaskListItem) *domain.ProcurementSummary {
	if item == nil || item.ProcurementStatus == nil {
		return nil
	}
	task := &domain.Task{
		TaskType:   item.TaskType,
		TaskStatus: item.TaskStatus,
	}
	record := &domain.ProcurementRecord{
		Status:             *item.ProcurementStatus,
		ProcurementPrice:   item.ProcurementPrice,
		Quantity:           item.ProcurementQuantity,
		SupplierName:       item.SupplierName,
		ExpectedDeliveryAt: item.ExpectedDeliveryAt,
	}
	var warehouse *domain.WarehouseReceipt
	if item.WarehouseStatus != nil {
		warehouse = &domain.WarehouseReceipt{Status: *item.WarehouseStatus}
	}
	summary := &domain.ProcurementSummary{
		Status:                   *item.ProcurementStatus,
		CoordinationStatus:       deriveProcurementCoordinationStatus(task, record, warehouse),
		CoordinationLabel:        deriveProcurementCoordinationLabel(task, record, warehouse),
		WarehouseStatus:          item.WarehouseStatus,
		WarehousePrepareReady:    item.Workflow.CanPrepareWarehouse,
		WarehouseReceiveReady:    canReceiveInWarehouse(task, warehouse),
		CategoryCode:             item.CategoryCode,
		CategoryName:             item.CategoryName,
		ProcurementPrice:         item.ProcurementPrice,
		CostPrice:                item.CostPrice,
		EstimatedCost:            item.EstimatedCost,
		CostRuleID:               item.CostRuleID,
		CostRuleName:             item.CostRuleName,
		CostRuleSource:           item.CostRuleSource,
		MatchedRuleVersion:       item.MatchedRuleVersion,
		PrefillSource:            item.PrefillSource,
		PrefillAt:                item.PrefillAt,
		RequiresManualReview:     item.RequiresManualReview,
		ManualCostOverride:       item.ManualCostOverride,
		ManualCostOverrideReason: item.ManualCostOverrideReason,
		OverrideActor:            item.OverrideActor,
		OverrideAt:               item.OverrideAt,
		Quantity:                 item.ProcurementQuantity,
		SupplierName:             item.SupplierName,
		ExpectedDeliveryAt:       item.ExpectedDeliveryAt,
		ProductSelection:         buildTaskProductSelectionSummaryFromListItem(item),
	}
	domain.HydrateProcurementSummaryPolicy(summary)
	return summary
}

func procurementCategoryCode(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.CategoryCode
}

func procurementCategoryName(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.CategoryName
}

func procurementCostPrice(detail *domain.TaskDetail) *float64 {
	if detail == nil {
		return nil
	}
	return detail.CostPrice
}

func procurementEstimatedCost(detail *domain.TaskDetail) *float64 {
	if detail == nil {
		return nil
	}
	return detail.EstimatedCost
}

func procurementCostRuleName(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.CostRuleName
}

func procurementCostRuleID(detail *domain.TaskDetail) *int64 {
	if detail == nil {
		return nil
	}
	return detail.CostRuleID
}

func procurementCostRuleSource(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.CostRuleSource
}

func procurementMatchedRuleVersion(detail *domain.TaskDetail) *int {
	if detail == nil {
		return nil
	}
	return detail.MatchedRuleVersion
}

func procurementPrefillSource(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.PrefillSource
}

func procurementPrefillAt(detail *domain.TaskDetail) *time.Time {
	if detail == nil {
		return nil
	}
	return detail.PrefillAt
}

func procurementRequiresManualReview(detail *domain.TaskDetail) bool {
	return detail != nil && detail.RequiresManualReview
}

func procurementManualCostOverride(detail *domain.TaskDetail) bool {
	return detail != nil && detail.ManualCostOverride
}

func procurementManualCostOverrideReason(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.ManualCostOverrideReason
}

func procurementOverrideActor(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return detail.OverrideActor
}

func procurementOverrideAt(detail *domain.TaskDetail) *time.Time {
	if detail == nil {
		return nil
	}
	return detail.OverrideAt
}

func deriveProcurementCoordinationStatus(task *domain.Task, record *domain.ProcurementRecord, warehouse *domain.WarehouseReceipt) domain.ProcurementCoordinationStatus {
	switch {
	case task == nil || record == nil:
		return domain.ProcurementCoordinationStatusPreparing
	case task.TaskStatus == domain.TaskStatusPendingClose || task.TaskStatus == domain.TaskStatusCompleted:
		return domain.ProcurementCoordinationStatusWarehouseDone
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusCompleted:
		return domain.ProcurementCoordinationStatusWarehouseDone
	case task.TaskStatus == domain.TaskStatusPendingWarehouseReceive:
		return domain.ProcurementCoordinationStatusHandedToWarehouse
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusReceived:
		return domain.ProcurementCoordinationStatusHandedToWarehouse
	case record.Status == domain.ProcurementStatusCompleted:
		return domain.ProcurementCoordinationStatusReadyForWarehouse
	case record.Status == domain.ProcurementStatusInProgress:
		return domain.ProcurementCoordinationStatusAwaitingArrival
	default:
		return domain.ProcurementCoordinationStatusPreparing
	}
}

func deriveProcurementCoordinationLabel(task *domain.Task, record *domain.ProcurementRecord, warehouse *domain.WarehouseReceipt) string {
	switch deriveProcurementCoordinationStatus(task, record, warehouse) {
	case domain.ProcurementCoordinationStatusAwaitingArrival:
		return "Awaiting arrival"
	case domain.ProcurementCoordinationStatusReadyForWarehouse:
		return "Ready for warehouse"
	case domain.ProcurementCoordinationStatusHandedToWarehouse:
		return "Handed to warehouse"
	case domain.ProcurementCoordinationStatusWarehouseDone:
		return "Warehouse completed"
	default:
		return "Preparing procurement"
	}
}

func warehouseStatusValue(warehouse *domain.WarehouseReceipt) *domain.WarehouseReceiptStatus {
	if warehouse == nil {
		return nil
	}
	status := warehouse.Status
	return &status
}

func canReceiveInWarehouse(task *domain.Task, warehouse *domain.WarehouseReceipt) bool {
	return task != nil &&
		task.TaskStatus == domain.TaskStatusPendingWarehouseReceive &&
		(warehouse == nil || warehouse.Status == domain.WarehouseReceiptStatusRejected)
}
