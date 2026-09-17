package domain

import "time"

// TaskOperationalBucket identifies one authoritative dashboard count definition.
// Task-list drilldowns use the same predicate so the displayed total matches the
// dashboard card for callers with the same data scope.
type TaskOperationalBucket string

const (
	TaskOperationalBucketActive                  TaskOperationalBucket = "active_tasks"
	TaskOperationalBucketDesignPending           TaskOperationalBucket = "design_pending"
	TaskOperationalBucketPendingAudit            TaskOperationalBucket = "pending_audit"
	TaskOperationalBucketHandover                TaskOperationalBucket = "handover"
	TaskOperationalBucketCustomizationInProgress TaskOperationalBucket = "customization_in_progress"
	TaskOperationalBucketOverdue                 TaskOperationalBucket = "overdue"
	TaskOperationalBucketDueToday                TaskOperationalBucket = "due_today"
	TaskOperationalBucketTodayCreated            TaskOperationalBucket = "today_created"
)

func (bucket TaskOperationalBucket) Valid() bool {
	switch bucket {
	case TaskOperationalBucketActive,
		TaskOperationalBucketDesignPending,
		TaskOperationalBucketPendingAudit,
		TaskOperationalBucketHandover,
		TaskOperationalBucketCustomizationInProgress,
		TaskOperationalBucketOverdue,
		TaskOperationalBucketDueToday,
		TaskOperationalBucketTodayCreated:
		return true
	default:
		return false
	}
}

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

type DesignDashboardDateBasis string

const (
	DesignDashboardDateCreated   DesignDashboardDateBasis = "created"
	DesignDashboardDateCompleted DesignDashboardDateBasis = "completed"
	DesignDashboardDateDeadline  DesignDashboardDateBasis = "deadline"
)

func (v DesignDashboardDateBasis) Valid() bool {
	return v == DesignDashboardDateCreated || v == DesignDashboardDateCompleted || v == DesignDashboardDateDeadline
}

type DesignDashboardGranularity string

const (
	DesignDashboardGranularityDay   DesignDashboardGranularity = "day"
	DesignDashboardGranularityWeek  DesignDashboardGranularity = "week"
	DesignDashboardGranularityMonth DesignDashboardGranularity = "month"
)

func (v DesignDashboardGranularity) Valid() bool {
	return v == DesignDashboardGranularityDay || v == DesignDashboardGranularityWeek || v == DesignDashboardGranularityMonth
}

type DesignDepartmentDashboardFilter struct {
	DepartmentID  int64
	StartAt       time.Time
	EndAt         time.Time
	DesignerIDs   []int64
	TeamIDs       []int64
	TaskTypes     []TaskType
	Statuses      []TaskStatus
	Priorities    []TaskPriority
	BusinessLanes []TaskBusinessLane
	DateBasis     DesignDashboardDateBasis
	Granularity   DesignDashboardGranularity
}

type DesignDepartmentMember struct {
	UserID      int64  `json:"user_id"`
	EmployeeNo  *int64 `json:"employee_no,omitempty"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
	TeamID      *int64 `json:"team_id,omitempty"`
	TeamName    string `json:"team_name"`
	Status      string `json:"status"`
}

type DesignDepartmentTaskFact struct {
	TaskID                int64
	TaskNo                string
	ProductName           string
	TaskType              TaskType
	TaskStatus            TaskStatus
	Priority              TaskPriority
	BusinessLane          TaskBusinessLane
	DesignerID            int64
	DesignerName          string
	DesignerStatus        string
	TeamID                *int64
	TeamName              string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CompletedAt           *time.Time
	DeadlineAt            *time.Time
	PrimaryAt             time.Time
	SKUCount              int64
	ResourceUnitCount     int64
	SourceFileCount       int64
	FinalFileCount        int64
	DesignSubmissionCount int64
	AuditRejectCount      int64
	AuditApproveCount     int64
}

type DesignDepartmentDashboardSummary struct {
	TaskCount               int64   `json:"task_count"`
	CompletedTaskCount      int64   `json:"completed_task_count"`
	ActiveTaskCount         int64   `json:"active_task_count"`
	OverdueTaskCount        int64   `json:"overdue_task_count"`
	DueSoonTaskCount        int64   `json:"due_soon_task_count"`
	DesignSubmissionCount   int64   `json:"design_submission_count"`
	SourceFileCount         int64   `json:"source_file_count"`
	FinalFileCount          int64   `json:"final_file_count"`
	DesignFileCount         int64   `json:"design_file_count"`
	SKUCount                int64   `json:"sku_count"`
	ResourceUnitCount       int64   `json:"resource_unit_count"`
	MemberCount             int64   `json:"member_count"`
	ContributingMemberCount int64   `json:"contributing_member_count"`
	CompletionRate          float64 `json:"completion_rate"`
	FirstPassRate           float64 `json:"first_pass_rate"`
	OnTimeRate              float64 `json:"on_time_rate"`
	AverageTurnaroundHours  float64 `json:"average_turnaround_hours"`
	MedianTurnaroundHours   float64 `json:"median_turnaround_hours"`
	P90TurnaroundHours      float64 `json:"p90_turnaround_hours"`
	AverageFilesPerTask     float64 `json:"average_files_per_task"`
	TurnaroundSampleCount   int64   `json:"turnaround_sample_count"`
	DeadlineSampleCount     int64   `json:"deadline_sample_count"`
	RejectedTaskCount       int64   `json:"rejected_task_count"`
	AuditRejectEventCount   int64   `json:"audit_reject_event_count"`
}

type DesignDepartmentDashboardBreakdown struct {
	Key             string  `json:"key"`
	Label           string  `json:"label"`
	TaskCount       int64   `json:"task_count"`
	DesignFileCount int64   `json:"design_file_count"`
	SKUCount        int64   `json:"sku_count"`
	Share           float64 `json:"share"`
}

type DesignDepartmentDashboardTrendPoint struct {
	Period          string `json:"period"`
	TaskCount       int64  `json:"task_count"`
	CompletedCount  int64  `json:"completed_count"`
	DesignFileCount int64  `json:"design_file_count"`
	SKUCount        int64  `json:"sku_count"`
}

type DesignDepartmentPersonMetric struct {
	UserID                 int64                                `json:"user_id"`
	DisplayName            string                               `json:"display_name"`
	Username               string                               `json:"username"`
	TeamID                 *int64                               `json:"team_id,omitempty"`
	TeamName               string                               `json:"team_name"`
	Status                 string                               `json:"status"`
	TaskCount              int64                                `json:"task_count"`
	CompletedTaskCount     int64                                `json:"completed_task_count"`
	ActiveTaskCount        int64                                `json:"active_task_count"`
	OverdueTaskCount       int64                                `json:"overdue_task_count"`
	DesignFileCount        int64                                `json:"design_file_count"`
	SourceFileCount        int64                                `json:"source_file_count"`
	FinalFileCount         int64                                `json:"final_file_count"`
	SKUCount               int64                                `json:"sku_count"`
	DesignSubmissionCount  int64                                `json:"design_submission_count"`
	RejectedTaskCount      int64                                `json:"rejected_task_count"`
	AuditRejectEventCount  int64                                `json:"audit_reject_event_count"`
	CompletionRate         float64                              `json:"completion_rate"`
	FirstPassRate          float64                              `json:"first_pass_rate"`
	OnTimeRate             float64                              `json:"on_time_rate"`
	AverageTurnaroundHours float64                              `json:"average_turnaround_hours"`
	WorkloadShare          float64                              `json:"workload_share"`
	TaskTypes              []DesignDepartmentDashboardBreakdown `json:"task_types"`
}

type DesignDepartmentTaskRow struct {
	TaskID           int64        `json:"task_id"`
	TaskNo           string       `json:"task_no"`
	ProductName      string       `json:"product_name"`
	DesignerID       int64        `json:"designer_id"`
	DesignerName     string       `json:"designer_name"`
	TaskType         TaskType     `json:"task_type"`
	TaskStatus       TaskStatus   `json:"task_status"`
	Priority         TaskPriority `json:"priority"`
	CreatedAt        time.Time    `json:"created_at"`
	CompletedAt      *time.Time   `json:"completed_at,omitempty"`
	DeadlineAt       *time.Time   `json:"deadline_at,omitempty"`
	TurnaroundHours  *float64     `json:"turnaround_hours,omitempty"`
	SourceFileCount  int64        `json:"source_file_count"`
	FinalFileCount   int64        `json:"final_file_count"`
	SKUCount         int64        `json:"sku_count"`
	AuditRejectCount int64        `json:"audit_reject_count"`
	Overdue          bool         `json:"overdue"`
}

type DesignDepartmentDashboardOptions struct {
	Members       []DesignDepartmentMember `json:"members"`
	TaskTypes     []string                 `json:"task_types"`
	Statuses      []string                 `json:"statuses"`
	Priorities    []string                 `json:"priorities"`
	Teams         []DesignDepartmentOption `json:"teams"`
	DateBases     []string                 `json:"date_bases"`
	Granularities []string                 `json:"granularities"`
}

type DesignDepartmentOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type DesignDepartmentDashboardDefinitions struct {
	TaskCount       string `json:"task_count"`
	DesignFileCount string `json:"design_file_count"`
	FirstPassRate   string `json:"first_pass_rate"`
	Turnaround      string `json:"turnaround"`
	OnTimeRate      string `json:"on_time_rate"`
}

type DesignDepartmentDashboard struct {
	GeneratedAt    time.Time                             `json:"generated_at"`
	TimeZone       string                                `json:"time_zone"`
	DepartmentID   int64                                 `json:"department_id"`
	DepartmentName string                                `json:"department_name"`
	PeriodStart    time.Time                             `json:"period_start"`
	PeriodEnd      time.Time                             `json:"period_end"`
	DateBasis      DesignDashboardDateBasis              `json:"date_basis"`
	Granularity    DesignDashboardGranularity            `json:"granularity"`
	Summary        DesignDepartmentDashboardSummary      `json:"summary"`
	Trend          []DesignDepartmentDashboardTrendPoint `json:"trend"`
	People         []DesignDepartmentPersonMetric        `json:"people"`
	TaskTypes      []DesignDepartmentDashboardBreakdown  `json:"task_types"`
	Statuses       []DesignDepartmentDashboardBreakdown  `json:"statuses"`
	Priorities     []DesignDepartmentDashboardBreakdown  `json:"priorities"`
	BusinessLanes  []DesignDepartmentDashboardBreakdown  `json:"business_lanes"`
	FileTypes      []DesignDepartmentDashboardBreakdown  `json:"file_types"`
	RecentTasks    []DesignDepartmentTaskRow             `json:"recent_tasks"`
	Options        DesignDepartmentDashboardOptions      `json:"options"`
	Definitions    DesignDepartmentDashboardDefinitions  `json:"definitions"`
}
