import type { Task } from './types/task'

/** 任务尚无设计处理人。 */
export function taskHasNoClaimHandler(task: Task): boolean {
  return !String(task.designerId ?? task.assigneeId ?? task.currentHandlerId ?? '').trim()
}

/**
 * v8 接单入口只服从后端动作合同。业务线、角色名称与活动 Tab 均不能自行扩权。
 */
export function canClaimTaskFromCenter(task: Task): boolean {
  return taskHasNoClaimHandler(task) && (task.allowedActions ?? []).includes('task.assign')
}

export function taskCenterClaimButtonLabel(_task: Task, claiming: boolean): string {
  return claiming ? '接单中...' : '接单'
}
