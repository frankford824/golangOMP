export type NotificationType =
  | 'task_assigned_to_me'
  | 'task_rejected'
  | 'task_pending_audit'
  | 'task_closed'
  | 'claim_conflict'
  | 'pool_reassigned'
  | 'task_cancelled'
  | 'system_broadcast'

export interface TaskAssignedPayload {
  task_id: number
  task_no?: string
  task_type?: string
  module_key?: string
  action?: string
  assigned_by?: number
  assigned_by_name?: string
  designer_id?: number
  previous_designer_id?: number | null
  previous_handler_id?: number | null
  reason?: string
  remark?: string
  batch_request_id?: string
}

export interface TaskRejectedPayload {
  task_id: number
  reject_reason: string
  task_no?: string
  module_key?: string
  rejected_by?: number
  rejected_by_name?: string
}

export interface TaskPendingAuditPayload {
  task_id: number
  task_no?: string
  module_key?: string
  pool_team_code?: string
  team_name?: string
  designer_id?: number
  designer_name?: string
}

export interface TaskClosedPayload {
  task_id: number
  task_no?: string
  creator_id?: number
  creator_name?: string
  designer_id?: number
  designer_name?: string
  closed_by?: number
  closed_by_name?: string
  warehouse_status?: string
  auto_release?: boolean
  remark?: string
}

export interface ClaimConflictPayload {
  task_id: number
  module_key: string
  task_no?: string
  winner_user_id?: number
  winner_user_name?: string
}

export interface PoolReassignedPayload {
  task_id: number
  module_key: string
  task_no?: string
  team_code?: string
  team_name?: string
  reassigned_by?: number
  reassigned_by_name?: string
}

export interface TaskCancelledPayload {
  task_id: number
  cancel_reason: string
  cancelled_by: number
  task_no?: string
  cancelled_by_name?: string
  module_key?: string
}

export interface SystemBroadcastPayload {
  title: string
  content: string
  broadcast_by?: number
  broadcast_by_name?: string
  broadcast_audience?: string
  broadcast_recipient_count?: number
}

export type NotificationPayload =
  | TaskAssignedPayload
  | TaskRejectedPayload
  | TaskPendingAuditPayload
  | TaskClosedPayload
  | ClaimConflictPayload
  | PoolReassignedPayload
  | TaskCancelledPayload
  | SystemBroadcastPayload
  | Record<string, unknown>

export interface NotificationRecord {
  id: number
  notification_type: NotificationType | string
  payload?: NotificationPayload
  is_read?: boolean
  created_at?: string
}
