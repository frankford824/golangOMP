import type { Task } from './types/task'

/** 任务尚无设计处理人。 */
export function taskHasNoClaimHandler(task: Task): boolean {
  return !String(task.designerId ?? task.assigneeId ?? task.currentHandlerId ?? '').trim()
}

/**
 * v8 接单入口服从后端动作合同：任务允许指派，并且当前账号具备设计提交能力。
 * 仅有管理侧 task.assign 的运营账号可以指派他人，但不能把它解释成自接单。
 */
export function canClaimTaskFromCenter(task: Task, canSubmitDesign: boolean): boolean {
  return canSubmitDesign
    && taskHasNoClaimHandler(task)
    && (task.allowedActions ?? []).includes('task.assign')
}

export function taskCenterClaimButtonLabel(_task: Task, claiming: boolean): string {
  return claiming ? '接单中...' : '接单'
}
