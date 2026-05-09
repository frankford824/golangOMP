import type { Task, WarehouseSubStatus } from '@/domain/types/task'

export interface ReceiptRow {
  taskId: string
  taskNo: string
  sku: string
  productName: string
  taskType: string
  workflowLane: 'normal' | 'customization' | string
  sourceDepartment?: string
  finalVersionRef: string
  specSummary?: string
  auditCompletedAt?: string
  isOutsourceReturn: boolean
  warehouseSubStatus: WarehouseSubStatus
  isBatchTask?: boolean
  batchItemCount?: number
  filing_status?: string
  missing_fields_summary_cn?: string
  filing_error_message?: string
}

function pickNestedTask(raw: Record<string, unknown>): Record<string, unknown> | null {
  const t = raw.task
  if (t && typeof t === 'object') return t as Record<string, unknown>
  const summary = raw.task_summary
  if (summary && typeof summary === 'object') return summary as Record<string, unknown>
  return null
}

function str(raw: Record<string, unknown>, ...keys: string[]): string {
  for (const k of keys) {
    const v = raw[k]
    if (typeof v === 'string' && v.trim()) return v.trim()
    if (typeof v === 'number' && Number.isFinite(v)) return String(v)
  }
  return ''
}

/** 将 GET /v1/warehouse/receipts 单条读模型映射为仓库表格行（兼容嵌套 task / task_summary） */
export function warehouseReceiptRawToReceiptRow(raw: Record<string, unknown>): ReceiptRow {
  const nested = pickNestedTask(raw) ?? {}
  const taskId =
    str(raw, 'task_id', 'taskId') ||
    str(nested, 'id', 'task_id', 'taskId') ||
    ''
  const taskNo = str(raw, 'task_no', 'taskNo') || str(nested, 'task_no', 'taskNo') || (taskId ? `#${taskId}` : '—')
  const sku = str(raw, 'sku', 'sku_code', 'skuCode') || str(nested, 'sku', 'sku_code', 'skuCode')
  const productName =
    str(raw, 'product_name', 'productName', 'product_name_snapshot') ||
    str(nested, 'product_name', 'productName', 'product_name_snapshot') ||
    '—'

  const laneRaw =
    str(raw, 'workflow_lane', 'workflowLane') ||
    str(nested, 'workflow_lane', 'workflowLane') ||
    'normal'
  const workflowLane =
    laneRaw === 'customization' || laneRaw === 'normal' ? laneRaw : (laneRaw || 'normal')

  const sourceDepartment =
    str(raw, 'source_department', 'sourceDepartment') ||
    str(nested, 'source_department', 'sourceDepartment') ||
    undefined

  const statusRaw = String(raw.status ?? raw.receipt_status ?? '').toLowerCase()
  let warehouseSubStatus: WarehouseSubStatus = 'PENDING_RECEIVE'
  if (statusRaw === 'received') warehouseSubStatus = 'RECEIVED'
  else if (statusRaw === 'rejected') warehouseSubStatus = 'RETURNED'
  else if (statusRaw === 'completed') warehouseSubStatus = 'DONE'
  else if (statusRaw === 'packing') warehouseSubStatus = 'PACKING'
  else if (
    statusRaw === 'pending_receive' ||
    statusRaw === 'pending' ||
    statusRaw === 'awaiting_receive' ||
    statusRaw === ''
  ) {
    warehouseSubStatus = 'PENDING_RECEIVE'
  }

  const nestedType = str(nested, 'business_type', 'businessType', 'task_type', 'taskType')
  const flatType = str(raw, 'business_type', 'businessType', 'task_type', 'taskType')
  const taskType = nestedType || flatType || 'ORIGINAL_PRODUCT_DEV'
  const tt = taskType.toLowerCase()
  const isPurchase = taskType === 'PURCHASE_TASK' || tt === 'purchase_task'

  const batchFlag = raw.is_batch_task ?? raw.isBatchTask ?? nested.is_batch_task ?? nested.isBatchTask
  const isBatchTask = batchFlag === true || batchFlag === 'true' || String(batchFlag).toLowerCase() === 'true'

  const batchCount = raw.batch_item_count ?? raw.batchItemCount ?? nested.batch_item_count ?? nested.batchItemCount
  const batchItemCount =
    typeof batchCount === 'number' && Number.isFinite(batchCount)
      ? batchCount
      : typeof batchCount === 'string' && /^\d+$/.test(batchCount)
        ? parseInt(batchCount, 10)
        : undefined

  const needOut =
    raw.need_outsource === true ||
    raw.needOutsource === true ||
    nested.need_outsource === true ||
    nested.needOutsource === true

  return {
    taskId,
    taskNo,
    sku,
    productName,
    taskType,
    workflowLane,
    sourceDepartment,
    finalVersionRef:
      str(raw, 'final_version_ref', 'finalVersionRef') ||
      (isPurchase ? '参考图（按资产列表）' : '定稿图（按资产列表）'),
    specSummary: str(raw, 'design_requirement', 'designRequirement', 'spec_summary') || undefined,
    auditCompletedAt: str(raw, 'audit_completed_at', 'updated_at', 'updatedAt') || undefined,
    isOutsourceReturn: needOut,
    warehouseSubStatus,
    isBatchTask,
    batchItemCount,
    filing_status: str(raw, 'filing_status', 'filingStatus') || str(nested, 'filing_status', 'filingStatus') || undefined,
    missing_fields_summary_cn:
      str(raw, 'missing_fields_summary_cn', 'missingFieldsSummaryCn') ||
      str(nested, 'missing_fields_summary_cn', 'missingFieldsSummaryCn') ||
      undefined,
    filing_error_message:
      str(raw, 'filing_error_message', 'filingErrorMessage') ||
      str(nested, 'filing_error_message', 'filingErrorMessage') ||
      undefined,
  }
}

/**
 * 将 tasksStore 里已进入「待接收」的任务合成为仓库表格行。
 *
 * 为什么需要这个 mapper：
 *   GET /v1/warehouse/receipts 的 WarehouseReceipt.status 枚举仅 [received, rejected, completed]
 *   （见 docs/openapi.yaml#WarehouseReceipt），后端不会在审核通过时预写 pending receipt。
 *   因此「待接收」这一 tab 必须从 /v1/tasks 侧合成，否则审核通过的任务永远进不了仓库中心。
 *
 * 仅应在任务命中「待接收」时调用（t.mainStatus === 'WAREHOUSE_PENDING' ||
 *   t.warehouseSubStatus === 'PENDING_RECEIVE' || t.status === 'PendingWarehouseReceive'），
 * 否则产出的 warehouseSubStatus 语义会错位。
 */
export function pendingReceiveTaskToReceiptRow(task: Task): ReceiptRow {
  const taskType = task.taskType || task.businessType || 'ORIGINAL_PRODUCT_DEV'
  const isPurchase = taskType === 'PURCHASE_TASK'
  const lane: ReceiptRow['workflowLane'] =
    task.workflowLane === 'customization' || task.workflowLane === 'normal'
      ? task.workflowLane
      : 'normal'

  return {
    taskId: String(task.id),
    taskNo: task.taskNo || (task.id ? `#${task.id}` : '—'),
    sku: task.sku || '',
    productName: task.productName || '—',
    taskType: String(taskType),
    workflowLane: lane,
    sourceDepartment: task.ownerDepartment || task.sourceDepartment || undefined,
    finalVersionRef: isPurchase ? '参考图（按资产列表）' : '定稿图（按资产列表）',
    specSummary: task.designRequirement || undefined,
    auditCompletedAt: task.updatedAt || undefined,
    isOutsourceReturn: Boolean(task.needOutsource),
    warehouseSubStatus: 'PENDING_RECEIVE',
    isBatchTask: task.isBatchTask,
    batchItemCount: task.batchItemCount,
    filing_status: task.filing_status,
    missing_fields_summary_cn: task.missing_fields_summary_cn,
    filing_error_message: task.filing_error_message,
  }
}

/** 判断任务是否处于「待仓库接收」阶段。与 TaskListView 的 WAREHOUSE_PENDING 队列口径对齐。 */
export function isPendingWarehouseReceiveTask(task: Task): boolean {
  return (
    task.mainStatus === 'WAREHOUSE_PENDING' ||
    task.warehouseSubStatus === 'PENDING_RECEIVE' ||
    task.status === 'PendingWarehouseReceive'
  )
}

export function extractWarehouseReceiptList(body: unknown): Record<string, unknown>[] {
  if (!body || typeof body !== 'object') return []
  const root = body as Record<string, unknown>
  const data = root.data
  if (Array.isArray(data)) return data.filter((x): x is Record<string, unknown> => x != null && typeof x === 'object') as Record<string, unknown>[]
  if (Array.isArray(root)) return root as Record<string, unknown>[]
  return []
}
