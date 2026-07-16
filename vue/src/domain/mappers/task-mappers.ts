import type {
  AuditSubStatus,
  DesignSubStatus,
  MainTaskStatus,
  ProductSource,
  Task,
  TaskStatus,
  TaskType,
} from '../types/task'
import { AuditSubStatusEnum, DesignSubStatusEnum } from '../enums/task-status'

export function mapToTaskBusinessType(taskType: TaskType, productSource: ProductSource): TaskType {
  if (taskType === 'SKU_PLANNING' || taskType === 'RETOUCH_TASK') return taskType
  if (taskType === 'NEW_PRODUCT_DEV' || productSource === 'new') return 'NEW_PRODUCT_DEV'
  return 'ORIGINAL_PRODUCT_DEV'
}

export function mapLegacyStatusToMain(status: TaskStatus): MainTaskStatus {
  switch (status) {
    case 'Draft': return 'DRAFT'
    case 'PendingAssign': return 'PENDING_ASSIGN'
    case 'Assigned': return 'ASSIGNED'
    case 'InProgress': return 'IN_PROGRESS'
    case 'PendingAudit': return 'PENDING_AUDIT'
    case 'Completed': return 'COMPLETED'
    case 'Archived': return 'ARCHIVED'
    case 'Cancelled': return 'CANCELLED'
    case 'Blocked': return 'BLOCKED'
    default: return 'BLOCKED'
  }
}

function defaultDesignSubStatus(task: Task): DesignSubStatus {
  if (task.taskType === 'SKU_PLANNING' || task.taskType === 'RETOUCH_TASK') {
    return DesignSubStatusEnum.NOT_REQUIRED
  }
  switch (task.status) {
    case 'PendingAssign': return DesignSubStatusEnum.PENDING_ASSIGN
    case 'Assigned':
    case 'InProgress': return DesignSubStatusEnum.IN_PROGRESS
    case 'PendingAudit': return DesignSubStatusEnum.PENDING_AUDIT
    case 'Completed':
    case 'Archived': return DesignSubStatusEnum.FINALIZED
    default: return DesignSubStatusEnum.NOT_REQUIRED
  }
}

function defaultAuditSubStatus(task: Task): AuditSubStatus {
  if (task.taskType === 'SKU_PLANNING' || task.taskType === 'RETOUCH_TASK') {
    return AuditSubStatusEnum.NOT_REQUIRED
  }
  if (task.status === 'PendingAudit') return AuditSubStatusEnum.PENDING
  if (task.status === 'Completed' || task.status === 'Archived') return AuditSubStatusEnum.PASSED
  return AuditSubStatusEnum.NOT_REQUIRED
}

export function enrichTaskDomainFields(partial: Task): Task {
  const businessType = partial.businessType ?? mapToTaskBusinessType(partial.taskType, partial.productSource)
  return {
    ...partial,
    businessType,
    mainStatus: mapLegacyStatusToMain(partial.status),
    designSubStatus: partial.designSubStatus ?? defaultDesignSubStatus(partial),
    auditSubStatus: partial.auditSubStatus ?? defaultAuditSubStatus(partial),
  }
}

export function mapStatusToMainAndSub(status: TaskStatus): {
  mainStatus: MainTaskStatus
  subStatus?: undefined
} {
  return { mainStatus: mapLegacyStatusToMain(status) }
}
