import type {
  Task,
  LegacyTaskStatus,
  TaskType,
  TaskBusinessType,
  MainTaskStatus,
  ProductSource,
  DesignSubStatus,
  AuditSubStatus,
  WarehouseSubStatus,
  PurchaseSubStatus,
  CloseStatus,
} from '../types/task'
import { DesignSubStatusEnum, WarehouseSubStatusEnum, AuditSubStatusEnum, PurchaseSubStatusEnum } from '../enums/task-status'
import { checkTaskCompletion } from '../task-completion'

/**
 * 业务分型：优先以 task_type（后端权威）为准，再用 product_source 兜底。
 * 避免接口未带 product_source（默认 existing）时把 new_product_development 误标成原品开发。
 */
export function mapToTaskBusinessType(taskType: TaskType, productSource: ProductSource): TaskBusinessType {
  if (taskType === 'PURCHASE_TASK') return 'PURCHASE_TASK'
  if (taskType === 'RETOUCH_TASK') return 'RETOUCH_TASK'
  if (taskType === 'NEW_PRODUCT_DEV') return 'NEW_PRODUCT_DEV'
  if (taskType === 'ORIGINAL_PRODUCT_DEV') return 'ORIGINAL_PRODUCT_DEV'
  if (productSource === 'new') return 'NEW_PRODUCT_DEV'
  if (productSource === 'existing') return 'ORIGINAL_PRODUCT_DEV'
  return 'ORIGINAL_PRODUCT_DEV'
}

// LegacyTaskStatus → MainTaskStatus（SCREAMING_SNAKE_CASE，与子状态对齐）
export function mapLegacyStatusToMain(status: LegacyTaskStatus): MainTaskStatus {
  switch (status) {
    case 'Draft':
      return 'DRAFT'
    case 'PendingAssign':
    case 'InProgress':
      return 'INFO_PENDING'
    case 'PendingAuditA':
    case 'RejectedByAuditA':
    case 'PendingAuditB':
    case 'RejectedByAuditB':
    case 'PendingOutsource':
    case 'Outsourcing':
    case 'PendingOutsourceReview':
      return 'INFO_PENDING'
    case 'PendingCustomizationReview':
    case 'PendingCustomizationProduction':
    case 'PendingEffectReview':
    case 'PendingEffectRevision':
    case 'PendingProductionTransfer':
    case 'PendingWarehouseQC':
    case 'RejectedByWarehouse':
    case 'PendingWarehouseReceive':
      return 'WAREHOUSE_PENDING'
    case 'PendingClose':
      return 'READY_TO_CLOSE'
    case 'Completed':
      return 'READY_TO_CLOSE'
    case 'Archived':
      return 'CLOSED'
    case 'Cancelled':
    case 'Blocked':
      return 'BLOCKED'
    default:
      return 'INFO_PENDING'
  }
}

/**
 * 后端读模型 workflow.main_status（小写枚举）→ 前端 MainTaskStatus。
 * 无法识别时返回 undefined，回退到 mapLegacyStatusToMain。
 */
export function mainStatusFromWorkflowMainStatus(w: string | undefined): MainTaskStatus | undefined {
  if (!w || typeof w !== 'string') return undefined
  const x = w.trim().toLowerCase().replace(/-/g, '_')
  if (x === 'closed') return 'CLOSED'
  if (x === 'pending_close' || x === 'ready_to_close') return 'READY_TO_CLOSE'
  if (x === 'warehouse_processing') return 'WAREHOUSE_PROCESSING'
  if (x === 'warehouse_pending') return 'WAREHOUSE_PENDING'
  return undefined
}

// 根据任务分型与当前状态判断仓库节点是否必须有资产版本
export function isWarehouseImageRequired(task: Task): boolean {
  if (task.businessType === 'PURCHASE_TASK') return false
  return true
}

// 任务是否可以进入完成态（不直接修改状态，仅做规则判断）
export function canCompleteTask(task: Task): boolean {
  return checkTaskCompletion(task).canComplete
}

function defaultDesignSubStatus(task: Task): DesignSubStatus {
  if (task.businessType === 'PURCHASE_TASK') return DesignSubStatusEnum.NOT_REQUIRED
  switch (task.status) {
    case 'Draft':
    case 'PendingAssign':
      return DesignSubStatusEnum.PENDING_ASSIGN
    case 'InProgress':
      return DesignSubStatusEnum.IN_PROGRESS
    case 'PendingAuditA':
    case 'PendingAuditB':
      return DesignSubStatusEnum.PENDING_AUDIT
    case 'RejectedByAuditA':
    case 'RejectedByAuditB':
      return DesignSubStatusEnum.REJECTED
    case 'PendingOutsource':
    case 'Outsourcing':
    case 'PendingOutsourceReview':
      return DesignSubStatusEnum.APPROVED
    case 'PendingCustomizationReview':
    case 'PendingCustomizationProduction':
    case 'PendingEffectReview':
    case 'PendingEffectRevision':
    case 'PendingProductionTransfer':
    case 'PendingWarehouseQC':
    case 'RejectedByWarehouse':
    case 'PendingWarehouseReceive':
      return DesignSubStatusEnum.APPROVED
    case 'PendingClose':
      return DesignSubStatusEnum.APPROVED
    case 'Completed':
    case 'Archived':
      return DesignSubStatusEnum.FINALIZED
    default:
      return DesignSubStatusEnum.PENDING_ASSIGN
  }
}

function defaultAuditSubStatus(task: Task): AuditSubStatus {
  if (task.businessType === 'PURCHASE_TASK') return AuditSubStatusEnum.NOT_REQUIRED
  switch (task.status) {
    case 'PendingAuditA':
    case 'PendingAuditB':
      return AuditSubStatusEnum.IN_PROGRESS
    case 'RejectedByAuditA':
    case 'RejectedByAuditB':
      return AuditSubStatusEnum.REJECTED
    case 'PendingOutsource':
    case 'Outsourcing':
    case 'PendingOutsourceReview':
      return AuditSubStatusEnum.TRANSFERRED
    case 'PendingCustomizationReview':
    case 'PendingCustomizationProduction':
    case 'PendingEffectReview':
    case 'PendingEffectRevision':
    case 'PendingProductionTransfer':
    case 'PendingWarehouseQC':
    case 'RejectedByWarehouse':
    case 'PendingWarehouseReceive':
    case 'PendingClose':
    case 'Completed':
    case 'Archived':
      return AuditSubStatusEnum.PASSED
    default:
      return AuditSubStatusEnum.NOT_REQUIRED
  }
}

function defaultWarehouseSubStatus(task: Task): WarehouseSubStatus {
  switch (task.warehouseReceiveStatus) {
    case 'received':
      return WarehouseSubStatusEnum.RECEIVED
    case 'returned':
      return WarehouseSubStatusEnum.RETURNED
    case 'archived':
      return WarehouseSubStatusEnum.DONE
    case 'pending':
      return WarehouseSubStatusEnum.PENDING_RECEIVE
    default:
      break
  }
  // 列表项常不带 workflow / warehouse_receive_status，仅依赖扁平 task_status
  if (task.status === 'PendingClose') {
    return WarehouseSubStatusEnum.DONE
  }
  if (task.status === 'PendingProductionTransfer' || task.status === 'PendingWarehouseQC') {
    return WarehouseSubStatusEnum.RECEIVED
  }
  if (task.status === 'RejectedByWarehouse') {
    return WarehouseSubStatusEnum.RETURNED
  }
  if (task.status === 'PendingWarehouseReceive') {
    return WarehouseSubStatusEnum.PENDING_RECEIVE
  }
  return WarehouseSubStatusEnum.NOT_REQUIRED
}

function defaultPurchaseSubStatus(task: Task): PurchaseSubStatus {
  if (task.businessType !== 'PURCHASE_TASK') return PurchaseSubStatusEnum.NOT_REQUIRED
  // 仓库已接收 → 采购流程完成
  if (task.warehouseReceiveStatus === 'received' || task.warehouseReceiveStatus === 'archived') {
    return PurchaseSubStatusEnum.INBOUND_DONE
  }
  // 已有采购信息：根据采购状态字段派生
  if (task.purchaseInfo?.status === 'Purchased') return PurchaseSubStatusEnum.PURCHASED
  if (task.purchaseInfo?.status === 'Purchasing') return PurchaseSubStatusEnum.IN_PROGRESS
  return PurchaseSubStatusEnum.PENDING
}

function deriveCloseStatus(task: Task): CloseStatus {
  if (task.status === 'Archived') return 'CLOSED'
  const result = checkTaskCompletion(task)
  if (result.canComplete) return 'READY'
  return 'NOT_READY'
}

/**
 * 读模型上顶层 task_status 未同步时：Legacy status 仍为 PendingWarehouseReceive，
 * 但 workflow/仓库子状态已进入接收后环节。仅在此明确定义的组合下修正 mainStatus（展示用），不修改 partial.status。
 */
function reconcileMainStatusIfWarehouseAheadOfLegacyStatus(
  partial: Task,
  warehouseSubStatus: WarehouseSubStatus,
  mappedMain: MainTaskStatus,
): MainTaskStatus {
  const warehousePhaseStatuses: ReadonlySet<string> = new Set([
    'PendingProductionTransfer',
    'PendingWarehouseQC',
    'RejectedByWarehouse',
    'PendingWarehouseReceive',
  ])
  if (!warehousePhaseStatuses.has(partial.status)) return mappedMain
  switch (warehouseSubStatus) {
    case 'RECEIVED':
    case 'PACKING':
      return 'WAREHOUSE_PROCESSING'
    case 'DONE':
      return 'READY_TO_CLOSE'
    default:
      return mappedMain
  }
}

// 统一在前端构造 Task 时补齐业务分型与主/子状态等领域字段
export function enrichTaskDomainFields(partial: Task): Task {
  const businessType =
    partial.businessType ?? mapToTaskBusinessType(partial.taskType, partial.productSource)

  const withBusiness: Task = { ...partial, businessType }

  // mainStatus 始终从 status 重新派生，不保留 spread 带入的旧值。
  // 原因：mainStatus 是 LegacyTaskStatus 的直接映射，不存在独立设置的场景；
  // 若保留旧值，每次 updateTask 只改 status 时 mainStatus 会卡住。
  const designSubStatus: DesignSubStatus =
    partial.designSubStatus ?? defaultDesignSubStatus(withBusiness)
  // 优先采用 GET 列表/详情归一化得到的 warehouseSubStatus（workflow.sub_status.warehouse 等）；
  // 无则再按 warehouseReceiveStatus + Legacy task_status 派生，避免把读模型上的待接收覆盖成 NOT_REQUIRED。
  const warehouseSubStatus: WarehouseSubStatus =
    partial.warehouseSubStatus ?? defaultWarehouseSubStatus(withBusiness)

  let mainStatus: MainTaskStatus = mapLegacyStatusToMain(partial.status)
  mainStatus = reconcileMainStatusIfWarehouseAheadOfLegacyStatus(
    partial,
    warehouseSubStatus,
    mainStatus,
  )
  const fromWorkflowMain = mainStatusFromWorkflowMainStatus(partial.workflowMainStatus)
  if (fromWorkflowMain) {
    mainStatus = fromWorkflowMain
  } else if (partial.status === 'Completed') {
    // 扁平 Completed 不再映射为 READY_TO_CLOSE（待结单），与后端「已完成/可关单」语义一致
    mainStatus = 'CLOSED'
  }
  const auditSubStatus: AuditSubStatus =
    partial.auditSubStatus ?? defaultAuditSubStatus(withBusiness)
  const purchaseSubStatus: PurchaseSubStatus =
    partial.purchaseSubStatus ?? defaultPurchaseSubStatus(withBusiness)

  const enriched: Task = {
    ...withBusiness,
    mainStatus,
    designSubStatus,
    auditSubStatus,
    warehouseSubStatus,
    purchaseSubStatus,
    requiresAssetVersions:
      partial.requiresAssetVersions ?? isWarehouseImageRequired(withBusiness),
  }

  // closeStatus 最后派生（依赖已完整的 enriched task）
  enriched.closeStatus = partial.closeStatus ?? deriveCloseStatus(enriched)

  return enriched
}

// 向后兼容：保留旧函数名供消费方平滑切换
/** @deprecated 使用 mapLegacyStatusToMain */
export function mapStatusToMainAndSub(status: LegacyTaskStatus): {
  mainStatus: MainTaskStatus
  subStatus?: undefined
} {
  return { mainStatus: mapLegacyStatusToMain(status) }
}
