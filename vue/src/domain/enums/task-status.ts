import type {
  LegacyTaskStatus,
  TaskStatus,
  TaskMainStatus,
  TaskSubStatus,
  MainTaskStatus,
  DesignSubStatus,
  AuditSubStatus,
  WarehouseSubStatus,
  PurchaseSubStatus,
  CloseStatus,
} from '../types/task'

export type {
  LegacyTaskStatus,
  TaskStatus,
  TaskMainStatus,
  TaskSubStatus,
  MainTaskStatus,
  DesignSubStatus,
  AuditSubStatus,
  WarehouseSubStatus,
  PurchaseSubStatus,
  CloseStatus,
}

// ─── 旧扁平状态文案（@deprecated，仅供过渡期使用）────────────────────────────
/** @deprecated 使用各子状态对应的 label 函数 */
export const TASK_STATUS_LABELS: Record<LegacyTaskStatus, string> = {
  Draft: '草稿',
  PendingAssign: '待指派',
  Assigned: '已指派',
  InProgress: '设计中',
  PendingAuditA: '待审核',
  RejectedByAuditA: '审核打回',
  PendingAuditB: '待审核',
  RejectedByAuditB: '审核打回',
  PendingOutsource: '待定制',
  Outsourcing: '定制中',
  PendingOutsourceReview: '定制中',
  PendingCustomizationReview: '待定制审核',
  PendingCustomizationProduction: '待定制生产',
  PendingEffectReview: '待效果审核',
  PendingEffectRevision: '待效果返修',
  PendingProductionTransfer: '待生产流转',
  PendingWarehouseQC: '待仓库质检',
  RejectedByWarehouse: '仓库驳回',
  PendingWarehouseReceive: '待仓库接收',
  PendingClose: '待结单',
  Completed: '已完成',
  Archived: '已归档',
  Blocked: '阻塞',
  Cancelled: '已取消',
}

/** @deprecated 使用 getMainTaskStatusLabel */
export function getTaskStatusLabel(status: LegacyTaskStatus): string {
  return TASK_STATUS_LABELS[status] ?? status
}

// ─── 主状态文案（修复旧版键不匹配 bug，与 MainTaskStatus SCREAMING_SNAKE_CASE 对齐）
export const TASK_MAIN_STATUS_LABELS: Record<MainTaskStatus, string> = {
  DRAFT: '草稿',
  CREATED: '已创建',
  CODE_GENERATED: '已生成编码',
  ERP_REGISTERED: '已入 ERP / 已建档',
  INFO_PENDING: '信息待完善',
  WAREHOUSE_PENDING: '待仓库接收',
  WAREHOUSE_PROCESSING: '仓库处理中',
  READY_TO_CLOSE: '待结单',
  CLOSED: '已结单',
  BLOCKED: '已阻塞',
}

export function getMainTaskStatusLabel(status: MainTaskStatus): string {
  return TASK_MAIN_STATUS_LABELS[status] ?? status
}

/** @deprecated 使用 getMainTaskStatusLabel */
export function getTaskMainStatusLabel(status: TaskMainStatus): string {
  return getMainTaskStatusLabel(status as MainTaskStatus)
}

// ─── 旧子状态文案（@deprecated）──────────────────────────────────────────────
/** @deprecated 使用各具体子状态 label 函数 */
export const TASK_SUB_STATUS_LABELS: Record<TaskSubStatus, string> = {
  PendingAssign: '待指派',
  PendingAuditA: '待审核',
  RejectedByAuditA: '审核打回',
  PendingAuditB: '待审核',
  RejectedByAuditB: '审核打回',
  PendingOutsource: '待定制',
  Outsourcing: '定制中',
  PendingOutsourceReview: '定制中',
  PendingCustomizationReview: '待定制审核',
  PendingCustomizationProduction: '待定制生产',
  PendingEffectReview: '待效果审核',
  PendingEffectRevision: '待效果返修',
  PendingProductionTransfer: '待生产流转',
  PendingWarehouseQC: '待仓库质检',
  RejectedByWarehouse: '仓库驳回',
  PendingWarehouseReceive: '待仓库接收',
}

/** @deprecated 使用具体子状态 label 函数 */
export function getTaskSubStatusLabel(status: TaskSubStatus): string {
  return TASK_SUB_STATUS_LABELS[status] ?? status
}

// ─── 主状态枚举常量 ───────────────────────────────────────────────────────────
export const MainTaskStatusEnum = {
  DRAFT: 'DRAFT',
  CREATED: 'CREATED',
  CODE_GENERATED: 'CODE_GENERATED',
  ERP_REGISTERED: 'ERP_REGISTERED',
  INFO_PENDING: 'INFO_PENDING',
  WAREHOUSE_PENDING: 'WAREHOUSE_PENDING',
  WAREHOUSE_PROCESSING: 'WAREHOUSE_PROCESSING',
  READY_TO_CLOSE: 'READY_TO_CLOSE',
  CLOSED: 'CLOSED',
  BLOCKED: 'BLOCKED',
} as const

export type MainTaskStatusEnumValue = (typeof MainTaskStatusEnum)[keyof typeof MainTaskStatusEnum]

// ─── 设计子状态 ───────────────────────────────────────────────────────────────
export const DesignSubStatusEnum = {
  NOT_REQUIRED: 'NOT_REQUIRED',
  PENDING_ASSIGN: 'PENDING_ASSIGN',
  IN_PROGRESS: 'IN_PROGRESS',
  PENDING_AUDIT: 'PENDING_AUDIT',
  REJECTED: 'REJECTED',
  APPROVED: 'APPROVED',
  FINALIZED: 'FINALIZED',
} as const

export type DesignSubStatusEnumValue = (typeof DesignSubStatusEnum)[keyof typeof DesignSubStatusEnum]

export const DESIGN_SUB_STATUS_LABELS: Record<DesignSubStatus, string> = {
  NOT_REQUIRED: '无需设计',
  PENDING_ASSIGN: '待指派',
  IN_PROGRESS: '设计中',
  PENDING_AUDIT: '待审核',
  REJECTED: '设计打回',
  APPROVED: '设计已通过审核',
  FINALIZED: '设计已定稿',
}

export function getDesignSubStatusLabel(status: DesignSubStatus): string {
  return DESIGN_SUB_STATUS_LABELS[status] ?? status
}

// ─── 审核子状态 ───────────────────────────────────────────────────────────────
export const AuditSubStatusEnum = {
  NOT_REQUIRED: 'NOT_REQUIRED',
  PENDING: 'PENDING',
  IN_PROGRESS: 'IN_PROGRESS',
  PASSED: 'PASSED',
  REJECTED: 'REJECTED',
  TRANSFERRED: 'TRANSFERRED',
  HANDED_OVER: 'HANDED_OVER',
} as const

export type AuditSubStatusEnumValue = (typeof AuditSubStatusEnum)[keyof typeof AuditSubStatusEnum]

export const AUDIT_SUB_STATUS_LABELS: Record<AuditSubStatus, string> = {
  NOT_REQUIRED: '无需审核',
  PENDING: '待审核',
  IN_PROGRESS: '审核中',
  PASSED: '审核通过',
  REJECTED: '审核打回',
  TRANSFERRED: '已转定制/转派',
  HANDED_OVER: '已交班',
}

export function getAuditSubStatusLabel(status: AuditSubStatus): string {
  return AUDIT_SUB_STATUS_LABELS[status] ?? status
}

// ─── 仓库子状态 ───────────────────────────────────────────────────────────────
export const WarehouseSubStatusEnum = {
  NOT_REQUIRED: 'NOT_REQUIRED',
  PENDING_RECEIVE: 'PENDING_RECEIVE',
  RECEIVED: 'RECEIVED',
  RETURNED: 'RETURNED',
  PACKING: 'PACKING',
  DONE: 'DONE',
} as const

export type WarehouseSubStatusEnumValue = (typeof WarehouseSubStatusEnum)[keyof typeof WarehouseSubStatusEnum]

export const WAREHOUSE_SUB_STATUS_LABELS: Record<WarehouseSubStatus, string> = {
  NOT_REQUIRED: '无需仓库',
  PENDING_RECEIVE: '待接收',
  RECEIVED: '已接收',
  RETURNED: '已退回',
  PACKING: '打包中',
  DONE: '仓库完成',
}

export function getWarehouseSubStatusLabel(status: WarehouseSubStatus): string {
  return WAREHOUSE_SUB_STATUS_LABELS[status] ?? status
}

// ─── 采购子状态 ───────────────────────────────────────────────────────────────
export const PurchaseSubStatusEnum = {
  NOT_REQUIRED: 'NOT_REQUIRED',
  PENDING: 'PENDING',
  IN_PROGRESS: 'IN_PROGRESS',
  PURCHASED: 'PURCHASED',
  INBOUND_DONE: 'INBOUND_DONE',
} as const

export type PurchaseSubStatusEnumValue = (typeof PurchaseSubStatusEnum)[keyof typeof PurchaseSubStatusEnum]

export const PURCHASE_SUB_STATUS_LABELS: Record<PurchaseSubStatus, string> = {
  NOT_REQUIRED: '无需采购',
  PENDING: '待采购',
  IN_PROGRESS: '采购中',
  PURCHASED: '已采购',
  INBOUND_DONE: '已入仓/到货',
}

export function getPurchaseSubStatusLabel(status: PurchaseSubStatus): string {
  return PURCHASE_SUB_STATUS_LABELS[status] ?? status
}

// ─── 结单状态 ─────────────────────────────────────────────────────────────────
export const CloseStatusEnum = {
  NOT_READY: 'NOT_READY',
  READY: 'READY',
  CLOSED: 'CLOSED',
} as const

export type CloseStatusEnumValue = (typeof CloseStatusEnum)[keyof typeof CloseStatusEnum]

export const CLOSE_STATUS_LABELS: Record<CloseStatus, string> = {
  NOT_READY: '未满足结单条件',
  READY: '可结单',
  CLOSED: '已结单',
}

export function getCloseStatusLabel(status: CloseStatus): string {
  return CLOSE_STATUS_LABELS[status] ?? status
}
