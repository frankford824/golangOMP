import type { Task } from './types/task'
import { isCustomizationTask } from './task-actions'

export type TaskCenterClaimActorGate = {
  canActAsCustomizationClaimActor: boolean
  canClaimFromDesignerPool: boolean
  activeTabIsPool: boolean
}

/** 任务尚无设计/美工负责人（接单前）。 */
export function taskHasNoClaimHandler(task: Task): boolean {
  const designerId = task.designerId ?? task.assigneeId
  const handlerId = task.currentHandlerId
  return (
    (designerId == null || String(designerId).trim() === '') &&
    (handlerId == null || String(handlerId).trim() === '')
  )
}

/** 当前用户是否可执行定制 lane 的 module claim（CustomizationOperator / Admin / SuperAdmin）。 */
export function userCanActAsCustomizationClaimActor(
  hasAnyRole: (roles: readonly string[]) => boolean,
  isCustomizationOperator: boolean,
): boolean {
  if (
    hasAnyRole([
      'Admin',
      'SuperAdmin',
      'admin',
      'super_admin',
      'CustomizationOperator',
      'customization_operator',
    ])
  ) {
    return true
  }
  if (isCustomizationOperator) return true
  return false
}

/** 纯 Designer（无定制美工/管理角色）不可领定制任务。 */
export function userIsPureDesignerForCustomizationClaim(
  hasAnyRole: (roles: readonly string[]) => boolean,
): boolean {
  const isDesigner = hasAnyRole(['Designer', 'designer'])
  if (!isDesigner) return false
  return !userCanActAsCustomizationClaimActor(hasAnyRole, false)
}

/**
 * 定制任务「美工接单」：PendingCustomizationProduction + 未分配 + 有权限。
 * 不限 pool Tab（全部/定制筛选下可见）。
 */
export function customizationClaimActorGate(
  hasAnyRole: (roles: readonly string[]) => boolean,
  isCustomizationOperator: boolean,
): Pick<TaskCenterClaimActorGate, 'canActAsCustomizationClaimActor'> {
  if (userIsPureDesignerForCustomizationClaim(hasAnyRole)) {
    return { canActAsCustomizationClaimActor: false }
  }
  return {
    canActAsCustomizationClaimActor: userCanActAsCustomizationClaimActor(
      hasAnyRole,
      isCustomizationOperator,
    ),
  }
}

export function canClaimCustomizationTask(
  task: Task,
  gate: Pick<TaskCenterClaimActorGate, 'canActAsCustomizationClaimActor'>,
): boolean {
  if (!gate.canActAsCustomizationClaimActor) return false
  if (!isCustomizationTask(task)) return false
  if (task.status !== 'PendingCustomizationProduction') return false
  return taskHasNoClaimHandler(task)
}

/** 任务详情定制美工区「美工接单」（与任务中心卡片同规则，不要求 pool Tab）。 */
export function canClaimCustomizationOnTaskDetail(
  task: Task,
  hasAnyRole: (roles: readonly string[]) => boolean,
  isCustomizationOperator: boolean,
): boolean {
  return canClaimCustomizationTask(
    task,
    customizationClaimActorGate(hasAnyRole, isCustomizationOperator),
  )
}

/** 常规任务池内「接单」：仅 pool Tab + PendingAssign + 设计池权限。 */
export function canClaimRegularDesignTask(
  task: Task,
  gate: Pick<TaskCenterClaimActorGate, 'activeTabIsPool' | 'canClaimFromDesignerPool'>,
): boolean {
  if (!gate.activeTabIsPool) return false
  if (!gate.canClaimFromDesignerPool) return false
  const status = String(task.status ?? '').toLowerCase()
  const isPendingAssignStatus =
    status === 'pendingassign' ||
    status === 'pending_assign' ||
    status === 'pendingclaim' ||
    status === 'pending_claim'
  return isPendingAssignStatus && taskHasNoClaimHandler(task)
}

export function canClaimTaskFromCenter(
  task: Task,
  gate: TaskCenterClaimActorGate,
): boolean {
  if (canClaimCustomizationTask(task, gate)) return true
  return canClaimRegularDesignTask(task, gate)
}

export function isCustomizationModuleClaimTask(task: Task): boolean {
  return isCustomizationTask(task) && task.status === 'PendingCustomizationProduction'
}

export function taskCenterClaimButtonLabel(task: Task, claiming: boolean): string {
  if (isCustomizationModuleClaimTask(task)) {
    return claiming ? '接单中...' : '美工接单'
  }
  return claiming ? '接单中...' : '接单'
}
