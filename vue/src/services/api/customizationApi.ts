import http from '@/services/http'
import type { CustomizationJobRaw } from '@/services/apiTypes'
import { normalizePriorityForApi, type TaskPriorityApi } from '@/domain/task-priority'
import { sanitizeCreateTaskPayload } from '@/domain/task-create-fields'
import { toRelativeAssetUrl } from '@/utils/url'

export interface CustomizationJobQuery {
  task_id?: number | string
  status?: string
  operator_id?: number | string
  page?: number
  page_size?: number
}

export interface CustomizationEffectPreviewPayload {
  operator_id?: number | string
  current_asset_id?: number | string | null
  note?: string
}

export interface CustomizationEffectReviewPayload {
  reviewer_id?: number | string
  customization_review_decision?: 'approved' | 'return_to_designer' | 'reviewer_fixed' | string
  customization_level_code?: string
  customization_level_name?: string
  customization_price?: number | null
  customization_weight_factor?: number | null
  customization_note?: string
  // customization_reviews 子表扩展字段（均可选）：与初审弹窗保持同形，方便审核工作台在两个阶段共用同一表单组件。
  market_tier?: string
  price_tier?: string
  ref_price?: number | null
  ref_inventory?: number | null
  order_no?: string
}

export interface CustomizationProductionTransferPayload {
  operator_id?: number | string
  current_asset_id?: number | string | null
  note?: string
}

export type CustomizationTaskType = 'ORIGINAL_PRODUCT_DEV' | 'NEW_PRODUCT_DEV'
export type CustomizationTaskPriority = TaskPriorityApi

export interface CustomizationTaskCreatePayload {
  task_type: CustomizationTaskType
  order_number: string
  design_requirement: string
  due_at: string
  owner_team: string
  owner_org_team?: string
  owner_department?: string
  designer_id?: string | number | null
  requester_id?: string | number | null
  requester_name?: string
  priority?: CustomizationTaskPriority
  reference_file_refs?: (Record<string, unknown> | string)[]
  material?: string
  material_other?: string
  /** 原有产品开发：ERP 选品结果 */
  product_id?: string | number | null
  sku_code?: string | null
  product_name?: string
  erp_product_snapshot?: Record<string, unknown>
  /**
   * 原有产品开发：产品分类编码预留接口（当前创建链路不强制提交，等待后端稳定返回）。
   */
  original_product_category_code?: string
  /** 新品开发字段 */
  category_code?: string
  product_short_name?: string
}

export interface CustomizationTaskCreateResult {
  task_id: string
  /** 预留读取：后端后续可能补齐原有产品分类编码 */
  original_product_category_code?: string
  raw: unknown
}

function toBackendTaskType(value: CustomizationTaskType): string {
  return value === 'ORIGINAL_PRODUCT_DEV'
    ? 'original_product_development'
    : 'new_product_development'
}

function toNumericIdOrNull(value: string | number | null | undefined): number | null {
  if (value == null) return null
  const text = String(value).trim()
  if (!text) return null
  if (!/^\d+$/.test(text)) return null
  const n = Number.parseInt(text, 10)
  if (Number.isFinite(n)) return n
  return null
}

function normalizePriority(value: CustomizationTaskPriority | undefined): TaskPriorityApi {
  return normalizePriorityForApi(value ?? 'normal')
}

function normalizeRefUrl(raw: unknown): string | undefined {
  if (typeof raw !== 'string') return undefined
  const text = raw.trim()
  if (!text) return undefined
  return toRelativeAssetUrl(text) ?? text
}

function sanitizeReferenceFileRefObject(raw: unknown): Record<string, unknown> | null {
  if (!raw || typeof raw !== 'object') return null
  const obj = raw as Record<string, unknown>
  const next: Record<string, unknown> = {}
  const copy = (key: string) => {
    if (obj[key] !== undefined) next[key] = obj[key]
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
  return Object.keys(next).length > 0 ? next : null
}

function optionalMaterialSnakeFields(
  materialRaw: unknown,
  materialOtherRaw: unknown,
): Record<string, unknown> {
  const material = String(materialRaw ?? '').trim()
  const materialOther = String(materialOtherRaw ?? '').trim()
  if (material === 'OTHER') {
    return materialOther ? { material_mode: 'other', material_other: materialOther } : {}
  }
  if (!material) return {}
  return { material_mode: 'other', material_other: material }
}

function buildTaskRemark(orderNumber: string): string {
  return `订单号：${orderNumber.trim()}`
}

function buildCustomizationTaskCreateBody(
  payload: CustomizationTaskCreatePayload,
): Record<string, unknown> {
  const isOriginal = payload.task_type === 'ORIGINAL_PRODUCT_DEV'
  const taskType = toBackendTaskType(payload.task_type)
  const cleanedRefs = (payload.reference_file_refs ?? [])
    .map((item) => sanitizeReferenceFileRefObject(item))
    .filter((item): item is Record<string, unknown> => item != null)
  const requesterId = toNumericIdOrNull(payload.requester_id)
  const designerId = toNumericIdOrNull(payload.designer_id)
  const productId = toNumericIdOrNull(payload.product_id)
  const productName = String(payload.product_name ?? '').trim()

  const body: Record<string, unknown> = {
    task_type: taskType,
    requester_id: requesterId,
    requester_name: payload.requester_name ?? '',
    designer_id: designerId,
    owner_department: payload.owner_department?.trim() || undefined,
    owner_org_team: payload.owner_org_team?.trim() || undefined,
    owner_team: payload.owner_team.trim(),
    deadline_at: payload.due_at,
    priority: normalizePriority(payload.priority),
    customization_required: true,
    customization_source_type: isOriginal ? 'existing_product' : 'new_product',
    reference_file_refs: cleanedRefs,
    remark: buildTaskRemark(payload.order_number),
  }

  if (isOriginal) {
    body.product_id = productId
    body.product_name = productName
    body.product_name_snapshot = productName
    body.change_request = payload.design_requirement.trim()
    body.sku_code = payload.sku_code?.trim() || null
    if (productId != null) {
      body.product_selection = { selected_product_id: productId }
    } else if (payload.erp_product_snapshot && typeof payload.erp_product_snapshot === 'object') {
      body.defer_local_product_binding = true
      body.product_selection = {
        defer_local_product_binding: true,
        erp_product: payload.erp_product_snapshot,
      }
    }
  } else {
    body.product_id = null
    body.product_name = productName
    body.product_name_snapshot = productName
    body.design_requirement = payload.design_requirement.trim()
    body.category_code = payload.category_code?.trim() || undefined
    body.product_short_name = payload.product_short_name?.trim() || undefined
  }

  // Round I.g · D2：material_* 仅对 new_product_development 合法；原品改款定制
  // （customization_required=true + task_type=original_product_development）
  // 也禁止携带 material_* —— 后端 validateTaskTypeFieldWhitelist 仅按 task_type
  // 查表，不管 customization_required。
  if (!isOriginal) {
    Object.assign(body, optionalMaterialSnakeFields(payload.material, payload.material_other))
  }

  return body
}

function pickTaskFromBody(body: unknown): Record<string, unknown> | null {
  if (!body || typeof body !== 'object') return null
  const root = body as Record<string, unknown>
  if (root.data && typeof root.data === 'object') {
    const data = root.data as Record<string, unknown>
    if (data.id != null) return data
  }
  if (root.id != null) return root
  return null
}

export function extractCustomizationJob(body: unknown): CustomizationJobRaw | null {
  if (!body || typeof body !== 'object') return null
  const root = body as Record<string, unknown>
  const candidate =
    root.data && typeof root.data === 'object'
      ? (root.data as Record<string, unknown>)
      : root
  if (candidate.id == null) return null
  return candidate as CustomizationJobRaw
}

export async function createCustomizationTask(
  payload: CustomizationTaskCreatePayload,
  signal?: AbortSignal,
): Promise<CustomizationTaskCreateResult> {
  const rawBody = buildCustomizationTaskCreateBody(payload)
  // Round I.g · D2：所有 POST /v1/tasks 入口统一过 sanitizer，避免「定制绿色通道」
  // 绕过普通 task-create 的字段白名单（见 Round I.f 历史回归）。
  const body = sanitizeCreateTaskPayload(rawBody, String(rawBody.task_type ?? ''))
  if (import.meta.env.DEV) {
    const stripped = Object.keys(rawBody).filter((k) => !(k in body))
    if (stripped.length > 0) {
      // eslint-disable-next-line no-console
      console.warn(
        '[POST /v1/tasks · customization] sanitizer stripped forbidden fields for task_type =',
        body.task_type,
        ':',
        stripped,
      )
    }
  }
  const res = await http.post<unknown>('/v1/tasks', body, { signal })
  const created = pickTaskFromBody(res.data)
  if (!created) {
    throw new Error('创建任务成功但未返回任务 ID')
  }
  return {
    task_id: String(created.id),
    original_product_category_code:
      typeof created.product_category_code === 'string' ? created.product_category_code : undefined,
    raw: res.data,
  }
}

export async function listCustomizationJobs(
  params?: CustomizationJobQuery,
  signal?: AbortSignal,
) {
  const res = await http.get<unknown>('/v1/customization-jobs', { params, signal })
  return res.data
}

export async function getCustomizationJobDetail(id: number | string, signal?: AbortSignal) {
  const res = await http.get<unknown>(`/v1/customization-jobs/${id}`, { signal })
  return res.data
}

export async function submitCustomizationEffectPreview(
  id: number | string,
  payload: CustomizationEffectPreviewPayload,
  signal?: AbortSignal,
) {
  const res = await http.post<unknown>(`/v1/customization-jobs/${id}/effect-preview`, payload, { signal })
  return res.data
}

export async function submitCustomizationEffectReview(
  id: number | string,
  payload: CustomizationEffectReviewPayload,
  signal?: AbortSignal,
) {
  const res = await http.post<unknown>(`/v1/customization-jobs/${id}/effect-review`, payload, { signal })
  return res.data
}

export async function submitCustomizationProductionTransfer(
  id: number | string,
  payload: CustomizationProductionTransferPayload,
  signal?: AbortSignal,
) {
  const res = await http.post<unknown>(`/v1/customization-jobs/${id}/production-transfer`, payload, { signal })
  return res.data
}
