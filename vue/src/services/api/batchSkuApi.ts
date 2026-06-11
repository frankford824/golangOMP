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
  category_code?: string
  cost_price_mode?: string
  quantity?: number
  base_sale_price?: number
  cost_price?: number
  purchase_sku?: string
  variant_json?: Record<string, unknown>
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

export function formatBatchViolationMessage(err: BatchViolation): string {
  const column = err.column?.trim()
  const fallback = err.message || err.code || '请检查这一行'
  if (err.code === 'duplicate_batch_item') {
    return err.message || '这一行和前面某一行内容重复，请删除重复行或修改产品信息/设计要求'
  }
  if (err.code === 'missing_required_field') {
    return column ? `${column} 未填写` : '必填内容未填写'
  }
  if (err.code === 'invalid_i_id') {
    return column ? `${column} 未匹配到系统中的款式编码` : '产品款式编码未匹配到系统中的可选项'
  }
  if (err.code === 'conflicting_product_i_id_columns') {
    return '产品款式编码与商品编码两列不一致，请保留一致内容后重新上传'
  }
  if (err.code === 'invalid_variant_json') {
    return column ? `${column} 格式不正确` : '变体信息格式不正确'
  }
  return column ? `${column}：${fallback}` : fallback
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

  const pickNum = (...keys: string[]): number | undefined => {
    for (const k of keys) {
      const v = r[k]
      if (typeof v === 'number' && Number.isFinite(v)) return v
      if (typeof v === 'string' && v.trim() !== '') {
        const n = Number(v)
        if (Number.isFinite(n)) return n
      }
    }
    return undefined
  }

  const category_code = pickStr('category_code', 'categoryCode')
  const cost_price_mode = pickStr('cost_price_mode', 'costPriceMode')
  const quantity = pickNum('quantity')
  const base_sale_price = pickNum('base_sale_price', 'baseSalePrice')
  const cost_price = pickNum('cost_price', 'costPrice', 'costPriceAmount')
  const purchase_sku = pickStr('purchase_sku', 'purchaseSku')

  let variant_json: Record<string, unknown> | undefined
  const variantRaw = r.variant_json ?? r.variantJson
  if (variantRaw && typeof variantRaw === 'object' && !Array.isArray(variantRaw)) {
    variant_json = variantRaw as Record<string, unknown>
  } else if (typeof variantRaw === 'string' && variantRaw.trim() !== '') {
    try {
      const parsed = JSON.parse(variantRaw) as unknown
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        variant_json = parsed as Record<string, unknown>
      }
    } catch {
      variant_json = undefined
    }
  }

  return {
    product_name: pickStr('product_name', 'productName') ?? r.product_name,
    design_requirement: pickStr('design_requirement', 'designRequirement') ?? r.design_requirement,
    ...(product_i_id !== undefined ? { product_i_id } : {}),
    ...(category_code !== undefined ? { category_code } : {}),
    ...(cost_price_mode !== undefined ? { cost_price_mode } : {}),
    ...(quantity !== undefined ? { quantity } : {}),
    ...(base_sale_price !== undefined ? { base_sale_price } : {}),
    ...(cost_price !== undefined ? { cost_price } : {}),
    ...(purchase_sku !== undefined ? { purchase_sku } : {}),
    ...(variant_json !== undefined ? { variant_json } : {}),
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
