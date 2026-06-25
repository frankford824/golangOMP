import http from '@/services/http'

export interface ExcelAssistViolation {
  row: number
  column?: string
  code: string
  message?: string
}

export type ExcelAssistSingleTaskType =
  | 'new_product_development'
  | 'purchase_task'
  | 'original_product_development'

export interface ExcelAssistERPProductDraft {
  product_id?: string
  sku_code?: string
  sku_id?: string
  name?: string
  product_name?: string
  category_code?: string
  category_name?: string
  image_url?: string
}

export interface SingleTaskExcelDraft {
  product_i_id?: string
  product_name?: string
  design_requirement?: string
  spec_text?: string
  quantity?: number
  material?: string
  material_other?: string
  remark?: string
  sku_code?: string
  change_request?: string
  product_id?: string
  sku_id?: string
  product_name_snapshot?: string
  category_code?: string
  category_name?: string
  image_url?: string
  erp_product?: ExcelAssistERPProductDraft
}

export interface ExcelAssistParseResult {
  task_type?: string
  mode?: string
  draft?: SingleTaskExcelDraft
  violations?: ExcelAssistViolation[]
}

function pickErpProduct(raw: unknown): ExcelAssistERPProductDraft | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const r = raw as Record<string, unknown>
  const pick = (...keys: string[]): string | undefined => {
    for (const k of keys) {
      const v = r[k]
      if (typeof v === 'string') {
        const t = v.trim()
        if (t !== '') return t
      }
    }
    return undefined
  }
  const erp: ExcelAssistERPProductDraft = {
    product_id: pick('product_id', 'productId'),
    sku_code: pick('sku_code', 'skuCode'),
    sku_id: pick('sku_id', 'skuId'),
    name: pick('name'),
    product_name: pick('product_name', 'productName'),
    category_code: pick('category_code', 'categoryCode'),
    category_name: pick('category_name', 'categoryName'),
    image_url: pick('image_url', 'imageUrl'),
  }
  return Object.values(erp).some((v) => v != null && v !== '') ? erp : undefined
}

export function normalizeSingleTaskDraft(raw: unknown): SingleTaskExcelDraft {
  if (!raw || typeof raw !== 'object') return {}
  const r = raw as Record<string, unknown>
  const pick = (...keys: string[]): string | undefined => {
    for (const k of keys) {
      const v = r[k]
      if (typeof v === 'string') {
        const t = v.trim()
        if (t !== '') return t
      }
    }
    return undefined
  }
  const pickQuantity = (): number | undefined => {
    const rawQty = r.quantity ?? r.Quantity
    if (typeof rawQty === 'number' && Number.isFinite(rawQty)) return rawQty
    if (typeof rawQty === 'string' && rawQty.trim() !== '') {
      const n = Number(rawQty)
      if (Number.isFinite(n)) return n
    }
    return undefined
  }

  return {
    product_i_id: pick('product_i_id', 'productIId'),
    product_name: pick('product_name', 'productName'),
    design_requirement: pick('design_requirement', 'designRequirement'),
    spec_text: pick('spec_text', 'specText'),
    quantity: pickQuantity(),
    material: pick('material'),
    material_other: pick('material_other', 'materialOther'),
    remark: pick('remark'),
    sku_code: pick('sku_code', 'skuCode'),
    change_request: pick('change_request', 'changeRequest'),
    product_id: pick('product_id', 'productId'),
    sku_id: pick('sku_id', 'skuId'),
    product_name_snapshot: pick('product_name_snapshot', 'productNameSnapshot'),
    category_code: pick('category_code', 'categoryCode'),
    category_name: pick('category_name', 'categoryName'),
    image_url: pick('image_url', 'imageUrl'),
    erp_product: pickErpProduct(r.erp_product ?? r.erpProduct),
  }
}

export const excelAssistApi = {
  downloadTemplate: (
    taskType: ExcelAssistSingleTaskType = 'new_product_development',
    mode: 'single' = 'single',
    signal?: AbortSignal,
  ) =>
    http.get('/v1/tasks/excel-assist/template.xlsx', {
      params: { task_type: taskType, mode },
      signal,
      responseType: 'blob',
    }),

  parseExcel: (
    file: File,
    taskType: ExcelAssistSingleTaskType = 'new_product_development',
    mode: 'single' = 'single',
    signal?: AbortSignal,
  ) => {
    const form = new FormData()
    form.append('task_type', taskType)
    form.append('mode', mode)
    form.append('file', file)
    return http.post<ExcelAssistParseResult>('/v1/tasks/excel-assist/parse-excel', form, {
      signal,
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}
