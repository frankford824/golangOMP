import type { Task } from './types/task'
import { TaskTypeEnum, normalizeTaskType } from './enums/task-type'
import { WarehouseSubStatusEnum, DesignSubStatusEnum } from './enums/task-status'

export interface TaskCompletionResult {
  canComplete: boolean
  reasons: string[]
}

function hasFinalAsset(task: Task): boolean {
  return task.assetVersions?.some((v) => v.type === 'final') ?? false
}

function isWarehouseReceived(task: Task): boolean {
  // 优先使用新子状态字段
  if (task.warehouseSubStatus) {
    return (
      task.warehouseSubStatus === WarehouseSubStatusEnum.RECEIVED ||
      task.warehouseSubStatus === WarehouseSubStatusEnum.DONE
    )
  }
  // 兼容旧字段
  return task.warehouseReceiveStatus === 'received' || task.warehouseReceiveStatus === 'archived'
}

function isWarehouseReturned(task: Task): boolean {
  if (task.warehouseSubStatus) {
    return task.warehouseSubStatus === WarehouseSubStatusEnum.RETURNED
  }
  return task.warehouseReceiveStatus === 'returned'
}

function needsDesign(task: Task): boolean {
  return task.businessType !== 'PURCHASE_TASK'
}

function isDesignFinalized(task: Task): boolean {
  if (!needsDesign(task)) return true
  // 优先使用新子状态字段
  if (task.designSubStatus) {
    return (
      task.designSubStatus === DesignSubStatusEnum.FINALIZED ||
      task.designSubStatus === DesignSubStatusEnum.APPROVED ||
      // 原品开发绑定已有商品等场景：后端显式标 not_required，等价于无需设计；不应阻挡结单
      task.designSubStatus === DesignSubStatusEnum.NOT_REQUIRED
    )
  }
  // 兼容旧字段：有终稿资产即视为设计完成
  return hasFinalAsset(task)
}

export function checkTaskCompletion(task: Task): TaskCompletionResult {
  const reasons: string[] = []
  const normalizedType = normalizeTaskType(task.taskType)
  const isPurchase = normalizedType === TaskTypeEnum.PURCHASE_TASK

  // 基础必备信息
  if (!task.taskNo) reasons.push('缺少任务号')
  if (!task.sku) reasons.push('缺少 SKU，任务不得结单')

  // 设计类任务：设计必须完成
  if (!isPurchase && !isDesignFinalized(task)) {
    reasons.push('设计尚未终稿，不能结单')
  }

  // 仓库流转约束：必须已接收且未退回
  if (!isWarehouseReceived(task)) reasons.push('仓库尚未接收，不能结单')
  if (isWarehouseReturned(task)) reasons.push('仓库已退回，需处理退回原因后才能结单')

  // 成本价仅对采购任务强制
  if (isPurchase && !task.costPrice) reasons.push('成本价未维护，不能结单')

  const canComplete = reasons.length === 0
  return { canComplete, reasons }
}
