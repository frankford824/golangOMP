import type { Task } from './types/task'
import { isCustomizationTask, taskHasNoDesignHandler } from './task-actions'
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

/**
 * 定制任务详情页（顶栏设计模块 pill、设计与资产卡片状态行）专用文案。
 * 不展示 design 子状态的「无需设计」，按扁平 task_status + 是否已指派美工语义覆盖。
 */
export function getCustomizationDetailStatusLabel(task: Task): string | null {
  if (!isCustomizationTask(task)) return null
  if (task.status === 'PendingCustomizationProduction') {
    return taskHasNoDesignHandler(task) ? '待指派美工' : '待美工处理'
  }
  if (task.status === 'PendingCustomizationReview') return '待定制审核'
  return null
}

/** 任务中心卡片最终展示文案（测试与单点调用）。 */
export function getTaskCenterCardStatusDisplayLabel(task: Task): string {
  const override = getTaskCenterCardStatusLabel(task)
  if (override) return override
  if (task.mainStatus) return getMainTaskStatusLabel(task.mainStatus)
  return String(task.status ?? '')
}
