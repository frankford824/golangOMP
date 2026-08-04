package domain

import "time"

// PaginationMeta is the shared pagination envelope for V7 list queries.
type PaginationMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type TaskSubStatusCode string

const (
	TaskSubStatusNotRequired    TaskSubStatusCode = "not_required"
	TaskSubStatusPendingDesign  TaskSubStatusCode = "pending_design"
	TaskSubStatusReworkRequired TaskSubStatusCode = "rework_required"
	TaskSubStatusPendingAudit   TaskSubStatusCode = "pending_audit"
	TaskSubStatusInProgress     TaskSubStatusCode = "in_progress"
	TaskSubStatusCompleted      TaskSubStatusCode = "completed"
	TaskSubStatusFinalReady     TaskSubStatusCode = "final_ready"
)

type TaskSubStatusSource string

const (
	TaskSubStatusSourceTaskType   TaskSubStatusSource = "task_type"
	TaskSubStatusSourceTaskStatus TaskSubStatusSource = "task_status"
)

type TaskSubStatusItem struct {
	Code   TaskSubStatusCode   `json:"code"`
	Label  string              `json:"label"`
	Source TaskSubStatusSource `json:"source"`
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
	SKUItems         []*TaskSKUItem               `json:"sku_items"`
	ProductSelection *TaskProductSelectionContext `json:"product_selection,omitempty"`
	// Frontend detail fields (v0.5)
	AssigneeID         *int64          `json:"assignee_id,omitempty"` // alias for designer_id
	AssigneeName       string          `json:"assignee_name,omitempty"`
	RequesterName      string          `json:"requester_name,omitempty"`
	DesignerName       string          `json:"designer_name,omitempty"`
	CurrentHandlerName string          `json:"current_handler_name,omitempty"`
	DesignRequirement  string          `json:"design_requirement,omitempty"`
	ChangeRequest      string          `json:"change_request,omitempty"`
	Note               string          `json:"note,omitempty"`
	SKUCodeType        TaskSKUCodeType `json:"-"`
	// Always JSON-encode as an array (including empty) so detail clients do not confuse omission with missing data.
	ReferenceFileRefs []ReferenceFileRef `json:"reference_file_refs"`
	// Always present for retouch_task reads; empty array for other task types and legacy retouch rows.
	RetouchRequirements []TaskRetouchRequirement `json:"retouch_requirements"`
	CreatorName         string                   `json:"creator_name,omitempty"`
}

type TaskFilterActorOption struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Username    string     `json:"username,omitempty"`
	DisplayName string     `json:"display_name,omitempty"`
	Department  string     `json:"department,omitempty"`
	Team        string     `json:"team,omitempty"`
	TaskCount   int64      `json:"task_count"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

type TaskFilterOptions struct {
	Creators  []TaskFilterActorOption `json:"creators"`
	Designers []TaskFilterActorOption `json:"designers"`
}

// TaskListItem is the frontend-oriented task list projection for STEP_05.
type TaskListItem struct {
	ID                           int64                        `json:"id"`
	TaskNo                       string                       `json:"task_no"`
	ProductID                    *int64                       `json:"product_id,omitempty"`
	SKUCode                      string                       `json:"sku_code"`
	PrimarySKUCode               string                       `json:"primary_sku_code,omitempty"`
	SKUCodeType                  TaskSKUCodeType              `json:"-"`
	ProductNameSnapshot          string                       `json:"product_name_snapshot"`
	TaskType                     TaskType                     `json:"task_type"`
	SourceMode                   TaskSourceMode               `json:"source_mode"`
	OwnerTeam                    string                       `json:"-"`
	OwnerDepartment              string                       `json:"owner_department"`
	OwnerDepartmentID            *int64                       `json:"owner_department_id,omitempty"`
	OwnerOrgTeam                 string                       `json:"owner_org_team"`
	OwnerTeamID                  *int64                       `json:"owner_team_id,omitempty"`
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
	WorkflowRevision             int64                        `json:"workflow_revision"`
	WorkflowContractVersion      int                          `json:"workflow_contract_version"`
	AllowedActions               []string                     `json:"allowed_actions"`
	CreatedAt                    time.Time                    `json:"created_at"`
	UpdatedAt                    time.Time                    `json:"updated_at"`
	DeadlineAt                   *time.Time                   `json:"deadline_at,omitempty"`
	BusinessLane                 TaskBusinessLane             `json:"business_lane"`
	CustomizationRequired        bool                         `json:"customization_required"`
	WorkflowLane                 WorkflowLane                 `json:"-"`
	IsBatchTask                  bool                         `json:"is_batch_task"`
	BatchItemCount               int                          `json:"batch_item_count"`
	BatchMode                    TaskBatchMode                `json:"batch_mode"`
	SKUItems                     []*TaskSKUItem               `json:"sku_items,omitempty"`
	ProductSelection             *TaskProductSelectionSummary `json:"product_selection,omitempty"`
	Category                     string                       `json:"-"`
	CategoryCode                 string                       `json:"-"`
	CategoryName                 string                       `json:"-"`
	SourceProductID              *int64                       `json:"-"`
	SourceProductName            string                       `json:"-"`
	SourceSearchEntryCode        string                       `json:"-"`
	SourceMatchType              string                       `json:"-"`
	SourceMatchRule              string                       `json:"-"`
	MatchedCategoryCode          string                       `json:"-"`
	MatchedSearchEntryCode       string                       `json:"-"`
	ProductSelectionSnapshotJSON string                       `json:"-"`
	SpecText                     string                       `json:"-"`
	Material                     string                       `json:"-"`
	SizeText                     string                       `json:"-"`
	CraftText                    string                       `json:"-"`
	CostPrice                    *float64                     `json:"-"`
	EstimatedCost                *float64                     `json:"-"`
	CostRuleID                   *int64                       `json:"-"`
	CostRuleName                 string                       `json:"-"`
	CostRuleSource               string                       `json:"-"`
	MatchedRuleVersion           *int                         `json:"-"`
	PrefillSource                string                       `json:"-"`
	PrefillAt                    *time.Time                   `json:"-"`
	RequiresManualReview         bool                         `json:"-"`
	ManualCostOverride           bool                         `json:"-"`
	ManualCostOverrideReason     string                       `json:"-"`
	OverrideActor                string                       `json:"-"`
	OverrideAt                   *time.Time                   `json:"-"`
	FilingStatus                 FilingStatus                 `json:"-"`
	FilingErrorMessage           string                       `json:"-"`
	FilingTriggerSource          string                       `json:"-"`
	LastFilingAttemptAt          *time.Time                   `json:"-"`
	LastFiledAt                  *time.Time                   `json:"-"`
	ERPSyncRequired              bool                         `json:"-"`
	ERPSyncVersion               int64                        `json:"-"`
	LastFilingPayloadHash        string                       `json:"-"`
	LastFilingPayloadJSON        string                       `json:"-"`
	MissingFields                []string                     `json:"-"`
	MissingFieldsSummaryCN       string                       `json:"-"`
	FiledAt                      *time.Time                   `json:"-"`
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
