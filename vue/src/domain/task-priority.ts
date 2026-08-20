/** 新任务只接受普通、加急和出单画图三档。旧值仅用于读取迁移前缓存。 */
export type TaskPriorityApi = 'normal' | 'high' | 'drawing'

export const TASK_PRIORITY_OPTIONS: ReadonlyArray<{ label: string; value: TaskPriorityApi }> = [
  { label: '普通', value: 'normal' },
  { label: '加急', value: 'high' },
  { label: '出单画图', value: 'drawing' },
]

export function normalizePriorityFromApi(raw: string | null | undefined): TaskPriorityApi {
  const s = String(raw ?? 'normal').trim().toLowerCase()
  if (s === 'low' || s === 'medium') return 'normal'
  if (s === 'critical' || s === 'urgent') return 'high'
  if (s === 'normal' || s === 'high' || s === 'drawing') return s
  return 'normal'
}

/** 提交 POST/PATCH 任务前对 priority 字段归一 */
export function normalizePriorityForApi(raw: string | null | undefined): TaskPriorityApi {
  return normalizePriorityFromApi(raw)
}
