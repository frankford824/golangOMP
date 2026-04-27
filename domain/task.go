package domain

import "time"

type FilingStatus string

const (
	FilingStatusNotFiled     FilingStatus = "not_filed"
	FilingStatusPending      FilingStatus = "pending_filing"
	FilingStatusFiling       FilingStatus = "filing"
	FilingStatusFiled        FilingStatus = "filed"
	FilingStatusFilingFailed FilingStatus = "filing_failed"
)

func (s FilingStatus) Valid() bool {
	switch s {
	case FilingStatusNotFiled, FilingStatusPending, FilingStatusFiling, FilingStatusFiled, FilingStatusFilingFailed:
		return true
	default:
		return false
	}
}

type WorkflowLane string

const (
	WorkflowLaneNormal        WorkflowLane = "normal"
	WorkflowLaneCustomization WorkflowLane = "customization"
)

func WorkflowLaneFromCustomizationRequired(customizationRequired bool) WorkflowLane {
	if customizationRequired {
		return WorkflowLaneCustomization
	}
	return WorkflowLaneNormal
}

// Task is the V7 business aggregate root (spec V7 §7.1).
// Every formal workflow must start with a Task that is bound to a SKU.
type Task struct {
	ID                          int64                   `db:"id"                    json:"id"`
	TaskNo                      string                  `db:"task_no"               json:"task_no"`
	SourceMode                  TaskSourceMode          `db:"source_mode"           json:"source_mode"`
	ProductID                   *int64                  `db:"product_id"            json:"product_id,omitempty"`
	SKUCode                     string                  `db:"sku_code"              json:"sku_code"`
	ProductNameSnapshot         string                  `db:"product_name_snapshot" json:"product_name_snapshot"`
	TaskType                    TaskType                `db:"task_type"             json:"task_type"`
	OperatorGroupID             *int64                  `db:"operator_group_id"     json:"operator_group_id,omitempty"`
	OwnerTeam                   string                  `db:"owner_team"            json:"owner_team"`
	OwnerDepartment             string                  `db:"owner_department"      json:"owner_department"`
	OwnerOrgTeam                string                  `db:"owner_org_team"        json:"owner_org_team"`
	CreatorID                   int64                   `db:"creator_id"            json:"creator_id"`
	RequesterID                 *int64                  `db:"requester_id"          json:"requester_id,omitempty"`
	DesignerID                  *int64                  `db:"designer_id"           json:"designer_id,omitempty"`
	CurrentHandlerID            *int64                  `db:"current_handler_id"    json:"current_handler_id,omitempty"`
	TaskStatus                  TaskStatus              `db:"task_status"           json:"task_status"`
	Priority                    TaskPriority            `db:"priority"              json:"priority"`
	DeadlineAt                  *time.Time              `db:"deadline_at"           json:"deadline_at,omitempty"`
	NeedOutsource               bool                    `db:"need_outsource"        json:"need_outsource"`
	IsOutsource                 bool                    `db:"is_outsource"          json:"is_outsource"`
	CustomizationRequired       bool                    `db:"customization_required" json:"customization_required"`
	CustomizationSourceType     CustomizationSourceType `db:"customization_source_type" json:"customization_source_type"`
	LastCustomizationOperatorID *int64                  `db:"last_customization_operator_id" json:"last_customization_operator_id,omitempty"`
	WarehouseRejectReason       string                  `db:"warehouse_reject_reason" json:"warehouse_reject_reason,omitempty"`
	WarehouseRejectCategory     string                  `db:"warehouse_reject_category" json:"warehouse_reject_category,omitempty"`
	IsBatchTask                 bool                    `db:"is_batch_task"         json:"is_batch_task"`
	BatchItemCount              int                     `db:"batch_item_count"      json:"batch_item_count"`
	BatchMode                   TaskBatchMode           `db:"batch_mode"            json:"batch_mode"`
	PrimarySKUCode              string                  `db:"primary_sku_code"      json:"primary_sku_code,omitempty"`
	SKUGenerationStatus         TaskSKUGenerationStatus `db:"sku_generation_status" json:"sku_generation_status"`
	CreatedAt                   time.Time               `db:"created_at"            json:"created_at"`
	UpdatedAt                   time.Time               `db:"updated_at"            json:"updated_at"`
}

func (t *Task) WorkflowLane() WorkflowLane {
	if t == nil {
		return WorkflowLaneNormal
	}
	return WorkflowLaneFromCustomizationRequired(t.CustomizationRequired)
}

// TaskDetail stores supplemental demand information for a Task.
type TaskDetail struct {
	ID                           int64                        `db:"id"                          json:"id"`
	TaskID                       int64                        `db:"task_id"                     json:"task_id"`
	DemandText                   string                       `db:"demand_text"                 json:"demand_text"`
	CopyText                     string                       `db:"copy_text"                   json:"copy_text"`
	StyleKeywords                string                       `db:"style_keywords"              json:"style_keywords"`
	Remark                       string                       `db:"remark"                      json:"remark"`
	Note                         string                       `db:"note"                        json:"note"`
	RiskFlagsJSON                string                       `db:"risk_flags_json"             json:"risk_flags_json"`
	Category                     string                       `db:"category"                    json:"category"`
	CategoryID                   *int64                       `db:"category_id"                 json:"category_id,omitempty"`
	CategoryCode                 string                       `db:"category_code"               json:"category_code"`
	CategoryName                 string                       `db:"category_name"               json:"category_name"`
	SourceProductID              *int64                       `db:"source_product_id"           json:"-"`
	SourceProductName            string                       `db:"source_product_name"         json:"-"`
	SourceSearchEntryCode        string                       `db:"source_search_entry_code"    json:"-"`
	SourceMatchType              string                       `db:"source_match_type"           json:"-"`
	SourceMatchRule              string                       `db:"source_match_rule"           json:"-"`
	MatchedCategoryCode          string                       `db:"matched_category_code"       json:"-"`
	MatchedSearchEntryCode       string                       `db:"matched_search_entry_code"   json:"-"`
	MatchedMappingRuleJSON       string                       `db:"matched_mapping_rule_json"   json:"-"`
	ProductSelectionSnapshotJSON string                       `db:"product_selection_snapshot_json" json:"-"`
	ProductSelection             *TaskProductSelectionContext `db:"-"         json:"product_selection,omitempty"`
	ChangeRequest                string                       `db:"change_request"              json:"change_request"`
	DesignRequirement            string                       `db:"design_requirement"          json:"design_requirement"`
	ProductShortName             string                       `db:"product_short_name"          json:"product_short_name"`
	MaterialMode                 string                       `db:"material_mode"               json:"material_mode"`
	MaterialOther                string                       `db:"material_other"              json:"material_other"`
	CostPriceMode                string                       `db:"cost_price_mode"             json:"cost_price_mode"`
	BaseSalePrice                *float64                     `db:"base_sale_price"             json:"base_sale_price,omitempty"`
	ProductChannel               string                       `db:"product_channel"             json:"product_channel"`
	ReferenceImagesJSON          string                       `db:"reference_images_json"       json:"reference_images_json"`
	ReferenceFileRefsJSON        string                       `db:"reference_file_refs_json"    json:"reference_file_refs_json"`
	ReferenceLink                string                       `db:"reference_link"              json:"reference_link"`
	SpecText                     string                       `db:"spec_text"                   json:"spec_text"`
	Material                     string                       `db:"material"                    json:"material"`
	SizeText                     string                       `db:"size_text"                   json:"size_text"`
	CraftText                    string                       `db:"craft_text"                  json:"craft_text"`
	Width                        *float64                     `db:"width"                       json:"width,omitempty"`
	Height                       *float64                     `db:"height"                      json:"height,omitempty"`
	Area                         *float64                     `db:"area"                        json:"area,omitempty"`
	Quantity                     *int64                       `db:"quantity"                    json:"quantity,omitempty"`
	Process                      string                       `db:"process"                     json:"process"`
	ProcurementPrice             *float64                     `db:"procurement_price"           json:"-"`
	CostPrice                    *float64                     `db:"cost_price"                  json:"cost_price,omitempty"`
	EstimatedCost                *float64                     `db:"estimated_cost"              json:"estimated_cost,omitempty"`
	CostRuleID                   *int64                       `db:"cost_rule_id"                json:"cost_rule_id,omitempty"`
	CostRuleName                 string                       `db:"cost_rule_name"              json:"cost_rule_name"`
	CostRuleSource               string                       `db:"cost_rule_source"            json:"cost_rule_source"`
	MatchedRuleVersion           *int                         `db:"matched_rule_version"        json:"matched_rule_version,omitempty"`
	PrefillSource                string                       `db:"prefill_source"              json:"prefill_source"`
	PrefillAt                    *time.Time                   `db:"prefill_at"                  json:"prefill_at,omitempty"`
	RequiresManualReview         bool                         `db:"requires_manual_review"      json:"requires_manual_review"`
	ManualCostOverride           bool                         `db:"manual_cost_override"        json:"manual_cost_override"`
	ManualCostOverrideReason     string                       `db:"manual_cost_override_reason" json:"manual_cost_override_reason"`
	OverrideActor                string                       `db:"override_actor"              json:"override_actor"`
	OverrideAt                   *time.Time                   `db:"override_at"                 json:"override_at,omitempty"`
	FilingStatus                 FilingStatus                 `db:"filing_status"               json:"filing_status"`
	FilingErrorMessage           string                       `db:"filing_error_message"        json:"filing_error_message"`
	FilingTriggerSource          string                       `db:"filing_trigger_source"       json:"filing_trigger_source,omitempty"`
	LastFilingAttemptAt          *time.Time                   `db:"last_filing_attempt_at"      json:"last_filing_attempt_at,omitempty"`
	LastFiledAt                  *time.Time                   `db:"last_filed_at"               json:"last_filed_at,omitempty"`
	ERPSyncRequired              bool                         `db:"erp_sync_required"           json:"erp_sync_required"`
	ERPSyncVersion               int64                        `db:"erp_sync_version"            json:"erp_sync_version"`
	LastFilingPayloadHash        string                       `db:"last_filing_payload_hash"    json:"-"`
	LastFilingPayloadJSON        string                       `db:"last_filing_payload_json"    json:"-"`
	FiledAt                      *time.Time                   `db:"filed_at"                    json:"filed_at,omitempty"`
	MissingFields                []string                     `db:"-"                           json:"missing_fields,omitempty"`
	MissingFieldsSummaryCN       string                       `db:"-"                           json:"missing_fields_summary_cn,omitempty"`
	CreatedAt                    time.Time                    `db:"created_at"                  json:"created_at"`
	UpdatedAt                    time.Time                    `db:"updated_at"                  json:"updated_at"`
}
