package domain

import (
	"encoding/json"
	"time"
)

type OMPSKUKind string

const (
	OMPSKUKindOrdinary OMPSKUKind = "ordinary"
	OMPSKUKindCombo    OMPSKUKind = "combo"
	OMPSKUKindUnknown  OMPSKUKind = "unknown"
)

type OMPSKURecord struct {
	SKUCode                string     `db:"sku_code" json:"sku_code"`
	SKUKind                OMPSKUKind `db:"sku_kind" json:"sku_kind"`
	FirstTaskID            *int64     `db:"first_task_id" json:"first_task_id,omitempty"`
	LastTaskID             *int64     `db:"last_task_id" json:"last_task_id,omitempty"`
	FirstTaskSKUItemID     *int64     `db:"first_task_sku_item_id" json:"first_task_sku_item_id,omitempty"`
	LastTaskSKUItemID      *int64     `db:"last_task_sku_item_id" json:"last_task_sku_item_id,omitempty"`
	SourceMode             string     `db:"source_mode" json:"source_mode"`
	TaskType               string     `db:"task_type" json:"task_type"`
	ProductName            string     `db:"product_name" json:"product_name"`
	ProductIID             string     `db:"product_i_id" json:"product_i_id"`
	CategoryCode           string     `db:"category_code" json:"category_code"`
	CategoryName           string     `db:"category_name" json:"category_name"`
	CostPrice              *float64   `db:"cost_price" json:"cost_price,omitempty"`
	EstimatedCost          *float64   `db:"estimated_cost" json:"estimated_cost,omitempty"`
	CostRuleID             *int64     `db:"cost_rule_id" json:"cost_rule_id,omitempty"`
	CostRuleName           string     `db:"cost_rule_name" json:"cost_rule_name"`
	CostRuleSource         string     `db:"cost_rule_source" json:"cost_rule_source"`
	ManualCostOverride     bool       `db:"manual_cost_override" json:"manual_cost_override"`
	RequiresManualReview   bool       `db:"requires_manual_review" json:"requires_manual_review"`
	LastERPSyncStatus      string     `db:"last_erp_sync_status" json:"last_erp_sync_status"`
	LastERPCallLogID       *int64     `db:"last_erp_call_log_id" json:"last_erp_call_log_id,omitempty"`
	CreatedBy             *int64     `db:"created_by" json:"created_by,omitempty"`
	LastOperatorID         *int64     `db:"last_operator_id" json:"last_operator_id,omitempty"`
	FirstSeenAt           time.Time  `db:"first_seen_at" json:"first_seen_at"`
	LastSeenAt            time.Time  `db:"last_seen_at" json:"last_seen_at"`
	TraceVersion          int64      `db:"trace_version" json:"trace_version"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

type OMPSKUCostSnapshot struct {
	ID                       int64           `db:"id" json:"id"`
	SKUCode                  string          `db:"sku_code" json:"sku_code"`
	SKUKind                  OMPSKUKind      `db:"sku_kind" json:"sku_kind"`
	TaskID                   *int64          `db:"task_id" json:"task_id,omitempty"`
	TaskSKUItemID            *int64          `db:"task_sku_item_id" json:"task_sku_item_id,omitempty"`
	EventSource              string          `db:"event_source" json:"event_source"`
	EventReason              string          `db:"event_reason" json:"event_reason"`
	OperatorID               *int64          `db:"operator_id" json:"operator_id,omitempty"`
	CostPrice                *float64        `db:"cost_price" json:"cost_price,omitempty"`
	CostPricePresent         bool            `db:"cost_price_present" json:"cost_price_present"`
	EstimatedCost            *float64        `db:"estimated_cost" json:"estimated_cost,omitempty"`
	EstimatedCostPresent     bool            `db:"estimated_cost_present" json:"estimated_cost_present"`
	CostRuleID               *int64          `db:"cost_rule_id" json:"cost_rule_id,omitempty"`
	CostRuleName             string          `db:"cost_rule_name" json:"cost_rule_name"`
	CostRuleSource           string          `db:"cost_rule_source" json:"cost_rule_source"`
	MatchedRuleVersion       *int            `db:"matched_rule_version" json:"matched_rule_version,omitempty"`
	PrefillSource            string          `db:"prefill_source" json:"prefill_source"`
	RequiresManualReview     bool            `db:"requires_manual_review" json:"requires_manual_review"`
	ManualCostOverride       bool            `db:"manual_cost_override" json:"manual_cost_override"`
	ManualCostOverrideReason string          `db:"manual_cost_override_reason" json:"manual_cost_override_reason"`
	InputSnapshotJSON        json.RawMessage `db:"input_snapshot_json" json:"input_snapshot_json,omitempty"`
	CalculationSnapshotJSON  json.RawMessage `db:"calculation_snapshot_json" json:"calculation_snapshot_json,omitempty"`
	CreatedAt                time.Time       `db:"created_at" json:"created_at"`
}

type OMPSKUERPTraceLog struct {
	ID                       int64           `db:"id" json:"id"`
	SKUCode                  string          `db:"sku_code" json:"sku_code"`
	SKUKind                  OMPSKUKind      `db:"sku_kind" json:"sku_kind"`
	TaskID                   *int64          `db:"task_id" json:"task_id,omitempty"`
	TaskSKUItemID            *int64          `db:"task_sku_item_id" json:"task_sku_item_id,omitempty"`
	CallLogID                *int64          `db:"call_log_id" json:"call_log_id,omitempty"`
	ConnectorKey             string          `db:"connector_key" json:"connector_key"`
	OperationKey             string          `db:"operation_key" json:"operation_key"`
	Direction                string          `db:"direction" json:"direction"`
	Status                   string          `db:"status" json:"status"`
	RequestCostPrice         *float64        `db:"request_cost_price" json:"request_cost_price,omitempty"`
	RequestCostPricePresent  bool            `db:"request_cost_price_present" json:"request_cost_price_present"`
	ResponseCostPrice        *float64        `db:"response_cost_price" json:"response_cost_price,omitempty"`
	ResponseCostPricePresent bool            `db:"response_cost_price_present" json:"response_cost_price_present"`
	RequestPayloadHash       string          `db:"request_payload_hash" json:"request_payload_hash"`
	RequestPayloadJSON       json.RawMessage `db:"request_payload_json" json:"request_payload_json,omitempty"`
	ResponsePayloadJSON      json.RawMessage `db:"response_payload_json" json:"response_payload_json,omitempty"`
	ErrorMessage             string          `db:"error_message" json:"error_message,omitempty"`
	CreatedAt                time.Time       `db:"created_at" json:"created_at"`
}

type OMPSKUComboRelation struct {
	ID              int64           `db:"id" json:"id"`
	ComboSKUCode    string          `db:"combo_sku_code" json:"combo_sku_code"`
	ChildSKUCode    string          `db:"child_sku_code" json:"child_sku_code"`
	Quantity        float64         `db:"quantity" json:"quantity"`
	Source          string          `db:"source" json:"source"`
	SourceCallLogID *int64          `db:"source_call_log_id" json:"source_call_log_id,omitempty"`
	RawPayloadJSON  json.RawMessage `db:"raw_payload_json" json:"raw_payload_json,omitempty"`
	FirstSeenAt     time.Time       `db:"first_seen_at" json:"first_seen_at"`
	LastSeenAt      time.Time       `db:"last_seen_at" json:"last_seen_at"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
}
