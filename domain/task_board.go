package domain

import "time"

type TaskBoardView string

const (
	TaskBoardViewAll         TaskBoardView = "all"
	TaskBoardViewOps         TaskBoardView = "ops"
	TaskBoardViewDesigner    TaskBoardView = "designer"
	TaskBoardViewAudit       TaskBoardView = "audit"
	TaskBoardViewProcurement TaskBoardView = "procurement"
	TaskBoardViewWarehouse   TaskBoardView = "warehouse"
)

func (v TaskBoardView) Valid() bool {
	switch v {
	case TaskBoardViewAll, TaskBoardViewOps, TaskBoardViewDesigner, TaskBoardViewAudit, TaskBoardViewProcurement, TaskBoardViewWarehouse:
		return true
	default:
		return false
	}
}

type TaskQueryFilterDefinition struct {
	Statuses                     []TaskStatus                    `json:"statuses,omitempty"`
	TaskTypes                    []TaskType                      `json:"task_types,omitempty"`
	SourceModes                  []TaskSourceMode                `json:"source_modes,omitempty"`
	WorkflowLanes                []WorkflowLane                  `json:"workflow_lanes,omitempty"`
	MainStatuses                 []TaskMainStatus                `json:"main_statuses,omitempty"`
	SubStatusScope               *TaskSubStatusScope             `json:"sub_status_scope,omitempty"`
	SubStatusCodes               []TaskSubStatusCode             `json:"sub_status_codes,omitempty"`
	CoordinationStatuses         []ProcurementCoordinationStatus `json:"coordination_statuses,omitempty"`
	OwnerDepartments             []string                        `json:"owner_departments,omitempty"`
	OwnerOrgTeams                []string                        `json:"owner_org_teams,omitempty"`
	WarehousePrepareReady        *bool                           `json:"warehouse_prepare_ready,omitempty"`
	WarehouseReceiveReady        *bool                           `json:"warehouse_receive_ready,omitempty"`
	WarehouseBlockingReasonCodes []WorkflowReasonCode            `json:"warehouse_blocking_reason_codes,omitempty"`
}

type TaskQueryTemplate struct {
	Status                      string `json:"status,omitempty"`
	TaskType                    string `json:"task_type,omitempty"`
	SourceMode                  string `json:"source_mode,omitempty"`
	WorkflowLane                string `json:"workflow_lane,omitempty"`
	MainStatus                  string `json:"main_status,omitempty"`
	SubStatusCode               string `json:"sub_status_code,omitempty"`
	SubStatusScope              string `json:"sub_status_scope,omitempty"`
	CoordinationStatus          string `json:"coordination_status,omitempty"`
	WarehouseBlockingReasonCode string `json:"warehouse_blocking_reason_code,omitempty"`
	Keyword                     string `json:"keyword,omitempty"`
	CreatorID                   *int64 `json:"creator_id,omitempty"`
	DesignerID                  *int64 `json:"designer_id,omitempty"`
	OwnerDepartment             string `json:"owner_department,omitempty"`
	OwnerOrgTeam                string `json:"owner_org_team,omitempty"`
	NeedOutsource               *bool  `json:"need_outsource,omitempty"`
	Overdue                     *bool  `json:"overdue,omitempty"`
	WarehousePrepareReady       *bool  `json:"warehouse_prepare_ready,omitempty"`
	WarehouseReceiveReady       *bool  `json:"warehouse_receive_ready,omitempty"`
}

type TaskBoardFiltersSchema struct {
	BoardViews                []TaskBoardView `json:"board_views"`
	SupportedGlobalFilters    []string        `json:"supported_global_filters"`
	QueueConditionFields      []string        `json:"queue_condition_fields"`
	TaskListEndpoint          string          `json:"task_list_endpoint"`
	TaskListPassthroughFields []string        `json:"task_list_passthrough_fields"`
}

// TaskBoardQueueOwnershipHints are lightweight frontend hints only.
// They do not enforce access control or real queue ownership.
type TaskBoardQueueOwnershipHints struct {
	SuggestedRoles     []Role `json:"suggested_roles,omitempty"`
	SuggestedActorType string `json:"suggested_actor_type,omitempty"`
	DefaultVisibility  string `json:"default_visibility,omitempty"`
	OwnershipHint      string `json:"ownership_hint,omitempty"`
}

type TaskBoardQueueSummary struct {
	QueueKey         string        `json:"queue_key"`
	QueueName        string        `json:"queue_name"`
	QueueDescription string        `json:"queue_description,omitempty"`
	BoardView        TaskBoardView `json:"board_view"`
	TaskBoardQueueOwnershipHints
	Filters            TaskQueryFilterDefinition `json:"filters"`
	NormalizedFilters  TaskQueryFilterDefinition `json:"normalized_filters"`
	QueryTemplate      TaskQueryTemplate         `json:"query_template"`
	Count              int64                     `json:"count"`
	SampleTasks        []*TaskListItem           `json:"sample_tasks"`
	PolicyMode         PolicyMode                `json:"policy_mode,omitempty"`
	VisibleToRoles     []Role                    `json:"visible_to_roles,omitempty"`
	ActionRoles        []ActionPolicySummary     `json:"action_roles,omitempty"`
	PolicyScopeSummary *PolicyScopeSummary       `json:"policy_scope_summary,omitempty"`
}

type TaskBoardSummary struct {
	BoardView          TaskBoardView           `json:"board_view"`
	BoardName          string                  `json:"board_name"`
	GeneratedAt        time.Time               `json:"generated_at"`
	FiltersSchema      TaskBoardFiltersSchema  `json:"filters_schema"`
	Queues             []TaskBoardQueueSummary `json:"queues"`
	PolicyMode         PolicyMode              `json:"policy_mode,omitempty"`
	VisibleToRoles     []Role                  `json:"visible_to_roles,omitempty"`
	ActionRoles        []ActionPolicySummary   `json:"action_roles,omitempty"`
	PolicyScopeSummary *PolicyScopeSummary     `json:"policy_scope_summary,omitempty"`
}

type TaskBoardQueue struct {
	QueueKey         string        `json:"queue_key"`
	QueueName        string        `json:"queue_name"`
	QueueDescription string        `json:"queue_description,omitempty"`
	BoardView        TaskBoardView `json:"board_view"`
	TaskBoardQueueOwnershipHints
	Filters            TaskQueryFilterDefinition `json:"filters"`
	NormalizedFilters  TaskQueryFilterDefinition `json:"normalized_filters"`
	QueryTemplate      TaskQueryTemplate         `json:"query_template"`
	Count              int64                     `json:"count"`
	Tasks              []*TaskListItem           `json:"tasks"`
	Pagination         PaginationMeta            `json:"pagination"`
	PolicyMode         PolicyMode                `json:"policy_mode,omitempty"`
	VisibleToRoles     []Role                    `json:"visible_to_roles,omitempty"`
	ActionRoles        []ActionPolicySummary     `json:"action_roles,omitempty"`
	PolicyScopeSummary *PolicyScopeSummary       `json:"policy_scope_summary,omitempty"`
}

type TaskBoardQueuesResponse struct {
	BoardView          TaskBoardView          `json:"board_view"`
	BoardName          string                 `json:"board_name"`
	GeneratedAt        time.Time              `json:"generated_at"`
	FiltersSchema      TaskBoardFiltersSchema `json:"filters_schema"`
	Queues             []TaskBoardQueue       `json:"queues"`
	PolicyMode         PolicyMode             `json:"policy_mode,omitempty"`
	VisibleToRoles     []Role                 `json:"visible_to_roles,omitempty"`
	ActionRoles        []ActionPolicySummary  `json:"action_roles,omitempty"`
	PolicyScopeSummary *PolicyScopeSummary    `json:"policy_scope_summary,omitempty"`
}
