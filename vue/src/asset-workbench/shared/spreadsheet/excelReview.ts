import type {
  WorkbenchSpreadsheetColumn,
  WorkbenchSpreadsheetRowsBySheet,
  WorkbenchSpreadsheetSheet,
  WorkbenchSpreadsheetSource,
  WorkbenchSpreadsheetValidation,
} from './types'

type ImportReviewKind = 'error-deduction' | 'supplement'

interface ParsedReviewSheet {
  name: string
  headers: string[]
  rows: Record<string, unknown>[]
  validations: WorkbenchSpreadsheetValidation[]
}

const SUPPLEMENT_REQUIRED_HEADERS = ['payee_user_id', 'order_no', 'difficulty_class', 'page_count', 'gross_amount']
const ERROR_REQUIRED_HEADER_GROUPS = [
  { label: '日期', aliases: ['日期', '出错日期', '发生日期'] },
  { label: '出错人', aliases: ['出错人', '人员', '姓名', '计件人'] },
  { label: '出错分类', aliases: ['出错分类', '分类', '难度', '难度类', '难度类别'] },
  { label: '出错张数', aliases: ['出错张数', '出错数', '错误数', '出错数量', '错误件数', '错误张数'] },
] as const

export async function buildImportReviewSource(kind: ImportReviewKind, files: File[], revision: string | number): Promise<WorkbenchSpreadsheetSource> {
  const sheets = await parseReviewFiles(kind, files)
  return {
    id: `import-review-${kind}`,
    revision,
    mode: 'import-review',
    title: kind === 'supplement' ? '补录导入校对' : '出错记录导入校对',
    description: '先在浏览器内核对 Excel 内容和明显缺失项，确认后再调用现有导入接口。',
    readonly: false,
    sheets,
    actions: [
      { key: 'confirm_import', label: '确认导入', tone: 'success', disabled: sheets.length === 0 },
      { key: 'cancel_import', label: '取消', tone: 'neutral' },
    ],
  }
}

export async function workbookReviewRowsToFiles(
  source: WorkbenchSpreadsheetSource,
  rowsBySheet: WorkbenchSpreadsheetRowsBySheet[],
  filenamePrefix: string,
): Promise<File[]> {
  const ExcelJS = await import('exceljs')
  const rowMap = new Map(rowsBySheet.map((sheet) => [sheet.sheetId, sheet.rows]))
  const files: File[] = []

  for (const sheet of source.sheets) {
    const rows = rowMap.get(sheet.id) ?? sheet.rows
    const columns = sheet.columns.filter((column) => column.key !== '__row_id')
    const workbook = new ExcelJS.Workbook()
    workbook.creator = 'asset-workbench'
    workbook.created = new Date()
    const worksheet = workbook.addWorksheet(safeSheetName(sheet.name))
    worksheet.columns = columns.map((column) => ({ header: column.label, key: column.key, width: Math.max(10, Math.round((column.width ?? 120) / 8)) }))
    for (const row of rows) {
      worksheet.addRow(Object.fromEntries(columns.map((column) => [column.key, normalizeExportValue(row[column.key])])))
    }
    const buffer = await workbook.xlsx.writeBuffer()
    files.push(
      new File([buffer], `${filenamePrefix}-${safeFilename(sheet.id)}.xlsx`, {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    )
  }

  return files
}

async function parseReviewFiles(kind: ImportReviewKind, files: File[]): Promise<WorkbenchSpreadsheetSheet[]> {
  const ExcelJS = await import('exceljs')
  const parsedSheets: WorkbenchSpreadsheetSheet[] = []

  for (const [fileIndex, file] of files.entries()) {
    const workbook = new ExcelJS.Workbook()
    await workbook.xlsx.load(await file.arrayBuffer())
    workbook.worksheets.forEach((worksheet, sheetIndex) => {
      const parsed = parseWorksheet(kind, worksheet)
      parsedSheets.push({
        id: `file_${fileIndex + 1}_sheet_${sheetIndex + 1}`,
        name: `${file.name} · ${parsed.name}`,
        rowKey: '__row_id',
        readonly: false,
        freezeHeader: true,
        columns: [
          { key: '__row_id', label: '行', width: 76, kind: 'number', readonly: true },
          ...parsed.headers.map((header) => toColumn(header)),
        ],
        rows: parsed.rows.map((row, index) => ({
          __row_id: index + 1,
          ...row,
        })),
        validations: parsed.validations,
      })
    })
  }

  return parsedSheets
}

function parseWorksheet(kind: ImportReviewKind, worksheet: import('exceljs').Worksheet): ParsedReviewSheet {
  const headerRowIndex = kind === 'error-deduction' && String(worksheet.getRow(1).getCell(1).value ?? '').startsWith('说明') ? 2 : 1
  const headers = normalizeHeaders(worksheet.getRow(headerRowIndex).values)
  const rows: Record<string, unknown>[] = []
  const validations: WorkbenchSpreadsheetValidation[] = []
  const requiredHeaders = kind === 'supplement'
    ? SUPPLEMENT_REQUIRED_HEADERS.map((header) => ({ label: header, header }))
    : ERROR_REQUIRED_HEADER_GROUPS.map((group) => ({ label: group.label, header: group.aliases.find((alias) => headers.includes(alias)) }))
  const missingHeaders = requiredHeaders.filter((item) => !item.header)

  for (const missing of missingHeaders) {
    validations.push({
      rowKey: 1,
      tone: 'danger',
      message: `缺少必要列：${missing.label}`,
    })
  }

  worksheet.eachRow((row, rowNumber) => {
    if (rowNumber <= headerRowIndex) return
    const item: Record<string, unknown> = {}
    let hasValue = false
    for (const [index, header] of headers.entries()) {
      const value = normalizeCell(row.getCell(index + 1).value)
      if (value !== '') hasValue = true
      item[header] = value
    }
    if (!hasValue) return
    const rowID = rows.length + 1
    rows.push(item)
    for (const required of requiredHeaders) {
      if (required.header && isEmptyValue(item[required.header])) {
        validations.push({
          rowKey: rowID,
          columnKey: required.header,
          tone: 'danger',
          message: `第 ${rowNumber} 行缺少 ${required.label}`,
        })
      }
    }
  })

  if (rows.length === 0) {
    validations.push({
      rowKey: 1,
      tone: 'warn',
      message: '这个工作表没有可导入的数据行',
    })
  }

  return {
    name: worksheet.name,
    headers,
    rows,
    validations,
  }
}

function normalizeHeaders(values: unknown) {
  const arrayValues = Array.isArray(values) ? values.slice(1) : []
  return arrayValues.map((value, index) => {
    const header = normalizeCell(value)
    return header === '' ? `列${index + 1}` : String(header)
  })
}

function normalizeCell(value: unknown): string | number | boolean {
  if (value == null) return ''
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value
  if (value instanceof Date) return value.toISOString().slice(0, 10)
  if (typeof value === 'object' && 'text' in value) return String((value as { text?: unknown }).text ?? '')
  if (typeof value === 'object' && 'result' in value) return normalizeCell((value as { result?: unknown }).result)
  return String(value)
}

function toColumn(header: string): WorkbenchSpreadsheetColumn {
  const key = header
  const lower = header.toLowerCase()
  if (lower.includes('amount') || header.includes('金额') || header.includes('扣款')) {
    return { key, label: header, width: 132, kind: 'money', align: 'right' }
  }
  if (lower.includes('count') || lower.includes('id') || header.includes('数') || header.includes('页')) {
    return { key, label: header, width: 112, kind: 'number', align: 'right' }
  }
  if (lower.includes('finalized')) {
    return { key, label: header, width: 108, kind: 'boolean', align: 'center' }
  }
  return { key, label: header, width: Math.min(220, Math.max(104, header.length * 16)) }
}

function isEmptyValue(value: unknown) {
  return value == null || String(value).trim() === ''
}

function normalizeExportValue(value: unknown) {
  if (value == null) return ''
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value
  return String(value)
}

function safeSheetName(name: string) {
  return (name || 'Sheet1').replace(/[\\/*?:[\]]/g, ' ').slice(0, 31) || 'Sheet1'
}

function safeFilename(value: string) {
  return value.replace(/[^a-zA-Z0-9_-]/g, '-').replace(/-+/g, '-')
}
