package domain

import "time"

type TaskCostOverrideAuditEventType string

const (
	TaskCostOverrideAuditEventApplied  TaskCostOverrideAuditEventType = "override_applied"
	TaskCostOverrideAuditEventUpdated  TaskCostOverrideAuditEventType = "override_updated"
	TaskCostOverrideAuditEventReleased TaskCostOverrideAuditEventType = "override_released"
)

type TaskCostOverrideAuditEvent struct {
	EventID               string                              `db:"event_id"                 json:"event_id"`
	TaskID                int64                               `db:"task_id"                  json:"task_id"`
	TaskDetailID          *int64                              `db:"task_detail_id"           json:"task_detail_id,omitempty"`
	Sequence              int64                               `db:"sequence"                 json:"sequence"`
	EventType             TaskCostOverrideAuditEventType      `db:"event_type"               json:"event_type"`
	CategoryCode          string                              `db:"category_code"            json:"category_code"`
	MatchedRuleID         *int64                              `db:"matched_rule_id"          json:"matched_rule_id,omitempty"`
	MatchedRuleVersion    *int                                `db:"matched_rule_version"     json:"matched_rule_version,omitempty"`
	MatchedRuleSource     string                              `db:"matched_rule_source"      json:"matched_rule_source"`
	GovernanceStatus      CostRuleGovernanceStatus            `db:"governance_status"        json:"governance_status"`
	PreviousEstimatedCost *float64                            `db:"previous_estimated_cost"  json:"previous_estimated_cost,omitempty"`
	PreviousCostPrice     *float64                            `db:"previous_cost_price"      json:"previous_cost_price,omitempty"`
	OverrideCost          *float64                            `db:"override_cost"            json:"override_cost,omitempty"`
	ResultCostPrice       *float64                            `db:"result_cost_price"        json:"result_cost_price,omitempty"`
	OverrideReason        string                              `db:"override_reason"          json:"override_reason"`
	OverrideActor         string                              `db:"override_actor"           json:"override_actor"`
	OverrideAt            time.Time                           `db:"override_at"              json:"override_at"`
	Source                string                              `db:"source"                   json:"source"`
	Note                  string                              `db:"note"                     json:"note"`
	CreatedAt             time.Time                           `db:"created_at"               json:"created_at"`
	OverrideBoundary      *TaskCostOverrideGovernanceBoundary `json:"override_governance_boundary,omitempty"`
}

type TaskGovernanceAuditSummary struct {
	AuditLayer            string                         `json:"audit_layer"`
	GeneralEventLayer     string                         `json:"general_event_layer"`
	EventCount            int                            `json:"event_count"`
	LatestEventID         string                         `json:"latest_event_id"`
	LatestEventType       TaskCostOverrideAuditEventType `json:"latest_event_type"`
	LatestEventAt         *time.Time                     `json:"latest_event_at,omitempty"`
	CurrentOverrideActive bool                           `json:"current_override_active"`
}

type TaskCostOverrideAuditTimeline struct {
	TaskID                 int64                               `json:"task_id"`
	Events                 []*TaskCostOverrideAuditEvent       `json:"events"`
	GovernanceAuditSummary *TaskGovernanceAuditSummary         `json:"governance_audit_summary,omitempty"`
	OverrideBoundary       *TaskCostOverrideGovernanceBoundary `json:"override_governance_boundary,omitempty"`
}
