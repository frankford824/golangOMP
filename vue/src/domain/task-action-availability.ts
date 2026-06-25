import type { Task } from './types/task'
import {
  canAssignCustomizationArtOperator,
  taskHasAssignee,
  isInDesignerReassignmentPhase,
  isInCustomizationArtReassignmentPhase,
  isRetouchTask,
} from './task-actions'

const WAREHOUSE_RECEIVE_BLOCKED_STATUSES = new Set([
  'PendingProductionTransfer',
  'PendingClose',
  'Completed',
  'Archived',
])

/**
 * 仓库接收/退回类按钮负向门禁：已接收、已完成或任务已离开待仓库接收阶段时隐藏。
 */
export function shouldHideWarehouseReceiveActions(task: Task): boolean {
  if (WAREHOUSE_RECEIVE_BLOCKED_STATUSES.has(task.status)) {
    return true
  }
  const sub = task.warehouseSubStatus
  if (sub === 'RECEIVED' || sub === 'DONE' || sub === 'PACKING') {
    return true
  }
  const recv = task.warehouseReceiveStatus
  if (recv === 'received' || recv === 'archived') {
    return true
  }
  const wfMain = String(task.workflowMainStatus ?? '')
    .trim()
    .toLowerCase()
  if (wfMain === 'pending_close' || wfMain === 'closed') {
    return true
  }
  return false
}

/** 仓库完成（推进 PendingClose）按钮负向门禁：仅 PendingProductionTransfer 且未完成仓库流时展示。 */
export function shouldHideWarehouseCompleteAction(task: Task): boolean {
  if (task.status === 'PendingClose' || task.status === 'Completed' || task.status === 'Archived') {
    return true
  }
  if (task.warehouseSubStatus === 'DONE') {
    return true
  }
  const wfMain = String(task.workflowMainStatus ?? '')
    .trim()
    .toLowerCase()
  if (wfMain === 'pending_close' || wfMain === 'closed') {
    return true
  }
  return false
}

/**
 * 前端最小状态显隐（体验优化），不替代后端权限与组织 scope 判定。
 */
export function getTaskActionAvailability(task: Task) {
  const status = task.status
  const isPendingAssign = status === 'PendingAssign'
  const notRetouch = !isRetouchTask(task)
  const isPendingAuditA = notRetouch && status === 'PendingAuditA'
  const isPendingAuditB = notRetouch && status === 'PendingAuditB'
  const isPendingWarehouseReceive = status === 'PendingWarehouseReceive'
  const isPendingProductionTransfer = status === 'PendingProductionTransfer'
  const isPendingClose = status === 'PendingClose'

  const canShowAssignCustomizationArt = canAssignCustomizationArtOperator(task)

  return {
    canShowAssign:
      (isPendingAssign && !taskHasAssignee(task)) || canShowAssignCustomizationArt,
    canShowReassign:
      isInDesignerReassignmentPhase(task) || isInCustomizationArtReassignmentPhase(task),
    canShowAuditA: isPendingAuditA,
    canShowAuditB: isPendingAuditB,
    canShowAuditActions: isPendingAuditA || isPendingAuditB,
    canShowWarehouseReceiveActions:
      isPendingWarehouseReceive && !shouldHideWarehouseReceiveActions(task),
    canShowWarehouseActions:
      isPendingWarehouseReceive && !shouldHideWarehouseReceiveActions(task),
    /** 仓库完成：已接收后需要仓库显式推进到 PendingClose（对应 POST /warehouse/complete） */
    canShowWarehouseComplete:
      isPendingProductionTransfer && !shouldHideWarehouseCompleteAction(task),
    canShowClose: isPendingClose,
  }
}

