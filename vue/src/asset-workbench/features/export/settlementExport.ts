import type {
  SettlementBatchDetail,
  SettlementItemRow,
  SettlementPayrollRow,
  SettlementPreview,
  SettlementPreviewRow,
} from '@aw/shared/api/assetWorkbenchApi'

interface SettlementWorkbookInput {
  businessMonth: string
  preview?: SettlementPreview | null
  batchDetail?: SettlementBatchDetail | null
}

export interface SettlementPayrollExportRow {
  payeeUserId: number
  businessMonth: string
  rowTypeLabel: string
  itemCount: number
  pageCount: number
  grossAmount: number
  errorCount: number
  deductionAmount: number
  welfareAmount: number
  supplementAmount: number
  adjustmentAmount: number
  netAmount: number
}

export function payrollRowLabel(rowType: string): string {
  return rowType === 'supplement_piecework' ? '补录计件工资' : '正常计件工资'
}

export function buildSettlementPayrollExportRows(rows: SettlementPayrollRow[]): SettlementPayrollExportRow[] {
  return rows.map((row) => ({
    payeeUserId: row.payee_user_id,
    businessMonth: row.business_month,
    rowTypeLabel: payrollRowLabel(row.row_type),
    itemCount: row.item_count,
    pageCount: row.page_count,
    grossAmount: row.gross_amount,
    errorCount: row.error_count,
    deductionAmount: row.deduction_amount,
    welfareAmount: row.welfare_amount,
    supplementAmount: row.supplement_amount,
    adjustmentAmount: row.adjustment_amount,
    netAmount: row.net_amount,
  }))
}

export async function exportSettlementWorkbook(input: SettlementWorkbookInput): Promise<void> {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'asset-workbench'
  workbook.created = new Date()

  const payrollRows = buildSettlementPayrollExportRows(
    input.batchDetail?.payroll_rows?.length ? input.batchDetail.payroll_rows : input.preview?.payroll_rows ?? [],
  )
  appendPayrollSheet(workbook, payrollRows)
  if (input.preview?.rows?.length) {
    appendSummarySheet(workbook, input.preview.rows, input.preview.totals)
  }
  if (input.batchDetail?.items?.length) {
    appendItemSheet(workbook, input.batchDetail.items)
  }

  const buffer = await workbook.xlsx.writeBuffer()
  downloadBlob(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    `asset-workbench-settlement-${sanitizeFilename(input.businessMonth)}.xlsx`,
  )
}

function appendPayrollSheet(workbook: import('exceljs').Workbook, rows: SettlementPayrollExportRow[]) {
  const sheet = workbook.addWorksheet('工资条')
  sheet.columns = [
    { header: '人员编号', key: 'payeeUserId', width: 12 },
    { header: '结算月', key: 'businessMonth', width: 12 },
    { header: '工资条类型', key: 'rowTypeLabel', width: 16 },
    { header: '单数', key: 'itemCount', width: 10 },
    { header: '页数', key: 'pageCount', width: 10 },
    { header: '毛额', key: 'grossAmount', width: 12 },
    { header: '出错数', key: 'errorCount', width: 10 },
    { header: '扣减', key: 'deductionAmount', width: 12 },
    { header: '福利', key: 'welfareAmount', width: 12 },
    { header: '补录', key: 'supplementAmount', width: 12 },
    { header: '调整', key: 'adjustmentAmount', width: 12 },
    { header: '净额', key: 'netAmount', width: 12 },
  ]
  sheet.addRows(rows)
  formatSheet(sheet)
}

function appendSummarySheet(workbook: import('exceljs').Workbook, rows: SettlementPreviewRow[], totals: SettlementPreviewRow) {
  const sheet = workbook.addWorksheet('人月汇总')
  sheet.columns = [
    { header: '人员编号', key: 'payee_user_id', width: 12 },
    { header: '单数', key: 'item_count', width: 10 },
    { header: '页数', key: 'page_count', width: 10 },
    { header: '毛额', key: 'gross_amount', width: 12 },
    { header: '出错数', key: 'error_count', width: 10 },
    { header: '扣减', key: 'deduction_amount', width: 12 },
    { header: '福利', key: 'welfare_amount', width: 12 },
    { header: '补录', key: 'supplement_amount', width: 12 },
    { header: '净额', key: 'net_amount', width: 12 },
  ]
  sheet.addRows(rows)
  sheet.addRow({ ...totals, payee_user_id: '合计' })
  formatSheet(sheet)
}

function appendItemSheet(workbook: import('exceljs').Workbook, rows: SettlementItemRow[]) {
  const sheet = workbook.addWorksheet('批次明细')
  sheet.columns = [
    { header: '类型', key: 'item_type', width: 18 },
    { header: '人员编号', key: 'payee_user_id', width: 12 },
    { header: '结算月', key: 'business_month', width: 12 },
    { header: '金额', key: 'amount', width: 12 },
    { header: '数量', key: 'quantity', width: 12 },
    { header: '单价', key: 'unit_price', width: 12 },
    { header: '方向', key: 'direction', width: 10 },
    { header: '来源', key: 'source_ref_type', width: 18 },
    { header: '来源编号', key: 'source_ref_id', width: 12 },
  ]
  sheet.addRows(rows)
  formatSheet(sheet)
}

function formatSheet(sheet: import('exceljs').Worksheet) {
  sheet.getRow(1).font = { bold: true }
  sheet.getRow(1).alignment = { vertical: 'middle', horizontal: 'center' }
  sheet.eachRow((row, index) => {
    row.height = index === 1 ? 22 : 20
  })
  sheet.views = [{ state: 'frozen', ySplit: 1 }]
}

function sanitizeFilename(value: string): string {
  return value.replace(/[^a-zA-Z0-9._-]/g, '-')
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
