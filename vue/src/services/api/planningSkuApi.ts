import http from '@/services/http'
import { assetsApi } from '@/services/api/assetsApi'
import { parseOssDirectPlan, runOssDirectUploadPlan } from '@/services/upload/ossDirectUpload'
import { resolveFileMimeType } from '@/utils/mime'

export interface PlanningSKUInput {
  client_item_id: string
  description_spec: string
  quantity: number
  target_price?: string
  note?: string
  reference_url?: string
  image_upload_ref?: string
  erp_product_i_id?: string
  erp_product_name?: string
}

export interface PlanningSKUResultItem {
  task_sku_item_id: number
  sequence_no: number
  sku_code: string
  quantity: number
  erp_status?: string
  revision?: {
    id: number
    version_no: number
    description_spec: string
    quantity: number
    target_price?: string
    note?: string
    reference_url?: string
    product_image_ref_id?: string
    product_image_url?: string
    product_image_name?: string
  }
}

export interface PlanningSKUCreateResult {
  task_id: number
  task_no: string
  task_status: 'Completed'
  workflow_revision: number
  items: PlanningSKUResultItem[]
}

const unwrap = <T>(response: { data?: { data?: T } | T }): T => {
  const body = response.data as { data?: T } | T
  return body && typeof body === 'object' && 'data' in body ? (body.data as T) : (body as T)
}

function downloadPlanningWorkbook(data: unknown, filename: string): void {
  const blob = data instanceof Blob
    ? data
    : new Blob([data as BlobPart], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const objectURL = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectURL
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(objectURL)
}

export const planningSkuApi = {
  async create(items: PlanningSKUInput[], erpSyncMode: 'none' | 'async', clientCreateId: string): Promise<PlanningSKUCreateResult> {
    return unwrap(await http.post('/v1/tasks', {
      task_type: 'sku_planning',
      client_create_id: clientCreateId,
      erp_sync_mode: erpSyncMode,
      planning_sku_items: items,
    }, { headers: { 'Idempotency-Key': clientCreateId } }))
  },
  async getTask(taskId: number): Promise<PlanningSKUCreateResult> {
    return unwrap(await http.get(`/v1/tasks/${taskId}/planning-skus`))
  },
  templateURL(erp = false): string {
    return `/v1/tasks/sku-planning/template.xlsx${erp ? '?erp=true' : ''}`
  },
  async parseExcel(file: File, erp = false): Promise<{ planning_sku_items: PlanningSKUInput[]; errors: Array<{ row: number; field: string; reason: string }>; valid: boolean }> {
    const data = new FormData()
    data.append('file', file)
    data.append('erp', String(erp))
    return unwrap(await http.post('/v1/tasks/sku-planning/parse-excel', data, { headers: { 'Content-Type': 'multipart/form-data' } }))
  },
  async uploadImage(file: File, clientCreateId: string, clientItemId: string): Promise<string> {
    const mimeType = resolveFileMimeType(file)
    const created = await http.post('/v1/tasks/sku-planning/image-upload-sessions', {
      client_create_id: clientCreateId,
      client_item_id: clientItemId,
      filename: file.name,
      expected_size: file.size,
      mime_type: mimeType,
    })
    const plan = parseOssDirectPlan(created.data)
    if (!plan.sessionId) throw new Error('图片上传会话未返回 session_id。')
    const completePayload: Record<string, unknown> = { upload_content_type: mimeType }
    try {
      if (plan.ossDirect) {
        const uploaded = await runOssDirectUploadPlan(file, plan.ossDirect, {
          progressByteTotal: file.size,
          mimeTypeForUpload: mimeType,
        })
        completePayload.oss_object_key = uploaded.oss_object_key
        completePayload.upload_content_type = uploaded.upload_content_type || mimeType
        if (uploaded.oss_upload_id) completePayload.oss_upload_id = uploaded.oss_upload_id
        if (uploaded.oss_parts?.length) completePayload.oss_parts = uploaded.oss_parts
      } else if (plan.remote) {
        const uploadMime = plan.remote.required_upload_content_type || mimeType
        await assetsApi.uploadToRemoteUrl(plan.remote.upload_url, file, {
          method: plan.remote.method || 'PUT',
          extraHeaders: { 'Content-Type': uploadMime },
        })
        completePayload.upload_content_type = uploadMime
      } else {
        throw new Error('图片上传入口未准备好。')
      }
      const completed = await http.post(
        plan.completeEndpoint || `/v1/tasks/sku-planning/image-upload-sessions/${plan.sessionId}/complete`,
        completePayload,
      )
      const data = unwrap<Record<string, unknown>>(completed)
      const reference = data.image_upload_ref as Record<string, unknown> | string | undefined
      const refId = typeof reference === 'string'
        ? reference
        : String(reference?.ref_id || reference?.asset_id || '')
      if (!refId) throw new Error('图片上传完成但未返回 image_upload_ref。')
      return refId
    } catch (error) {
      await http.post(`/v1/tasks/sku-planning/image-upload-sessions/${plan.sessionId}/abort`, {}).catch(() => undefined)
      throw error
    }
  },
  exportTaskURL(taskId: number): string {
    return `/v1/tasks/${taskId}/planning-skus/export.xlsx`
  },
  async downloadTask(taskId: number): Promise<void> {
    const response = await http.get(`/v1/tasks/${taskId}/planning-skus/export.xlsx`, { responseType: 'blob' })
    downloadPlanningWorkbook(response.data, `策划SKU_任务_${taskId}.xlsx`)
  },
  async retryFailedERP(taskId: number): Promise<{ queued: number; resync: false }> {
    return unwrap(await http.post(`/v1/tasks/${taskId}/planning-skus/erp-retry`, {}))
  },
  async exportSelection(taskSkuItemIds: number[]): Promise<void> {
    const response = await http.post('/v1/planning-skus/export.xlsx', {
      task_sku_item_ids: taskSkuItemIds,
    }, { responseType: 'blob' })
    downloadPlanningWorkbook(response.data, `策划SKU_勾选结果_${new Date().toISOString().slice(0, 10).replace(/-/g, '')}.xlsx`)
  },
}
