import type {
  ClaimConflictPayload,
  NotificationType,
  PoolReassignedPayload,
  SystemBroadcastPayload,
  TaskAssignedPayload,
  TaskCancelledPayload,
  TaskClosedPayload,
  TaskPendingAuditPayload,
  TaskRejectedPayload,
} from '@/services/v1Types'

export interface NotificationText {
  title: string
  content: string
}

function taskLabel(payload: Record<string, unknown>): string {
  const taskNo = String(payload.task_no ?? '').trim()
  return taskNo ? `任务 ${taskNo}` : '该任务'
}

export function formatNotification(
  type: NotificationType | string | undefined,
  payload: Record<string, unknown> | undefined,
): NotificationText {
  const safePayload = payload ?? {}
  const task = taskLabel(safePayload)

  switch (type) {
    case 'task_assigned_to_me': {
      const p = safePayload as unknown as TaskAssignedPayload
      if (p.action === 'reassign') {
        const actor = p.assigned_by_name ? `${p.assigned_by_name} 已将` : ''
        return { title: '任务重新分配', content: `${actor}${task}重新分配给你` }
      }
      const actor = p.assigned_by_name ? `${p.assigned_by_name} 为你分配了` : ''
      return { title: '新任务分配', content: `${actor}${task}` }
    }
    case 'task_rejected': {
      const p = safePayload as unknown as TaskRejectedPayload
      const reason = String(p.reject_reason ?? '').trim()
      return {
        title: '任务被驳回',
        content: reason ? `${task}被驳回：${reason}` : `${task}被驳回`,
      }
    }
    case 'task_pending_audit': {
      const p = safePayload as unknown as TaskPendingAuditPayload
      const team = String(p.team_name ?? p.pool_team_code ?? '').trim()
      return {
        title: '任务待审核',
        content: team ? `${task}设计完成，等待 ${team} 审核` : `${task}设计完成，等待审核`,
      }
    }
    case 'task_closed': {
      const p = safePayload as unknown as TaskClosedPayload
      const actor = String(p.closed_by_name ?? '').trim()
      return {
        title: '任务已结单',
        content: actor ? `${task}已由 ${actor} 结单` : `${task}已结单`,
      }
    }
    case 'claim_conflict': {
      const p = safePayload as unknown as ClaimConflictPayload
      const winner = p.winner_user_name ? `（${p.winner_user_name} 已领取）` : ''
      return { title: '领取冲突', content: `${task}已被他人领取${winner}` }
    }
    case 'pool_reassigned': {
      const p = safePayload as unknown as PoolReassignedPayload
      const team = p.team_name ? `至 ${p.team_name}` : ''
      return { title: '任务池调整', content: `${task}已重新分配${team}` }
    }
    case 'task_cancelled': {
      const p = safePayload as unknown as TaskCancelledPayload
      const actor = p.cancelled_by_name ? `${p.cancelled_by_name} ` : ''
      const reason = String(p.cancel_reason ?? '').trim()
      return {
        title: '任务已取消',
        content: reason ? `${task}已被${actor}取消：${reason}` : `${task}已被${actor}取消`,
      }
    }
    case 'system_broadcast': {
      const p = safePayload as unknown as SystemBroadcastPayload
      const title = String(p.title ?? '').trim() || '系统广播'
      const content = String(p.content ?? '').trim() || '你收到一条系统广播'
      return { title, content }
    }
    default:
      return { title: '系统通知', content: '你收到一条新通知' }
  }
}
