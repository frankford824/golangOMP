package domain

// TaskStatus for V7 Task state machine (spec V7 §8.1).
type TaskStatus string

const (
	TaskStatusDraft                          TaskStatus = "Draft"
	TaskStatusPendingAssign                  TaskStatus = "PendingAssign"
	TaskStatusAssigned                       TaskStatus = "Assigned"
	TaskStatusInProgress                     TaskStatus = "InProgress"
	TaskStatusPendingAuditA                  TaskStatus = "PendingAuditA"
	TaskStatusRejectedByAuditA               TaskStatus = "RejectedByAuditA"
	TaskStatusPendingAuditB                  TaskStatus = "PendingAuditB"
	TaskStatusRejectedByAuditB               TaskStatus = "RejectedByAuditB"
	TaskStatusPendingOutsource               TaskStatus = "PendingOutsource"
	TaskStatusOutsourcing                    TaskStatus = "Outsourcing"
	TaskStatusPendingOutsourceReview         TaskStatus = "PendingOutsourceReview"
	TaskStatusPendingCustomizationReview     TaskStatus = "PendingCustomizationReview"
	TaskStatusPendingCustomizationProduction TaskStatus = "PendingCustomizationProduction"
	TaskStatusPendingEffectReview            TaskStatus = "PendingEffectReview"
	TaskStatusPendingEffectRevision          TaskStatus = "PendingEffectRevision"
	TaskStatusPendingProductionTransfer      TaskStatus = "PendingProductionTransfer"
	TaskStatusPendingWarehouseQC             TaskStatus = "PendingWarehouseQC"
	TaskStatusRejectedByWarehouse            TaskStatus = "RejectedByWarehouse"
	TaskStatusPendingWarehouseReceive        TaskStatus = "PendingWarehouseReceive"
	TaskStatusPendingClose                   TaskStatus = "PendingClose"
	TaskStatusCompleted                      TaskStatus = "Completed"
	TaskStatusArchived                       TaskStatus = "Archived"
	TaskStatusBlocked                        TaskStatus = "Blocked"
	TaskStatusCancelled                      TaskStatus = "Cancelled"
)

// TaskType identifies the task business category.
type TaskType string

const (
	TaskTypeOriginalProductDevelopment TaskType = "original_product_development"
	TaskTypeNewProductDevelopment      TaskType = "new_product_development"
	TaskTypePurchaseTask               TaskType = "purchase_task"
)

// TaskSourceMode controls how a Task binds its SKU (spec V7 §4.2).
type TaskSourceMode string

const (
	TaskSourceModeExistingProduct TaskSourceMode = "existing_product"
	TaskSourceModeNewProduct      TaskSourceMode = "new_product"
)

// TaskPriority for task scheduling.
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityNormal   TaskPriority = "normal"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// MaterialMode for new product material selection.
type MaterialMode string

const (
	MaterialModePreset MaterialMode = "preset"
	MaterialModeOther  MaterialMode = "other"
)

func (m MaterialMode) Valid() bool {
	switch m {
	case MaterialModePreset, MaterialModeOther:
		return true
	default:
		return false
	}
}

// CostPriceMode for cost price entry.
type CostPriceMode string

const (
	CostPriceModeManual   CostPriceMode = "manual"
	CostPriceModeTemplate CostPriceMode = "template"
)

func (m CostPriceMode) Valid() bool {
	switch m {
	case CostPriceModeManual, CostPriceModeTemplate:
		return true
	default:
		return false
	}
}

// CodeRuleType identifies the numbering namespace (spec V7 §5.1).
type CodeRuleType string

const (
	CodeRuleTypeTaskNo      CodeRuleType = "task_no"
	CodeRuleTypeNewSKU      CodeRuleType = "new_sku"
	CodeRuleTypeOutsourceNo CodeRuleType = "outsource_no"
	CodeRuleTypeHandoverNo  CodeRuleType = "handover_no"
)

// ResetCycle controls when a CodeRule sequence resets.
type ResetCycle string

const (
	ResetCycleNone    ResetCycle = "none"
	ResetCycleDaily   ResetCycle = "daily"
	ResetCycleMonthly ResetCycle = "monthly"
)

// AuditRecordStage identifies the audit gate in V7 AuditRecord.
// Distinct from V6 AuditStage (which is SKU-scoped and has only A/B).
type AuditRecordStage string

const (
	AuditRecordStageA               AuditRecordStage = "A"
	AuditRecordStageB               AuditRecordStage = "B"
	AuditRecordStageOutsourceReview AuditRecordStage = "outsource_review"
)

// AuditActionType enumerates the V7 task-centric audit actions.
// Distinct from V6 AuditDecision (Approve/Reject only).
type AuditActionType string

const (
	AuditActionTypeClaim    AuditActionType = "claim"
	AuditActionTypeApprove  AuditActionType = "approve"
	AuditActionTypeReject   AuditActionType = "reject"
	AuditActionTypeTransfer AuditActionType = "transfer"
	AuditActionTypeHandover AuditActionType = "handover"
	AuditActionTypeTakeover AuditActionType = "takeover"
)

// AuditIssueCategory provides structured tagging for audit rejection reasons.
type AuditIssueCategory string

const (
	AuditIssueCategoryColorError   AuditIssueCategory = "color_error"
	AuditIssueCategorySizeError    AuditIssueCategory = "size_error"
	AuditIssueCategoryTextError    AuditIssueCategory = "text_error"
	AuditIssueCategoryLayoutError  AuditIssueCategory = "layout_error"
	AuditIssueCategoryQualityIssue AuditIssueCategory = "quality_issue"
	AuditIssueCategoryOther        AuditIssueCategory = "other"
)

// HandoverStatus for AuditHandover lifecycle.
type HandoverStatus string

const (
	HandoverStatusPendingTakeover HandoverStatus = "pending_takeover"
	HandoverStatusTakenOver       HandoverStatus = "taken_over"
	HandoverStatusCancelled       HandoverStatus = "cancelled"
)

// OutsourceStatus for OutsourceOrder lifecycle (spec V7 §8.5).
type OutsourceStatus string

const (
	OutsourceStatusCreated      OutsourceStatus = "created"
	OutsourceStatusPackaged     OutsourceStatus = "packaged"
	OutsourceStatusSent         OutsourceStatus = "sent"
	OutsourceStatusInProduction OutsourceStatus = "in_production"
	OutsourceStatusReturned     OutsourceStatus = "returned"
	OutsourceStatusReviewing    OutsourceStatus = "reviewing"
	OutsourceStatusApproved     OutsourceStatus = "approved"
	OutsourceStatusRejected     OutsourceStatus = "rejected"
	OutsourceStatusClosed       OutsourceStatus = "closed"
)

// WarehouseReceiptStatus for warehouse receipt lifecycle.
type WarehouseReceiptStatus string

const (
	WarehouseReceiptStatusReceived  WarehouseReceiptStatus = "received"
	WarehouseReceiptStatusRejected  WarehouseReceiptStatus = "rejected"
	WarehouseReceiptStatusCompleted WarehouseReceiptStatus = "completed"
)

// ProcurementStatus captures the explicit purchase-preparation lifecycle.
type ProcurementStatus string

const (
	ProcurementStatusDraft      ProcurementStatus = "draft"
	ProcurementStatusPrepared   ProcurementStatus = "prepared"
	ProcurementStatusInProgress ProcurementStatus = "in_progress"
	ProcurementStatusCompleted  ProcurementStatus = "completed"
)

func (t TaskType) RequiresDesign() bool {
	switch t {
	case TaskTypeOriginalProductDevelopment, TaskTypeNewProductDevelopment, TaskTypeRetouchTask:
		return true
	default:
		return false
	}
}

func (t TaskType) RequiresAudit() bool {
	switch t {
	case TaskTypeOriginalProductDevelopment, TaskTypeNewProductDevelopment:
		return true
	default:
		return false
	}
}

func (t TaskType) Valid() bool {
	switch t {
	case TaskTypeOriginalProductDevelopment, TaskTypeNewProductDevelopment, TaskTypePurchaseTask:
		return true
	case TaskTypeRetouchTask, TaskTypeCustomerCustomization, TaskTypeRegularCustomization:
		return true
	default:
		return false
	}
}

func (t TaskType) DefaultSourceMode() (TaskSourceMode, bool) {
	switch t {
	case TaskTypeOriginalProductDevelopment:
		return TaskSourceModeExistingProduct, true
	case TaskTypeNewProductDevelopment, TaskTypePurchaseTask, TaskTypeRetouchTask, TaskTypeCustomerCustomization, TaskTypeRegularCustomization:
		return TaskSourceModeNewProduct, true
	default:
		return "", false
	}
}

func (s ProcurementStatus) Valid() bool {
	switch s {
	case ProcurementStatusDraft, ProcurementStatusPrepared, ProcurementStatusInProgress, ProcurementStatusCompleted:
		return true
	default:
		return false
	}
}

func (s ProcurementStatus) AllowsWarehousePrepare() bool {
	return s == ProcurementStatusCompleted
}

func (s ProcurementStatus) AllowsClose() bool {
	return s == ProcurementStatusCompleted
}

func (s ProcurementStatus) CanTransit(action ProcurementAction) bool {
	switch action {
	case ProcurementActionPrepare:
		return s == ProcurementStatusDraft
	case ProcurementActionStart:
		return s == ProcurementStatusPrepared
	case ProcurementActionComplete:
		return s == ProcurementStatusPrepared || s == ProcurementStatusInProgress
	case ProcurementActionReopen:
		return s == ProcurementStatusPrepared || s == ProcurementStatusInProgress || s == ProcurementStatusCompleted
	default:
		return false
	}
}
