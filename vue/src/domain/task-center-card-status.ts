import type { Task } from './types/task'
import { getMainTaskStatusLabel, getTaskStatusLabel } from './enums/task-status'

/** 任务中心只展示 v8 活动主状态，不根据业务线派生并行节点。 */
export function getTaskCenterCardStatusLabel(_task: Task): string | null {
  return null
}

export function getCustomizationDetailStatusLabel(_task: Task): string | null {
  return null
}

export function getTaskCenterCardStatusDisplayLabel(task: Task): string {
  if (task.mainStatus) return getMainTaskStatusLabel(task.mainStatus)
  return getTaskStatusLabel(task.status)
}
