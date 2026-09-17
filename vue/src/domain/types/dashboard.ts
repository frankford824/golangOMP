export interface DashboardSummary {
  todayPendingCount: number
  pendingAuditCount: number
  handoverCount: number
  todayCompletedCount: number
  todayCreatedCount: number
  overdueCount: number
}

export interface RecentEvent {
  id: string
  /** 业务事件类型：含 GET /v1/tasks/{id}/events 返回的 event_type 及仪表盘摘要类型 */
  type: string
  title: string
  /**
   * 自 `event_type` + `payload` 等后端字段拼出的可读摘要（有则优先于仅有 title 的展示）。
   */
  summary?: string
  refId: string
  refNo: string
  actor: string
  at: string
  /** GET /v1/tasks/{id}/events 的 `created_at`（RFC3339），用于需尊重偏移的二次格式化（如侧栏 MM-DD HH:mm）。 */
  createdAtIso?: string
  previous_asset_id?: string
  current_asset_id?: string
  replacement_actor_id?: string
  replacement_actor_name?: string
  replacement_note?: string
  replacement_task_id?: string
  workflow_lane?: string
  source_department?: string
}

export interface RiskItem {
  id: string
  level: 'high' | 'medium' | 'low'
  message: string
  refId?: string
  refNo?: string
  /** 非任务详情跳转时使用，如 /tasks?tab=pool */
  route?: string
}

export interface TaskOperationalCounts {
  total_tasks: number
  active_tasks: number
  design_pending: number
	pending_audit: number
	handover: number
	customization_in_progress: number
  overdue: number
  due_today: number
  today_created: number
  today_completed: number
}

export interface TaskOperationalKpis {
  week_created: number
  week_created_completed: number
  week_completion_rate: number
  week_audit_decisions: number
  week_audit_rejected: number
  week_reject_rate: number
  week_completed: number
  average_processing_hours: number
  average_processing_sample_count: number
  exact_completion_sample_count: number
  fallback_completion_sample_count: number
  completion_event_coverage_rate: number
}

export interface TaskOperationalTrendPoint {
  date: string
  created: number
  completed: number
  due: number
}

export interface TaskOperationalStatusBucket {
	key: 'design_ops' | 'audit' | 'customization' | 'blocked' | 'completed'
  name: string
  count: number
}

export interface TaskOperationalEvent {
  id: string
  event_type: string
  title: string
  task_id: number
  task_no: string
  actor_name: string
  created_at: string
}

export interface TaskOperationalRecentTask {
  task_id: number
  task_no: string
  product_name: string
  owner_name: string
  task_status: string
  deadline_at?: string | null
}

export interface TaskOperationalOverview {
  generated_at: string
  time_zone: 'Asia/Shanghai'
  period_start: string
  period_end: string
  health_status: 'ok'
  counts: TaskOperationalCounts
  kpis: TaskOperationalKpis
  trend: TaskOperationalTrendPoint[]
  status_distribution: TaskOperationalStatusBucket[]
  recent_tasks: TaskOperationalRecentTask[]
  recent_events: TaskOperationalEvent[]
}

export interface DesignDepartmentDashboardBreakdown {
  key: string
  label: string
  task_count: number
  design_file_count: number
  sku_count: number
  share: number
}

export interface DesignDepartmentMember {
  user_id: number
  employee_no?: number | null
  display_name: string
  username: string
  team_id?: number | null
  team_name: string
  status: string
}

export interface DesignDepartmentPersonMetric extends DesignDepartmentMember {
  task_count: number
  completed_task_count: number
  active_task_count: number
  overdue_task_count: number
  design_file_count: number
  source_file_count: number
  final_file_count: number
  sku_count: number
  design_submission_count: number
  rejected_task_count: number
  audit_reject_event_count: number
  completion_rate: number
  first_pass_rate: number
  on_time_rate: number
  average_turnaround_hours: number
  workload_share: number
  task_types: DesignDepartmentDashboardBreakdown[]
}

export interface DesignDepartmentTrendPoint {
  period: string
  task_count: number
  completed_count: number
  design_file_count: number
  sku_count: number
}

export interface DesignDepartmentTaskRow {
  task_id: number
  task_no: string
  product_name: string
  designer_id: number
  designer_name: string
  task_type: string
  task_status: string
  priority: string
  created_at: string
  completed_at?: string | null
  deadline_at?: string | null
  turnaround_hours?: number | null
  source_file_count: number
  final_file_count: number
  sku_count: number
  audit_reject_count: number
  overdue: boolean
}

export interface DesignDepartmentDashboardSummary {
  task_count: number
  completed_task_count: number
  active_task_count: number
  overdue_task_count: number
  due_soon_task_count: number
  design_submission_count: number
  source_file_count: number
  final_file_count: number
  design_file_count: number
  sku_count: number
  resource_unit_count: number
  member_count: number
  contributing_member_count: number
  completion_rate: number
  first_pass_rate: number
  on_time_rate: number
  average_turnaround_hours: number
  median_turnaround_hours: number
  p90_turnaround_hours: number
  average_files_per_task: number
  turnaround_sample_count: number
  deadline_sample_count: number
  rejected_task_count: number
  audit_reject_event_count: number
}

export interface DesignDepartmentDashboard {
  generated_at: string
  time_zone: 'Asia/Shanghai'
  department_id: number
  department_name: string
  period_start: string
  period_end: string
  date_basis: 'created' | 'completed' | 'deadline'
  granularity: 'day' | 'week' | 'month'
  summary: DesignDepartmentDashboardSummary
  trend: DesignDepartmentTrendPoint[]
  people: DesignDepartmentPersonMetric[]
  task_types: DesignDepartmentDashboardBreakdown[]
  statuses: DesignDepartmentDashboardBreakdown[]
  priorities: DesignDepartmentDashboardBreakdown[]
  business_lanes: DesignDepartmentDashboardBreakdown[]
  file_types: DesignDepartmentDashboardBreakdown[]
  recent_tasks: DesignDepartmentTaskRow[]
  options: {
    members: DesignDepartmentMember[]
    task_types: string[]
    statuses: string[]
    priorities: string[]
    teams: Array<{ id: string; label: string }>
    date_bases: string[]
    granularities: string[]
  }
  definitions: Record<string, string>
}

export interface DesignDepartmentDashboardParams {
  start_date?: string
  end_date?: string
  date_basis?: DesignDepartmentDashboard['date_basis']
  granularity?: DesignDepartmentDashboard['granularity']
  designer_ids?: string
  team_ids?: string
  task_types?: string
  statuses?: string
  priorities?: string
  business_lanes?: string
}
