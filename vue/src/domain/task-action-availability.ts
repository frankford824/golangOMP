import type { Task } from './types/task'
import {
  canAssignCustomizationArtOperator,
  taskHasAssignee,
  isInDesignerReassignmentPhase,
  isInCustomizationArtReassignmentPhase,
  isRetouchTask,
} from './task-actions'

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
    canShowWarehouseReceiveActions: isPendingWarehouseReceive,
    canShowWarehouseActions: isPendingWarehouseReceive,
    /** 仓库完成：已接收后需要仓库显式推进到 PendingClose（对应 POST /warehouse/complete） */
    canShowWarehouseComplete: isPendingProductionTransfer,
    canShowClose: isPendingClose,
  }
}

