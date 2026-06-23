/**
 * Step 87：建档/ERP 同步状态映射
 * 统一前端展示文案，不自行推导业务规则，以后端返回状态为准。
 */

export type FilingStatusTone = 'success' | 'warning' | 'error' | 'info' | 'neutral'

function isRetouchTaskType(taskType: string | undefined | null): boolean {
  if (!taskType) return false
  const normalized = String(taskType).trim()
  return normalized === 'RETOUCH_TASK' || normalized === 'retouch_task'
}

/** 状态 -> 展示标签 */
export function getTaskFilingStatusLabel(
  status: string | undefined | null,
  taskType?: string | null,
): string {
  if (isRetouchTaskType(taskType)) return '无需同步'
  const normalized = String(status ?? '').trim().toLowerCase()
  if (!normalized) return '--'
  const map: Record<string, string> = {
    filed: '已同步',
    pending_filing: '待补齐后自动同步',
    pending: '待同步',
    filing: '同步中',
    filing_failed: '同步失败',
    not_filed: '未同步',
    queued: '排队中',
    syncing: '同步中',
    synced: '已同步',
    pending_sync: '待同步',
    cooling_down: '稍后重试',
    failed: '同步失败',
    error: '异常',
  }
  return map[normalized] ?? '状态待确认'
}

/** 状态 -> 色调（用于 badge/tag 样式） */
export function getTaskFilingStatusTone(
  status: string | undefined | null,
  taskType?: string | null,
): FilingStatusTone {
  if (isRetouchTaskType(taskType)) return 'neutral'
  const normalized = String(status ?? '').trim().toLowerCase()
  if (!normalized) return 'neutral'
  const map: Record<string, FilingStatusTone> = {
    filed: 'success',
    pending_filing: 'warning',
    pending: 'warning',
    filing: 'info',
    filing_failed: 'error',
    not_filed: 'neutral',
    queued: 'info',
    syncing: 'info',
    synced: 'success',
    pending_sync: 'warning',
    cooling_down: 'warning',
    failed: 'error',
    error: 'error',
  }
  return map[normalized] ?? 'neutral'
}

/** 状态 -> 辅助说明（可选，用于 tooltip 等） */
export function getTaskFilingStatusDescription(
  status: string | undefined | null,
  taskType?: string | null,
  opts?: { missingFieldsSummary?: string; errorMessage?: string },
): string {
  if (isRetouchTaskType(taskType)) return '该任务类型不需要 ERP 同步'
  const normalized = String(status ?? '').trim().toLowerCase()
  if (!normalized) return ''
  if ((normalized === 'filing_failed' || normalized === 'failed' || normalized === 'error') && opts?.errorMessage) {
    return opts.errorMessage
  }
  if ((normalized === 'pending_filing' || normalized === 'pending') && opts?.missingFieldsSummary) {
    return opts.missingFieldsSummary
  }
  return getTaskFilingStatusLabel(status, taskType)
}
