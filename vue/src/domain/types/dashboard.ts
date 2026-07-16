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
  pending_warehouse_receive: number
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
  key: 'design_ops' | 'audit' | 'customization' | 'warehouse' | 'completed'
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
