package domain

import "time"

// PaginationMeta is the shared pagination envelope for V7 list queries.
type PaginationMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// AvailableAction is a frontend-only task action suggestion derived from current aggregate state.
// It never replaces server-side state validation.
type AvailableAction string

const (
	AvailableActionAssign            AvailableAction = "assign"
	AvailableActionSubmitDesign      AvailableAction = "submit_design"
	AvailableActionPrepareWarehouse  AvailableAction = "prepare_warehouse"
	AvailableActionClose             AvailableAction = "close"
	AvailableActionClaimAudit        AvailableAction = "claim_audit"
	AvailableActionApproveAudit      AvailableAction = "approve_audit"
	AvailableActionRejectAudit       AvailableAction = "reject_audit"
	AvailableActionHandover          AvailableAction = "handover"
	AvailableActionCreateOutsource   AvailableAction = "create_outsource"
	AvailableActionWarehouseReceive  AvailableAction = "warehouse_receive"
	AvailableActionWarehouseReject   AvailableAction = "warehouse_reject"
	AvailableActionWarehouseComplete AvailableAction = "warehouse_complete"
)

type TaskMainStatus string

const (
	TaskMainStatusDraft                   TaskMainStatus = "draft"
	TaskMainStatusCreated                 TaskMainStatus = "created"
	TaskMainStatusFiled                   TaskMainStatus = "filed"
	TaskMainStatusPendingWarehouseReceive TaskMainStatus = "pending_warehouse_receive"
	TaskMainStatusWarehouseProcessing     TaskMainStatus = "warehouse_processing"
	TaskMainStatusPendingClose            TaskMainStatus = "pending_close"
	TaskMainStatusClosed                  TaskMainStatus = "closed"
)

type WorkflowReasonCode string

const (
	WorkflowReasonTaskNotFound               WorkflowReasonCode = "task_not_found"
	WorkflowReasonTaskDetailMissing          WorkflowReasonCode = "task_detail_missing"
	WorkflowReasonTaskAlreadyPendingWH       WorkflowReasonCode = "task_already_pending_warehouse"
	WorkflowReasonTaskAlreadyClosed          WorkflowReasonCode = "task_already_closed"
	WorkflowReasonTaskAwaitingClose          WorkflowReasonCode = "task_awaiting_close"
	WorkflowReasonTaskBlocked                WorkflowReasonCode = "task_blocked"
	WorkflowReasonWarehouseAlreadyReceived   WorkflowReasonCode = "warehouse_already_received"
	WorkflowReasonWarehouseAlreadyDone       WorkflowReasonCode = "warehouse_already_completed"
	WorkflowReasonMissingFinalAsset          WorkflowReasonCode = "missing_final_design_asset"
	WorkflowReasonAuditNotApproved           WorkflowReasonCode = "audit_not_approved"
	WorkflowReasonMissingTaskNo              WorkflowReasonCode = "missing_task_no"
	WorkflowReasonMissingSKU                 WorkflowReasonCode = "missing_sku"
	WorkflowReasonWarehouseNotReceived       WorkflowReasonCode = "warehouse_not_received"
	WorkflowReasonWarehouseRejected          WorkflowReasonCode = "warehouse_rejected_pending_resolution"
	WorkflowReasonWarehouseNotCompleted      WorkflowReasonCode = "warehouse_not_completed"
	WorkflowReasonPendingException           WorkflowReasonCode = "pending_exception_resolution"
	WorkflowReasonFiledAtMissing             WorkflowReasonCode = "filed_at_missing"
	WorkflowReasonCategoryMissing            WorkflowReasonCode = "category_missing"
	WorkflowReasonSpecMissing                WorkflowReasonCode = "spec_text_missing"
	WorkflowReasonCostPriceMissing           WorkflowReasonCode = "cost_price_missing"
	WorkflowReasonProcurementMissing         WorkflowReasonCode = "procurement_record_missing"
	WorkflowReasonProcurementPriceMissing    WorkflowReasonCode = "procurement_price_missing"
	WorkflowReasonProcurementQuantityMissing WorkflowReasonCode = "procurement_quantity_missing"
	WorkflowReasonProcurementNotReady        WorkflowReasonCode = "procurement_not_ready"
	WorkflowReasonNotPendingClose            WorkflowReasonCode = "not_pending_close"
)

type WorkflowReason struct {
	Code    WorkflowReasonCode `json:"code"`
	Message string             `json:"message"`
}

type TaskSubStatusCode string

const (
	TaskSubStatusNotRequired    TaskSubStatusCode = "not_required"
	TaskSubStatusNotTriggered   TaskSubStatusCode = "not_triggered"
	TaskSubStatusNotStarted     TaskSubStatusCode = "not_started"
	TaskSubStatusPendingDesign  TaskSubStatusCode = "pending_design"
	TaskSubStatusDesigning      TaskSubStatusCode = "designing"
	TaskSubStatusReworkRequired TaskSubStatusCode = "rework_required"
	TaskSubStatusPendingAudit   TaskSubStatusCode = "pending_audit"
	TaskSubStatusInReview       TaskSubStatusCode = "in_review"
	TaskSubStatusRejected       TaskSubStatusCode = "rejected"
	TaskSubStatusOutsourcing    TaskSubStatusCode = "outsourcing"
	TaskSubStatusOutsourced     TaskSubStatusCode = "outsourced"
	TaskSubStatusPreparing      TaskSubStatusCode = "preparing"
	TaskSubStatusReady          TaskSubStatusCode = "ready"
	TaskSubStatusInProgress     TaskSubStatusCode = "in_progress"
	TaskSubStatusPendingInbound TaskSubStatusCode = "pending_inbound"
	TaskSubStatusPendingReceive TaskSubStatusCode = "pending_receive"
	TaskSubStatusReceived       TaskSubStatusCode = "received"
	TaskSubStatusCompleted      TaskSubStatusCode = "completed"
	TaskSubStatusPendingReview  TaskSubStatusCode = "pending_review"
	TaskSubStatusFinalReady     TaskSubStatusCode = "final_ready"
	TaskSubStatusApproved       TaskSubStatusCode = "approved"
	TaskSubStatusReserved       TaskSubStatusCode = "reserved"
)

type TaskSubStatusSource string

const (
	TaskSubStatusSourceTaskType         TaskSubStatusSource = "task_type"
	TaskSubStatusSourceTaskStatus       TaskSubStatusSource = "task_status"
	TaskSubStatusSourceTaskAsset        TaskSubStatusSource = "task_asset"
	TaskSubStatusSourceWarehouseReceipt TaskSubStatusSource = "warehouse_receipt"
	TaskSubStatusSourceProcurement      TaskSubStatusSource = "procurement_record"
	TaskSubStatusSourceReserved         TaskSubStatusSource = "reserved"
)

type TaskSubStatusItem struct {
	Code   TaskSubStatusCode   `json:"code"`
	Label  string              `json:"label"`
	Source TaskSubStatusSource `json:"source"`
}

type TaskSubStatusScope string

const (
	TaskSubStatusScopeDesign        TaskSubStatusScope = "design"
	TaskSubStatusScopeAudit         TaskSubStatusScope = "audit"
	TaskSubStatusScopeProcurement   TaskSubStatusScope = "procurement"
	TaskSubStatusScopeWarehouse     TaskSubStatusScope = "warehouse"
	TaskSubStatusScopeCustomization TaskSubStatusScope = "customization"
	// TaskSubStatusScopeOutsource is a compatibility alias retained for migration safety.
	TaskSubStatusScopeOutsource  TaskSubStatusScope = "outsource"
	TaskSubStatusScopeProduction TaskSubStatusScope = "production"
)

func (s TaskSubStatusScope) Valid() bool {
	switch s {
	case TaskSubStatusScopeDesign,
		TaskSubStatusScopeAudit,
		TaskSubStatusScopeProcurement,
		TaskSubStatusScopeWarehouse,
		TaskSubStatusScopeCustomization,
		TaskSubStatusScopeOutsource,
		TaskSubStatusScopeProduction:
		return true
	default:
		return false
	}
}

type TaskSubStatusSnapshot struct {
	Design        TaskSubStatusItem `json:"design"`
	Audit         TaskSubStatusItem `json:"audit"`
	Procurement   TaskSubStatusItem `json:"procurement"`
	Warehouse     TaskSubStatusItem `json:"warehouse"`
	Customization TaskSubStatusItem `json:"customization"`
	// Outsource is a compatibility projection alias and mirrors customization lane status.
	Outsource  TaskSubStatusItem `json:"outsource"`
	Production TaskSubStatusItem `json:"production"`
}

type TaskWorkflowSnapshot struct {
	MainStatus               TaskMainStatus        `json:"main_status"`
	SubStatus                TaskSubStatusSnapshot `json:"sub_status"`
	CanPrepareWarehouse      bool                  `json:"can_prepare_warehouse"`
	WarehouseBlockingReasons []WorkflowReason      `json:"warehouse_blocking_reasons"`
	CanClose                 bool                  `json:"can_close"`
	Closable                 bool                  `json:"closable"`
	CannotCloseReasons       []WorkflowReason      `json:"cannot_close_reasons"`
}

type TaskMatchedRuleSnapshot struct {
	RuleID               int64                    `json:"rule_id"`
	RuleName             string                   `json:"rule_name"`
	RuleVersion          int                      `json:"rule_version"`
	RuleSource           string                   `json:"rule_source"`
	GovernanceStatus     CostRuleGovernanceStatus `json:"governance_status"`
	PrefillSource        string                   `json:"prefill_source"`
	PrefillAt            *time.Time               `json:"prefill_at,omitempty"`
	RequiresManualReview bool                     `json:"requires_manual_review"`
	IsCurrentRule        bool                     `json:"is_current_rule"`
}

type TaskMatchedRuleGovernance struct {
	MatchedRule            *TaskMatchedRuleSnapshot     `json:"matched_rule,omitempty"`
	CurrentRule            *CostRuleVersionRef          `json:"current_rule,omitempty"`
	VersionChainSummary    *CostRuleVersionChainSummary `json:"version_chain_summary,omitempty"`
	IsRuleOutdated         bool                         `json:"is_rule_outdated"`
	CurrentRuleVersionHint *int                         `json:"current_rule_version_hint,omitempty"`
}

type TaskCostOverrideEventSummary struct {
	EventID               string                         `json:"event_id"`
	Sequence              int64                          `json:"sequence"`
	EventType             TaskCostOverrideAuditEventType `json:"event_type"`
	CostPrice             *float64                       `json:"cost_price,omitempty"`
	PreviousEstimatedCost *float64                       `json:"previous_estimated_cost,omitempty"`
	PreviousCostPrice     *float64                       `json:"previous_cost_price,omitempty"`
	OverrideCost          *float64                       `json:"override_cost,omitempty"`
	ResultCostPrice       *float64                       `json:"result_cost_price,omitempty"`
	CategoryCode          string                         `json:"category_code"`
	MatchedRuleID         *int64                         `json:"matched_rule_id,omitempty"`
	MatchedRuleVersion    *int                           `json:"matched_rule_version,omitempty"`
	MatchedRuleSource     string                         `json:"matched_rule_source"`
	GovernanceStatus      CostRuleGovernanceStatus       `json:"governance_status"`
	Reason                string                         `json:"reason"`
	Actor                 string                         `json:"actor"`
	Source                string                         `json:"source"`
	Note                  string                         `json:"note"`
	OccurredAt            time.Time                      `json:"occurred_at"`
}

type TaskCostOverrideSummary struct {
	CurrentOverrideActive bool                          `json:"current_override_active"`
	CurrentOverrideReason string                        `json:"current_override_reason"`
	CurrentOverrideActor  string                        `json:"current_override_actor"`
	CurrentOverrideAt     *time.Time                    `json:"current_override_at,omitempty"`
	CurrentCostPrice      *float64                      `json:"current_cost_price,omitempty"`
	OverrideEventCount    int                           `json:"override_event_count"`
	LatestOverrideEvent   *TaskCostOverrideEventSummary `json:"latest_override_event,omitempty"`
	LatestReleaseEvent    *TaskCostOverrideEventSummary `json:"latest_release_event,omitempty"`
	LatestAuditEvent      *TaskCostOverrideEventSummary `json:"latest_audit_event,omitempty"`
	HistorySource         string                        `json:"history_source"`
}

type TaskReadModel struct {
	Task
	DesignAssets           []*DesignAsset                      `json:"design_assets"`
	AssetVersions          []*DesignAssetVersion               `json:"asset_versions"`
	SKUItems               []*TaskSKUItem                      `json:"sku_items"`
	Workflow               TaskWorkflowSnapshot                `json:"workflow"`
	Procurement            *ProcurementRecord                  `json:"procurement,omitempty"`
	ProcurementSummary     *ProcurementSummary                 `json:"procurement_summary,omitempty"`
	ProductSelection       *TaskProductSelectionContext        `json:"product_selection,omitempty"`
	MatchedRuleGovernance  *TaskMatchedRuleGovernance          `json:"matched_rule_governance,omitempty"`
	OverrideSummary        *TaskCostOverrideSummary            `json:"override_summary,omitempty"`
	GovernanceAuditSummary *TaskGovernanceAuditSummary         `json:"governance_audit_summary,omitempty"`
	OverrideBoundary       *TaskCostOverrideGovernanceBoundary `json:"override_governance_boundary,omitempty"`
	PolicyMode             PolicyMode                          `json:"policy_mode,omitempty"`
	VisibleToRoles         []Role                              `json:"visible_to_roles,omitempty"`
	ActionRoles            []ActionPolicySummary               `json:"action_roles,omitempty"`
	PolicyScopeSummary     *PolicyScopeSummary                 `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary  *PlatformEntryBoundary              `json:"platform_entry_boundary,omitempty"`
	// Frontend detail fields (v0.5)
	AssigneeID         *int64 `json:"assignee_id,omitempty"` // alias for designer_id
	AssigneeName       string `json:"assignee_name,omitempty"`
	RequesterName      string `json:"requester_name,omitempty"`
	DesignerName       string `json:"designer_name,omitempty"`
	CurrentHandlerName string `json:"current_handler_name,omitempty"`
	DesignRequirement  string `json:"design_requirement,omitempty"`
	ChangeRequest      string `json:"change_request,omitempty"`
	Note               string `json:"note,omitempty"`
	// Always JSON-encode as an array (including empty) so detail clients do not confuse omission with missing data.
	ReferenceFileRefs []ReferenceFileRef `json:"reference_file_refs"`
	CreatorName       string             `json:"creator_name,omitempty"`
}

// TaskListItem is the frontend-oriented task list projection for STEP_05.
type TaskListItem struct {
	ID                           int64                        `json:"id"`
	TaskNo                       string                       `json:"task_no"`
	ProductID                    *int64                       `json:"-"`
	SKUCode                      string                       `json:"sku_code"`
	PrimarySKUCode               string                       `json:"primary_sku_code,omitempty"`
	ProductNameSnapshot          string                       `json:"product_name_snapshot"`
	TaskType                     TaskType                     `json:"task_type"`
	SourceMode                   TaskSourceMode               `json:"source_mode"`
	OwnerTeam                    string                       `json:"owner_team"`
	OwnerDepartment              string                       `json:"owner_department"`
	OwnerOrgTeam                 string                       `json:"owner_org_team"`
	Priority                     TaskPriority                 `json:"priority"`
	CreatorID                    int64                        `json:"creator_id"`
	RequesterID                  *int64                       `json:"requester_id,omitempty"`
	DesignerID                   *int64                       `json:"designer_id,omitempty"`
	CurrentHandlerID             *int64                       `json:"current_handler_id,omitempty"`
	RequesterName                string                       `json:"requester_name,omitempty"`
	CreatorName                  string                       `json:"creator_name,omitempty"`
	DesignerName                 string                       `json:"designer_name,omitempty"`
	CurrentHandlerName           string                       `json:"current_handler_name,omitempty"`
	TaskStatus                   TaskStatus                   `json:"task_status"`
	CreatedAt                    time.Time                    `json:"created_at"`
	UpdatedAt                    time.Time                    `json:"updated_at"`
	DeadlineAt                   *time.Time                   `json:"deadline_at,omitempty"`
	NeedOutsource                bool                         `json:"need_outsource"`
	IsOutsource                  bool                         `json:"is_outsource"`
	CustomizationRequired        bool                         `json:"customization_required"`
	WorkflowLane                 WorkflowLane                 `json:"workflow_lane"`
	CustomizationSourceType      CustomizationSourceType      `json:"customization_source_type"`
	LastCustomizationOperatorID  *int64                       `json:"last_customization_operator_id,omitempty"`
	WarehouseRejectReason        string                       `json:"warehouse_reject_reason,omitempty"`
	WarehouseRejectCategory      string                       `json:"warehouse_reject_category,omitempty"`
	IsBatchTask                  bool                         `json:"is_batch_task"`
	BatchItemCount               int                          `json:"batch_item_count"`
	BatchMode                    TaskBatchMode                `json:"batch_mode"`
	WarehouseStatus              *WarehouseReceiptStatus      `json:"warehouse_status,omitempty"`
	LatestAssetType              *TaskAssetType               `json:"latest_asset_type,omitempty"`
	Workflow                     TaskWorkflowSnapshot         `json:"workflow"`
	ProcurementSummary           *ProcurementSummary          `json:"procurement_summary,omitempty"`
	ProductSelection             *TaskProductSelectionSummary `json:"product_selection,omitempty"`
	PolicyMode                   PolicyMode                   `json:"policy_mode,omitempty"`
	VisibleToRoles               []Role                       `json:"visible_to_roles,omitempty"`
	ActionRoles                  []ActionPolicySummary        `json:"action_roles,omitempty"`
	PolicyScopeSummary           *PolicyScopeSummary          `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary        *PlatformEntryBoundary       `json:"platform_entry_boundary,omitempty"`
	Category                     string                       `json:"category,omitempty"`
	CategoryCode                 string                       `json:"category_code,omitempty"`
	CategoryName                 string                       `json:"category_name,omitempty"`
	SourceProductID              *int64                       `json:"source_product_id,omitempty"`
	SourceProductName            string                       `json:"source_product_name,omitempty"`
	SourceSearchEntryCode        string                       `json:"source_search_entry_code,omitempty"`
	SourceMatchType              string                       `json:"source_match_type,omitempty"`
	SourceMatchRule              string                       `json:"source_match_rule,omitempty"`
	MatchedCategoryCode          string                       `json:"matched_category_code,omitempty"`
	MatchedSearchEntryCode       string                       `json:"matched_search_entry_code,omitempty"`
	ProductSelectionSnapshotJSON string                       `json:"product_selection_snapshot_json,omitempty"`
	SpecText                     string                       `json:"spec_text,omitempty"`
	Material                     string                       `json:"material,omitempty"`
	SizeText                     string                       `json:"size_text,omitempty"`
	CraftText                    string                       `json:"craft_text,omitempty"`
	ProcurementPrice             *float64                     `json:"procurement_price,omitempty"`
	ProcurementStatus            *ProcurementStatus           `json:"procurement_status,omitempty"`
	ProcurementQuantity          *int64                       `json:"procurement_quantity,omitempty"`
	SupplierName                 string                       `json:"supplier_name,omitempty"`
	ExpectedDeliveryAt           *time.Time                   `json:"expected_delivery_at,omitempty"`
	CostPrice                    *float64                     `json:"cost_price,omitempty"`
	EstimatedCost                *float64                     `json:"estimated_cost,omitempty"`
	CostRuleID                   *int64                       `json:"cost_rule_id,omitempty"`
	CostRuleName                 string                       `json:"cost_rule_name,omitempty"`
	CostRuleSource               string                       `json:"cost_rule_source,omitempty"`
	MatchedRuleVersion           *int                         `json:"matched_rule_version,omitempty"`
	PrefillSource                string                       `json:"prefill_source,omitempty"`
	PrefillAt                    *time.Time                   `json:"prefill_at,omitempty"`
	RequiresManualReview         bool                         `json:"requires_manual_review,omitempty"`
	ManualCostOverride           bool                         `json:"manual_cost_override,omitempty"`
	ManualCostOverrideReason     string                       `json:"manual_cost_override_reason,omitempty"`
	OverrideActor                string                       `json:"override_actor,omitempty"`
	OverrideAt                   *time.Time                   `json:"override_at,omitempty"`
	FilingStatus                 FilingStatus                 `json:"filing_status,omitempty"`
	FilingErrorMessage           string                       `json:"filing_error_message,omitempty"`
	FilingTriggerSource          string                       `json:"filing_trigger_source,omitempty"`
	LastFilingAttemptAt          *time.Time                   `json:"last_filing_attempt_at,omitempty"`
	LastFiledAt                  *time.Time                   `json:"last_filed_at,omitempty"`
	ERPSyncRequired              bool                         `json:"erp_sync_required,omitempty"`
	ERPSyncVersion               int64                        `json:"erp_sync_version,omitempty"`
	LastFilingPayloadHash        string                       `json:"-"`
	LastFilingPayloadJSON        string                       `json:"-"`
	MissingFields                []string                     `json:"missing_fields,omitempty"`
	MissingFieldsSummaryCN       string                       `json:"missing_fields_summary_cn,omitempty"`
	FiledAt                      *time.Time                   `json:"filed_at,omitempty"`
}

type TaskFilingStatusView struct {
	TaskID                  int64        `json:"task_id"`
	TaskType                TaskType     `json:"task_type"`
	TaskStatus              TaskStatus   `json:"task_status"`
	FilingStatus            FilingStatus `json:"filing_status"`
	FilingErrorMessage      string       `json:"filing_error_message,omitempty"`
	FilingTriggerSource     string       `json:"filing_trigger_source,omitempty"`
	LastFilingAttemptAt     *time.Time   `json:"last_filing_attempt_at,omitempty"`
	LastFiledAt             *time.Time   `json:"last_filed_at,omitempty"`
	ERPSyncRequired         bool         `json:"erp_sync_required"`
	ERPSyncVersion          int64        `json:"erp_sync_version"`
	FiledAt                 *time.Time   `json:"filed_at,omitempty"`
	MissingFields           []string     `json:"missing_fields,omitempty"`
	MissingFieldsSummaryCN  string       `json:"missing_fields_summary_cn,omitempty"`
	CanRetry                bool         `json:"can_retry"`
	LastFilingPayloadHash   string       `json:"-"`
	LastFilingPayloadSample string       `json:"-"`
}
