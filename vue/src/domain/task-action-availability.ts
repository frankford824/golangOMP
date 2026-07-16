import type { Task } from './types/task'

const hasAction = (task: Task, action: string): boolean =>
  (task.allowedActions ?? []).includes(action)

/**
 * v8 动作显隐只消费后端 `allowed_actions`，不再由角色、组织名称或状态推断。
 * `workflow_contract_version=2` 下空数组表示后端明确禁止全部动作。
 */
export function getTaskActionAvailability(task: Task) {
  const canApprove = hasAction(task, 'task.audit.approve')
  const canReturnToDesign = hasAction(task, 'task.audit.return_to_design')

  return {
    canShowAssign: hasAction(task, 'task.assign'),
    canShowReassign: hasAction(task, 'task.reassign'),
    canShowAuditActions: canApprove || canReturnToDesign,
    canApprove,
    canReturnToDesign,
    canReopen: hasAction(task, 'task.reopen'),
  }
}
