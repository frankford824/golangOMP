package domain

import "time"

type TaskCostOverrideReviewStatus string

const (
	TaskCostOverrideReviewStatusNotRequired TaskCostOverrideReviewStatus = "not_required"
	TaskCostOverrideReviewStatusPending     TaskCostOverrideReviewStatus = "pending"
	TaskCostOverrideReviewStatusApproved    TaskCostOverrideReviewStatus = "approved"
	TaskCostOverrideReviewStatusRejected    TaskCostOverrideReviewStatus = "rejected"
)

type TaskCostOverrideFinanceStatus string

const (
	TaskCostOverrideFinanceStatusNotRequired   TaskCostOverrideFinanceStatus = "not_required"
	TaskCostOverrideFinanceStatusPending       TaskCostOverrideFinanceStatus = "pending"
	TaskCostOverrideFinanceStatusReadyForView  TaskCostOverrideFinanceStatus = "ready_for_view"
	TaskCostOverrideFinanceStatusMarkedForView TaskCostOverrideFinanceStatus = "marked_for_view"
)

type TaskCostOverrideReviewRecord struct {
	RecordID        int64                        `db:"record_id"         json:"record_id"`
	OverrideEventID string                       `db:"override_event_id" json:"override_event_id"`
	TaskID          int64                        `db:"task_id"           json:"task_id"`
	ReviewRequired  bool                         `db:"review_required"   json:"review_required"`
	ReviewStatus    TaskCostOverrideReviewStatus `db:"review_status"     json:"review_status"`
	ReviewNote      string                       `db:"review_note"       json:"review_note"`
	ReviewActor     string                       `db:"review_actor"      json:"review_actor"`
	ReviewedAt      *time.Time                   `db:"reviewed_at"       json:"reviewed_at,omitempty"`
	CreatedAt       time.Time                    `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time                    `db:"updated_at"        json:"updated_at"`
}

type TaskCostFinanceFlag struct {
	RecordID        int64                         `db:"record_id"         json:"record_id"`
	OverrideEventID string                        `db:"override_event_id" json:"override_event_id"`
	TaskID          int64                         `db:"task_id"           json:"task_id"`
	FinanceRequired bool                          `db:"finance_required"  json:"finance_required"`
	FinanceStatus   TaskCostOverrideFinanceStatus `db:"finance_status"    json:"finance_status"`
	FinanceNote     string                        `db:"finance_note"      json:"finance_note"`
	FinanceMarkedBy string                        `db:"finance_marked_by" json:"finance_marked_by"`
	FinanceMarkedAt *time.Time                    `db:"finance_marked_at" json:"finance_marked_at,omitempty"`
	CreatedAt       time.Time                     `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time                     `db:"updated_at"        json:"updated_at"`
}

type TaskCostOverrideBoundaryActionSummary struct {
	ActionType string     `json:"action_type"`
	Status     string     `json:"status"`
	Actor      string     `json:"actor"`
	ActedAt    *time.Time `json:"acted_at,omitempty"`
	Note       string     `json:"note"`
	Source     string     `json:"source"`
}

type TaskCostOverrideApprovalPlaceholderSummary struct {
	OverrideEventID           string                                 `json:"override_event_id"`
	TaskID                    int64                                  `json:"task_id"`
	ReviewRecordID            *int64                                 `json:"review_record_id,omitempty"`
	ReviewRequired            bool                                   `json:"review_required"`
	ReviewStatus              TaskCostOverrideReviewStatus           `json:"review_status"`
	ApprovalPlaceholderStatus TaskCostOverrideReviewStatus           `json:"approval_placeholder_status"`
	ReviewNote                string                                 `json:"review_note"`
	ReviewActor               string                                 `json:"review_actor"`
	ReviewedAt                *time.Time                             `json:"reviewed_at,omitempty"`
	LatestReviewAction        *TaskCostOverrideBoundaryActionSummary `json:"latest_review_action,omitempty"`
	Source                    string                                 `json:"source"`
	IsPlaceholderBoundaryOnly bool                                   `json:"is_placeholder_boundary_only"`
}

type TaskCostOverrideFinancePlaceholderSummary struct {
	OverrideEventID           string                                 `json:"override_event_id"`
	TaskID                    int64                                  `json:"task_id"`
	FinanceRecordID           *int64                                 `json:"finance_record_id,omitempty"`
	FinanceRequired           bool                                   `json:"finance_required"`
	FinanceStatus             TaskCostOverrideFinanceStatus          `json:"finance_status"`
	FinancePlaceholderStatus  TaskCostOverrideFinanceStatus          `json:"finance_placeholder_status"`
	FinanceNote               string                                 `json:"finance_note"`
	FinanceMarkedBy           string                                 `json:"finance_marked_by"`
	FinanceMarkedAt           *time.Time                             `json:"finance_marked_at,omitempty"`
	FinanceViewReady          bool                                   `json:"finance_view_ready"`
	LatestFinanceAction       *TaskCostOverrideBoundaryActionSummary `json:"latest_finance_action,omitempty"`
	Source                    string                                 `json:"source"`
	IsPlaceholderBoundaryOnly bool                                   `json:"is_placeholder_boundary_only"`
}

type TaskCostOverrideGovernanceBoundarySummary struct {
	ReviewRequired            bool                                   `json:"review_required"`
	ReviewStatus              TaskCostOverrideReviewStatus           `json:"review_status"`
	FinanceRequired           bool                                   `json:"finance_required"`
	FinanceStatus             TaskCostOverrideFinanceStatus          `json:"finance_status"`
	FinanceViewReady          bool                                   `json:"finance_view_ready"`
	LatestReviewAction        *TaskCostOverrideBoundaryActionSummary `json:"latest_review_action,omitempty"`
	LatestFinanceAction       *TaskCostOverrideBoundaryActionSummary `json:"latest_finance_action,omitempty"`
	LatestBoundaryActor       string                                 `json:"latest_boundary_actor"`
	LatestBoundaryAt          *time.Time                             `json:"latest_boundary_at,omitempty"`
	IsPlaceholderBoundaryOnly bool                                   `json:"is_placeholder_boundary_only"`
}

type TaskCostOverrideGovernanceBoundary struct {
	OverrideEventID            string                                      `json:"override_event_id"`
	TaskID                     int64                                       `json:"task_id"`
	ReviewRecordID             *int64                                      `json:"review_record_id,omitempty"`
	FinanceRecordID            *int64                                      `json:"finance_record_id,omitempty"`
	ReviewRequired             bool                                        `json:"review_required"`
	ReviewStatus               TaskCostOverrideReviewStatus                `json:"review_status"`
	ApprovalPlaceholderStatus  TaskCostOverrideReviewStatus                `json:"approval_placeholder_status"`
	ReviewNote                 string                                      `json:"review_note"`
	ReviewActor                string                                      `json:"review_actor"`
	ReviewedAt                 *time.Time                                  `json:"reviewed_at,omitempty"`
	FinanceRequired            bool                                        `json:"finance_required"`
	FinanceStatus              TaskCostOverrideFinanceStatus               `json:"finance_status"`
	FinancePlaceholderStatus   TaskCostOverrideFinanceStatus               `json:"finance_placeholder_status"`
	FinanceNote                string                                      `json:"finance_note"`
	FinanceMarkedBy            string                                      `json:"finance_marked_by"`
	FinanceMarkedAt            *time.Time                                  `json:"finance_marked_at,omitempty"`
	FinanceViewReady           bool                                        `json:"finance_view_ready"`
	LatestReviewAction         *TaskCostOverrideBoundaryActionSummary      `json:"latest_review_action,omitempty"`
	LatestFinanceAction        *TaskCostOverrideBoundaryActionSummary      `json:"latest_finance_action,omitempty"`
	LatestBoundaryActor        string                                      `json:"latest_boundary_actor"`
	LatestBoundaryAt           *time.Time                                  `json:"latest_boundary_at,omitempty"`
	GovernanceBoundarySummary  *TaskCostOverrideGovernanceBoundarySummary  `json:"governance_boundary_summary,omitempty"`
	ApprovalPlaceholderSummary *TaskCostOverrideApprovalPlaceholderSummary `json:"approval_placeholder_summary,omitempty"`
	FinancePlaceholderSummary  *TaskCostOverrideFinancePlaceholderSummary  `json:"finance_placeholder_summary,omitempty"`
	IsPlaceholderBoundaryOnly  bool                                        `json:"is_placeholder_boundary_only"`
	PolicyMode                 PolicyMode                                  `json:"policy_mode,omitempty"`
	VisibleToRoles             []Role                                      `json:"visible_to_roles,omitempty"`
	ActionRoles                []ActionPolicySummary                       `json:"action_roles,omitempty"`
	PolicyScopeSummary         *PolicyScopeSummary                         `json:"policy_scope_summary,omitempty"`
	PlatformEntryBoundary      *PlatformEntryBoundary                      `json:"platform_entry_boundary,omitempty"`
}
