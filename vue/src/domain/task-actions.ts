import type { Task, TaskStatus } from './types/task'

const actions = (task: Task): ReadonlySet<string> => new Set(task.allowedActions ?? [])
const hasAnyAction = (task: Task, values: readonly string[]): boolean => {
  const available = actions(task)
  return values.some((value) => available.has(value))
}

export const DONE_STATUSES: readonly TaskStatus[] = ['Completed', 'Archived', 'Cancelled']
export const TODAY_PENDING_STATUSES: readonly TaskStatus[] = [
  'PendingAssign', 'Assigned', 'InProgress', 'PendingAudit',
]
export const TASK_PRODUCT_INFO_MAINTAINER_ROLES = [] as const

export const isRetouchTask = (task: Task): boolean => task.taskType === 'RETOUCH_TASK'
export const isCustomizationTask = (task: Task): boolean =>
  task.businessLane === 'customization' || task.workflowLane === 'customization'
export const shouldShowDesignerMetaOnTaskCenterCard = (task: Task): boolean =>
  task.taskType !== 'SKU_PLANNING'
export const taskHasNoDesignHandler = (task: Task): boolean =>
  !String(task.designerId ?? task.assigneeId ?? task.currentHandlerId ?? '').trim()
export const taskHasAssignee = (task: Task): boolean => !taskHasNoDesignHandler(task)
export const taskHasRecordedDesignOutput = (task: Task): boolean => Boolean(task.assetVersions?.length)
export const canAssignCustomizationArtOperator = (task: Task): boolean =>
  isCustomizationTask(task) && actions(task).has('task.assign')
export const canAssign = (task: Task): boolean => actions(task).has('task.assign')
export const isInCustomizationArtReassignmentPhase = (task: Task): boolean =>
  isCustomizationTask(task) && actions(task).has('task.reassign')
export const isInDesignerReassignmentPhase = (task: Task): boolean =>
  actions(task).has('task.reassign')
export const canReassignDesigner = (task: Task): boolean => actions(task).has('task.reassign')
export const canSubmitAudit = (task: Task): boolean =>
  hasAnyAction(task, ['task.design.submit', 'task.submit_design'])
export const isLegacyTaskStatusInDesignerEditablePhase = (task: Task): boolean =>
  hasAnyAction(task, ['task.design.submit', 'task.reassign'])
export function canMaintainTaskProductInfoAtAnyStage(
  _hasAnyRole: (roles: readonly string[]) => boolean,
  inTaskScope: boolean,
): boolean {
  return inTaskScope
}
export const canUploadDesignDelivery = (task: Task): boolean => canSubmitAudit(task)
export const canUploadAuditAsset = (task: Task): boolean => actions(task).has('task.audit.approve')
export const canAudit = (task: Task): boolean =>
  hasAnyAction(task, ['task.audit.approve', 'task.audit.return_to_design'])
export const isDoneStatus = (task: Task): boolean => DONE_STATUSES.includes(task.status)
export const isCompletedOrArchived = (task: Task): boolean =>
  task.status === 'Completed' || task.status === 'Archived'
export const isInAuditQueue = (task: Task): boolean => task.status === 'PendingAudit'
export const isInCustomizationFlow = (task: Task): boolean =>
  isCustomizationTask(task) && !isDoneStatus(task)

export type AuditActionKind = 'approve' | 'return_to_design'
export function auditActionForRow(task: Task): AuditActionKind | null {
  if (actions(task).has('task.audit.approve')) return 'approve'
  if (actions(task).has('task.audit.return_to_design')) return 'return_to_design'
  return null
}
