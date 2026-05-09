import type { Task } from '@/domain/types/task'

/** v0.9 列表/导出等：空姓名统一占位 */
export function dashDisplay(name: string | null | undefined): string {
  const s = name?.trim()
  return s ? s : '-'
}

/**
 * 设计师展示名：优先 `designer_*`；仅当后端仍只返回兼容字段时回退 `assignee_*`。
 */
export function taskDesignerDisplayName(task: Task): string {
  const n = task.designerName?.trim()
  if (n) return n
  return dashDisplay(task.assigneeName)
}

/** 与 taskDesignerDisplayName 对称的 id（字符串），无则 null */
export function taskDesignerIdString(task: Task): string | null {
  const id = task.designerId ?? task.assigneeId
  if (id == null) return null
  const s = String(id).trim()
  return s === '' ? null : s
}

export function taskCreatorDisplayName(task: Task): string {
  return dashDisplay(task.creatorName)
}

export function taskCurrentHandlerDisplayName(task: Task): string {
  return dashDisplay(task.currentHandlerName)
}

export function taskMatchesDesignerUser(task: Task, userId: string | null | undefined): boolean {
  if (userId == null || String(userId).trim() === '') return false
  const did = taskDesignerIdString(task)
  return did != null && did === String(userId)
}

function normActorIdForCompare(id: string | null | undefined): string | null {
  if (id == null) return null
  const s = String(id).trim()
  if (s === '' || s === 'anonymous') return null
  return s
}

/** 发起人与创建人是否为不同主体；false 时列表/详情可隐藏「发起人」避免与「创建人」重复 */
export function isRequesterDistinctFromCreator(task: Task): boolean {
  const rId = normActorIdForCompare(task.requesterId)
  const cId = normActorIdForCompare(task.creatorId)
  if (rId != null && cId != null) return rId !== cId
  const rn = (task.requesterName ?? '').trim()
  const cn = (task.creatorName ?? '').trim()
  if (rn && cn) return rn !== cn
  return true
}

/** 当前处理人与设计师是否为不同主体；false 时列表/详情可隐藏「当前处理人」避免与「设计师」重复 */
export function isCurrentHandlerDistinctFromDesigner(task: Task): boolean {
  const dId = taskDesignerIdString(task)
  const hId = normActorIdForCompare(task.currentHandlerId)
  if (dId != null && hId != null) return dId !== hId
  const dn = (task.designerName ?? task.assigneeName ?? '').trim()
  const hn = (task.currentHandlerName ?? '').trim()
  if (dn && hn) return dn !== hn
  return true
}
