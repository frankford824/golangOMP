import type { Task, LegacyTaskStatus } from './types/task'
import { RoleEnum } from '@/types'
import { TaskTypeEnum, normalizeTaskType } from './enums/task-type'
import {
  DesignSubStatusEnum,
  AuditSubStatusEnum,
  WarehouseSubStatusEnum,
  MainTaskStatusEnum,
} from './enums/task-status'
import { checkTaskCompletion } from './task-completion'

/**
 * @deprecated 旧扁平状态集合，仅用于过渡期。新代码基于主/子状态字段判断。
 */
export const DONE_STATUSES: readonly LegacyTaskStatus[] = [
  'Completed',
  'Archived',
  'Cancelled',
] as const

/**
 * @deprecated 旧扁平状态集合，仅用于过渡期。
 */
export const TODAY_PENDING_STATUSES: readonly LegacyTaskStatus[] = [
  'PendingAssign',
  'InProgress',
  'RejectedByAuditA',
  'RejectedByAuditB',
] as const

const WAREHOUSE_ELIGIBLE_LEGACY_STATUSES: readonly LegacyTaskStatus[] = [
  'PendingAuditA',
  'PendingAuditB',
  'Outsourcing',
  'PendingOutsourceReview',
] as const

export const TASK_PRODUCT_INFO_MAINTAINER_ROLES = [
  RoleEnum.OPS,
  RoleEnum.WAREHOUSE,
  'Admin',
  'SuperAdmin',
  RoleEnum.SUPER_ADMIN,
  'HRAdmin',
  RoleEnum.HR_ADMIN,
  'RoleAdmin',
  'DepartmentAdmin',
  RoleEnum.DEPT_ADMIN,
  'TeamLead',
  RoleEnum.GROUP_LEADER,
  'DesignDirector',
] as const

// ─── 任务分型判断 ──────────────────────────────────────────────────────────────

export function isPurchaseTask(task: Task): boolean {
  const normalized = normalizeTaskType(task.taskType)
  if (normalized === TaskTypeEnum.PURCHASE_TASK) return true
  return task.businessType === TaskTypeEnum.PURCHASE_TASK
}

export function isRetouchTask(task: Task): boolean {
  const normalized = normalizeTaskType(task.taskType)
  if (normalized === TaskTypeEnum.RETOUCH_TASK) return true
  return task.businessType === TaskTypeEnum.RETOUCH_TASK
}

export function isCustomizationTask(task: Task): boolean {
  return (
    task.workflowLane === 'customization' ||
    task.businessLane === 'customization' ||
    task.customizationRequired === true
  )
}

/** 任务中心卡片 meta：是否展示「设计」行（设计师姓名）。 */
export function shouldShowDesignerMetaOnTaskCenterCard(task: Task): boolean {
  if (isPurchaseTask(task)) return false
  if (task.designSubStatus === DesignSubStatusEnum.NOT_REQUIRED) return false
  return true
}

// ─── 设计流程操作谓词 ──────────────────────────────────────────────────────────

/** 任务尚无设计/美工负责人（指派/接单前）。 */
export function taskHasNoDesignHandler(task: Task): boolean {
  const designerId = task.designerId ?? task.assigneeId
  const handlerId = task.currentHandlerId
  return (
    (designerId == null || String(designerId).trim() === '') &&
    (handlerId == null || String(handlerId).trim() === '')
  )
}

/**
 * 定制任务「指派美工」：PendingCustomizationProduction + 未分配。
 * 与 module claim 分工：运营指派走 POST /assign，美工自领走 customization/claim。
 */
export function canAssignCustomizationArtOperator(task: Task): boolean {
  if (!isCustomizationTask(task)) return false
  if (isPurchaseTask(task)) return false
  if (task.status !== 'PendingCustomizationProduction') return false
  return taskHasNoDesignHandler(task)
}

/** 是否可执行「指派设计师」或定制 lane「指派美工」 */
export function canAssign(task: Task): boolean {
  if (canAssignCustomizationArtOperator(task)) return true
  if (isPurchaseTask(task)) return false
  if (task.designSubStatus != null) {
    return task.designSubStatus === DesignSubStatusEnum.PENDING_ASSIGN
  }
  return task.status === 'PendingAssign'
}

/**
 * 是否已有负责人（用于区分首次指派 vs 重新指派入口）
 */
export function taskHasAssignee(task: Task): boolean {
  const id = task.designerId ?? task.assigneeId
  return id != null && String(id).trim() !== ''
}

/**
 * 是否已有「设计侧产出」记录（读模型 `asset_versions` / `design_asset_versions` 归一化结果）。
 * 用于交互提示（风险告知），不作为重派硬拦截条件。
 */
export function taskHasRecordedDesignOutput(task: Task): boolean {
  return (task.assetVersions?.length ?? 0) > 0
}

/** 主状态已进入仓库 / 结单等，禁止换人 */
function isMainStatusBlockingReassignment(task: Task): boolean {
  const m = task.mainStatus
  if (!m) return false
  return (
    m === MainTaskStatusEnum.WAREHOUSE_PENDING ||
    m === MainTaskStatusEnum.WAREHOUSE_PROCESSING ||
    m === MainTaskStatusEnum.READY_TO_CLOSE ||
    m === MainTaskStatusEnum.CLOSED ||
    m === MainTaskStatusEnum.BLOCKED
  )
}

function hasWarehousePipelineEntered(task: Task): boolean {
  if (task.warehouseSubStatus == null) return false
  return task.warehouseSubStatus !== WarehouseSubStatusEnum.NOT_REQUIRED
}

/** 扁平 task_status 已进入审核责任链 / 定制 / 仓库 / 结单前序等 */
function isLegacyAfterDesignerReassignmentPhase(task: Task): boolean {
  const s = task.status
  return (
    s === 'PendingAuditA' ||
    s === 'RejectedByAuditA' ||
    s === 'PendingAuditB' ||
    s === 'RejectedByAuditB' ||
    s === 'PendingOutsource' ||
    s === 'Outsourcing' ||
    s === 'PendingOutsourceReview' ||
    s === 'PendingWarehouseReceive' ||
    s === 'PendingClose'
  )
}

/** 设计子状态已进入待审 / 打回重做 / 通过等，禁止换人（打回后由原设计师继续） */
function isDesignSubStatusAfterReassignmentPhase(task: Task): boolean {
  if (task.designSubStatus == null) return false
  const d = task.designSubStatus
  return (
    d === DesignSubStatusEnum.PENDING_AUDIT ||
    d === DesignSubStatusEnum.REJECTED ||
    d === DesignSubStatusEnum.APPROVED ||
    d === DesignSubStatusEnum.FINALIZED
  )
}

/**
 * 审核子状态只要进入审核责任链就禁止换人。
 */
function isAuditSubStatusBlockingReassignment(task: Task): boolean {
  if (task.auditSubStatus == null) return false
  const a = task.auditSubStatus
  return a !== AuditSubStatusEnum.NOT_REQUIRED
}

/**
 * 定制任务「美工已指派、可重新指派」阶段：
 * PendingCustomizationProduction + 已有设计侧负责人（designer/assignee/current_handler）。
 * 与首次「指派美工」互斥；仍走 POST /v1/tasks/{id}/assign。
 */
export function isInCustomizationArtReassignmentPhase(task: Task): boolean {
  if (!isCustomizationTask(task)) return false
  if (task.status !== 'PendingCustomizationProduction') return false
  return !taskHasNoDesignHandler(task)
}

/**
 * 是否仍处于「设计责任人可调整」阶段：
 * 已指派，且仍在设计池（PendingAssign/InProgress），但尚未进入审核责任链。
 */
export function isInDesignerReassignmentPhase(task: Task): boolean {
  if (!taskHasAssignee(task)) return false
  if (task.designSubStatus != null) {
    const d = task.designSubStatus
    if (d === DesignSubStatusEnum.PENDING_ASSIGN || d === DesignSubStatusEnum.IN_PROGRESS) return true
    if (
      isRetouchTask(task) &&
      d === DesignSubStatusEnum.PENDING_AUDIT &&
      (task.status === 'InProgress' || task.status === 'PendingAssign')
    ) {
      return true
    }
    return false
  }
  const s = task.status
  return s === 'PendingAssign' || s === 'InProgress'
}

/**
 * 是否可「重新指派设计师」：已指派 + 仍在设计阶段 + 未进入审核责任链及之后阶段。
 * 设计产出仅用于提示，不做硬拦截。
 * 仍复用 `POST /v1/tasks/{id}/assign`；若后端状态机与此前提不一致可能 409。
 */
export function canReassignDesigner(task: Task): boolean {
  const customizationReassign = isInCustomizationArtReassignmentPhase(task)
  if (!customizationReassign && !taskHasAssignee(task)) return false
  if (isPurchaseTask(task)) return false
  if (isDoneStatus(task)) return false
  // 定制 PendingCustomizationProduction 的 main/design/audit 子状态由读模型推导，
  // 与常规设计重派阶段不同；仅对定制美工重派跳过这些常规门禁。
  if (!customizationReassign) {
    if (isMainStatusBlockingReassignment(task)) return false
    if (hasWarehousePipelineEntered(task)) return false
    if (!isRetouchTask(task)) {
      if (isLegacyAfterDesignerReassignmentPhase(task)) return false
      if (isDesignSubStatusAfterReassignmentPhase(task)) return false
    } else {
      const d = task.designSubStatus
      if (d === DesignSubStatusEnum.APPROVED || d === DesignSubStatusEnum.FINALIZED) return false
    }
    if (isAuditSubStatusBlockingReassignment(task)) return false
  }
  if (customizationReassign) return true
  return isInDesignerReassignmentPhase(task)
}

/**
 * 是否可调用 POST /v1/tasks/{id}/submit-design（业务提交/审核流转，非「写出详情里的版本列表」）。
 * 必须与后端状态机一致：仅 InProgress / RejectedByAuditA / RejectedByAuditB；
 * 以权威扁平 task_status 为准，避免 design_sub_status 与列表缓存不一致时误调（409）。
 */
export function canSubmitAudit(task: Task): boolean {
  if (isPurchaseTask(task)) return false
  if (isRetouchTask(task)) return Boolean(task.designerId)
  if (isCustomizationTask(task)) return task.status === 'PendingCustomizationProduction'
  const s = task.status
  return s === 'InProgress' || s === 'RejectedByAuditA' || s === 'RejectedByAuditB'
}

/**
 * 扁平 task_status 仍处于设计侧可上传/可提交区间（与 submit-design、upload-sessions 后端策略一致）。
 * 已进入 PendingAuditA 等则不应出现在设计工作台队列，也不应展示交付上传。
 */
export function isLegacyTaskStatusInDesignerEditablePhase(task: Task): boolean {
  if (isPurchaseTask(task)) return false
  const s = task.status
  if (isCustomizationTask(task)) {
    return s === 'PendingCustomizationProduction'
  }
  return (
    s === 'PendingAssign' ||
    s === 'InProgress' ||
    s === 'RejectedByAuditA' ||
    s === 'RejectedByAuditB'
  )
}

/**
 * 商品/ERP 基础资料维护不受设计、审核、仓库节点限制；流程范围仍由详情页的数据范围判断兜底。
 */
export function canMaintainTaskProductInfoAtAnyStage(
  hasAnyRole: (roles: readonly string[]) => boolean,
  hasTaskScopeAccess: boolean,
): boolean {
  return hasTaskScopeAccess && hasAnyRole(TASK_PRODUCT_INFO_MAINTAINER_ROLES)
}

/**
 * 交付设计稿上传区是否应展示：设计子状态 + 权威扁平状态双重要求，避免 design_sub_status 滞后导致 403。
 */
export function canUploadDesignDelivery(task: Task): boolean {
  if (isPurchaseTask(task)) return false
  if (isRetouchTask(task)) return Boolean(task.designerId)
  if (isCustomizationTask(task)) {
    return task.status === 'PendingCustomizationProduction'
  }
  if (!isLegacyTaskStatusInDesignerEditablePhase(task)) return false
  return task.designSubStatus === 'IN_PROGRESS' || task.designSubStatus === 'REJECTED'
}

/**
 * 审核工作台「审核稿上传区」（AuditAssetPanel）的可见性门闩。
 *
 * 与 docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md 的上传会话状态策略一致：
 * 后端在 audit 阶段（PendingAuditA / PendingAuditB）等非设计可编辑状态下，
 * 禁止 `POST /v1/assets/upload-sessions` 创建新会话，会返回
 * `PERMISSION_DENIED + deny_code=task_status_not_actionable`。
 * 因此 UI 在这些状态下不应暴露上传入口——前端是门禁，不是第二个后端。
 */
export function canUploadAuditAsset(task: Task): boolean {
  if (isPurchaseTask(task)) return false
  return isLegacyTaskStatusInDesignerEditablePhase(task)
}

/** 是否处于审核节点（可通过/打回/转交/交班） */
export function canAudit(task: Task): boolean {
  if (isPurchaseTask(task) || isRetouchTask(task)) return false
  if (isCustomizationTask(task)) return false
  return task.status === 'PendingAuditA' || task.status === 'PendingAuditB'
}

/** 是否可调用结单/归档接口（与结单条件面板 checkTaskCompletion 对齐，避免仅 READY_TO_CLOSE 与读模型滞后冲突） */
export function canArchive(task: Task): boolean {
  if (task.mainStatus === 'CLOSED' || task.mainStatus === 'BLOCKED') return false
  if (task.status === 'Completed' || task.status === 'Archived' || task.status === 'Cancelled') return false
  if (task.mainStatus === 'READY_TO_CLOSE') return true
  return checkTaskCompletion(task).canComplete
}

/** 是否可执行「重新提交复审」 */
export function canResubmitAuditB(task: Task): boolean {
  if (task.auditSubStatus != null) return task.auditSubStatus === AuditSubStatusEnum.REJECTED
  return task.status === 'RejectedByAuditB'
}

/**
 * 是否可执行「创建外协单」（API：`POST /v1/tasks/{id}/outsource`）。
 * `need_outsource` 仅为已持久化的外协意图标志，不作为「已有 customization job」的证明；
 * 此处不按 needOutsource 硬门禁，以免与审核工作台实际入口不一致。
 */
export function canCreateOutsource(task: Task): boolean {
  return task.status === 'RejectedByAuditB' || task.status === 'PendingOutsource'
}

// ─── 仓库操作谓词 ──────────────────────────────────────────────────────────────

/** 是否可进入「待仓库接收」 */
export function canGoWarehouse(task: Task): boolean {
  if (task.warehouseSubStatus != null) {
    return task.warehouseSubStatus === WarehouseSubStatusEnum.PENDING_RECEIVE
  }
  return task.status === 'PendingWarehouseReceive'
}

/** 是否应出现在仓库接收列表 */
export function isInWarehouseReceiptList(task: Task): boolean {
  if (task.warehouseSubStatus != null) {
    return (
      task.warehouseSubStatus === WarehouseSubStatusEnum.PENDING_RECEIVE ||
      task.warehouseSubStatus === WarehouseSubStatusEnum.RECEIVED ||
      task.warehouseSubStatus === WarehouseSubStatusEnum.DONE
    )
  }
  return (
    task.status === 'PendingWarehouseReceive' ||
    task.status === 'Completed' ||
    task.warehouseReceiveStatus != null
  )
}

// ─── 综合状态判断 ──────────────────────────────────────────────────────────────

/** 任务是否已处于结束态 */
export function isDoneStatus(task: Task): boolean {
  if (task.mainStatus != null) {
    return task.mainStatus === 'CLOSED' || task.mainStatus === 'BLOCKED'
  }
  return (DONE_STATUSES as readonly string[]).includes(task.status)
}

/** 是否已完成或已归档 */
export function isCompletedOrArchived(task: Task): boolean {
  if (task.mainStatus != null) {
    return task.mainStatus === 'READY_TO_CLOSE' || task.mainStatus === 'CLOSED'
  }
  return task.status === 'Completed' || task.status === 'Archived'
}

/**
 * 是否处于审核工作台应当消费的队列状态。
 *
 * 包含普通审核链路（A/B）与定制审核链路（初审 / 效果审核）两个 lane，
 * 供审核工作台的「待审核 / 我处理中」Tab 共享。Rejected 态不在队列内
 * （退回设计师处理），不纳入此谓词。
 */
export function isInAuditQueue(task: Task): boolean {
  if (isRetouchTask(task)) return false
  return (
    task.status === 'PendingAuditA' ||
    task.status === 'PendingAuditB' ||
    task.status === 'PendingCustomizationReview' ||
    task.status === 'PendingEffectReview'
  )
}

/** 是否处于定制流程链路 */
export function isInCustomizationFlow(task: Task): boolean {
  return (
    isCustomizationTask(task) ||
    task.status === 'PendingOutsource' ||
    task.status === 'Outsourcing' ||
    task.status === 'PendingOutsourceReview' ||
    task.status === 'PendingCustomizationReview'
  )
}

/** 是否待审核（A 阶段） */
export function isPendingAuditA(task: Task): boolean {
  if (isRetouchTask(task)) return false
  return task.status === 'PendingAuditA'
}

/** 是否待审核（B 阶段） */
export function isPendingAuditB(task: Task): boolean {
  if (isRetouchTask(task)) return false
  return task.status === 'PendingAuditB'
}

/** 是否待定制初审（审核超管在定制 lane 首次审核节点） */
export function isPendingCustomizationReview(task: Task): boolean {
  return task.status === 'PendingCustomizationReview'
}

/** 是否待效果审核（定制 lane 二次审核节点） */
export function isPendingEffectReview(task: Task): boolean {
  return task.status === 'PendingEffectReview'
}

/**
 * 审核工作台右侧操作区 UI 分支决策器。
 *
 * 前端门禁原则：仅基于后端下发的 `task_status` 选择对应的操作面板，
 * 不在此处做状态机推导或自愈。返回值直接映射到:
 *   - `audit_ab`             → 普通审核面板（通过 / 打回 / 交班 / 转交）
 *   - `customization_review` → 定制初审面板（调 /v1/tasks/:id/customization/review）
 *   - `effect_review`        → 定制二次效果审核面板（调 /v1/customization-jobs/:jobId/effect-review）
 *   - `null`                 → 当前任务不在审核责任链内
 */
export type AuditActionKind =
  | 'audit_ab'
  | 'customization_review'
  | 'effect_review'

export function auditActionForRow(task: Task): AuditActionKind | null {
  if (isPurchaseTask(task) || isRetouchTask(task)) return null
  switch (task.status) {
    case 'PendingAuditA':
    case 'PendingAuditB':
      return 'audit_ab'
    case 'PendingCustomizationReview':
      return 'customization_review'
    case 'PendingEffectReview':
      return 'effect_review'
    default:
      return null
  }
}

/** 是否处于定制流程中间态 */
export function isPendingOutsourceReview(task: Task): boolean {
  return task.status === 'PendingOutsourceReview'
}

/** 是否可执行 audit claim（按 task_status 判断，避免无效请求）
 * stage 与 POST /v1/tasks/{id}/audit/claim 一致：A | B */
export function canClaimAudit(task: Task, stage: string): boolean {
  if (isPurchaseTask(task) || isRetouchTask(task)) return false
  switch (stage) {
    case 'A':
      return task.status === 'PendingAuditA'
    case 'B':
      return task.status === 'PendingAuditB'
    default:
      return false
  }
}

/** 是否可执行「转交审核B」 */
export function canTransferToAuditB(task: Task): boolean {
  if (isPurchaseTask(task) || isRetouchTask(task)) return false
  return task.status === 'PendingAuditA'
}

/** 是否可从当前状态进入「待仓库接收」 */
export function canTransitionToPendingWarehouse(task: Task): boolean {
  if (isPurchaseTask(task)) return !isDoneStatus(task)
  // 新子状态体系：定制已转交（TRANSFERRED）时允许推入仓库，
  // 不依赖 legacy status，确保接入真实后端后逻辑同样正确。
  if (task.auditSubStatus === AuditSubStatusEnum.TRANSFERRED) return true
  return (WAREHOUSE_ELIGIBLE_LEGACY_STATUSES as readonly string[]).includes(task.status)
}

/**
 * 是否处于可发起外协单创建后的跟进行为的状态（已由审核员转定制，任务在外协链路上）。
 * 与「定制管理列表是否已有 job」无必然关系；job 仍由定制审核路径创建。
 */
export function isEligibleForOutsourceCreate(task: Task): boolean {
  return task.status === 'Outsourcing' && task.auditSubStatus === 'TRANSFERRED'
}
