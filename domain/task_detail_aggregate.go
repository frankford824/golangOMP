package domain

// TaskDetailAggregate is the frontend-oriented detail view for a task.
// It keeps child fields present even when the related collection is empty.
type TaskDetailAggregate struct {
	Task                   *Task                               `json:"task"`
	TaskDetail             *TaskDetail                         `json:"task_detail"`
	DesignAssets           []*DesignAsset                      `json:"design_assets"`
	AssetVersions          []*DesignAssetVersion               `json:"asset_versions"`
	SKUItems               []*TaskSKUItem                      `json:"sku_items"`
	Product                *Product                            `json:"product"`
	Assets                 []*TaskAsset                        `json:"assets"`
	AuditRecords           []*AuditRecord                      `json:"audit_records"`
	AuditHandovers         []*AuditHandover                    `json:"audit_handovers"`
	OutsourceOrders        []*OutsourceOrder                   `json:"outsource_orders"`
	WarehouseReceipt       *WarehouseReceipt                   `json:"warehouse_receipt"`
	Procurement            *ProcurementRecord                  `json:"procurement"`
	ProcurementSummary     *ProcurementSummary                 `json:"procurement_summary,omitempty"`
	ProductSelection       *TaskProductSelectionContext        `json:"product_selection,omitempty"`
	MatchedRuleGovernance  *TaskMatchedRuleGovernance          `json:"matched_rule_governance,omitempty"`
	OverrideSummary        *TaskCostOverrideSummary            `json:"override_summary,omitempty"`
	GovernanceAuditSummary *TaskGovernanceAuditSummary         `json:"governance_audit_summary,omitempty"`
	OverrideBoundary       *TaskCostOverrideGovernanceBoundary `json:"override_governance_boundary,omitempty"`
	CreatorID              *int64                              `json:"creator_id,omitempty"`
	RequesterID            *int64                              `json:"requester_id,omitempty"`
	DesignerID             *int64                              `json:"designer_id,omitempty"`
	CurrentHandlerID       *int64                              `json:"current_handler_id,omitempty"`
	CreatorName            string                              `json:"creator_name,omitempty"`
	RequesterName          string                              `json:"requester_name,omitempty"`
	DesignerName           string                              `json:"designer_name,omitempty"`
	CurrentHandlerName     string                              `json:"current_handler_name,omitempty"`
	AssigneeID             *int64                              `json:"assignee_id,omitempty"`
	AssigneeName           string                              `json:"assignee_name,omitempty"`
	EventLogs              []*TaskEvent                        `json:"event_logs"`
	AvailableActions       []AvailableAction                   `json:"available_actions"`
	Workflow               TaskWorkflowSnapshot                `json:"workflow"`
	PolicyMode             PolicyMode                          `json:"policy_mode,omitempty"`
	VisibleToRoles         []Role                              `json:"visible_to_roles,omitempty"`
	ActionRoles            []ActionPolicySummary               `json:"action_roles,omitempty"`
	PolicyScopeSummary     *PolicyScopeSummary                 `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary  *PlatformEntryBoundary              `json:"platform_entry_boundary,omitempty"`
}
