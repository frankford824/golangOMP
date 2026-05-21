import type { Task } from './types/task'
import { isCustomizationTask } from './task-actions'
import { getMainTaskStatusLabel } from './enums/task-status'

/** 任务中心卡片：定制 lane 下扁平 task_status 的专用展示文案（覆盖错误的 mainStatus 映射）。 */
const CUSTOMIZATION_CARD_STATUS_LABELS: Readonly<
  Partial<Record<Task['status'], string>>
> = {
  PendingCustomizationProduction: '待美工处理',
  PendingCustomizationReview: '待定制审核',
  PendingWarehouseReceive: '待仓库接收',
}

/**
 * 定制任务在任务中心卡片上的状态文案覆盖。
 * 非定制任务或无需覆盖的状态返回 null，由调用方继续走 mainStatus / TaskStatusTag。
 */
export function getTaskCenterCardStatusLabel(task: Task): string | null {
  if (!isCustomizationTask(task)) return null
  return CUSTOMIZATION_CARD_STATUS_LABELS[task.status] ?? null
}

/** 任务中心卡片最终展示文案（测试与单点调用）。 */
export function getTaskCenterCardStatusDisplayLabel(task: Task): string {
  const override = getTaskCenterCardStatusLabel(task)
  if (override) return override
  if (task.mainStatus) return getMainTaskStatusLabel(task.mainStatus)
  return String(task.status ?? '')
}
