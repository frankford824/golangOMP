import { PermissionEnum, type PermissionEnumValue, type PermissionUser } from '@/types'
import type { Task } from '@/domain/types/task'

function isReferenceSupplementPhase(task: Task): boolean {
  if (task.status !== 'InProgress') return false
  return task.designSubStatus === 'IN_PROGRESS' || task.designSubStatus === 'REJECTED'
}

function isTaskCreatorOrRequester(task: Task, user: PermissionUser): boolean {
  const uid = String(user.id ?? '').trim()
  if (!uid) return false
  const creatorId = String(task.creatorId ?? '').trim()
  const requesterId = String(task.requesterId ?? '').trim()
  return uid === creatorId || uid === requesterId
}

/**
 * 任务详情页补传参考图（运营创建者路径）门禁：
 * - 已指派且设计仍在进行中（未提交待审）；
 * - 任务创建人/发起人且具备显式 task.create 能力。
 */
export function canUserSupplementReferenceOnTaskDetail(
  task: Task,
  user: PermissionUser | null,
  ctx: {
    hasPermission: (p: PermissionEnumValue) => boolean
    hasAnyRole: (codes: readonly string[]) => boolean
  },
): boolean {
  if (!user) return false
  if (!isReferenceSupplementPhase(task)) return false
  return isTaskCreatorOrRequester(task, user) && ctx.hasPermission(PermissionEnum.TASK_CREATE)
}
