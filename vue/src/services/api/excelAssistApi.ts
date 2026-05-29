import http from '@/services/http'

export interface ExcelAssistViolation {
  row: number
  column?: string
  code: string
  message?: string
}

export interface SingleTaskExcelDraft {
  product_i_id?: string
  product_name?: string
  design_requirement?: string
  spec_text?: string
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
  return {
    product_i_id: pick('product_i_id', 'productIId'),
    product_name: pick('product_name', 'productName'),
    design_requirement: pick('design_requirement', 'designRequirement'),
    spec_text: pick('spec_text', 'specText'),
    material: pick('material'),
    material_other: pick('material_other', 'materialOther'),
    remark: pick('remark'),
  }
}

export const excelAssistApi = {
  downloadTemplate: (
    taskType: 'new_product_development' = 'new_product_development',
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
    taskType: 'new_product_development' = 'new_product_development',
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
