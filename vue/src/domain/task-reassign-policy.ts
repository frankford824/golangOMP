import type { Task } from './types/task'
import type { PermissionUser } from '@/types'
import { PermissionEnum, type PermissionEnumValue } from '@/types'

type DesignerDispatchPolicyCtx = {
  hasPermission: (p: PermissionEnumValue | PermissionEnumValue[]) => boolean
  hasAction?: (key: string) => boolean
  isGroupLeader: boolean
}

function isRequesterOrInitiator(task: Task, user: PermissionUser): boolean {
  const uid = String(user.id ?? '').trim()
  if (uid === '') return false
  const isRequester = uid === String(task.requesterId ?? '').trim()
  const creatorRaw = task.creatorId != null ? String(task.creatorId).trim() : ''
  const isInitiator = creatorRaw !== '' && uid === creatorRaw
  return isRequester || isInitiator
}

function canScheduleWithinTeam(
  task: Task,
  user: PermissionUser,
  ctx: DesignerDispatchPolicyCtx,
  teamActionKeys: readonly string[],
): boolean {
  if (!ctx.isGroupLeader) return false
  if (!teamActionKeys.some((key) => (ctx.hasAction ? ctx.hasAction(key) : false))) {
    // 历史兜底：组长若仍沿用 design.work 语义，也允许在同组内调度。
    if (!ctx.hasPermission(PermissionEnum.DESIGN_WORK)) return false
  }
  const taskTeam = String(task.ownerOrgTeam ?? '').trim()
  const userTeam = String(user.groupId ?? '').trim()
  return taskTeam !== '' && userTeam !== '' && taskTeam === userTeam
}

/**
 * 谁可以执行「首次指派设计师」。
 *
 * 最小门禁：
 * - `task.assign` 全局动作（HRAdmin / SuperAdmin 等顶层管理路径）
 * - 请求人 / 发起人：`task.create` 且当前用户为 `requester_id` 或 `creator_id`
 * - 合法 scope 内组长：`task.assign.team`（历史兜底：`design.work` + 同组）
 * - 部门范围指派：`task.assign.department`
 */
export function canUserScheduleDesignerAssignment(
  task: Task,
  user: PermissionUser,
  ctx: DesignerDispatchPolicyCtx,
): boolean {
  const act = (key: string): boolean => (ctx.hasAction ? ctx.hasAction(key) : false)

  if (act('task.assign')) return true
  if (act('task.assign.department')) return true
  if (canScheduleWithinTeam(task, user, ctx, ['task.assign.team'])) return true
  if (ctx.hasPermission(PermissionEnum.TASK_CREATE) && isRequesterOrInitiator(task, user)) {
    return true
  }
  return false
}

/**
 * 谁可以执行「设计资源再调度 / 重新指派设计师」。
 *
 * 与「列表可见」解耦：能进任务池不代表可改派。v1.8 后端已下发显式 action
 * （`task.reassign.department`），此处统一以 action key 门禁，避免角色名兜底。
 *
 * 最小门禁：
 * - `task.reassign` 全局动作（HRAdmin / SuperAdmin 等顶层管理路径）
 * - 请求人 / 发起人：`task.create` 且当前用户为 `requester_id` 或 `creator_id`
 * - 合法 scope 内组长：`task.reassign.team` 或（历史兜底）`design.work` + 同组
 * - 部门范围改派：`task.reassign.department`（v1.8 新增，DepartmentAdmin）
 */
export function canUserScheduleDesignerReassignment(
  task: Task,
  user: PermissionUser,
  ctx: DesignerDispatchPolicyCtx,
): boolean {
  const act = (key: string): boolean => (ctx.hasAction ? ctx.hasAction(key) : false)

  // v1.8 Round I：完全走 action key；SuperAdmin / HRAdmin 在后端 frontend_access 中
  // 被显式授予 `task.reassign`，前端无需再做角色名兜底。
  if (act('task.reassign')) return true
  if (act('task.reassign.department')) return true
  if (canScheduleWithinTeam(task, user, ctx, ['task.reassign.team'])) return true
  if (ctx.hasPermission(PermissionEnum.TASK_CREATE) && isRequesterOrInitiator(task, user)) {
    return true
  }
  return false
}
