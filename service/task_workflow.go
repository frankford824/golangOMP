package service

import (
	"strings"

	"workflow/domain"
)

func buildTaskWorkflowSnapshot(task *domain.Task, detail *domain.TaskDetail, procurement *domain.ProcurementRecord, hasDeliveryAsset bool, warehouse *domain.WarehouseReceipt) domain.TaskWorkflowSnapshot {
	snapshot := domain.TaskWorkflowSnapshot{
		MainStatus:               deriveTaskMainStatus(task, detail, warehouse),
		SubStatus:                deriveTaskSubStatus(task, procurement, hasDeliveryAsset, warehouse),
		WarehouseBlockingReasons: warehouseBlockingReasons(task, detail, procurement, hasDeliveryAsset, warehouse),
		CannotCloseReasons:       cannotCloseReasons(task, detail, procurement, hasDeliveryAsset, warehouse),
	}
	snapshot.CanPrepareWarehouse = len(snapshot.WarehouseBlockingReasons) == 0
	snapshot.CanClose = len(snapshot.CannotCloseReasons) == 0
	snapshot.Closable = snapshot.CanClose
	return snapshot
}

func filingCompleted(detail *domain.TaskDetail) bool {
	if detail == nil {
		return false
	}
	if detail.FilingStatus.Valid() {
		return detail.FilingStatus == domain.FilingStatusFiled
	}
	// Backward compatibility for legacy records before filing_status formalization.
	return detail.FiledAt != nil
}

func buildTaskWorkflowSnapshotFromListItem(item *domain.TaskListItem) domain.TaskWorkflowSnapshot {
	if item == nil {
		return domain.TaskWorkflowSnapshot{}
	}

	task := &domain.Task{
		TaskType:              item.TaskType,
		TaskStatus:            item.TaskStatus,
		TaskNo:                item.TaskNo,
		SKUCode:               item.SKUCode,
		NeedOutsource:         item.NeedOutsource,
		CustomizationRequired: item.CustomizationRequired,
	}
	detail := &domain.TaskDetail{
		Category:     item.Category,
		SpecText:     item.SpecText,
		Material:     item.Material,
		SizeText:     item.SizeText,
		CraftText:    item.CraftText,
		CostPrice:    item.CostPrice,
		FilingStatus: item.FilingStatus,
		FiledAt:      item.FiledAt,
	}

	var procurement *domain.ProcurementRecord
	if item.ProcurementStatus != nil || item.ProcurementPrice != nil {
		status := domain.ProcurementStatusDraft
		if item.ProcurementStatus != nil {
			status = *item.ProcurementStatus
		}
		procurement = &domain.ProcurementRecord{
			Status:             status,
			ProcurementPrice:   item.ProcurementPrice,
			Quantity:           item.ProcurementQuantity,
			SupplierName:       item.SupplierName,
			ExpectedDeliveryAt: item.ExpectedDeliveryAt,
		}
	}

	hasDeliveryAsset := item.LatestAssetType != nil && item.LatestAssetType.IsDelivery()
	var warehouse *domain.WarehouseReceipt
	if item.WarehouseStatus != nil {
		warehouse = &domain.WarehouseReceipt{Status: *item.WarehouseStatus}
	}
	return buildTaskWorkflowSnapshot(task, detail, procurement, hasDeliveryAsset, warehouse)
}

func hasFinalTaskAsset(assets []*domain.TaskAsset) bool {
	for _, asset := range assets {
		if asset != nil && domain.NormalizeTaskAssetType(asset.AssetType).IsDelivery() {
			return true
		}
	}
	return false
}

func deriveTaskMainStatus(task *domain.Task, detail *domain.TaskDetail, warehouse *domain.WarehouseReceipt) domain.TaskMainStatus {
	if task == nil {
		return domain.TaskMainStatusDraft
	}

	switch {
	case task.TaskStatus == domain.TaskStatusCompleted:
		return domain.TaskMainStatusClosed
	case task.TaskStatus == domain.TaskStatusPendingClose:
		return domain.TaskMainStatusPendingClose
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusCompleted:
		return domain.TaskMainStatusPendingClose
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusReceived:
		return domain.TaskMainStatusWarehouseProcessing
	case task.TaskStatus == domain.TaskStatusPendingWarehouseReceive:
		return domain.TaskMainStatusPendingWarehouseReceive
	case task.TaskStatus == domain.TaskStatusPendingCustomizationReview,
		task.TaskStatus == domain.TaskStatusPendingCustomizationProduction,
		task.TaskStatus == domain.TaskStatusPendingEffectReview,
		task.TaskStatus == domain.TaskStatusPendingEffectRevision,
		task.TaskStatus == domain.TaskStatusPendingProductionTransfer,
		task.TaskStatus == domain.TaskStatusPendingWarehouseQC,
		task.TaskStatus == domain.TaskStatusRejectedByWarehouse:
		return domain.TaskMainStatusCreated
	case filingCompleted(detail):
		return domain.TaskMainStatusFiled
	default:
		return domain.TaskMainStatusCreated
	}
}

func deriveTaskSubStatus(task *domain.Task, procurement *domain.ProcurementRecord, hasDeliveryAsset bool, warehouse *domain.WarehouseReceipt) domain.TaskSubStatusSnapshot {
	if task == nil {
		return domain.TaskSubStatusSnapshot{}
	}
	customization := deriveOutsourceSubStatus(task)

	return domain.TaskSubStatusSnapshot{
		Design:        deriveDesignSubStatus(task, hasDeliveryAsset),
		Audit:         deriveAuditSubStatus(task),
		Procurement:   deriveProcurementSubStatus(task, procurement, warehouse),
		Warehouse:     deriveWarehouseSubStatus(task, warehouse),
		Customization: customization,
		Outsource:     customization,
		Production:    statusItem(domain.TaskSubStatusReserved, "Reserved", domain.TaskSubStatusSourceReserved),
	}
}

func deriveDesignSubStatus(task *domain.Task, hasDeliveryAsset bool) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresDesign() {
		return statusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskType)
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingCustomizationReview,
		domain.TaskStatusPendingCustomizationProduction,
		domain.TaskStatusPendingEffectReview,
		domain.TaskStatusPendingEffectRevision,
		domain.TaskStatusPendingProductionTransfer,
		domain.TaskStatusPendingWarehouseQC,
		domain.TaskStatusRejectedByWarehouse:
		// Customization lane bypasses normal design workbench.
		return statusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskStatus)
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingAssign:
		return statusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusInProgress:
		return statusItem(domain.TaskSubStatusDesigning, "Designing", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB, domain.TaskStatusBlocked:
		return statusItem(domain.TaskSubStatusReworkRequired, "Rework required", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview:
		return statusItem(domain.TaskSubStatusPendingAudit, "Pending audit", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsource, domain.TaskStatusOutsourcing:
		return statusItem(domain.TaskSubStatusOutsourcing, "Outsourcing", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		if hasDeliveryAsset {
			return statusItem(domain.TaskSubStatusFinalReady, "Final ready", domain.TaskSubStatusSourceTaskAsset)
		}
	}

	if hasDeliveryAsset {
		return statusItem(domain.TaskSubStatusFinalReady, "Final ready", domain.TaskSubStatusSourceTaskAsset)
	}
	return statusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
}

func deriveAuditSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresAudit() {
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview:
		return statusItem(domain.TaskSubStatusInReview, "In review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB, domain.TaskStatusBlocked:
		return statusItem(domain.TaskSubStatusRejected, "Rejected", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsource, domain.TaskStatusOutsourcing:
		return statusItem(domain.TaskSubStatusOutsourced, "Outsourced", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return statusItem(domain.TaskSubStatusApproved, "Approved", domain.TaskSubStatusSourceTaskStatus)
	default:
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
}

func deriveProcurementSubStatus(task *domain.Task, procurement *domain.ProcurementRecord, warehouse *domain.WarehouseReceipt) domain.TaskSubStatusItem {
	if task == nil || task.TaskType != domain.TaskTypePurchaseTask {
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}

	switch {
	case task.TaskStatus == domain.TaskStatusPendingClose || task.TaskStatus == domain.TaskStatusCompleted:
		return statusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusCompleted:
		return statusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceWarehouseReceipt)
	case procurement == nil:
		return statusItem(domain.TaskSubStatusNotStarted, "Not started", domain.TaskSubStatusSourceTaskType)
	case procurement.Status == domain.ProcurementStatusCompleted:
		return statusItem(domain.TaskSubStatusReady, "Ready for warehouse", domain.TaskSubStatusSourceProcurement)
	case procurement.Status == domain.ProcurementStatusInProgress:
		return statusItem(domain.TaskSubStatusPendingInbound, "Awaiting arrival", domain.TaskSubStatusSourceProcurement)
	default:
		return statusItem(domain.TaskSubStatusPreparing, "Preparing", domain.TaskSubStatusSourceProcurement)
	}
}

func deriveWarehouseSubStatus(task *domain.Task, warehouse *domain.WarehouseReceipt) domain.TaskSubStatusItem {
	switch {
	case task == nil:
		return domain.TaskSubStatusItem{}
	case task.TaskStatus == domain.TaskStatusPendingWarehouseReceive &&
		(warehouse == nil || warehouse.Status == domain.WarehouseReceiptStatusRejected):
		return statusItem(domain.TaskSubStatusPendingReceive, "Pending receive", domain.TaskSubStatusSourceTaskStatus)
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusCompleted:
		return statusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceWarehouseReceipt)
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusReceived:
		return statusItem(domain.TaskSubStatusReceived, "Received", domain.TaskSubStatusSourceWarehouseReceipt)
	case warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusRejected:
		return statusItem(domain.TaskSubStatusRejected, "Rejected", domain.TaskSubStatusSourceWarehouseReceipt)
	default:
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
}

func deriveOutsourceSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil {
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}
	if !task.CustomizationRequired &&
		!task.NeedOutsource &&
		task.TaskStatus != domain.TaskStatusPendingOutsource &&
		task.TaskStatus != domain.TaskStatusOutsourcing &&
		task.TaskStatus != domain.TaskStatusPendingOutsourceReview {
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingCustomizationReview:
		return statusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingCustomizationProduction, domain.TaskStatusPendingEffectRevision:
		return statusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingEffectReview:
		return statusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingProductionTransfer:
		return statusItem(domain.TaskSubStatusReady, "Ready", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseQC:
		return statusItem(domain.TaskSubStatusPendingReceive, "Pending warehouse QC", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByWarehouse:
		return statusItem(domain.TaskSubStatusRejected, "Rejected", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsource, domain.TaskStatusOutsourcing:
		return statusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsourceReview:
		return statusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return statusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
	default:
		return statusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
}

func warehouseBlockingReasons(task *domain.Task, detail *domain.TaskDetail, procurement *domain.ProcurementRecord, hasDeliveryAsset bool, warehouse *domain.WarehouseReceipt) []domain.WorkflowReason {
	if task == nil {
		return []domain.WorkflowReason{reason(domain.WorkflowReasonTaskNotFound, "Task does not exist.")}
	}

	reasons := commonBusinessInfoReasons(detail)
	reasons = append(reasons, procurementWarehouseReadinessReasons(task, procurement)...)

	switch task.TaskStatus {
	case domain.TaskStatusPendingWarehouseReceive:
		reasons = append(reasons, reason(domain.WorkflowReasonTaskAlreadyPendingWH, "Task is already pending warehouse receive."))
	case domain.TaskStatusPendingClose:
		reasons = append(reasons, reason(domain.WorkflowReasonTaskAwaitingClose, "Task is already awaiting close."))
	case domain.TaskStatusCompleted:
		reasons = append(reasons, reason(domain.WorkflowReasonTaskAlreadyClosed, "Task is already closed."))
	case domain.TaskStatusBlocked:
		reasons = append(reasons, reason(domain.WorkflowReasonTaskBlocked, "Task is blocked and must be resolved first."))
	}

	if warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusReceived {
		reasons = append(reasons, reason(domain.WorkflowReasonWarehouseAlreadyReceived, "Warehouse has already received the task."))
	}
	if warehouse != nil && warehouse.Status == domain.WarehouseReceiptStatusCompleted {
		reasons = append(reasons, reason(domain.WorkflowReasonWarehouseAlreadyDone, "Warehouse has already completed the task."))
	}

	if task.TaskType.RequiresDesign() {
		if !hasDeliveryAsset {
			reasons = append(reasons, reason(domain.WorkflowReasonMissingFinalAsset, "Final design asset is missing."))
		}
		if !auditApproved(task) {
			reasons = append(reasons, reason(domain.WorkflowReasonAuditNotApproved, "Audit has not been approved yet."))
		}
	}

	return uniqueReasons(reasons)
}

func cannotCloseReasons(task *domain.Task, detail *domain.TaskDetail, procurement *domain.ProcurementRecord, hasDeliveryAsset bool, warehouse *domain.WarehouseReceipt) []domain.WorkflowReason {
	if task == nil {
		return []domain.WorkflowReason{reason(domain.WorkflowReasonTaskNotFound, "Task does not exist.")}
	}

	reasons := commonBusinessInfoReasons(detail)
	reasons = append(reasons, procurementCloseReadinessReasons(task, procurement)...)

	if task.TaskStatus == domain.TaskStatusCompleted {
		reasons = append(reasons, reason(domain.WorkflowReasonTaskAlreadyClosed, "Task is already closed."))
	}
	if deriveTaskMainStatus(task, detail, warehouse) != domain.TaskMainStatusPendingClose {
		reasons = append(reasons, reason(domain.WorkflowReasonNotPendingClose, "Task is not in pending-close state."))
	}
	if task.TaskNo == "" {
		reasons = append(reasons, reason(domain.WorkflowReasonMissingTaskNo, "Task number is missing."))
	}
	if task.SKUCode == "" {
		reasons = append(reasons, reason(domain.WorkflowReasonMissingSKU, "SKU is missing."))
	}

	if task.TaskType.RequiresDesign() {
		if !hasDeliveryAsset {
			reasons = append(reasons, reason(domain.WorkflowReasonMissingFinalAsset, "Final design asset is missing."))
		}
		if !auditApproved(task) {
			reasons = append(reasons, reason(domain.WorkflowReasonAuditNotApproved, "Audit has not been approved yet."))
		}
	}

	switch {
	case warehouse == nil:
		reasons = append(reasons, reason(domain.WorkflowReasonWarehouseNotReceived, "Warehouse has not received the task."))
	case warehouse.Status == domain.WarehouseReceiptStatusRejected:
		reasons = append(reasons, reason(domain.WorkflowReasonWarehouseRejected, "Warehouse rejection is still pending resolution."))
	case warehouse.Status != domain.WarehouseReceiptStatusCompleted:
		reasons = append(reasons, reason(domain.WorkflowReasonWarehouseNotCompleted, "Warehouse flow is not completed yet."))
	}

	if task.TaskStatus == domain.TaskStatusBlocked ||
		task.TaskStatus == domain.TaskStatusRejectedByAuditA ||
		task.TaskStatus == domain.TaskStatusRejectedByAuditB {
		reasons = append(reasons, reason(domain.WorkflowReasonPendingException, "Task still has unresolved exception states."))
	}

	return uniqueReasons(reasons)
}

func commonBusinessInfoReasons(detail *domain.TaskDetail) []domain.WorkflowReason {
	reasons := []domain.WorkflowReason{}
	if detail == nil {
		return []domain.WorkflowReason{reason(domain.WorkflowReasonTaskDetailMissing, "Task detail is missing.")}
	}
	if !filingCompleted(detail) {
		reasons = append(reasons, reason(domain.WorkflowReasonFiledAtMissing, "Filed/ERP linkage is missing."))
	}
	if strings.TrimSpace(detail.Category) == "" {
		reasons = append(reasons, reason(domain.WorkflowReasonCategoryMissing, "Category is missing."))
	}
	if strings.TrimSpace(detail.SpecText) == "" {
		reasons = append(reasons, reason(domain.WorkflowReasonSpecMissing, "Specification text is missing."))
	}
	if detail.CostPrice == nil {
		reasons = append(reasons, reason(domain.WorkflowReasonCostPriceMissing, "Cost price is missing."))
	}
	return reasons
}

func procurementWarehouseReadinessReasons(task *domain.Task, procurement *domain.ProcurementRecord) []domain.WorkflowReason {
	if task == nil || task.TaskType != domain.TaskTypePurchaseTask {
		return []domain.WorkflowReason{}
	}
	if procurement == nil {
		return []domain.WorkflowReason{reason(domain.WorkflowReasonProcurementMissing, "Procurement record is missing.")}
	}

	reasons := []domain.WorkflowReason{}
	if procurement.ProcurementPrice == nil {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementPriceMissing, "Procurement price is missing."))
	}
	if procurement.Quantity == nil || *procurement.Quantity <= 0 {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementQuantityMissing, "Procurement quantity is missing."))
	}
	if !procurement.Status.AllowsWarehousePrepare() {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementNotReady, "Procurement arrival is not completed yet."))
	}
	return reasons
}

func procurementCloseReadinessReasons(task *domain.Task, procurement *domain.ProcurementRecord) []domain.WorkflowReason {
	if task == nil || task.TaskType != domain.TaskTypePurchaseTask {
		return []domain.WorkflowReason{}
	}
	if procurement == nil {
		return []domain.WorkflowReason{reason(domain.WorkflowReasonProcurementMissing, "Procurement record is missing.")}
	}

	reasons := []domain.WorkflowReason{}
	if procurement.ProcurementPrice == nil {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementPriceMissing, "Procurement price is missing."))
	}
	if procurement.Quantity == nil || *procurement.Quantity <= 0 {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementQuantityMissing, "Procurement quantity is missing."))
	}
	if !procurement.Status.AllowsClose() {
		reasons = append(reasons, reason(domain.WorkflowReasonProcurementNotReady, "Procurement is not completed yet."))
	}
	return reasons
}

func auditApproved(task *domain.Task) bool {
	if task == nil || !task.TaskType.RequiresAudit() {
		return true
	}

	switch task.TaskStatus {
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return true
	default:
		return false
	}
}

func warehouseReadinessErrorDetails(task *domain.Task, workflow domain.TaskWorkflowSnapshot) map[string]interface{} {
	missingFields, missingSummaryCN := missingFieldsFromReasons(workflow.WarehouseBlockingReasons)
	return map[string]interface{}{
		"task_type":                  task.TaskType,
		"workflow":                   workflow,
		"can_prepare_warehouse":      workflow.CanPrepareWarehouse,
		"warehouse_blocking_reasons": workflow.WarehouseBlockingReasons,
		"missing_fields":             missingFields,
		"missing_fields_summary_cn":  missingSummaryCN,
	}
}

func closeReadinessErrorDetails(task *domain.Task, workflow domain.TaskWorkflowSnapshot) map[string]interface{} {
	missingFields, missingSummaryCN := missingFieldsFromReasons(workflow.CannotCloseReasons)
	return map[string]interface{}{
		"task_type":                 task.TaskType,
		"workflow":                  workflow,
		"closable":                  workflow.Closable,
		"cannot_close_reasons":      workflow.CannotCloseReasons,
		"main_status":               workflow.MainStatus,
		"sub_status":                workflow.SubStatus,
		"missing_fields":            missingFields,
		"missing_fields_summary_cn": missingSummaryCN,
	}
}

func missingFieldsFromReasons(reasons []domain.WorkflowReason) ([]string, string) {
	if len(reasons) == 0 {
		return []string{}, ""
	}
	fields := make([]string, 0, len(reasons))
	labels := make([]string, 0, len(reasons))
	seen := map[string]struct{}{}
	appendField := func(field, label string) {
		if field == "" || label == "" {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
		labels = append(labels, label)
	}
	for _, r := range reasons {
		switch r.Code {
		case domain.WorkflowReasonFiledAtMissing:
			appendField("filed_at", "建档时间")
		case domain.WorkflowReasonCategoryMissing:
			appendField("category", "类目")
		case domain.WorkflowReasonSpecMissing:
			appendField("spec_text", "规格")
		case domain.WorkflowReasonCostPriceMissing:
			appendField("cost_price", "成本价")
		case domain.WorkflowReasonProcurementMissing:
			appendField("procurement_record", "采购记录")
		case domain.WorkflowReasonProcurementPriceMissing:
			appendField("procurement_price", "采购价")
		case domain.WorkflowReasonProcurementQuantityMissing:
			appendField("procurement_quantity", "采购数量")
		case domain.WorkflowReasonMissingFinalAsset:
			appendField("delivery_asset", "交付稿")
		case domain.WorkflowReasonMissingTaskNo:
			appendField("task_no", "任务编号")
		case domain.WorkflowReasonMissingSKU:
			appendField("sku_code", "SKU")
		}
	}
	if len(labels) == 0 {
		return []string{}, ""
	}
	return fields, "缺少" + strings.Join(labels, "、")
}

func reason(code domain.WorkflowReasonCode, message string) domain.WorkflowReason {
	return domain.WorkflowReason{Code: code, Message: message}
}

func statusItem(code domain.TaskSubStatusCode, label string, source domain.TaskSubStatusSource) domain.TaskSubStatusItem {
	return domain.TaskSubStatusItem{
		Code:   code,
		Label:  label,
		Source: source,
	}
}

func uniqueReasons(reasons []domain.WorkflowReason) []domain.WorkflowReason {
	if len(reasons) == 0 {
		return []domain.WorkflowReason{}
	}

	seen := map[domain.WorkflowReasonCode]struct{}{}
	out := make([]domain.WorkflowReason, 0, len(reasons))
	for _, item := range reasons {
		item.Message = strings.TrimSpace(item.Message)
		if item.Code == "" || item.Message == "" {
			continue
		}
		if _, ok := seen[item.Code]; ok {
			continue
		}
		seen[item.Code] = struct{}{}
		out = append(out, item)
	}
	return out
}
