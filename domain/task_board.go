package domain

import "time"

// TaskQueryFilterDefinition is the shared, transport-neutral filter contract
// used by the current task list and audit handover queries.
type TaskQueryFilterDefinition struct {
	Statuses         []TaskStatus       `json:"statuses,omitempty"`
	Priorities       []TaskPriority     `json:"priorities,omitempty"`
	TaskTypes        []TaskType         `json:"task_types,omitempty"`
	SourceModes      []TaskSourceMode   `json:"source_modes,omitempty"`
	BusinessLanes    []TaskBusinessLane `json:"business_lanes,omitempty"`
	OwnerDepartments []string           `json:"owner_departments,omitempty"`
	OwnerOrgTeams    []string           `json:"owner_org_teams,omitempty"`
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
