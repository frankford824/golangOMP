package domain

import "time"

type ProcurementAction string

const (
	ProcurementActionPrepare  ProcurementAction = "prepare"
	ProcurementActionStart    ProcurementAction = "start"
	ProcurementActionComplete ProcurementAction = "complete"
	ProcurementActionReopen   ProcurementAction = "reopen"
)

func (a ProcurementAction) Valid() bool {
	switch a {
	case ProcurementActionPrepare, ProcurementActionStart, ProcurementActionComplete, ProcurementActionReopen:
		return true
	default:
		return false
	}
}

// ProcurementRecord isolates purchase-preparation data from generic task details.
type ProcurementRecord struct {
	ID                 int64             `db:"id"                   json:"id"`
	TaskID             int64             `db:"task_id"              json:"task_id"`
	Status             ProcurementStatus `db:"status"               json:"status"`
	ProcurementPrice   *float64          `db:"procurement_price"    json:"procurement_price,omitempty"`
	Quantity           *int64            `db:"quantity"             json:"quantity,omitempty"`
	SupplierName       string            `db:"supplier_name"        json:"supplier_name"`
	PurchaseRemark     string            `db:"purchase_remark"      json:"purchase_remark"`
	ExpectedDeliveryAt *time.Time        `db:"expected_delivery_at" json:"expected_delivery_at,omitempty"`
	CreatedAt          time.Time         `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time         `db:"updated_at"           json:"updated_at"`
}

type ProcurementCoordinationStatus string

const (
	ProcurementCoordinationStatusPreparing         ProcurementCoordinationStatus = "preparing"
	ProcurementCoordinationStatusAwaitingArrival   ProcurementCoordinationStatus = "awaiting_arrival"
	ProcurementCoordinationStatusReadyForWarehouse ProcurementCoordinationStatus = "ready_for_warehouse"
	ProcurementCoordinationStatusHandedToWarehouse ProcurementCoordinationStatus = "handed_to_warehouse"
	ProcurementCoordinationStatusWarehouseDone     ProcurementCoordinationStatus = "warehouse_completed"
)

type ProcurementSummary struct {
	Status                   ProcurementStatus                   `json:"status"`
	CoordinationStatus       ProcurementCoordinationStatus       `json:"coordination_status"`
	CoordinationLabel        string                              `json:"coordination_label"`
	WarehouseStatus          *WarehouseReceiptStatus             `json:"warehouse_status,omitempty"`
	WarehousePrepareReady    bool                                `json:"warehouse_prepare_ready"`
	WarehouseReceiveReady    bool                                `json:"warehouse_receive_ready"`
	CategoryCode             string                              `json:"category_code"`
	CategoryName             string                              `json:"category_name"`
	ProcurementPrice         *float64                            `json:"procurement_price,omitempty"`
	CostPrice                *float64                            `json:"cost_price,omitempty"`
	EstimatedCost            *float64                            `json:"estimated_cost,omitempty"`
	CostRuleID               *int64                              `json:"cost_rule_id,omitempty"`
	CostRuleName             string                              `json:"cost_rule_name"`
	CostRuleSource           string                              `json:"cost_rule_source"`
	MatchedRuleVersion       *int                                `json:"matched_rule_version,omitempty"`
	PrefillSource            string                              `json:"prefill_source"`
	PrefillAt                *time.Time                          `json:"prefill_at,omitempty"`
	RequiresManualReview     bool                                `json:"requires_manual_review"`
	ManualCostOverride       bool                                `json:"manual_cost_override"`
	ManualCostOverrideReason string                              `json:"manual_cost_override_reason"`
	OverrideActor            string                              `json:"override_actor"`
	OverrideAt               *time.Time                          `json:"override_at,omitempty"`
	Quantity                 *int64                              `json:"quantity,omitempty"`
	SupplierName             string                              `json:"supplier_name"`
	ExpectedDeliveryAt       *time.Time                          `json:"expected_delivery_at,omitempty"`
	ProductSelection         *TaskProductSelectionSummary        `json:"product_selection,omitempty"`
	MatchedRuleGovernance    *TaskMatchedRuleGovernance          `json:"matched_rule_governance,omitempty"`
	OverrideSummary          *TaskCostOverrideSummary            `json:"override_summary,omitempty"`
	GovernanceAuditSummary   *TaskGovernanceAuditSummary         `json:"governance_audit_summary,omitempty"`
	OverrideBoundary         *TaskCostOverrideGovernanceBoundary `json:"override_governance_boundary,omitempty"`
	PolicyMode               PolicyMode                          `json:"policy_mode,omitempty"`
	VisibleToRoles           []Role                              `json:"visible_to_roles,omitempty"`
	ActionRoles              []ActionPolicySummary               `json:"action_roles,omitempty"`
	PolicyScopeSummary       *PolicyScopeSummary                 `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary    *PlatformEntryBoundary              `json:"platform_entry_boundary,omitempty"`
}
