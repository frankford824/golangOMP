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
	Priorities                   []TaskPriority                  `json:"priorities,omitempty"`
	TaskTypes                    []TaskType                      `json:"task_types,omitempty"`
	SourceModes                  []TaskSourceMode                `json:"source_modes,omitempty"`
	BusinessLanes                []TaskBusinessLane              `json:"business_lanes,omitempty"`
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

// TaskOperationalCounts contains global task-flow counts for the main operations dashboard.
// All date boundaries are evaluated in TimeZone and returned with GeneratedAt so the frontend
// never has to infer reporting windows from a paginated task list.
type TaskOperationalCounts struct {
	TotalTasks              int64 `json:"total_tasks"`
	ActiveTasks             int64 `json:"active_tasks"`
	DesignPending           int64 `json:"design_pending"`
	PendingAudit            int64 `json:"pending_audit"`
	Handover                int64 `json:"handover"`
	CustomizationInProgress int64 `json:"customization_in_progress"`
	Overdue                 int64 `json:"overdue"`
	DueToday                int64 `json:"due_today"`
	TodayCreated            int64 `json:"today_created"`
	TodayCompleted          int64 `json:"today_completed"`
}

type TaskOperationalKPIs struct {
	WeekCreated                   int64   `json:"week_created"`
	WeekCreatedCompleted          int64   `json:"week_created_completed"`
	WeekCompletionRate            float64 `json:"week_completion_rate"`
	WeekAuditDecisions            int64   `json:"week_audit_decisions"`
	WeekAuditRejected             int64   `json:"week_audit_rejected"`
	WeekRejectRate                float64 `json:"week_reject_rate"`
	WeekCompleted                 int64   `json:"week_completed"`
	AverageProcessingHours        float64 `json:"average_processing_hours"`
	AverageProcessingSampleCount  int64   `json:"average_processing_sample_count"`
	ExactCompletionSampleCount    int64   `json:"exact_completion_sample_count"`
	FallbackCompletionSampleCount int64   `json:"fallback_completion_sample_count"`
	CompletionEventCoverageRate   float64 `json:"completion_event_coverage_rate"`
}

type TaskOperationalTrendPoint struct {
	Date      string `json:"date"`
	Created   int64  `json:"created"`
	Completed int64  `json:"completed"`
	Due       int64  `json:"due"`
}

type TaskOperationalStatusBucket struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type TaskOperationalEvent struct {
	ID        string    `json:"id"`
	EventType string    `json:"event_type"`
	Title     string    `json:"title"`
	TaskID    int64     `json:"task_id"`
	TaskNo    string    `json:"task_no"`
	ActorName string    `json:"actor_name"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskOperationalRecentTask struct {
	TaskID      int64      `json:"task_id"`
	TaskNo      string     `json:"task_no"`
	ProductName string     `json:"product_name"`
	OwnerName   string     `json:"owner_name"`
	TaskStatus  TaskStatus `json:"task_status"`
	DeadlineAt  *time.Time `json:"deadline_at,omitempty"`
}

type TaskOperationalOverview struct {
	GeneratedAt        time.Time                     `json:"generated_at"`
	TimeZone           string                        `json:"time_zone"`
	PeriodStart        time.Time                     `json:"period_start"`
	PeriodEnd          time.Time                     `json:"period_end"`
	HealthStatus       string                        `json:"health_status"`
	Counts             TaskOperationalCounts         `json:"counts"`
	KPIs               TaskOperationalKPIs           `json:"kpis"`
	Trend              []TaskOperationalTrendPoint   `json:"trend"`
	StatusDistribution []TaskOperationalStatusBucket `json:"status_distribution"`
	RecentTasks        []TaskOperationalRecentTask   `json:"recent_tasks"`
	RecentEvents       []TaskOperationalEvent        `json:"recent_events"`
}
