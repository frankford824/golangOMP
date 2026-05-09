import http from '@/services/http'

export interface ReferenceFileRef {
  asset_id: string
  ref_id: string
  filename: string
  mime_type: string
  file_size: number
  download_url: string
  storage_key: string
}

export interface BatchPreviewRow {
  product_name?: string
  design_requirement?: string
  product_i_id?: string
  reference_file_refs?: ReferenceFileRef[]
}

export interface BatchViolation {
  row: number
  column: string
  code: string
  message?: string
}

export interface BatchSkuParseResult {
  task_type?: string
  preview?: BatchPreviewRow[]
  violations?: BatchViolation[]
}

/** 合并后端可能使用的字段名（`product_i_id` / `i_id` / camelCase），避免解析预览与创建透传丢失款式编码。 */
export function normalizeBatchPreviewRow(row: unknown): BatchPreviewRow {
  if (!row || typeof row !== 'object') return {}
  const r = row as Record<string, unknown> & BatchPreviewRow

  const pickStr = (...keys: string[]): string | undefined => {
    for (const k of keys) {
      const v = r[k]
      if (typeof v === 'string') {
        const t = v.trim()
        if (t !== '') return t
      }
    }
    return undefined
  }

  const product_i_id = pickStr('product_i_id', 'i_id', 'productIId')
  const refsRaw = r.reference_file_refs ?? r.referenceFileRefs
  const reference_file_refs =
    Array.isArray(refsRaw) && refsRaw.length ? (refsRaw as ReferenceFileRef[]) : undefined
  return {
    product_name: pickStr('product_name', 'productName') ?? r.product_name,
    design_requirement: pickStr('design_requirement', 'designRequirement') ?? r.design_requirement,
    ...(product_i_id !== undefined ? { product_i_id } : {}),
    ...(reference_file_refs ? { reference_file_refs } : {}),
  }
}

export const batchSkuApi = {
  downloadTemplate: (taskType = 'new_product_development', signal?: AbortSignal) =>
    http.get('/v1/tasks/batch-create/template.xlsx', {
      params: { task_type: taskType },
      signal,
      responseType: 'blob',
    }),

  parseExcel: (
    file: File,
    taskType: 'new_product_development' | 'purchase_task' = 'new_product_development',
    signal?: AbortSignal,
  ) => {
    const form = new FormData()
    form.append('task_type', taskType)
    form.append('file', file)
    return http.post<BatchSkuParseResult>('/v1/tasks/batch-create/parse-excel', form, {
      signal,
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}
