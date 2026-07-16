import type {
  ActiveTaskStatus,
  AuditSubStatus,
  DesignSubStatus,
  LegacyTaskStatus,
  MainTaskStatus,
  TaskMainStatus,
  TaskStatus,
  TaskSubStatus,
} from '../types/task'

export type {
  ActiveTaskStatus,
  AuditSubStatus,
  DesignSubStatus,
  LegacyTaskStatus,
  MainTaskStatus,
  TaskMainStatus,
  TaskStatus,
  TaskSubStatus,
}

export const TASK_STATUS_LABELS: Record<ActiveTaskStatus, string> = {
  Draft: '草稿',
  PendingAssign: '待指派',
  Assigned: '已指派',
  InProgress: '设计中',
  PendingAudit: '待审核',
  Completed: '已结单',
  Archived: '已归档',
  Blocked: '阻塞',
  Cancelled: '已取消',
}

export function getTaskStatusLabel(status: TaskStatus | string): string {
  return TASK_STATUS_LABELS[status as ActiveTaskStatus] ?? String(status)
}

export const TASK_MAIN_STATUS_LABELS: Record<MainTaskStatus, string> = {
  DRAFT: '草稿',
  PENDING_ASSIGN: '待指派',
  ASSIGNED: '已指派',
  IN_PROGRESS: '设计中',
  PENDING_AUDIT: '待审核',
  COMPLETED: '已结单',
  ARCHIVED: '已归档',
  CANCELLED: '已取消',
  BLOCKED: '阻塞',
}

export function getMainTaskStatusLabel(status: MainTaskStatus): string {
  return TASK_MAIN_STATUS_LABELS[status] ?? status
}

export function getTaskMainStatusLabel(status: TaskMainStatus): string {
  return getMainTaskStatusLabel(status)
}

export const TASK_SUB_STATUS_LABELS: Record<TaskSubStatus, string> = {
  PendingAssign: '待指派',
  PendingAudit: '待审核',
}

export function getTaskSubStatusLabel(status: TaskSubStatus): string {
  return TASK_SUB_STATUS_LABELS[status] ?? status
}

export const MainTaskStatusEnum = {
  DRAFT: 'DRAFT',
  PENDING_ASSIGN: 'PENDING_ASSIGN',
  ASSIGNED: 'ASSIGNED',
  IN_PROGRESS: 'IN_PROGRESS',
  PENDING_AUDIT: 'PENDING_AUDIT',
  COMPLETED: 'COMPLETED',
  ARCHIVED: 'ARCHIVED',
  CANCELLED: 'CANCELLED',
  BLOCKED: 'BLOCKED',
} as const

export type MainTaskStatusEnumValue = (typeof MainTaskStatusEnum)[keyof typeof MainTaskStatusEnum]

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
  APPROVED: '审核通过',
  FINALIZED: '已定稿',
}

export function getDesignSubStatusLabel(status: DesignSubStatus): string {
  return DESIGN_SUB_STATUS_LABELS[status] ?? status
}

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
  TRANSFERRED: '已转交',
  HANDED_OVER: '已交班',
}

export function getAuditSubStatusLabel(status: AuditSubStatus): string {
  return AUDIT_SUB_STATUS_LABELS[status] ?? status
}
