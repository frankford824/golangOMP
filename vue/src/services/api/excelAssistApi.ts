import http from '@/services/http'

export interface ExcelAssistViolation {
  row: number
  column?: string
  code: string
  message?: string
}

export type ExcelAssistSingleTaskType = 'new_product_development' | 'purchase_task'

export interface SingleTaskExcelDraft {
  product_i_id?: string
  product_name?: string
  design_requirement?: string
  spec_text?: string
  quantity?: number
  material?: string
  material_other?: string
  remark?: string
}

export interface ExcelAssistParseResult {
  task_type?: string
  mode?: string
  draft?: SingleTaskExcelDraft
  violations?: ExcelAssistViolation[]
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
    const raw = r.quantity ?? r.Quantity
    if (typeof raw === 'number' && Number.isFinite(raw)) return raw
    if (typeof raw === 'string' && raw.trim() !== '') {
      const n = Number(raw)
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
