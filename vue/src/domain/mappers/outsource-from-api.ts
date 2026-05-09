import type { BackendOutsourceOrderRaw } from '@/services/apiTypes'
import type { OutsourceOrder, OutsourceOrderStatus } from '@/domain/types/outsource'
import { nowISO } from '@/utils/date'

/** 从 GET /v1/tasks/{id} 读模型提取列表展示所需摘要（与任务 store 解析路径对齐） */
export interface OutsourceTaskSummary {
  taskNo: string
  sku: string
  productName: string
}

export function extractTaskSummaryFromTaskResponse(raw: unknown): OutsourceTaskSummary {
  const empty = { taskNo: '—', sku: '—', productName: '—' }
  if (raw == null || typeof raw !== 'object') return empty
  const r = raw as Record<string, unknown>
  const inner =
    r.data != null && typeof r.data === 'object' ? (r.data as Record<string, unknown>) : r
  const taskNoRaw = inner.task_no ?? inner.taskNo
  const taskNo = typeof taskNoRaw === 'string' && taskNoRaw.trim() !== '' ? taskNoRaw.trim() : '—'
  const ps = inner.product_selection as Record<string, unknown> | undefined
  const erp = ps?.erp_product as Record<string, unknown> | undefined
  const skuRaw = inner.sku_code ?? inner.skuCode ?? erp?.sku_code
  const sku = typeof skuRaw === 'string' && skuRaw.trim() !== '' ? skuRaw.trim() : '—'
  const nameRaw = inner.product_name ?? inner.productName ?? erp?.product_name ?? erp?.name
  const productName =
    typeof nameRaw === 'string' && nameRaw.trim() !== '' ? nameRaw.trim() : '—'
  return { taskNo, sku, productName }
}

function mapBackendOutsourceStatus(status: string): OutsourceOrderStatus {
  const m: Record<string, OutsourceOrderStatus> = {
    created: 'draft',
    packaged: 'sent',
    sent: 'sent',
    in_production: 'in_progress',
    returned: 'returned',
    reviewing: 'reviewing',
    approved: 'review_passed',
    rejected: 'review_rejected',
    closed: 'closed',
  }
  return m[status] ?? 'draft'
}

function deriveReviewResult(
  mapped: OutsourceOrderStatus,
): 'passed' | 'rejected' | undefined {
  if (mapped === 'review_passed') return 'passed'
  if (mapped === 'review_rejected') return 'rejected'
  return undefined
}

export function parseOutsourceListBody(body: unknown): {
  items: BackendOutsourceOrderRaw[]
  pagination: { page?: number; page_size?: number; total?: number }
} {
  const b = typeof body === 'object' && body !== null ? (body as Record<string, unknown>) : {}
  const arr = Array.isArray(b.data) ? b.data : []
  const p = (b.pagination as Record<string, unknown> | undefined) ?? {}
  return {
    items: arr as BackendOutsourceOrderRaw[],
    pagination: {
      page: typeof p.page === 'number' ? p.page : undefined,
      page_size: typeof p.page_size === 'number' ? p.page_size : undefined,
      total: typeof p.total === 'number' ? p.total : undefined,
    },
  }
}

/**
 * 将 OpenAPI OutsourceOrder 转为前端列表/详情模型。
 * 任务号、SKU、产品名优先使用列表项扩展字段，否则由调用方传入 `summary`（来自 GET /v1/tasks/{id}）。
 */
export function mapBackendOutsourceOrderToDomain(
  raw: BackendOutsourceOrderRaw,
  summary?: OutsourceTaskSummary | null,
): OutsourceOrder {
  const mappedStatus = mapBackendOutsourceStatus(String(raw.status ?? ''))
  const taskNo =
    typeof raw.task_no === 'string' && raw.task_no.trim() !== ''
      ? raw.task_no.trim()
      : (summary?.taskNo ?? '—')
  const sku =
    typeof raw.sku === 'string' && raw.sku.trim() !== '' ? raw.sku.trim() : (summary?.sku ?? '—')
  const productName =
    typeof raw.product_name === 'string' && raw.product_name.trim() !== ''
      ? raw.product_name.trim()
      : (summary?.productName ?? '—')

  const createdAt = raw.created_at || nowISO()
  const returnedAt =
    raw.returned_at != null && String(raw.returned_at).trim() !== ''
      ? String(raw.returned_at)
      : undefined

  return {
    id: String(raw.id),
    orderNo: String(raw.outsource_no ?? ''),
    taskId: String(raw.task_id),
    taskNo,
    sku,
    productName,
    outsourceType: String(raw.outsource_type ?? ''),
    supplierId: '',
    supplierName: String(raw.vendor_name ?? ''),
    deliveryRequirement: String(raw.delivery_requirement ?? ''),
    specNote: String(raw.settlement_note ?? ''),
    status: mappedStatus,
    createdAt,
    returnedAt,
    reviewResult: deriveReviewResult(mappedStatus),
  }
}
