/**
 * v1.21：线上仅接受 low | normal | high | critical。
 * 读模型兼容历史 urgent/medium；写接口前必须归一到四态。
 */
export type TaskPriorityApi = 'low' | 'normal' | 'high' | 'critical'

export function normalizePriorityFromApi(raw: string | null | undefined): TaskPriorityApi {
  const s = String(raw ?? 'normal').trim().toLowerCase()
  if (s === 'medium') return 'normal'
  if (s === 'urgent') return 'critical'
  if (s === 'low' || s === 'normal' || s === 'high' || s === 'critical') return s
  return 'normal'
}

/** 提交 POST/PATCH 任务前对 priority 字段归一 */
export function normalizePriorityForApi(raw: string | null | undefined): TaskPriorityApi {
  return normalizePriorityFromApi(raw)
}
