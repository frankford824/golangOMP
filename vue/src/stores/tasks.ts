import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
  DesignSubStatus,
  LegacyTaskStatus,
  Task,
  TaskAssetVersion,
  WarehouseSubStatus,
} from '@/domain/types/task'
import { tasksApi } from '@/services/api/tasksApi'
import type { AssignTaskPayload, SubmitDesignPayload, TaskListParams } from '@/services/apiTypes'
import { useSyncStore } from './sync'
import { enrichTaskDomainFields } from '@/domain/mappers/task-mappers'
import { mergeDetailEnvelopeIntoTaskRaw } from '@/domain/mappers/task-detail-envelope'
import { normalizeAssetVersionsFromTaskRaw } from '@/domain/mappers/asset-versions-from-api'
import { parseReferenceFileRefs } from '@/domain/mappers/reference-file-refs'
import { nowISO } from '@/utils/date'
import type { ModuleSummary } from '@/services/apiTypes'
import {
  isPurchaseTask,
  isRetouchTask,
  canAssign,
  canReassignDesigner,
  canSubmitAudit,
  canAudit,
  canResubmitAuditB,
  canGoWarehouse,
  canArchive,
  canTransferToAuditB,
  isDoneStatus,
  isCompletedOrArchived,
  canTransitionToPendingWarehouse,
  isPendingAuditA,
} from '@/domain/task-actions'
import { canCloseTaskForArchive } from '@/domain/task-close-eligibility'
import { checkTaskCompletion } from '@/domain/task-completion'
import { sanitizeCreateTaskPayload } from '@/domain/task-create-fields'
import { buildClearDesignerAssigneePayload } from '@/domain/task-assignment-payload'
import type { PurchaseInfo } from '@/domain/types/purchase'
import { DesignSubStatusEnum } from '@/domain/enums/task-status'
import { toRelativeAssetUrl } from '@/utils/url'
import { normalizePriorityForApi, normalizePriorityFromApi } from '@/domain/task-priority'
import { hasModuleAction } from '@/domain/module-actions'

const BACKEND_TASK_TYPE_TO_FRONTEND: Record<string, Task['taskType']> = {
  original_product_development: 'ORIGINAL_PRODUCT_DEV',
  new_product_development: 'NEW_PRODUCT_DEV',
  purchase_task: 'PURCHASE_TASK',
  retouch_task: 'RETOUCH_TASK',
}

function parseCostPriceFromRaw(raw: unknown): Task['costPrice'] | undefined {
  if (raw == null) return undefined
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return { amount: raw, currency: 'CNY' }
  }
  if (typeof raw === 'object' && raw !== null) {
    const o = raw as Record<string, unknown>
    if (typeof o.amount === 'number' && Number.isFinite(o.amount)) {
      return { amount: o.amount, currency: String(o.currency ?? 'CNY') }
    }
  }
  return undefined
}

function objectOrUndefined(raw: unknown): Record<string, unknown> | undefined {
  return raw && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : undefined
}

function mergeReadModelCostGovernanceFields(
  target: Record<string, unknown>,
  readModel: Record<string, unknown> | undefined,
) {
  if (!readModel) return
  for (const key of [
    'procurement_summary',
    'procurementSummary',
    'matched_rule_governance',
    'matchedRuleGovernance',
    'override_summary',
    'overrideSummary',
    'governance_audit_summary',
    'governanceAuditSummary',
    'override_governance_boundary',
    'overrideGovernanceBoundary',
    'platform_entry_boundary',
    'platformEntryBoundary',
  ] as const) {
    if (readModel[key] != null) target[key] = readModel[key]
  }
  for (const key of [
    'cost_price',
    'costPrice',
    'cost_price_mode',
    'costPriceMode',
    'estimated_cost',
    'estimatedCost',
    'base_sale_price',
    'baseSalePrice',
    'quantity',
    'manual_cost_override',
    'manualCostOverride',
    'manual_cost_override_reason',
    'manualCostOverrideReason',
    'override_actor',
    'overrideActor',
    'override_at',
    'overrideAt',
    'erp_sync_required',
    'erpSyncRequired',
    'erp_sync_version',
    'erpSyncVersion',
  ] as const) {
    if (target[key] == null && readModel[key] != null) target[key] = readModel[key]
  }
}

function parseModuleSummariesFromEnvelope(envelope: unknown): ModuleSummary[] | undefined {
  if (!envelope || typeof envelope !== 'object') return undefined
  const modules = (envelope as Record<string, unknown>).modules
  if (!Array.isArray(modules)) return undefined
  return modules.filter(
    (module): module is ModuleSummary =>
      module != null &&
      typeof module === 'object' &&
      typeof (module as Record<string, unknown>).module_key === 'string',
  )
}

function backendAllowsDesignerReassignment(task: Task): boolean {
  const keys = [
    'reassign',
    'pool_reassign',
    'task.reassign',
    'task.reassign.team',
    'task.reassign.department',
  ] as const
  const designModule = task.moduleSummaries?.find((module) => module.module_key === 'design')
  const retouchModule = task.moduleSummaries?.find((module) => module.module_key === 'retouch')
  return (
    hasModuleAction(designModule, [...keys]) ||
    hasModuleAction(retouchModule, [...keys])
  )
}

function mapPurchaseStatus(s: unknown): PurchaseInfo['status'] {
  const v = String(s ?? '')
  const m: Record<string, PurchaseInfo['status']> = {
    NotRequired: 'NotRequired',
    PendingPurchase: 'PendingPurchase',
    pending_purchase: 'PendingPurchase',
    Purchasing: 'Purchasing',
    purchasing: 'Purchasing',
    Purchased: 'Purchased',
    purchased: 'Purchased',
    Cancelled: 'Cancelled',
    cancelled: 'Cancelled',
    draft: 'PendingPurchase',
    Draft: 'PendingPurchase',
    prepared: 'Purchasing',
    in_progress: 'Purchasing',
    completed: 'Purchased',
  }
  return m[v] ?? 'PendingPurchase'
}

/**
 * GET task 若返回 `procurement` 而非 `purchase_info`，结构与创建时 PATCH 体接近，在此归一化。
 */
function parseProcurementRecord(raw: unknown): PurchaseInfo | undefined {
  if (raw == null || typeof raw !== 'object') return undefined
  const p = raw as Record<string, unknown>
  const qty = p.quantity ?? p.purchase_quantity ?? p.qty
  const supplierName = (p.supplier_name ?? p.supplierName) as string | undefined
  const purchaseRemark = (p.purchase_remark ?? p.purchaseRemark) as string | undefined
  const note = ((p.note as string | undefined) ?? purchaseRemark)?.trim()
  const purchasePriceRaw = p.procurement_price ?? p.purchase_price ?? p.purchasePrice
  let purchasePrice: PurchaseInfo['purchasePrice']
  if (purchasePriceRaw != null && typeof purchasePriceRaw === 'object') {
    const pr = purchasePriceRaw as Record<string, unknown>
    if (typeof pr.amount === 'number') {
      purchasePrice = { amount: pr.amount, currency: String(pr.currency ?? 'CNY') }
    }
  } else if (typeof purchasePriceRaw === 'number' && Number.isFinite(purchasePriceRaw)) {
    purchasePrice = { amount: purchasePriceRaw, currency: 'CNY' }
  }
  const hasAny =
    typeof qty === 'number' ||
    (supplierName != null && String(supplierName).trim() !== '') ||
    (note != null && note !== '') ||
    purchasePrice != null ||
    p.status != null
  if (!hasAny) return undefined
  return {
    status: mapPurchaseStatus(p.status),
    supplierId:
      p.supplier_id != null
        ? String(p.supplier_id)
        : p.supplierId != null
          ? String(p.supplierId)
          : undefined,
    supplierName: supplierName ?? undefined,
    quantity: typeof qty === 'number' ? qty : undefined,
    unit: (p.unit as string) ?? undefined,
    purchasePrice,
    expectedArrivalAt: (p.expected_arrival_at ?? p.expectedArrivalAt) as string | undefined,
    warehouseLocationCode: (p.warehouse_location_code ?? p.warehouseLocationCode) as string | undefined,
    warehouseLocationName: (p.warehouse_location_name ?? p.warehouseLocationName) as string | undefined,
    note: note || undefined,
  }
}

function parsePurchaseInfoObject(raw: unknown): PurchaseInfo | undefined {
  if (raw == null || typeof raw !== 'object') return undefined
  const p = raw as Record<string, unknown>
  const purchasePriceRaw = p.purchase_price ?? p.purchasePrice
  let purchasePrice: { amount: number; currency: string } | undefined
  if (purchasePriceRaw != null && typeof purchasePriceRaw === 'object') {
    const pr = purchasePriceRaw as Record<string, unknown>
    if (typeof pr.amount === 'number') {
      purchasePrice = { amount: pr.amount, currency: String(pr.currency ?? 'CNY') }
    }
  } else if (typeof purchasePriceRaw === 'number' && Number.isFinite(purchasePriceRaw)) {
    purchasePrice = { amount: purchasePriceRaw, currency: 'CNY' }
  }
  const qty = p.quantity
  return {
    status: mapPurchaseStatus(p.status),
    supplierId:
      p.supplier_id != null
        ? String(p.supplier_id)
        : p.supplierId != null
          ? String(p.supplierId)
          : undefined,
    supplierName: (p.supplier_name ?? p.supplierName) as string | undefined,
    quantity: typeof qty === 'number' ? qty : undefined,
    unit: (p.unit as string) ?? undefined,
    purchasePrice,
    expectedArrivalAt: (p.expected_arrival_at ?? p.expectedArrivalAt) as string | undefined,
    warehouseLocationCode: (p.warehouse_location_code ?? p.warehouseLocationCode) as string | undefined,
    warehouseLocationName: (p.warehouse_location_name ?? p.warehouseLocationName) as string | undefined,
    note: (p.note as string) ?? undefined,
  }
}

/** 合并后端 TaskReadModel.procurement_summary（采购任务读模型） */
function mergeProcurementSummaryIntoPurchase(
  base: PurchaseInfo | undefined,
  summary: Record<string, unknown> | null | undefined,
): PurchaseInfo | undefined {
  if (!summary || typeof summary !== 'object') return base
  const quantity =
    base?.quantity ??
    (typeof summary.quantity === 'number' ? summary.quantity : undefined) ??
    (typeof summary.purchase_quantity === 'number' ? summary.purchase_quantity : undefined)
  const supplierFromSummary =
    typeof summary.supplier_name === 'string' ? summary.supplier_name.trim() : ''
  const supplierName =
    (base?.supplierName && String(base.supplierName).trim()) || supplierFromSummary || undefined
  let purchasePrice = base?.purchasePrice
  if (!purchasePrice && typeof summary.procurement_price === 'number') {
    purchasePrice = { amount: summary.procurement_price, currency: 'CNY' }
  }
  const expectedArrivalAt =
    base?.expectedArrivalAt ??
    (typeof summary.expected_delivery_at === 'string' ? summary.expected_delivery_at : undefined)
  const status = base?.status ?? mapPurchaseStatus(summary.status)
  const merged: PurchaseInfo = {
    status,
    supplierId: base?.supplierId,
    supplierName,
    quantity,
    unit: base?.unit,
    purchasePrice,
    expectedArrivalAt,
    warehouseLocationCode: base?.warehouseLocationCode,
    warehouseLocationName: base?.warehouseLocationName,
    note: base?.note,
  }
  const hasAny =
    merged.quantity != null ||
    (merged.supplierName != null && merged.supplierName !== '') ||
    merged.purchasePrice != null ||
    merged.expectedArrivalAt != null ||
    (base != null && Object.keys(base).length > 0)
  if (!base && !hasAny) return undefined
  return merged
}

/** 与登录用户 id（permissions 中统一为 string）比较前，将后端 number/string 规范为 string；空值为 null */
function normalizeOptionalUserId(raw: unknown): string | null {
  if (raw == null) return null
  const s = String(raw).trim()
  return s === '' ? null : s
}

function normalizeTaskLane(raw: unknown): 'normal' | 'customization' | undefined {
  const lane = String(raw ?? '').trim().toLowerCase()
  if (lane === 'normal' || lane === 'customization') {
    return lane
  }
  return undefined
}

const DESIGN_SUB_STATUS_SET = new Set<string>([
  'NOT_REQUIRED',
  'PENDING_ASSIGN',
  'IN_PROGRESS',
  'PENDING_AUDIT',
  'REJECTED',
  'APPROVED',
  'FINALIZED',
])

/** 详情/列表若带 design_sub_status，优先采用，避免仅靠 status 推导与列表不一致 */
function parseOptionalDesignSubStatus(raw: unknown): DesignSubStatus | undefined {
  if (raw == null || typeof raw !== 'string') return undefined
  const v = raw.trim().replace(/-/g, '_').toUpperCase()
  return DESIGN_SUB_STATUS_SET.has(v) ? (v as DesignSubStatus) : undefined
}

/**
 * 从 workflow.sub_status.design.code 解析设计子状态。
 * 后端读模型在详情接口中只通过 workflow.sub_status 返回，不一定有扁平 design_sub_status 字段。
 */
function parseDesignSubStatusFromWorkflow(raw: Record<string, unknown>): DesignSubStatus | undefined {
  const workflow = raw.workflow as Record<string, unknown> | undefined
  const subStatus = workflow?.sub_status as Record<string, unknown> | undefined
  const design = subStatus?.design as Record<string, unknown> | undefined
  const codeRaw = design?.code
  if (typeof codeRaw !== 'string' || !codeRaw.trim()) return undefined
  const code = codeRaw.trim().toLowerCase().replace(/-/g, '_')

  const workflowMap: Record<string, DesignSubStatus> = {
    not_required: 'NOT_REQUIRED',
    not_triggered: 'NOT_REQUIRED',
    pending_assign: 'PENDING_ASSIGN',
    in_progress: 'IN_PROGRESS',
    pending_audit: 'PENDING_AUDIT',
    rejected: 'REJECTED',
    approved: 'APPROVED',
    finalized: 'FINALIZED',
  }
  return workflowMap[code]
}

/** P 图读模型：design 子状态由 workflow.sub_status.retouch（或后端回填的扁平字段）承载。 */
function parseDesignSubStatusFromRetouchWorkflow(
  raw: Record<string, unknown>,
): DesignSubStatus | undefined {
  const workflow = raw.workflow as Record<string, unknown> | undefined
  const subStatus = workflow?.sub_status as Record<string, unknown> | undefined
  const retouch = subStatus?.retouch as Record<string, unknown> | undefined
  const codeRaw = retouch?.code
  if (typeof codeRaw !== 'string' || !codeRaw.trim()) return undefined
  const code = codeRaw.trim().toLowerCase().replace(/-/g, '_')

  const map: Record<string, DesignSubStatus> = {
    not_triggered: 'NOT_REQUIRED',
    pending_claim: 'PENDING_ASSIGN',
    in_progress: 'IN_PROGRESS',
    submitted: 'FINALIZED',
    closed: 'FINALIZED',
    completed: 'FINALIZED',
  }
  return map[code]
}

const WAREHOUSE_SUB_STATUS_SET = new Set<string>([
  'NOT_REQUIRED',
  'PENDING_RECEIVE',
  'RECEIVED',
  'RETURNED',
  'PACKING',
  'DONE',
])

function parseOptionalWarehouseSubStatus(raw: unknown): WarehouseSubStatus | undefined {
  if (raw == null || typeof raw !== 'string') return undefined
  const v = raw.trim()
  return WAREHOUSE_SUB_STATUS_SET.has(v) ? (v as WarehouseSubStatus) : undefined
}

/**
 * 从扁平字段或 workflow.sub_status.warehouse.code 解析仓库维度（读模型常见为后者）。
 * 见后端 task read model：workflow.sub_status.warehouse.code = pending_receive 等。
 */
function parseWarehouseFieldsFromApi(raw: Record<string, unknown>): Partial<
  Pick<Task, 'warehouseReceiveStatus' | 'warehouseSubStatus'>
> {
  const flatSub = parseOptionalWarehouseSubStatus(
    raw.warehouse_sub_status ?? raw.warehouseSubStatus,
  )
  const flatRecvRaw = raw.warehouse_receive_status ?? raw.warehouseReceiveStatus
  if (typeof flatRecvRaw === 'string' && flatRecvRaw.trim()) {
    const fr = flatRecvRaw.trim().toLowerCase()
    const fromFlat: Partial<Pick<Task, 'warehouseReceiveStatus' | 'warehouseSubStatus'>> = {}
    if (fr === 'pending') {
      fromFlat.warehouseReceiveStatus = 'pending'
      fromFlat.warehouseSubStatus = flatSub ?? 'PENDING_RECEIVE'
    } else if (fr === 'received') {
      fromFlat.warehouseReceiveStatus = 'received'
      fromFlat.warehouseSubStatus = flatSub ?? 'RECEIVED'
    } else if (fr === 'returned') {
      fromFlat.warehouseReceiveStatus = 'returned'
      fromFlat.warehouseSubStatus = flatSub ?? 'RETURNED'
    } else if (fr === 'archived') {
      fromFlat.warehouseReceiveStatus = 'archived'
      fromFlat.warehouseSubStatus = flatSub ?? 'DONE'
    }
    if (fromFlat.warehouseReceiveStatus != null) {
      return {
        warehouseReceiveStatus: fromFlat.warehouseReceiveStatus,
        warehouseSubStatus: flatSub ?? fromFlat.warehouseSubStatus,
      }
    }
  }
  if (flatSub != null) {
    return { warehouseSubStatus: flatSub }
  }

  const workflow = raw.workflow as Record<string, unknown> | undefined
  const subStatus = workflow?.sub_status as Record<string, unknown> | undefined
  const wh = subStatus?.warehouse as Record<string, unknown> | undefined
  const codeRaw = wh?.code
  if (typeof codeRaw !== 'string' || !codeRaw.trim()) return {}
  const code = codeRaw.trim().toLowerCase().replace(/-/g, '_')

  const workflowMap: Record<
    string,
    { warehouseSubStatus: WarehouseSubStatus; warehouseReceiveStatus?: Task['warehouseReceiveStatus'] }
  > = {
    pending_receive: { warehouseSubStatus: 'PENDING_RECEIVE', warehouseReceiveStatus: 'pending' },
    received: { warehouseSubStatus: 'RECEIVED', warehouseReceiveStatus: 'received' },
    returned: { warehouseSubStatus: 'RETURNED', warehouseReceiveStatus: 'returned' },
    packing: { warehouseSubStatus: 'PACKING' },
    done: { warehouseSubStatus: 'DONE', warehouseReceiveStatus: 'archived' },
    complete: { warehouseSubStatus: 'DONE', warehouseReceiveStatus: 'archived' },
    warehouse_complete: { warehouseSubStatus: 'DONE', warehouseReceiveStatus: 'archived' },
    not_required: { warehouseSubStatus: 'NOT_REQUIRED' },
    not_triggered: { warehouseSubStatus: 'NOT_REQUIRED' },
  }

  const mapped = workflowMap[code]
  if (mapped) return mapped
  return {}
}

function parseWorkflowCloseFields(raw: Record<string, unknown>): Partial<
  Pick<Task, 'workflowCanClose' | 'cannotCloseReasons' | 'workflowMainStatus'>
> {
  const workflowRaw = raw.workflow as Record<string, unknown> | undefined
  if (!workflowRaw) return {}
  const wfMainRaw = workflowRaw.main_status ?? workflowRaw.mainStatus
  let workflowMainStatus: string | undefined
  if (typeof wfMainRaw === 'string' && wfMainRaw.trim()) {
    workflowMainStatus = wfMainRaw.trim().toLowerCase()
  }
  let workflowCanClose: boolean | undefined
  if (typeof workflowRaw.can_close === 'boolean') workflowCanClose = workflowRaw.can_close
  else if (typeof workflowRaw.closable === 'boolean') workflowCanClose = workflowRaw.closable

  const ccr = workflowRaw.cannot_close_reasons
  let cannotCloseReasons: Task['cannotCloseReasons']
  if (Array.isArray(ccr)) {
    cannotCloseReasons = ccr
      .map((item) => {
        if (!item || typeof item !== 'object') return undefined
        const o = item as Record<string, unknown>
        const code = typeof o.code === 'string' ? o.code : ''
        const message = typeof o.message === 'string' ? o.message : ''
        if (!code && !message) return undefined
        return { code: code || 'unknown', message: message || code }
      })
      .filter((x): x is { code: string; message: string } => x != null)
    if (cannotCloseReasons.length === 0) cannotCloseReasons = undefined
  }

  return {
    ...(workflowMainStatus ? { workflowMainStatus } : {}),
    ...(workflowCanClose !== undefined ? { workflowCanClose } : {}),
    ...(cannotCloseReasons != null ? { cannotCloseReasons } : {}),
  }
}

const KNOWN_LEGACY_STATUSES = new Set<string>([
  'Draft',
  'PendingAssign',
  'InProgress',
  'PendingAuditA',
  'RejectedByAuditA',
  'PendingAuditB',
  'RejectedByAuditB',
  'PendingOutsource',
  'Outsourcing',
  'PendingOutsourceReview',
  'PendingCustomizationReview',
  'PendingCustomizationProduction',
  'PendingEffectReview',
  'PendingEffectRevision',
  'PendingProductionTransfer',
  'PendingWarehouseQC',
  'RejectedByWarehouse',
  'PendingWarehouseReceive',
  'PendingClose',
  'Completed',
  'Archived',
  'Blocked',
  'Cancelled',
])

/** OpenAPI TaskListItem.task_status 含 `Assigned` 等；列表也可能只推进 workflow.sub_status 而扁平字段滞后 */
function normalizeFlatTaskStatusFromApi(value: unknown): LegacyTaskStatus {
  const s = String(value ?? '').trim()
  if (!s) return 'Draft'
  if (KNOWN_LEGACY_STATUSES.has(s)) return s as LegacyTaskStatus
  const lower = s.toLowerCase().replace(/-/g, '_')
  const map: Record<string, LegacyTaskStatus> = {
    assigned: 'InProgress',
    in_progress: 'InProgress',
    pending_assign: 'PendingAssign',
    pending_audit_a: 'PendingAuditA',
    pending_audit_b: 'PendingAuditB',
    rejected_by_audit_a: 'RejectedByAuditA',
    rejected_by_audit_b: 'RejectedByAuditB',
    pending_outsource: 'PendingOutsource',
    outsourcing: 'Outsourcing',
    pending_outsource_review: 'PendingOutsourceReview',
    pending_customization_review: 'PendingCustomizationReview',
    pending_customization_production: 'PendingCustomizationProduction',
    pending_effect_review: 'PendingEffectReview',
    pending_effect_revision: 'PendingEffectRevision',
    pending_production_transfer: 'PendingProductionTransfer',
    pending_warehouse_qc: 'PendingWarehouseQC',
    rejected_by_warehouse: 'RejectedByWarehouse',
    pending_warehouse_receive: 'PendingWarehouseReceive',
    pending_close: 'PendingClose',
    completed: 'Completed',
    archived: 'Archived',
    blocked: 'Blocked',
    cancelled: 'Cancelled',
  }
  const mapped = map[lower]
  if (mapped) return mapped
  return s as LegacyTaskStatus
}

/**
 * 当扁平 task_status 仍为「设计中」但 workflow.sub_status.audit 已进入待审/复审时，
 * 对齐为 PendingAudit*，否则审核工作台左侧（仅看 tasksStore.list + isPendingAudit*）会漏单。
 * retouch_task 不进入审核轨，禁止根据 audit 子状态抬升为 PendingAudit*。
 */
function reconcileLegacyStatusWithWorkflowAudit(
  raw: Record<string, unknown>,
  flatStatus: LegacyTaskStatus,
): LegacyTaskStatus {
  if (flatStatus === 'PendingAuditA' || flatStatus === 'PendingAuditB') return flatStatus

  const rawTaskType = String(raw.task_type ?? raw.taskType ?? '').toLowerCase()
  if (rawTaskType === 'retouch_task') return flatStatus

  const wf = raw.workflow as Record<string, unknown> | undefined
  if (!wf || typeof wf !== 'object') return flatStatus
  const sub = wf.sub_status as Record<string, unknown> | undefined
  if (!sub || typeof sub !== 'object') return flatStatus
  const audit = sub.audit as Record<string, unknown> | undefined
  if (!audit || typeof audit !== 'object') return flatStatus
  const codeRaw = audit.code
  if (typeof codeRaw !== 'string' || !codeRaw.trim()) return flatStatus
  const code = codeRaw.trim().toLowerCase().replace(/-/g, '_')

  const designLike =
    flatStatus === 'InProgress' ||
    flatStatus === 'PendingAssign' ||
    flatStatus === 'RejectedByAuditA' ||
    flatStatus === 'RejectedByAuditB'

  if (!designLike) return flatStatus

  if (code === 'pending_audit' || code === 'pending_review' || code === 'in_review') {
    const stage = String(raw.audit_stage ?? raw.auditStage ?? '').toUpperCase()
    if (stage === 'B' || stage === 'AUDIT_B') return 'PendingAuditB'
    return 'PendingAuditA'
  }
  return flatStatus
}

/** 将后端任务列表项转为前端 Task 形状（供 enrichTaskDomainFields 使用） */
function normalizeBackendTask(raw: Record<string, unknown>): Task {
  const workflow = (raw.workflow ?? raw) as Record<string, unknown>
  const rawStatus = workflow.status ?? raw.status ?? raw.task_status ?? 'Draft'
  let status = normalizeFlatTaskStatusFromApi(rawStatus)
  status = reconcileLegacyStatusWithWorkflowAudit(raw, status)
  const now = nowISO()
  const rawTaskType = (raw.task_type ?? raw.taskType ?? 'ORIGINAL_PRODUCT_DEV') as string
  const taskType = BACKEND_TASK_TYPE_TO_FRONTEND[rawTaskType] ?? (rawTaskType as Task['taskType'])
  /**
   * 列表/详情统一：`item|data.product_selection.erp_product`（及平级 `source_match_type`）。
   * 顶层 `sku_code` / `product_id` / `product_name` 可能缺省，需从 erp_product 回填。
   */
  const productSelection = raw.product_selection as Record<string, unknown> | undefined
  const erpProduct = productSelection?.erp_product as Record<string, unknown> | undefined
  const sourceMatchRaw = productSelection?.source_match_type ?? productSelection?.sourceMatchType
  const productSelectionSourceMatchType =
    typeof sourceMatchRaw === 'string' && sourceMatchRaw.trim() !== ''
      ? sourceMatchRaw.trim()
      : undefined
  const erpSkuIdRaw = erpProduct?.sku_id ?? erpProduct?.skuId
  const erpProductSkuId =
    erpSkuIdRaw != null && String(erpSkuIdRaw).trim() !== '' ? String(erpSkuIdRaw).trim() : undefined
  const rawProductId = raw.product_id ?? raw.productId ?? erpProduct?.product_id ?? erpProduct?.productId
  const productImageUrl =
    typeof erpProduct?.image_url === 'string' && erpProduct.image_url ? erpProduct.image_url : undefined
  const erpCategoryCode =
    typeof erpProduct?.category_code === 'string' && String(erpProduct.category_code).trim() !== ''
      ? String(erpProduct.category_code)
      : undefined
  const erpCategoryName =
    typeof erpProduct?.category_name === 'string' && String(erpProduct.category_name).trim() !== ''
      ? String(erpProduct.category_name)
      : undefined
  const erpIId =
    typeof (erpProduct?.i_id ?? erpProduct?.iId) === 'string' &&
    String(erpProduct?.i_id ?? erpProduct?.iId).trim() !== ''
      ? String(erpProduct?.i_id ?? erpProduct?.iId).trim()
      : undefined
  const erpProductNameRaw = erpProduct?.product_name ?? erpProduct?.productName ?? erpProduct?.name
  const erpProductName =
    typeof erpProductNameRaw === 'string' && erpProductNameRaw.trim() !== ''
      ? erpProductNameRaw.trim()
      : undefined

  const explicitProductSource = raw.product_source ?? raw.productSource
  const productSource: Task['productSource'] =
    explicitProductSource === 'new' || explicitProductSource === 'existing'
      ? explicitProductSource
      : taskType === 'NEW_PRODUCT_DEV'
        ? 'new'
        : 'existing'

  const procurementSummary = objectOrUndefined(raw.procurement_summary ?? raw.procurementSummary)
  const costOverrideSummary = objectOrUndefined(raw.override_summary ?? raw.overrideSummary)
  const governanceAuditSummary = objectOrUndefined(raw.governance_audit_summary ?? raw.governanceAuditSummary)
  const costOverrideBoundary = objectOrUndefined(
    raw.override_governance_boundary ?? raw.overrideGovernanceBoundary,
  )
  const procurementRaw = raw.procurement as Record<string, unknown> | undefined
  const procurementApiStatus =
    typeof procurementRaw?.status === 'string' ? procurementRaw.status : undefined

  const workflowRaw = raw.workflow as Record<string, unknown> | undefined
  const canPrepareWarehouse =
    typeof workflowRaw?.can_prepare_warehouse === 'boolean'
      ? workflowRaw.can_prepare_warehouse
      : undefined
  const whBlockRaw = workflowRaw?.warehouse_blocking_reasons
  let warehouseBlockingReasons: Task['warehouseBlockingReasons']
  if (Array.isArray(whBlockRaw)) {
    warehouseBlockingReasons = whBlockRaw
      .map((r: unknown) => {
        if (!r || typeof r !== 'object') return undefined
        const o = r as Record<string, unknown>
        const code = typeof o.code === 'string' ? o.code : ''
        const message = typeof o.message === 'string' ? o.message : ''
        if (!code && !message) return undefined
        return { code: code || 'unknown', message: message || code }
      })
      .filter((x): x is { code: string; message: string } => x !== undefined)
    if (warehouseBlockingReasons.length === 0) warehouseBlockingReasons = undefined
  }
  const warehousePrepareReady =
    procurementSummary != null && typeof procurementSummary.warehouse_prepare_ready === 'boolean'
      ? procurementSummary.warehouse_prepare_ready
      : undefined
  const warehouseReceiveReady =
    procurementSummary != null && typeof procurementSummary.warehouse_receive_ready === 'boolean'
      ? procurementSummary.warehouse_receive_ready
      : undefined

  const rawCatCode = raw.category_code ?? raw.categoryCode
  const rawCategoryName = raw.category_name ?? raw.categoryName
  const categoryName =
    typeof rawCategoryName === 'string' && String(rawCategoryName).trim() !== ''
      ? String(rawCategoryName).trim()
      : undefined
  const rawCategory = raw.category
  const category =
    typeof rawCategory === 'string' && String(rawCategory).trim() !== ''
      ? String(rawCategory).trim()
      : undefined
  const newProductCategoryCode =
    typeof rawCatCode === 'string' && String(rawCatCode).trim() !== ''
      ? String(rawCatCode).trim()
      : categoryName ?? category
  const matRaw = raw.material ?? raw.material_code ?? raw.materialCode
  const newProductMaterial =
    typeof matRaw === 'string' && String(matRaw).trim() !== '' ? String(matRaw) : undefined
  const matOtherRaw = raw.material_other ?? raw.materialOther
  const newProductMaterialOther =
    typeof matOtherRaw === 'string' && String(matOtherRaw).trim() !== ''
      ? String(matOtherRaw)
      : undefined
  const psShort = raw.product_short_name ?? raw.productShortName
  const productShortName =
    typeof psShort === 'string' && String(psShort).trim() !== '' ? String(psShort) : undefined
  const refLinkRaw = raw.reference_link ?? raw.product_reference_url ?? raw.productReferenceUrl
  const productReferenceUrl =
    typeof refLinkRaw === 'string' && String(refLinkRaw).trim() !== '' ? String(refLinkRaw) : undefined
  const qtyRaw = raw.quantity ?? raw.purchase_quantity ?? procurementSummary?.quantity
  let newProductQuantity: number | undefined =
    typeof qtyRaw === 'number' && Number.isFinite(qtyRaw) ? qtyRaw : undefined
  const costUnitRaw = raw.cost_unit_price ?? raw.costUnitPrice ?? procurementSummary?.cost_unit_price
  let newProductCostUnitPrice: number | undefined =
    typeof costUnitRaw === 'number' && Number.isFinite(costUnitRaw) ? costUnitRaw : undefined
  const chRaw = raw.product_channel ?? raw.productChannel
  const productChannel =
    typeof chRaw === 'string' && String(chRaw).trim() !== '' ? String(chRaw) : undefined
  const specTr = raw.spec_text ?? raw.specText
  const specText =
    typeof specTr === 'string' && String(specTr).trim() !== '' ? String(specTr).trim() : undefined
  const sizeTr = raw.size_text ?? raw.sizeText
  const sizeText =
    typeof sizeTr === 'string' && String(sizeTr).trim() !== '' ? String(sizeTr).trim() : undefined

  let purchaseInfo =
    parsePurchaseInfoObject(raw.purchase_info ?? raw.purchaseInfo) ??
    parseProcurementRecord(raw.procurement)
  purchaseInfo =
    mergeProcurementSummaryIntoPurchase(purchaseInfo, procurementSummary) ?? purchaseInfo

  let costPrice = parseCostPriceFromRaw(raw.cost_price ?? raw.costPrice)
  if (!costPrice && costOverrideSummary && typeof costOverrideSummary.current_cost_price === 'number') {
    costPrice = { amount: costOverrideSummary.current_cost_price, currency: 'CNY' }
  }
  if (!costPrice && procurementSummary && typeof procurementSummary.cost_price === 'number') {
    costPrice = { amount: procurementSummary.cost_price, currency: 'CNY' }
  }

  const bspRaw = raw.base_sale_price ?? raw.baseSalePrice
  let basePriceAmount: number | undefined =
    typeof bspRaw === 'number' && Number.isFinite(bspRaw) ? bspRaw : undefined
  if (basePriceAmount == null && procurementSummary && typeof procurementSummary === 'object') {
    const ps = procurementSummary as Record<string, unknown>
    const psBsp = ps.base_sale_price ?? ps.baseSalePrice
    if (typeof psBsp === 'number' && Number.isFinite(psBsp)) basePriceAmount = psBsp
  }

  let costPriceMode: Task['costPriceMode'] | undefined =
      raw.cost_price_mode === 'manual' || raw.cost_price_mode === 'template'
        ? (raw.cost_price_mode as Task['costPriceMode'])
        : raw.costPriceMode === 'manual' || raw.costPriceMode === 'template'
          ? (raw.costPriceMode as Task['costPriceMode'])
          : undefined
  if (costPriceMode == null && procurementSummary && typeof procurementSummary === 'object') {
    const ps = procurementSummary as Record<string, unknown>
    const cpm = ps.cost_price_mode ?? ps.costPriceMode
    if (cpm === 'manual' || cpm === 'template') costPriceMode = cpm as Task['costPriceMode']
    if (costPriceMode == null && ps.manual_cost_override === true) costPriceMode = 'manual'
  }
  if (costPriceMode == null) {
    const manualOverride = raw.manual_cost_override ?? raw.manualCostOverride ?? costOverrideSummary?.current_override_active
    if (manualOverride === true) costPriceMode = 'manual'
  }

  const isRetouchType = String(rawTaskType).toLowerCase() === 'retouch_task'
  const designSubStatusFromApi =
    parseOptionalDesignSubStatus(raw.design_sub_status ?? raw.designSubStatus) ??
    (isRetouchType ? parseDesignSubStatusFromRetouchWorkflow(raw) : undefined) ??
    parseDesignSubStatusFromWorkflow(raw)
  const skuItemsRaw = raw.sku_items ?? raw.skuItems
  const skuItems = Array.isArray(skuItemsRaw)
    ? skuItemsRaw
      .map((item) => {
        if (!item || typeof item !== 'object') return undefined
        const o = item as Record<string, unknown>
        const itemRefs = parseReferenceFileRefs(o.reference_file_refs ?? o.referenceFileRefs)
        const itemDesignReq =
          (o.design_requirement as string | undefined)?.trim() ||
          (o.designRequirement as string | undefined)?.trim() ||
          (o.change_request as string | undefined)?.trim() ||
          (o.changeRequest as string | undefined)?.trim() ||
          undefined
        return {
	          id: typeof o.id === 'number' ? o.id : undefined,
	          sequenceNo: typeof o.sequence_no === 'number' ? o.sequence_no : undefined,
	          skuCode: typeof o.sku_code === 'string' ? o.sku_code : undefined,
	          skuCodeType: typeof o.sku_code_type === 'string' ? o.sku_code_type : undefined,
	          quantity:
            typeof o.quantity === 'number' && Number.isFinite(o.quantity) ? o.quantity : undefined,
          skuStatus: typeof o.sku_status === 'string' ? o.sku_status : undefined,
          productNameSnapshot:
            typeof o.product_name_snapshot === 'string' ? o.product_name_snapshot : undefined,
          productShortName:
            typeof o.product_short_name === 'string' ? o.product_short_name : undefined,
          productIId: (() => {
            const a = typeof o.product_i_id === 'string' ? o.product_i_id.trim() : ''
            if (a !== '') return a
            const b = typeof o.i_id === 'string' ? o.i_id.trim() : ''
            if (b !== '') return b
            return undefined
          })(),
          erpProductId: typeof o.erp_product_id === 'string' ? o.erp_product_id : undefined,
          categoryCode: typeof o.category_code === 'string' ? o.category_code : undefined,
          materialMode: typeof o.material_mode === 'string' ? o.material_mode : undefined,
          costPriceMode: typeof o.cost_price_mode === 'string' ? o.cost_price_mode : undefined,
          costPrice:
            typeof o.cost_price === 'number' && Number.isFinite(o.cost_price) ? o.cost_price : undefined,
          estimatedCost:
            typeof o.estimated_cost === 'number' && Number.isFinite(o.estimated_cost)
              ? o.estimated_cost
              : undefined,
          costRuleId:
            typeof o.cost_rule_id === 'number' && Number.isFinite(o.cost_rule_id)
              ? o.cost_rule_id
              : undefined,
          costRuleName: typeof o.cost_rule_name === 'string' ? o.cost_rule_name : undefined,
          costRuleSource: typeof o.cost_rule_source === 'string' ? o.cost_rule_source : undefined,
          matchedRuleVersion:
            typeof o.matched_rule_version === 'number' && Number.isFinite(o.matched_rule_version)
              ? o.matched_rule_version
              : undefined,
          prefillSource: typeof o.prefill_source === 'string' ? o.prefill_source : undefined,
          prefillAt:
            typeof o.prefill_at === 'string'
              ? o.prefill_at
              : o.prefill_at === null
                ? null
                : undefined,
          requiresManualReview:
            typeof o.requires_manual_review === 'boolean' ? o.requires_manual_review : undefined,
          manualCostOverride:
            typeof o.manual_cost_override === 'boolean' ? o.manual_cost_override : undefined,
          manualCostOverrideReason:
            typeof o.manual_cost_override_reason === 'string' ? o.manual_cost_override_reason : undefined,
          overrideActor: typeof o.override_actor === 'string' ? o.override_actor : undefined,
          overrideAt:
            typeof o.override_at === 'string'
              ? o.override_at
              : o.override_at === null
                ? null
                : undefined,
          filing_status: typeof o.filing_status === 'string' ? o.filing_status : undefined,
          erp_sync_status: typeof o.erp_sync_status === 'string' ? o.erp_sync_status : undefined,
          erp_sync_required:
            typeof o.erp_sync_required === 'boolean' ? o.erp_sync_required : undefined,
          erp_sync_version:
            typeof o.erp_sync_version === 'number' && Number.isFinite(o.erp_sync_version)
              ? o.erp_sync_version
              : undefined,
          last_filed_at:
            typeof o.last_filed_at === 'string'
              ? o.last_filed_at
              : o.last_filed_at === null
                ? null
                : undefined,
          filing_error_message:
            typeof o.filing_error_message === 'string' ? o.filing_error_message : undefined,
          ...(itemRefs.length ? { referenceFileRefs: itemRefs } : {}),
          ...(typeof itemDesignReq === 'string' && itemDesignReq.trim() !== ''
            ? { designRequirement: itemDesignReq.trim() }
            : {}),
        }
      })
      .filter((x): x is NonNullable<typeof x> => x != null)
    : undefined

  const skuCoalesced = raw.sku ?? raw.sku_code ?? raw.skuCode ?? erpProduct?.sku_code ?? erpProduct?.skuCode
  const skuNormalized =
    skuCoalesced != null && String(skuCoalesced).trim() !== '' ? String(skuCoalesced).trim() : null

  return {
    id: String(raw.id ?? ''),
    taskNo: String(raw.task_no ?? raw.taskNo ?? ''),
    sku: skuNormalized,
    productId: rawProductId != null && String(rawProductId).trim() !== '' ? String(rawProductId).trim() : null,
    productName: String(
      raw.product_name ?? raw.product_name_snapshot ?? raw.productName ?? erpProductName ?? '',
    ),
    productImageUrl,
    productSource,
    taskType,
    ...(erpProductSkuId != null ? { erpProductSkuId } : {}),
    ...(productSelectionSourceMatchType != null ? { productSelectionSourceMatchType } : {}),
    erpIId,
    erpCategoryCode,
    erpCategoryName,
    categoryName,
    category,
    newProductCategoryCode,
    newProductMaterial,
    newProductMaterialOther,
    productShortName,
    productReferenceUrl,
    newProductQuantity,
    newProductCostUnitPrice,
    productChannel,
    ...(specText != null ? { specText } : {}),
    ...(sizeText != null ? { sizeText } : {}),
    status,
    requesterId: String(raw.requester_id ?? raw.requesterId ?? ''),
    requesterName: String(raw.requester_name ?? raw.requesterName ?? ''),
    creatorId: normalizeOptionalUserId(raw.creator_id ?? raw.creatorId),
    creatorName: (() => {
      const v = raw.creator_name ?? raw.creatorName
      if (v == null || String(v).trim() === '') return null
      return String(v).trim()
    })(),
    designerId: (() => {
      const d = normalizeOptionalUserId(raw.designer_id ?? raw.designerId)
      if (d != null) return d
      return normalizeOptionalUserId(raw.assignee_id ?? raw.assigneeId)
    })(),
    designerName: (() => {
      const dn = raw.designer_name ?? raw.designerName
      if (dn != null && String(dn).trim() !== '') return String(dn).trim()
      const an = raw.assignee_name ?? raw.assigneeName
      if (an != null && String(an).trim() !== '') return String(an).trim()
      return null
    })(),
    currentHandlerId: normalizeOptionalUserId(raw.current_handler_id ?? raw.currentHandlerId),
    currentHandlerName: (() => {
      const v = raw.current_handler_name ?? raw.currentHandlerName
      if (v == null || String(v).trim() === '') return null
      return String(v).trim()
    })(),
    assigneeId: (() => {
      const d = normalizeOptionalUserId(raw.designer_id ?? raw.designerId)
      if (d != null) return d
      return normalizeOptionalUserId(raw.assignee_id ?? raw.assigneeId)
    })(),
    assigneeName: (() => {
      const dn = raw.designer_name ?? raw.designerName
      if (dn != null && String(dn).trim() !== '') return String(dn).trim()
      const an = raw.assignee_name ?? raw.assigneeName
      if (an != null && String(an).trim() !== '') return String(an).trim()
      return null
    })(),
    groupId: String(raw.group_id ?? raw.groupId ?? ''),
    groupName: String(raw.group_name ?? raw.owner_team ?? raw.groupName ?? ''),
    ownerDepartment:
      typeof (raw.owner_department ?? raw.ownerDepartment) === 'string' &&
      String(raw.owner_department ?? raw.ownerDepartment).trim() !== ''
        ? String(raw.owner_department ?? raw.ownerDepartment).trim()
        : undefined,
    ownerOrgTeam:
      typeof (raw.owner_org_team ?? raw.ownerOrgTeam) === 'string' &&
      String(raw.owner_org_team ?? raw.ownerOrgTeam).trim() !== ''
        ? String(raw.owner_org_team ?? raw.ownerOrgTeam).trim()
        : undefined,
    workflowLane:
      typeof (raw.workflow_lane ?? raw.workflowLane) === 'string' &&
      String(raw.workflow_lane ?? raw.workflowLane).trim() !== ''
        ? (String(raw.workflow_lane ?? raw.workflowLane).trim() as Task['workflowLane'])
        : undefined,
    businessLane: normalizeTaskLane(raw.business_lane ?? raw.businessLane),
    sourceDepartment:
      typeof (raw.source_department ?? raw.sourceDepartment) === 'string' &&
      String(raw.source_department ?? raw.sourceDepartment).trim() !== ''
        ? String(raw.source_department ?? raw.sourceDepartment).trim()
        : undefined,
    // GET /v1/tasks/{id} 顶层 `reference_file_refs`：统一按规范引用对象/字符串解析，可来自 canonical session 或 pre-task fallback
    referenceFileRefs: parseReferenceFileRefs(raw.reference_file_refs ?? raw.referenceFileRefs),
    // 原品开发类型任务提交时写入 `change_request`（见 customizationApi / stores 提交逻辑），
    // 查询返回同样走此字段；设计工作台 / 任务详情统一读取 `designRequirement` 一个位置，
    // 这里按 design_requirement → designRequirement → change_request → changeRequest 顺序回退，
    // 保证原品/新品两路数据都能映射到同一个展示字段。
    // 使用 || 而非 ?? —— 后端可能同时返回 design_requirement="" 和 change_request="有值"，
    // ?? 不穿透空字符串，导致读到空值。
    designRequirement:
      (raw.design_requirement as string | undefined)?.trim() ||
      (raw.designRequirement as string | undefined)?.trim() ||
      (raw.change_request as string | undefined)?.trim() ||
      (raw.changeRequest as string | undefined)?.trim() ||
      undefined,
    copyContent: (raw.copy_content ?? raw.copyContent) as string | undefined ?? undefined,
    styleKeywords: (raw.style_keywords ?? raw.styleKeywords) as string | undefined ?? undefined,
    // 同理：后端可能 note="" 而 remark="有值"（创建时 note 以 remark 提交）
    note:
      (raw.note as string | undefined)?.trim() ||
      (raw.remark as string | undefined)?.trim() ||
      undefined,
    dueAt: (raw.due_at ?? raw.deadline_at ?? raw.dueAt) as string | null ?? null,
    priority: normalizePriorityFromApi(raw.priority as string | undefined),
    /** 持久化外协意图标志；与 customization job 是否存在无必然关系 */
    needOutsource: Boolean(raw.need_outsource ?? raw.needOutsource ?? false),
    customizationRequired:
      typeof (raw.customization_required ?? raw.customizationRequired) === 'boolean'
        ? Boolean(raw.customization_required ?? raw.customizationRequired)
        : undefined,
    customizationSourceType:
      raw.customization_source_type != null || raw.customizationSourceType != null
        ? String(raw.customization_source_type ?? raw.customizationSourceType)
        : null,
    lastCustomizationOperatorId: normalizeOptionalUserId(
      raw.last_customization_operator_id ?? raw.lastCustomizationOperatorId,
    ),
    warehouseRejectReason:
      raw.warehouse_reject_reason != null || raw.warehouseRejectReason != null
        ? String(raw.warehouse_reject_reason ?? raw.warehouseRejectReason)
        : null,
    warehouseRejectCategory:
      raw.warehouse_reject_category != null || raw.warehouseRejectCategory != null
        ? String(raw.warehouse_reject_category ?? raw.warehouseRejectCategory)
        : null,
    assetVersions: normalizeAssetVersionsFromTaskRaw(raw),
    createdAt: String(raw.created_at ?? raw.createdAt ?? now),
    updatedAt: String(raw.updated_at ?? raw.updatedAt ?? now),
    // Step 87：建档/ERP 同步字段（空值保护，兼容老数据）
    filing_status: (raw.filing_status ?? raw.filingStatus) as string | undefined,
    filing_error_message: (raw.filing_error_message ?? raw.filingErrorMessage) as string | undefined,
    missing_fields: Array.isArray(raw.missing_fields ?? raw.missingFields) ? (raw.missing_fields ?? raw.missingFields) as string[] : undefined,
    missing_fields_summary_cn: (raw.missing_fields_summary_cn ?? raw.missingFieldsSummaryCn) as string | undefined,
    last_filed_at: (raw.last_filed_at ?? raw.lastFiledAt) as string | null | undefined,
    erp_sync_required: (raw.erp_sync_required ?? raw.erpSyncRequired) as boolean | undefined,
    filing_trigger_source: (raw.filing_trigger_source ?? raw.filingTriggerSource) as string | undefined,
    last_filing_attempt_at: (raw.last_filing_attempt_at ?? raw.lastFilingAttemptAt) as string | null | undefined,
    erp_sync_version: (raw.erp_sync_version ?? raw.erpSyncVersion) as number | undefined,
    isBatchTask:
      typeof (raw.is_batch_task ?? raw.isBatchTask) === 'boolean'
        ? Boolean(raw.is_batch_task ?? raw.isBatchTask)
        : undefined,
    batchItemCount:
      typeof (raw.batch_item_count ?? raw.batchItemCount) === 'number'
        ? Number(raw.batch_item_count ?? raw.batchItemCount)
        : undefined,
    batchMode:
      typeof (raw.batch_mode ?? raw.batchMode) === 'string'
        ? String(raw.batch_mode ?? raw.batchMode)
        : undefined,
	    primarySkuCode:
	      typeof (raw.primary_sku_code ?? raw.primarySkuCode) === 'string'
	        ? String(raw.primary_sku_code ?? raw.primarySkuCode)
	        : undefined,
	    skuCodeType:
	      typeof (raw.sku_code_type ?? raw.skuCodeType) === 'string'
	        ? String(raw.sku_code_type ?? raw.skuCodeType)
	        : undefined,
    skuGenerationStatus:
      typeof (raw.sku_generation_status ?? raw.skuGenerationStatus) === 'string'
        ? String(raw.sku_generation_status ?? raw.skuGenerationStatus)
        : undefined,
    ...(skuItems != null ? { skuItems } : {}),
    costPrice,
    purchaseInfo,
    basePriceAmount,
    costPriceMode,
    ...(procurementSummary != null ? { procurementSummary } : {}),
    ...(costOverrideSummary != null ? { costOverrideSummary } : {}),
    ...(governanceAuditSummary != null ? { governanceAuditSummary } : {}),
    ...(costOverrideBoundary != null ? { costOverrideBoundary } : {}),
    ...(designSubStatusFromApi != null ? { designSubStatus: designSubStatusFromApi } : {}),
    ...parseWarehouseFieldsFromApi(raw),
    ...(canPrepareWarehouse !== undefined ? { canPrepareWarehouse } : {}),
    ...(warehouseBlockingReasons != null ? { warehouseBlockingReasons } : {}),
    ...(warehousePrepareReady !== undefined ? { warehousePrepareReady } : {}),
    ...(warehouseReceiveReady !== undefined ? { warehouseReceiveReady } : {}),
    ...(procurementApiStatus != null ? { procurementApiStatus } : {}),
    ...parseWorkflowCloseFields(raw),
  }
}

export interface TaskEvent {
  event_id: string
  sequence: number
  type: string
  payload: Record<string, unknown>
}

function generateActionId(): string {
  // crypto.randomUUID() 是 Web 标准 API，Vite/现代浏览器均支持，满足 UUID v4 幂等性要求
  return typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`
}

/**
 * 详情 GET 未带 design_sub_status 时合并列表缓存：若扁平 task_status 已离开「设计中/打回」，
 * 勿沿用列表里滞后的 IN_PROGRESS，否则 canSubmitAudit 与后端不一致、交付上传后会误调 submit-design（409）。
 */
function mergeDesignSubStatusOnLoad(parsed: Task, existing: Task | null): DesignSubStatus | undefined {
  if (parsed.designSubStatus != null) return parsed.designSubStatus
  const ex = existing?.designSubStatus
  if (ex == null) return undefined
  const ps = parsed.status
  const keepListSubStatus =
    ps === 'InProgress' ||
    ps === 'RejectedByAuditA' ||
    ps === 'RejectedByAuditB' ||
    ps === 'PendingAssign'
  if (keepListSubStatus) return ex
  if (
    ex === DesignSubStatusEnum.IN_PROGRESS ||
    ex === DesignSubStatusEnum.PENDING_ASSIGN ||
    ex === DesignSubStatusEnum.REJECTED
  ) {
    return undefined
  }
  return ex
}

function nonEmptyTrimmed(s: string | null | undefined): boolean {
  return s != null && String(s).trim() !== ''
}

/**
 * 整表拉列表（GET /v1/tasks）后，列表行常为瘦模型；若内存中已有同 id 的详情 GET 结果，合并保留
 * reference_file_refs / sku_items / asset_versions / 负责人 / 发起人等，避免覆盖成详情「空白态」。
 */
function mergeListRowWithCachedDetail(prev: Task | undefined, listRow: Task): Task {
  if (!prev || prev.id !== listRow.id) return listRow

  const base: Task = {
    ...listRow,
    designerId: listRow.designerId ?? prev.designerId ?? null,
    designerName: listRow.designerName ?? prev.designerName ?? null,
    creatorId: listRow.creatorId ?? prev.creatorId ?? null,
    creatorName: listRow.creatorName ?? prev.creatorName ?? null,
    // currentHandlerId/Name 不回落 prev：列表响应是该字段的 source of truth，
    // 后端此前未下发或明确为 null 时，必须覆盖掉 prev 里可能残留的脏值
    // （例如 TaskCreate 本地回写、或早期详情接口返回的过期 handler）。
    // 若 ?? 回落到 prev，会出现「后端说无人领取、前端仍显示被设计师领」的错位，
    // 导致 AuditQueuePanel pending tab 用 currentHandlerId === '' 判断时漏掉任务。
    currentHandlerId: listRow.currentHandlerId ?? null,
    currentHandlerName: listRow.currentHandlerName ?? null,
    assigneeId: listRow.assigneeId ?? prev.assigneeId ?? null,
    assigneeName: listRow.assigneeName ?? prev.assigneeName ?? null,
    requesterId: nonEmptyTrimmed(listRow.requesterId)
      ? listRow.requesterId
      : nonEmptyTrimmed(prev.requesterId)
        ? prev.requesterId
        : listRow.requesterId,
    requesterName: nonEmptyTrimmed(listRow.requesterName)
      ? listRow.requesterName
      : nonEmptyTrimmed(prev.requesterName)
        ? prev.requesterName
        : listRow.requesterName,
    designSubStatus: mergeDesignSubStatusOnLoad(listRow, prev),
    warehouseReceiveStatus: listRow.warehouseReceiveStatus ?? prev.warehouseReceiveStatus,
    warehouseSubStatus: listRow.warehouseSubStatus ?? prev.warehouseSubStatus,
    canPrepareWarehouse: listRow.canPrepareWarehouse ?? prev.canPrepareWarehouse,
    warehouseBlockingReasons: listRow.warehouseBlockingReasons ?? prev.warehouseBlockingReasons,
    warehousePrepareReady: listRow.warehousePrepareReady ?? prev.warehousePrepareReady,
    warehouseReceiveReady: listRow.warehouseReceiveReady ?? prev.warehouseReceiveReady,
    procurementApiStatus: listRow.procurementApiStatus ?? prev.procurementApiStatus,
    workflowCanClose:
      listRow.workflowCanClose !== undefined ? listRow.workflowCanClose : prev.workflowCanClose,
    cannotCloseReasons:
      listRow.cannotCloseReasons !== undefined ? listRow.cannotCloseReasons : prev.cannotCloseReasons,
    workflowMainStatus:
      listRow.workflowMainStatus !== undefined ? listRow.workflowMainStatus : prev.workflowMainStatus,
    businessLane: listRow.businessLane ?? prev.businessLane,
  }

  const strEmpty = (s: string | null | undefined) => s == null || String(s).trim() === ''
  if (strEmpty(listRow.designRequirement) && !strEmpty(prev.designRequirement)) {
    base.designRequirement = prev.designRequirement
  }
  if (strEmpty(listRow.note) && !strEmpty(prev.note)) {
    base.note = prev.note
  }
  if (strEmpty(listRow.copyContent) && !strEmpty(prev.copyContent)) {
    base.copyContent = prev.copyContent
  }
  if (strEmpty(listRow.styleKeywords) && !strEmpty(prev.styleKeywords)) {
    base.styleKeywords = prev.styleKeywords
  }
  if (strEmpty(listRow.specText) && !strEmpty(prev.specText)) {
    base.specText = prev.specText
  }
  if (strEmpty(listRow.sizeText) && !strEmpty(prev.sizeText)) {
    base.sizeText = prev.sizeText
  }
  if (strEmpty(listRow.newProductCategoryCode) && !strEmpty(prev.newProductCategoryCode)) {
    base.newProductCategoryCode = prev.newProductCategoryCode
  }
  if (strEmpty(listRow.erpCategoryCode) && !strEmpty(prev.erpCategoryCode)) {
    base.erpCategoryCode = prev.erpCategoryCode
  }
  if (strEmpty(listRow.erpCategoryName) && !strEmpty(prev.erpCategoryName)) {
    base.erpCategoryName = prev.erpCategoryName
  }
  if (strEmpty(listRow.productShortName) && !strEmpty(prev.productShortName)) {
    base.productShortName = prev.productShortName
  }
  if (strEmpty(listRow.productReferenceUrl) && !strEmpty(prev.productReferenceUrl)) {
    base.productReferenceUrl = prev.productReferenceUrl
  }

  const listRefs = listRow.referenceFileRefs
  const prevRefs = prev.referenceFileRefs
  if (
    Array.isArray(prevRefs) &&
    prevRefs.length > 0 &&
    (!Array.isArray(listRefs) || listRefs.length === 0)
  ) {
    base.referenceFileRefs = prevRefs
  }

  const listSku = listRow.skuItems
  const prevSku = prev.skuItems
  if (
    Array.isArray(prevSku) &&
    prevSku.length > 0 &&
    (!Array.isArray(listSku) || listSku.length === 0)
  ) {
    base.skuItems = prevSku
    if (prev.isBatchTask === true) {
      base.isBatchTask = true
      if (prev.batchItemCount != null) base.batchItemCount = prev.batchItemCount
      if (prev.batchMode != null) base.batchMode = prev.batchMode
      if (prev.primarySkuCode != null) base.primarySkuCode = prev.primarySkuCode
      if (prev.skuGenerationStatus != null) base.skuGenerationStatus = prev.skuGenerationStatus
    }
  }

  const listAv = listRow.assetVersions
  const prevAv = prev.assetVersions
  if (
    listRow.isBatchTask !== true &&
    Array.isArray(prevAv) &&
    prevAv.length > 0 &&
    (!Array.isArray(listAv) || listAv.length === 0)
  ) {
    base.assetVersions = prevAv
  }

  return enrichTaskDomainFields(base)
}

/** 创建任务可选材质：自由文本以 material_mode=other + material_other 提交（兼容旧版 OTHER+materialOther）。 */
function optionalMaterialSnakeFields(materialRaw: unknown, materialOtherRaw?: unknown): Record<string, unknown> {
  const trim = (v: unknown) => (v == null ? '' : String(v).trim())
  const main = trim(materialRaw)
  const other = trim(materialOtherRaw)
  if (main === 'OTHER') {
    if (other) return { material_mode: 'other', material_other: other }
    return {}
  }
  if (main) return { material_mode: 'other', material_other: main }
  return {}
}

function normalizeRefUrl(raw: unknown): string | undefined {
  if (typeof raw !== 'string') return undefined
  const t = raw.trim()
  if (!t) return undefined
  return toRelativeAssetUrl(t) ?? t
}

function sanitizeReferenceFileRefObject(raw: unknown): Record<string, unknown> | null {
  if (!raw || typeof raw !== 'object') return null
  const obj = raw as Record<string, unknown>
  const next: Record<string, unknown> = {}
  const copy = (k: string) => {
    if (obj[k] !== undefined) next[k] = obj[k]
  }
  copy('asset_id')
  copy('ref_id')
  copy('upload_request_id')
  copy('filename')
  copy('mime_type')
  copy('file_size')
  copy('source')
  copy('status')
  copy('storage_key')

  const downloadUrl = normalizeRefUrl(obj.download_url) ?? normalizeRefUrl(obj.url)
  if (downloadUrl) next.download_url = downloadUrl

  return Object.keys(next).length ? next : null
}

/**
 * PERSISTENCE GUARD: reference_file_refs[].download_url 是 15 分钟有效的 OSS presigned URL。
 * 如果未来接入 pinia-plugin-persistedstate，必须将 referenceFileRefs（以及 skuItems[].referenceFileRefs）
 * 从序列化白名单中排除，或在反序列化时清空 download_url，否则刷新后恢复的 URL 必然过期。
 */
export const useTasksStore = defineStore('tasks', () => {
  // 初始为空，通过 loadTasks() 从 service 层填充，解耦 store 与 mock 数据
  const items = ref<Task[]>([])
  const loading = ref(false)
  const loadError = ref<string | null>(null)
  const lastSequence = ref(0)
  const appliedEventIds = ref<Set<string>>(new Set())
  /**
   * 已初始化标志。
   * SPA 内切换角色（logout → login）不会销毁 Pinia store，
   * initialized 为 true 时 loadTasks 直接返回，保留当前内存状态，
   * 避免重新拉取 mock 导致覆盖之前角色的操作结果。
   */
  const initialized = ref(false)
  let queuedForceRefreshParams: TaskListParams | undefined

  const list = computed(() => items.value)
  /** 非 append 的列表拉取成功并整表替换 items 后递增；单条 loadTaskById 不递增。供设计工作台在整表刷新后重建「待设计」快照 */
  const fullListReplaceGeneration = ref(0)
  /** 服务端分页：当前查询条件下的总条数（来自 pagination.total） */
  const listTotal = ref(0)
  const getById = (id: string) => items.value.find((t) => t.id === id)

  const mainStatusOf = (id: string) => getById(id)?.mainStatus
  const designStatusOf = (id: string) => getById(id)?.designSubStatus
  const auditStatusOf = (id: string) => getById(id)?.auditSubStatus
  const warehouseStatusOf = (id: string) => getById(id)?.warehouseSubStatus
  const purchaseStatusOf = (id: string) => getById(id)?.purchaseSubStatus

  /** 方案 B：服务端分页 + 搜索。拉取任务列表（append 逻辑在 fetchAndApplyTaskList）
   * 后端 TaskListResponse: { data: [...], pagination: { total, page, page_size } } */
  async function loadTaskList(params: TaskListParams = {}): Promise<{ items: Task[]; total: number }> {
    const res = await tasksApi.list(params)
    const data = res?.data
    const body = (typeof data === 'object' && data !== null) ? data : {}
    const rawItems = Array.isArray(body.data)
      ? body.data
      : (Array.isArray(body) ? body : (body?.items ?? body?.tasks ?? []))
    const pagination = body.pagination as { total?: number; page?: number; page_size?: number } | undefined
    const total =
      pagination != null && typeof pagination.total === 'number' ? pagination.total : rawItems.length
    try {
      const tasks = rawItems.map((raw: Record<string, unknown>) =>
        enrichTaskDomainFields(normalizeBackendTask(raw)),
      )
      return { items: tasks, total }
    } catch (error) {
      throw new Error(error instanceof Error ? error.message : '加载任务列表失败')
    }
  }

  /** 拉取并更新 store（供 TaskListView 调用，支持服务端分页 + 搜索 + 加载更多）
   * append 时不设 loading，避免滚动加载更多时整页闪白 */
  async function fetchAndApplyTaskList(
    params: TaskListParams,
    options?: { append?: boolean },
  ) {
    const isAppend = options?.append === true
    if (loading.value && !isAppend) return
    if (!isAppend) loading.value = true
    loadError.value = null
    try {
      const { items: tasks, total } = await loadTaskList(params)
      listTotal.value = total
      if (isAppend) {
        const ids = new Set(items.value.map((t) => t.id))
        const newOnes = tasks.filter((t) => !ids.has(t.id))
        items.value = [...items.value, ...newOnes]
      } else {
        const prevById = new Map(items.value.map((t) => [t.id, t]))
        items.value = tasks.map((t) => mergeListRowWithCachedDetail(prevById.get(t.id), t))
        fullListReplaceGeneration.value += 1
      }
      initialized.value = true
    } catch (e) {
      loadError.value = e instanceof Error ? e.message : '加载任务列表失败'
      throw e
    } finally {
      if (!isAppend) {
        loading.value = false
        // 无论本次加载由谁触发，只要期间有 forceRefresh 排队，都在此统一补跑，避免刷新请求被吞掉。
        if (queuedForceRefreshParams) {
          const queued = queuedForceRefreshParams
          queuedForceRefreshParams = undefined
          await fetchAndApplyTaskList(queued)
        }
      }
    }
  }

  async function loadTasks() {
    // 方式 B：已初始化则直接复用内存状态，不重新覆盖（跨角色切换核心保障）
    if (initialized.value && items.value.length > 0) return
    await fetchAndApplyTaskList({ page: 1, page_size: 500 })
  }

  /** v0.6 对齐：FRONTEND_ALIGNMENT_v0.5(1).md E 节 + 2026-03-18
   * 强制重新拉取。若传入 params 则按 params 拉取；否则按 page=1, page_size=500 拉取 */
  async function forceRefreshList(params?: TaskListParams) {
    const p = params ?? { page: 1, page_size: 500 }
    // 避免刷新请求在列表加载中被直接吞掉：先排队，等待当前加载结束统一补跑。
    if (loading.value) {
      queuedForceRefreshParams = p
      return
    }
    await fetchAndApplyTaskList(p)
  }

  /** 方案 B：任务中心专用，支持服务端分页 + 搜索 + 加载更多 */
  async function loadTaskListForView(
    params: TaskListParams,
    options?: { append?: boolean },
  ) {
    await fetchAndApplyTaskList(params, options)
  }

  /**
   * 按 id 拉取任务聚合详情并更新本地列表。
   * V1 详情页必须优先使用 GET /v1/tasks/{id}/detail，避免并发拼旧子接口。
   */
  async function loadTaskById(id: string) {
    try {
      const res = await tasksApi.getDetail(id)
      const envelope = res?.data?.data ?? res?.data ?? res
      const raw: Record<string, unknown> =
        envelope && typeof envelope === 'object' && 'task' in (envelope as Record<string, unknown>)
          ? mergeDetailEnvelopeIntoTaskRaw(envelope as Record<string, unknown>)
          : (envelope as Record<string, unknown>)
      let richEnvelope: Record<string, unknown> | undefined
      try {
        const richRes = await tasksApi.getById(id)
        const richData = richRes?.data?.data ?? richRes?.data ?? richRes
        if (richData && typeof richData === 'object') {
          richEnvelope = richData as Record<string, unknown>
          mergeReadModelCostGovernanceFields(raw, richEnvelope)
        }
      } catch {
        // 成本治理补充读模型失败不阻塞详情主流程，详情仍使用 /detail 主响应。
      }
      const moduleSummaries = parseModuleSummariesFromEnvelope(envelope)
      const parsed = normalizeBackendTask(raw)
      const needsSkuItemsBackfill =
        (parsed.isBatchTask === true || (parsed.batchItemCount ?? 0) > 0) &&
        (!Array.isArray(parsed.skuItems) || parsed.skuItems.length === 0)
      if (needsSkuItemsBackfill) {
        try {
          // /detail 未返回 sku_items 时，仅对子项做一次 rich detail 回填。
          if (richEnvelope && typeof richEnvelope === 'object') {
            const richParsed = normalizeBackendTask(richEnvelope as Record<string, unknown>)
            if (Array.isArray(richParsed.skuItems) && richParsed.skuItems.length > 0) {
              parsed.skuItems = richParsed.skuItems
            }
          }
        } catch {
          // 资产/子项补取失败不阻塞详情主流程
        }
      }
      const idx = items.value.findIndex((t) => t.id === id)
      const existing = idx !== -1 ? items.value[idx] : null
      // 详情若未带 designer_id / assignee_id，勿用 null 覆盖列表已有设计师，否则设计工作台左侧队列会误筛掉该行
      // design_sub_status 合并见 mergeDesignSubStatusOnLoad
      const base: Task = {
        ...parsed,
        designerId: parsed.designerId ?? existing?.designerId ?? null,
        designerName: parsed.designerName ?? existing?.designerName ?? null,
        creatorId: parsed.creatorId ?? existing?.creatorId ?? null,
        creatorName: parsed.creatorName ?? existing?.creatorName ?? null,
        // currentHandlerId/Name 不回落 existing：详情响应是该字段的 source of truth，
        // 与 mergeListRowWithCachedDetail 对齐；细节见该函数内同名注释。
        currentHandlerId: parsed.currentHandlerId ?? null,
        currentHandlerName: parsed.currentHandlerName ?? null,
        assigneeId: parsed.assigneeId ?? existing?.assigneeId ?? null,
        assigneeName: parsed.assigneeName ?? existing?.assigneeName ?? null,
        requesterId: nonEmptyTrimmed(parsed.requesterId)
          ? parsed.requesterId
          : nonEmptyTrimmed(existing?.requesterId)
            ? existing!.requesterId
            : parsed.requesterId,
        requesterName: nonEmptyTrimmed(parsed.requesterName)
          ? parsed.requesterName
          : nonEmptyTrimmed(existing?.requesterName)
            ? existing!.requesterName
            : parsed.requesterName,
        designSubStatus: mergeDesignSubStatusOnLoad(parsed, existing),
        warehouseReceiveStatus: parsed.warehouseReceiveStatus ?? existing?.warehouseReceiveStatus,
        warehouseSubStatus: parsed.warehouseSubStatus ?? existing?.warehouseSubStatus,
        canPrepareWarehouse: parsed.canPrepareWarehouse ?? existing?.canPrepareWarehouse,
        warehouseBlockingReasons: parsed.warehouseBlockingReasons ?? existing?.warehouseBlockingReasons,
        warehousePrepareReady: parsed.warehousePrepareReady ?? existing?.warehousePrepareReady,
        warehouseReceiveReady: parsed.warehouseReceiveReady ?? existing?.warehouseReceiveReady,
        procurementApiStatus: parsed.procurementApiStatus ?? existing?.procurementApiStatus,
        assetVersions: parsed.assetVersions,
        workflowCanClose:
          parsed.workflowCanClose !== undefined
            ? parsed.workflowCanClose
            : existing?.workflowCanClose,
        cannotCloseReasons:
          parsed.cannotCloseReasons !== undefined
            ? parsed.cannotCloseReasons
            : existing?.cannotCloseReasons,
        workflowMainStatus:
          parsed.workflowMainStatus !== undefined
            ? parsed.workflowMainStatus
            : existing?.workflowMainStatus,
        // `GET /v1/tasks/{id}` 详情响应不保证回传 workflow_lane；
        // 若详情缺字段，必须回落到列表缓存的 lane，否则审核工作台按 lane 过滤
        // （AuditQueuePanel.matchesLane）会在详情刷入后把 `undefined` 当作
        // `'normal'` 处理，导致已选中的定制任务从定制 Tab 中瞬间消失。
        workflowLane: parsed.workflowLane ?? existing?.workflowLane,
        businessLane: parsed.businessLane ?? existing?.businessLane,
        moduleSummaries: moduleSummaries ?? existing?.moduleSummaries,
      }
      const updated = enrichTaskDomainFields(base)
      if (idx !== -1) items.value[idx] = updated
      else items.value.push(updated)
    } catch {
      // 静默失败，由调用方按需处理
      throw new Error('刷新任务失败')
    }
  }

  /**
   * 刷新单个任务的 reference_file_refs presigned URLs。
   * 幂等 + 去重：同一个 taskId 在 500ms 冷却期内多次调用会合并为一次请求，
   * 避免批量 <img onerror> 打爆接口。
   */
  const _refreshPending = new Map<string, Promise<void>>()

  async function refreshReferenceUrls(taskId: string): Promise<void> {
    const existing = _refreshPending.get(taskId)
    if (existing) return existing
    const p = (async () => {
      try {
        await loadTaskById(taskId)
      } finally {
        setTimeout(() => _refreshPending.delete(taskId), 500)
      }
    })()
    _refreshPending.set(taskId, p)
    return p
  }

  /** 前端 TaskKind -> 后端 task_type（后端期望 snake_case） */
  const TASK_TYPE_TO_BACKEND: Record<string, string> = {
    ORIGINAL_PRODUCT_DEV: 'original_product_development',
    NEW_PRODUCT_DEV: 'new_product_development',
    PURCHASE_TASK: 'purchase_task',
    RETOUCH_TASK: 'retouch_task',
    CUSTOMER_CUSTOMIZATION: 'customer_customization',
    REGULAR_CUSTOMIZATION: 'regular_customization',
  }

  /** 将前端 Task 转为后端 POST /v1/tasks 请求体（snake_case）
   * v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 B 节
   * 按 task_type 过滤：仅 original_product_development 允许 product_id/sku/product_selection
   * source_mode 由后端根据 task_type 自动推断，前端不再传递
   */
  function buildCreatePayload(task: Partial<Task>): Record<string, unknown> {
    const t = task as Record<string, unknown>
    const frontendTaskType = (t.taskType ?? task.taskType ?? 'ORIGINAL_PRODUCT_DEV') as string
    const taskType = TASK_TYPE_TO_BACKEND[frontendTaskType] ?? frontendTaskType
    const isOriginal = frontendTaskType === 'ORIGINAL_PRODUCT_DEV'
    const isRetouch = frontendTaskType === 'RETOUCH_TASK'
    const businessLane =
      normalizeTaskLane(t.businessLane ?? t.workflowLane) ??
      (Boolean(t.customizationRequired ?? task.customizationRequired) ? 'customization' : 'normal')
    const normalizedLaneSkuCodeType = businessLane === 'customization' ? 'customization' : 'regular'
    const skuModeRaw = (t.skuMode ?? 'single') as string
	    const isBatchMode = skuModeRaw === 'multiple' && !isOriginal && !isRetouch
	    const skuCodeType = normalizedLaneSkuCodeType

    const ownerTeam = t.groupId ?? task.groupId ?? ''
    const ownerDepartment =
      t.ownerDepartment ??
      task.ownerDepartment ??
      (typeof t.departmentId === 'string' ? t.departmentId : undefined) ??
      ''
    const ownerOrgTeam = t.ownerOrgTeam ?? task.ownerOrgTeam ?? ownerTeam
    const designerIdRaw = t.designerId ?? t.assigneeId ?? task.designerId ?? task.assigneeId
    const designerId =
      designerIdRaw != null && String(designerIdRaw).trim() !== ''
        ? parseInt(String(designerIdRaw), 10)
        : null
    const priorityRaw = t.priority ?? task.priority ?? 'normal'
    const priority = normalizePriorityForApi(String(priorityRaw))

    // 后端要求：只传 reference_file_refs，不再传 reference_images
    const refs = (t.referenceFileRefs ?? task.referenceFileRefs ?? []) as unknown[]
    const referenceFileRefs: Record<string, unknown>[] = []
    for (const r of refs) {
      const cleaned = sanitizeReferenceFileRefObject(r)
      if (cleaned) referenceFileRefs.push(cleaned)
      // base64 字符串不再传入；参考图统一先拿 reference_file_refs，再随创建请求提交
    }

    // 后端 Go 要求 requester_id 为 int64，不能传字符串
    const requesterIdRaw = t.requesterId ?? task.requesterId ?? ''
    const requesterIdNum =
      requesterIdRaw !== '' && requesterIdRaw !== 'anonymous'
        ? parseInt(String(requesterIdRaw), 10)
        : null

    const payload: Record<string, unknown> = {
      task_type: taskType,
      designer_id: Number.isNaN(designerId as number) ? null : designerId,
      requester_id: requesterIdNum != null && !Number.isNaN(requesterIdNum) ? requesterIdNum : null,
      requester_name: t.requesterName ?? task.requesterName ?? '',
      owner_department: ownerDepartment || undefined,
      owner_org_team: ownerOrgTeam || undefined,
      // Legacy compatibility: keep owner_team for old backend branches.
      owner_team: ownerTeam,
      deadline_at: t.dueAt ?? task.dueAt ?? null,
	      priority,
	      business_lane: businessLane,
	      workflow_lane: businessLane,
	      sku_code_type: skuCodeType,
	      customization_required:
          businessLane === 'customization' ||
          Boolean(t.customizationRequired ?? task.customizationRequired ?? false),
      customization_source_type:
        (t.customizationRequired ?? task.customizationRequired)
          ? (t.customizationSourceType ?? task.customizationSourceType ?? undefined)
          : undefined,
      // 兼容字段保留，不再由前端新逻辑驱动。
      is_outsource: false,
      reference_file_refs: referenceFileRefs,
      // 原品用 change_request；采购任务 POST 白名单不允许 design_requirement（见后端 INVALID_REQUEST）
      ...(isOriginal || isRetouch || taskType === 'purchase_task' || isBatchMode
        ? {}
        : { design_requirement: t.designRequirement ?? task.designRequirement ?? undefined }),
      copy_content: t.copyContent ?? task.copyContent ?? undefined,
      style_keywords: t.styleKeywords ?? task.styleKeywords ?? undefined,
      remark: t.note ?? task.note ?? undefined,
    }
    if (taskType === 'new_product_development' || taskType === 'purchase_task') {
      payload.sync_erp_on_create = t.syncErpOnCreate !== false
    }
    const sourceDraftId = t.sourceDraftId ?? t.draftId ?? (task as Record<string, unknown>).sourceDraftId ?? (task as Record<string, unknown>).draftId
    if (sourceDraftId != null && String(sourceDraftId).trim() !== '') {
      payload.source_draft_id = String(sourceDraftId).trim()
    }
    const specText = typeof t.prefillSpecText === 'string'
      ? t.prefillSpecText.trim()
      : typeof (task as Record<string, unknown>).prefillSpecText === 'string'
        ? String((task as Record<string, unknown>).prefillSpecText).trim()
        : ''
    if (specText) payload.spec_text = specText

    // 仅 original_product_development 需要携带 ERP 已有 sku_code
    const skuValue = t.sku ?? task.sku ?? null
    const productName = t.productName ?? task.productName ?? ''
    const productNameSnapshot = productName || (t.product_name_snapshot as string) || ''
    const topCategoryCode =
      (typeof t.category === 'string' && t.category.trim() !== ''
        ? t.category.trim()
        : undefined) ??
      (typeof t.productCategoryCode === 'string' && t.productCategoryCode.trim() !== ''
        ? t.productCategoryCode.trim()
        : undefined)

    if (isRetouch) {
      const retouchRequirement = t.designRequirement ?? task.designRequirement ?? ''
      payload.product_name = productName || '修图任务名称'
      payload.product_name_snapshot = productNameSnapshot || '修图任务名称'
      payload.demand_text = retouchRequirement
      payload.design_requirement = retouchRequirement
    } else if (isOriginal) {
      const productIdRaw = t.productId ?? task.productId
      const productIdNum =
        productIdRaw != null && String(productIdRaw).trim() !== ''
          ? parseInt(String(productIdRaw), 10)
          : null
      const productIdValid = productIdNum != null && !Number.isNaN(productIdNum)
      payload.product_id = productIdValid ? productIdNum : null
      payload.product_name = productName
      payload.product_name_snapshot = productNameSnapshot
      payload.change_request = t.designRequirement ?? task.designRequirement ?? ''
      payload.sku_code = skuValue
      const erpSnapshot = t.erpProductSnapshot ?? (task as Record<string, unknown>).erpProductSnapshot
      if (productIdValid) {
        payload.product_selection = { selected_product_id: productIdNum }
      } else if (erpSnapshot && typeof erpSnapshot === 'object') {
        // ERP 返回 product_id 为 SKU 字符串时，传 erp_product 供后端解析绑定
        payload.defer_local_product_binding = true
        payload.product_selection = {
          defer_local_product_binding: true,
          erp_product: erpSnapshot,
        }
      }
      Object.assign(payload, optionalMaterialSnakeFields(t.material, t.materialOther))
    } else if (!isBatchMode) {
      payload.product_id = null
      payload.product_name = productName
      payload.product_name_snapshot = productNameSnapshot
      if (taskType === 'new_product_development') {
        payload.i_id = topCategoryCode ?? (task as Record<string, unknown>).category ?? undefined
        Object.assign(
          payload,
          optionalMaterialSnakeFields(
            t.material ?? (task as Record<string, unknown>).material,
            t.materialOther ?? (task as Record<string, unknown>).materialOther,
          ),
        )
      } else if (taskType === 'purchase_task') {
        payload.i_id = topCategoryCode
      }
      // 统一由后端自动编码：不再携带 purchase_sku / sku_code
    } else {
      // batch_sku_mode=multiple：
      // - 后端要求 batch_items.length >= 2（violation: insufficient_batch_items）
      // - new_product_development 当前仅需每行 product_name + design_requirement
      payload.product_id = null
      payload.batch_sku_mode = 'multiple'
      if (taskType === 'purchase_task' && topCategoryCode) payload.i_id = topCategoryCode
      const rawBatchItems = Array.isArray(t.batchItems) ? t.batchItems : []
      payload.batch_items = rawBatchItems.map((itemRaw) => {
	        const item = itemRaw as Record<string, unknown>
	        const baseItem: Record<string, unknown> = {
	          product_name: item.productName ?? '',
	          sku_code_type: normalizedLaneSkuCodeType,
	        }
        if (taskType === 'new_product_development') {
          baseItem.design_requirement = item.designRequirement ?? undefined
          if (item.productIId) baseItem.product_i_id = item.productIId
          const refs = item.referenceFileRefs as unknown[] | undefined
          if (Array.isArray(refs) && refs.length) {
            const cleaned = refs
              .map((r) => sanitizeReferenceFileRefObject(r))
              .filter((r): r is Record<string, unknown> => r != null)
            if (cleaned.length) baseItem.reference_file_refs = cleaned
          }
        } else if (taskType === 'purchase_task') {
          baseItem.i_id = item.categoryCode ?? topCategoryCode ?? undefined
          baseItem.cost_price_mode = item.costPriceMode ?? undefined
          baseItem.quantity = item.quantity ?? undefined
          baseItem.base_sale_price = item.baseSalePrice ?? undefined
          if (item.productChannel) baseItem.product_channel = item.productChannel
          if (item.costPriceMode === 'manual' && item.costPriceAmount != null && !Number.isNaN(item.costPriceAmount)) {
            baseItem.cost_price = item.costPriceAmount
          }
          const refs = item.referenceFileRefs as unknown[] | undefined
          if (Array.isArray(refs) && refs.length) {
            const cleaned = refs
              .map((r) => sanitizeReferenceFileRefObject(r))
              .filter((r): r is Record<string, unknown> => r != null)
            if (cleaned.length) baseItem.reference_file_refs = cleaned
          }
        }
        if (item.variantJson && typeof item.variantJson === 'object') {
          baseItem.variant_json = item.variantJson
        }
        return baseItem
      })
    }
    // 后端 POST：purchase_task 与 new_product_development 的 cost_price 均为 float64；发对象会 400
    if (task.costPrice != null && !isBatchMode) {
      payload.cost_price =
        taskType === 'purchase_task'
          ? task.costPrice.amount
          : { amount: task.costPrice.amount, currency: task.costPrice.currency }
    }
    if (task.purchaseInfo != null && !isBatchMode) {
      payload.purchase_info = {
        status: task.purchaseInfo.status,
        supplier_name: task.purchaseInfo.supplierName,
        quantity: task.purchaseInfo.quantity,
        unit: task.purchaseInfo.unit,
        purchase_price: task.purchaseInfo.purchasePrice,
        expected_arrival_at: task.purchaseInfo.expectedArrivalAt,
        warehouse_location_code: task.purchaseInfo.warehouseLocationCode,
        warehouse_location_name: task.purchaseInfo.warehouseLocationName,
      }
    }
    if (taskType === 'purchase_task' && !isBatchMode) {
      const purchasePayload = payload as Record<string, unknown>
      purchasePayload.cost_price_mode =
        t.costPriceMode ?? (task as Record<string, unknown>).costPriceMode ?? undefined
      purchasePayload.quantity =
        task.purchaseInfo?.quantity ?? (t.purchaseInfo as { quantity?: number })?.quantity
      purchasePayload.base_sale_price =
        t.basePriceAmount ?? (task as Record<string, unknown>).basePriceAmount ?? undefined
      Object.assign(purchasePayload, optionalMaterialSnakeFields(t.material, t.materialOther))
    }
    if (!isOriginal && 'product_selection' in payload) {
      delete payload.product_selection
    }
    return payload
  }

  async function addTask(task: Partial<Task>, _action_id?: string): Promise<Task> {
    const rawPayload = buildCreatePayload(task)
    // Defense in depth: 在进入网络层前再按后端字段白名单过滤一次，
    // 避免任何回归 UI 把 forbidden 字段送入 POST /v1/tasks。
    const payload = sanitizeCreateTaskPayload(rawPayload, String(rawPayload.task_type ?? ''))
    if (import.meta.env.DEV) {
      const stripped = Object.keys(rawPayload).filter((k) => !(k in payload))
      if (stripped.length > 0) {
        console.warn(
          '[POST /v1/tasks] sanitizer stripped forbidden fields for task_type =',
          payload.task_type,
          ':',
          stripped,
        )
      }
      console.log('[POST /v1/tasks] payload 类型:', Array.isArray(payload) ? '数组' : '对象')
      console.log('[POST /v1/tasks] payload 完整 JSON:', JSON.stringify(payload, null, 2))
      console.log('[POST /v1/tasks] payload snapshot:', {
        task_type: payload.task_type,
        product_selection: payload.product_selection,
        product_id: payload.product_id,
        sku_code: payload.sku_code,
        owner_department: payload.owner_department,
        owner_org_team: payload.owner_org_team,
        owner_team: payload.owner_team,
        deadline_at: payload.deadline_at,
        customization_required: payload.customization_required,
        customization_source_type: payload.customization_source_type,
      })
    }
    const res = await tasksApi.create(payload)
    const raw = res?.data?.data ?? res?.data ?? res
    const created = enrichTaskDomainFields(normalizeBackendTask(raw as Record<string, unknown>))
    items.value.push(created)
    try {
      await loadTaskById(created.id)
    } catch {
      // POST 已写入本地；GET 偶发失败时仍返回创建快照，详情页挂载会再拉
    }
    return getById(created.id) ?? created
  }

  /**
   * 可选预展示：向后端请求“将要使用”的 SKU 码，但不创建任务。
   * 后端负责去重/并发分配；前端不再自行拼接序号。
   */
  async function prepareProductCodes(task: Partial<Task>, signal?: AbortSignal): Promise<{
    skuCode?: string
    skuItems?: string[]
  }> {
    const payload = buildCreatePayload(task)
    const payloadTaskType = String(payload.task_type ?? '')
    if (payloadTaskType === 'original_product_development') {
      throw new Error('原品开发不走预编码接口')
    }

    const normalizeCode = (v: unknown): string => {
      if (typeof v !== 'string') return ''
      return v.trim()
    }

    const pickTopCategoryCode = (): string => {
      const t = task as Record<string, unknown>
      return (
        normalizeCode(t.category) ||
        normalizeCode(t.productCategoryCode) ||
        normalizeCode(payload.category_code)
      )
    }

    // prepare-product-codes 与 create-task 的字段白名单不同：
    // - 单个模式：必须有顶层 category_code（当前端点尚未切 i_id 字段名）
    // - 批量模式：每个 batch_items[i] 必须有 category_code
	    const preparePayload: Record<string, unknown> = { task_type: payloadTaskType }
	    const businessLane = normalizeTaskLane(payload.business_lane ?? payload.workflow_lane) ?? 'normal'
	    const skuCodeType = businessLane === 'customization' ? 'customization' : 'regular'
	    preparePayload.business_lane = businessLane
	    preparePayload.workflow_lane = businessLane
	    preparePayload.sku_code_type = skuCodeType
    const rawBatchItems = Array.isArray((task as Record<string, unknown>).batchItems)
      ? ((task as Record<string, unknown>).batchItems as Array<Record<string, unknown>>)
      : []

    if (rawBatchItems.length > 0) {
      const fallbackCategoryCode = pickTopCategoryCode()
      const batch_items = rawBatchItems.map((item, idx) => {
        const categoryCode =
          normalizeCode(item.categoryCode) ||
          normalizeCode(item.category_code) ||
          fallbackCategoryCode
        if (!categoryCode) {
          throw new Error(`批量第 ${idx + 1} 行缺少产品款式编码，无法预展示 SKU`)
        }
	        return {
	          category_code: categoryCode,
	          sku_code_type: skuCodeType,
	        }
      })
      preparePayload.batch_items = batch_items
    } else {
      const categoryCode = pickTopCategoryCode()
      if (!categoryCode) {
        throw new Error('请选择产品款式编码后再预展示 SKU')
      }
      preparePayload.category_code = categoryCode
      preparePayload.count = 1
    }

    const res = await tasksApi.prepareProductCodes(preparePayload, signal)
    const raw = res?.data?.data ?? res?.data ?? res
    const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>

    // Backend contract (openapi): data.codes[].sku_code, each item includes index/category_code/sku_code.
    const codesRaw = Array.isArray(data.codes) ? (data.codes as unknown[]) : []

    const codes = codesRaw
      .map((it) => {
        const o = it && typeof it === 'object' ? (it as Record<string, unknown>) : {}
        const indexRaw = o.index
        const index = typeof indexRaw === 'number' ? indexRaw : Number(indexRaw)
        const skuRaw = o.sku_code ?? o.skuCode
        const sku_code = typeof skuRaw === 'string' ? skuRaw.trim() : ''
        return { index: Number.isFinite(index) ? index : undefined, sku_code }
      })
      .filter((c) => c.sku_code !== '')

    if (codes.length === 0) return {}

    // Heuristic:
    // - if only 1 code -> treat as single
    // - otherwise -> treat as batch and keep stable order by index
    if (codes.length === 1) {
      return { skuCode: codes[0].sku_code }
    }

    const maxIndex = codes.reduce((m, c) => (c.index != null ? Math.max(m, c.index) : m), 0)
    const skuItems = Array.from({ length: maxIndex + 1 }, () => '')
    for (const c of codes) {
      if (c.index == null) continue
      skuItems[c.index] = c.sku_code
    }
    return { skuItems: skuItems.filter(Boolean) }
  }

  function updateTask(id: string, patch: Partial<Task>, _action_id?: string) {
    const i = items.value.findIndex((t) => t.id === id)
    if (i !== -1) {
      const merged: Task = { ...items.value[i], ...patch, updatedAt: nowISO() }
      const next = enrichTaskDomainFields(merged)
      items.value = items.value.map((t, idx) => (idx === i ? next : t))
    }
  }

  function applyEvent(event: TaskEvent): boolean {
    if (appliedEventIds.value.has(event.event_id)) return false
    const nextSeq = lastSequence.value + 1
    if (event.sequence < nextSeq) return false
    if (event.sequence > nextSeq) {
      const syncStore = useSyncStore()
      syncStore.setSequenceGap(true)
      return false
    }
    appliedEventIds.value = new Set([...appliedEventIds.value, event.event_id])
    lastSequence.value = event.sequence

    // 预留事件驱动更新入口：如 payload 中携带 taskId + patch，则应用到本地 Task
    const maybeTaskId = (event.payload.taskId ?? (event.payload as Record<string, unknown>)['task_id']) as
      | string
      | undefined
    const maybePatch = event.payload.patch as Partial<Task> | undefined
    if (maybeTaskId && maybePatch) {
      updateTask(maybeTaskId, maybePatch)
    }

    return true
  }

  async function assignTask(taskId: string, payload: { assigneeId: string; assigneeName: string }) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不走设计指派流程')
    if (!canAssign(task)) throw new Error('当前状态不可指派')
    const designerIdNum = parseInt(payload.assigneeId, 10)
    if (Number.isNaN(designerIdNum)) throw new Error('设计师 ID 无效，请从用户列表选择')
    const assignPayload: AssignTaskPayload = {
      designer_id: designerIdNum,
      designer_name: payload.assigneeName,
    }
    await tasksApi.assign(taskId, assignPayload)
    await loadTaskById(taskId)
  }

  /**
   * 重新指派设计师：与首次指派共用 `POST /v1/tasks/{id}/assign`。
   * 若详情模块已下发 reassign 动作，以后端投影为准；否则保留本地阶段判断作为旧接口兜底。
   */
  async function reassignDesignerTask(
    taskId: string,
    payload: { assigneeId: string; assigneeName: string },
  ) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不走设计指派流程')
    if (!backendAllowsDesignerReassignment(task) && !canReassignDesigner(task)) {
      throw new Error('当前状态不可重新指派')
    }
    const currentDesignerId = String(task.designerId ?? task.assigneeId ?? '')
    if (currentDesignerId === String(payload.assigneeId)) {
      throw new Error('新设计师不能与当前负责人相同')
    }
    const designerIdNum = parseInt(payload.assigneeId, 10)
    if (Number.isNaN(designerIdNum)) throw new Error('设计师 ID 无效，请从用户列表选择')
    const assignPayload: AssignTaskPayload = {
      designer_id: designerIdNum,
      designer_name: payload.assigneeName,
    }
    await tasksApi.assign(taskId, assignPayload)
    await loadTaskById(taskId)
  }

  /** 清空设计负责人：与指派同接口，designer_id 传 null 回到待指派。 */
  async function clearDesignerAssignee(taskId: string, remark = '清空指派') {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不走设计指派流程')
    if (!backendAllowsDesignerReassignment(task) && !canReassignDesigner(task)) {
      throw new Error('当前状态不可重新指派')
    }
    const clearPayload = buildClearDesignerAssigneePayload(remark)
    await tasksApi.assign(taskId, clearPayload)
    await loadTaskById(taskId)
  }

  /** 调用 submit-design 推进审核流转；详情中的资产/版本以 loadTaskById（GET 详情）与上传完成链为准 */
  async function submitDesign(
    taskId: string,
    payload: SubmitDesignPayload,
  ) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不走设计提审流程')
    if (!canSubmitAudit(task)) throw new Error('当前状态不可提交审核')
    await tasksApi.submitDesign(taskId, payload)
    await loadTaskById(taskId)
  }

  /** retouch 模块领取：POST /v1/tasks/{id}/modules/retouch/claim */
  async function claimRetouchModule(taskId: string) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (!isRetouchTask(task)) throw new Error('当前任务不是 P 图任务')
    await tasksApi.claimModule(taskId, 'retouch')
    await loadTaskById(taskId)
  }

  /** retouch_task 完成动作：POST /v1/tasks/{id}/modules/retouch/actions/submit */
  async function submitRetouch(taskId: string) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (!isRetouchTask(task)) throw new Error('当前任务不是 P 图任务')
    await tasksApi.submitRetouchModule(taskId)
    await loadTaskById(taskId)
  }

  /** 审核通过，调用 POST /v1/tasks/{id}/audit/approve */
  /** 审核领取，调用 POST /v1/tasks/{id}/audit/claim（后端要求 stage 必填） */
  async function claimAudit(taskId: string, stage: string = 'A') {
    await tasksApi.auditClaim(taskId, { stage })
    await loadTaskById(taskId)
  }

  /** 审核通过，后端要求 stage、next_status 必填 */
  async function passAudit(
    taskId: string,
    options: { stage: string; next_status: string; comment?: string },
  ) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不通过该入口流转')
    if (!canAudit(task)) throw new Error('当前状态不可审核通过')
    await tasksApi.auditApprove(taskId, {
      stage: options.stage,
      next_status: options.next_status,
      comment: options.comment ?? '通过',
    })
    await loadTaskById(taskId)
  }

  /** 审核驳回，调用 POST /v1/tasks/{id}/audit/reject（后端要求 stage、comment 必填） */
  async function rejectAudit(taskId: string, payload: { stage: string; comment: string }) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不进入设计审核打回链路')
    if (!canAudit(task)) throw new Error('当前状态不可驳回')
    await tasksApi.auditReject(taskId, payload)
    await loadTaskById(taskId)
  }

  /** 审核转交，调用 POST /v1/tasks/{id}/audit/transfer */
  async function transferAudit(
    taskId: string,
    payload: { to_auditor_id: number; reason?: string },
  ) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    await tasksApi.auditTransfer(taskId, payload)
    await loadTaskById(taskId)
  }

  function markPendingWarehouse(taskId: string) {
    const task = getById(taskId)
    if (!task) return
    // 采购任务：由采购完成后直接进入仓库节点，重置仓库接收子状态
    if (isPurchaseTask(task)) {
      if (isDoneStatus(task)) return
      updateTask(taskId, {
        status: 'PendingWarehouseReceive',
        warehouseReceiveStatus: 'pending',
      })
      return
    }
    // 非采购任务：仅允许从审核/定制相关节点进入仓库
    if (canTransitionToPendingWarehouse(task)) {
      updateTask(taskId, {
        status: 'PendingWarehouseReceive',
        warehouseReceiveStatus: 'pending',
        auditSubStatus: 'PASSED',
      })
    }
  }

  /** 转复审（审核 B），调用审核通过接口，后端要求 stage、next_status 必填 */
  async function transferToAuditB(taskId: string, comment?: string) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (isPurchaseTask(task)) throw new Error('采购任务不转复审')
    if (!canTransferToAuditB(task)) throw new Error('当前状态不可转复审')
    await tasksApi.auditApprove(taskId, {
      stage: 'A',
      next_status: 'PendingAuditB',
      comment: comment ?? '转复审',
    })
    await loadTaskById(taskId)
  }

  async function submitCustomizationReview(
    taskId: string,
    payload: {
      reviewer_id?: number | string
      source_asset_id?: number | string | null
      customization_level_code?: string
      customization_level_name?: string
      customization_price?: number | null
      customization_weight_factor?: number | null
      customization_note?: string
      customization_review_decision: 'approved' | 'return_to_designer' | 'reviewer_fixed' | string
      // customization_reviews 子表扩展字段（均可选）：审核工作台的定制弹窗会附带这些层级/参考信息，后端按需消费。
      market_tier?: string
      price_tier?: string
      ref_price?: number | null
      ref_inventory?: number | null
      order_no?: string
    },
  ) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    await tasksApi.submitCustomizationReview(taskId, payload)
    await loadTaskById(taskId)
  }

  type TaskProcurementPatchPayload = {
    status?: 'draft' | 'prepared' | 'in_progress' | 'completed'
    procurement_price?: number
    quantity?: number
    supplier_name?: string
    purchase_remark?: string
    expected_delivery_at?: string
    remark?: string
    operator_id?: number
  }

  type TaskProcurementBootstrapPayload = {
    procurement_price: number
    quantity: number
    supplier_name?: string
    purchase_remark?: string
    expected_delivery_at?: string
    remark?: string
  }

  /** 采购任务：保存/更新 procurement 草稿记录 */
  async function saveProcurementRecord(taskId: string, payload: TaskProcurementPatchPayload) {
    const body: Record<string, unknown> = {
      status: payload.status ?? 'draft',
      procurement_price: payload.procurement_price,
      quantity: payload.quantity,
      supplier_name: payload.supplier_name,
      purchase_remark: payload.purchase_remark,
      expected_delivery_at: payload.expected_delivery_at,
      remark: payload.remark,
      operator_id: payload.operator_id,
    }
    Object.keys(body).forEach((k) => {
      if (body[k] === undefined) delete body[k]
    })
    await tasksApi.patchTaskProcurement(taskId, body)
    await loadTaskById(taskId)
  }

  function procurementAdvanceActionsByStatus(status: string | undefined): Array<'prepare' | 'start' | 'complete'> {
    const v = String(status ?? '').toLowerCase()
    if (v === 'completed') return []
    if (v === 'in_progress') return ['complete']
    if (v === 'prepared') return ['start', 'complete']
    return ['prepare', 'start', 'complete']
  }

  /** 采购任务：推进 procurement 到 completed（按当前状态跳过已完成步骤） */
  async function completeProcurementFlow(taskId: string) {
    const task = getById(taskId)
    const supplierName = String(task?.purchaseInfo?.supplierName ?? '').trim() || '[默认]'
    await saveProcurementRecord(taskId, {
      status: (task?.procurementApiStatus as TaskProcurementPatchPayload['status']) ?? 'draft',
      supplier_name: supplierName,
    })
    const steps = procurementAdvanceActionsByStatus(task?.procurementApiStatus)
    for (const action of steps) {
      await tasksApi.advanceTaskProcurement(taskId, { action })
    }
    await loadTaskById(taskId)
  }

  /** 采购任务：一次性写 procurement 草稿并推进到 completed */
  async function bootstrapProcurement(taskId: string, payload: TaskProcurementBootstrapPayload) {
    await saveProcurementRecord(taskId, {
      status: 'draft',
      procurement_price: payload.procurement_price,
      quantity: payload.quantity,
      supplier_name: String(payload.supplier_name ?? '').trim() || '[默认]',
      purchase_remark: payload.purchase_remark,
      expected_delivery_at: payload.expected_delivery_at,
      remark: payload.remark,
    })
    await completeProcurementFlow(taskId)
  }

  function startOutsourcing(taskId: string) {
    const task = getById(taskId)
    if (!task) return
    if (isPurchaseTask(task)) return
    // 定制流程仅允许从初审阶段（PendingAuditA）触发，防止已归档/仓库等状态误触发
    if (!isPendingAuditA(task)) return
    updateTask(taskId, { status: 'Outsourcing', auditSubStatus: 'TRANSFERRED' })
  }

  function rejectAfterOutsourceReview(taskId: string) {
    const task = getById(taskId)
    if (!task) return
    if (isPurchaseTask(task)) return
    updateTask(taskId, { status: 'RejectedByAuditB', auditSubStatus: 'REJECTED' })
  }

  function resubmitAuditB(taskId: string) {
    const task = getById(taskId)
    if (!task) return
    if (isPurchaseTask(task)) return
    if (!canResubmitAuditB(task)) return
    // 复审打回后重新提交复审，同时清理仓库退回标记，允许后续再次进仓库
    updateTask(taskId, { status: 'PendingAuditB', warehouseReceiveStatus: undefined, auditSubStatus: 'IN_PROGRESS' })
  }

  /** 仓库接收，调用 POST /v1/tasks/{id}/warehouse/receive */
  async function receiveInWarehouse(taskId: string) {
    const task = getById(taskId)
    if (task) {
      if (!task.sku) throw new Error('任务无 SKU，无法仓库接收')
      if (task.warehouseReceiveStatus === 'returned') throw new Error('该任务已退回，无法再次接收')
      if (task.warehouseReceiveStatus === 'received') return
    }
    await tasksApi.warehouseReceive(taskId)
    await loadTaskById(taskId)
  }

  /** 仓库驳回，调用 POST /v1/tasks/{id}/warehouse/reject */
  async function rejectInWarehouse(
    taskId: string,
    payload: {
      reject_reason?: string
      reject_category?: string
      remark?: string
      receiver_id?: number
    },
  ) {
    await tasksApi.warehouseReject(taskId, payload)
    await loadTaskById(taskId)
  }

  function markCompleted(taskId: string) {
    const task = getById(taskId)
    if (!task) return
    if (isCompletedOrArchived(task)) return
    const result = checkTaskCompletion(task)
    if (!result.canComplete) {
      // 仅在控制台输出原因，UI 层后续通过 checkTaskCompletion 独立展示
      console.warn('[tasks] cannot complete task', task.id, result.reasons)
      return
    }
    updateTask(taskId, { status: 'Completed', warehouseSubStatus: 'DONE' })
  }

  /**
   * 仓库侧一键结单：
   * - PendingProductionTransfer：先调 warehouse/complete 推进到 PendingClose，再调 close 正式关单
   * - PendingClose：直接调 close
   */
  async function archiveTask(taskId: string) {
    const task = getById(taskId)
    if (!task) throw new Error('任务不存在')
    if (task.status === 'PendingClose') {
      if (!canArchive(task)) throw new Error('当前状态不可结单')
      await tasksApi.closeTask(taskId, {})
      await loadTaskById(taskId)
      return
    }
    if (task.status !== 'PendingProductionTransfer') {
      throw new Error('当前状态不可结单')
    }
    await tasksApi.warehouseComplete(taskId)
    await loadTaskById(taskId)
    const refreshed = getById(taskId)
    if (refreshed?.status !== 'PendingClose') {
      throw new Error('仓库流转已完成，但任务尚未进入待结单状态，请刷新后重试')
    }
    const closeReadiness = canCloseTaskForArchive(refreshed)
    if (!closeReadiness.allowed) {
      const suffix = closeReadiness.reasons.length > 0 ? `：${closeReadiness.reasons.join('；')}` : ''
      throw new Error(`仓库流转已完成，任务已进入待结单；当前还不能正式结单${suffix}`)
    }
    await tasksApi.closeTask(taskId, {})
    await loadTaskById(taskId)
  }

  /**
   * 演示用：将一批图片（同一次上传的所有文件）作为一个新版本存入 task.assetVersions。
   * 接入真实后端后，此处改为调用上传 API，fileRefs 替换为 CDN URL 数组。
   */
  function uploadAssetVersion(
    taskId: string,
    payload: {
      dataUrls: string[]
      fileNames: string[]
      uploaderName: string
      uploaderId: string
    },
  ) {
    const task = getById(taskId)
    if (!task || !payload.dataUrls.length) return
    const newVersion: TaskAssetVersion = {
      id: `av-${Date.now()}`,
      type: 'draft',
      uploaderId: payload.uploaderId,
      uploaderName: payload.uploaderName,
      uploadedAt: nowISO(),
      note: payload.fileNames.join('、'),
      fileRefs: payload.dataUrls,
    }
    updateTask(taskId, {
      assetVersions: [...task.assetVersions, newVersion],
    })
  }

  function returnFromWarehouse(taskId: string) {
    const task = getById(taskId)
    if (!task || !canGoWarehouse(task)) return

    if (isPurchaseTask(task)) {
      // 采购任务没有设计/审核节点，退回后停留在仓库层等待重新采购/核查
      // 不应写入 RejectedByAuditB（该状态对采购任务毫无意义）
      updateTask(taskId, {
        warehouseReceiveStatus: 'returned',
        warehouseSubStatus: 'RETURNED',
      })
      return
    }

    // 设计类任务：退回仓库后回退到复审打回节点，由审核/设计侧整改后重新提交
    updateTask(taskId, {
      warehouseReceiveStatus: 'returned',
      warehouseSubStatus: 'RETURNED',
      status: 'RejectedByAuditB',
    })
  }

  /** 登出或切换账号时调用，清空列表并允许下次重新拉取 */
  function resetToInitialState() {
    items.value = []
    initialized.value = false
    loadError.value = null
  }

  return {
    list,
    loadTaskListSnapshot: loadTaskList,
    fullListReplaceGeneration,
    listTotal,
    loading,
    loadError,
    getById,
    loadTasks,
    loadTaskById,
    forceRefreshList,
    loadTaskListForView,
    mainStatusOf,
    designStatusOf,
    auditStatusOf,
    warehouseStatusOf,
    purchaseStatusOf,
    addTask,
    prepareProductCodes,
    updateTask,
    lastSequence,
    applyEvent,
    generateActionId,
    assignTask,
    reassignDesignerTask,
    clearDesignerAssignee,
    submitDesign,
    claimRetouchModule,
    submitRetouch,
    claimAudit,
    passAudit,
    rejectAudit,
    transferAudit,
    markPendingWarehouse,
    transferToAuditB,
    submitCustomizationReview,
    saveProcurementRecord,
    completeProcurementFlow,
    bootstrapProcurement,
    startOutsourcing,
    rejectAfterOutsourceReview,
    resubmitAuditB,
    receiveInWarehouse,
    rejectInWarehouse,
    markCompleted,
    archiveTask,
    returnFromWarehouse,
    uploadAssetVersion,
    refreshReferenceUrls,
    resetToInitialState,
  }
})
