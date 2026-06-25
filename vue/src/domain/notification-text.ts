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

function businessTeamLabel(value: unknown, fallback = '对应小组'): string {
  const raw = String(value ?? '').trim()
  const labels: Record<string, string> = {
    design_standard: '设计组',
    design_retouch: '精修组',
    audit_standard: '常规审核组',
    audit_a: '常规审核组',
    audit_customization: '定制审核组',
    audit_b: '定制审核组',
    customization_art: '定制美工组',
    warehouse_main: '云仓组',
    procurement_main: '采购组',
  }
  if (labels[raw]) return labels[raw]
  if (!raw || raw.includes('_') || raw.includes('.')) return fallback
  return raw
}

function businessModuleLabel(value: unknown, fallback = '相关环节'): string {
  const raw = String(value ?? '').trim()
  const labels: Record<string, string> = {
    basic_info: '任务信息',
    design: '设计',
    retouch: '精修',
    audit: '审核',
    customization: '定制美工',
    warehouse: '仓库',
    procurement: '采购',
  }
  if (labels[raw]) return labels[raw]
  if (!raw || raw.includes('_') || raw.includes('.')) return fallback
  return raw
}

function displayTeamLabel(primary: unknown, code: unknown, fallback = '对应小组'): string {
  return businessTeamLabel(primary, '') || businessTeamLabel(code, fallback)
}

function displayModuleLabel(primary: unknown, code: unknown, fallback = '相关环节'): string {
  return businessModuleLabel(primary, '') || businessModuleLabel(code, fallback)
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
      const team = displayTeamLabel(p.team_name, p.pool_team_code, '审核组')
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
      const raw = safePayload as Record<string, unknown>
      const team = displayTeamLabel(raw.team_name, p.team_code, '')
      const module = displayModuleLabel(raw.module_name, p.module_key, '任务池')
      return {
        title: '任务池调整',
        content: team ? `${task}已进入${team}的${module}` : `${task}已进入新的${module}`,
      }
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
