import type { Task } from './types/task'

/** 任务尚无设计处理人。 */
export function taskHasNoClaimHandler(task: Task): boolean {
  return !String(task.designerId ?? task.assigneeId ?? task.currentHandlerId ?? '').trim()
}

/**
 * v8 接单入口服从后端动作合同的 task.claim。
 * 此前用 task.assign 当开关，把「能指派别人」误当成「能自己接单」：
 * 纯设计师没有 task.assign 所以看不到按钮，兼任设计角色的运营反而看得到。
 */
export function canClaimTaskFromCenter(task: Task, canSubmitDesign: boolean): boolean {
  return canSubmitDesign
    && taskHasNoClaimHandler(task)
    && (task.allowedActions ?? []).includes('task.claim')
}

export function taskCenterClaimButtonLabel(_task: Task, claiming: boolean): string {
  return claiming ? '接单中...' : '接单'
}
