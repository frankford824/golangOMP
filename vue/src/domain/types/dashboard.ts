export interface DashboardSummary {
  todayPendingCount: number
  pendingAuditCount: number
  handoverCount: number
  pendingOutsourceReturnCount: number
  pendingWarehouseReceiveCount: number
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
