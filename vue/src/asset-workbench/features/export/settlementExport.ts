import type {
  SettlementBatchDetail,
  SettlementItemRow,
  SettlementPayrollRow,
  SettlementPreview,
  SettlementPreviewRow,
  SettlementReport,
  SettlementReportDifficultyMetric,
  SettlementReportRow,
} from '@aw/shared/api/assetWorkbenchApi'

interface SettlementWorkbookInput {
  businessMonth: string
  preview?: SettlementPreview | null
  batchDetail?: SettlementBatchDetail | null
}

interface SettlementReportWorkbookInput {
  businessMonth: string
  report: SettlementReport
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

export async function exportErrorImportTemplateWorkbook(): Promise<void> {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'asset-workbench'
  workbook.created = new Date()

  const sheet = workbook.addWorksheet('出错导入模板')
  sheet.columns = [
    { header: 'order_no', key: 'order_no', width: 24 },
    { header: 'error_count', key: 'error_count', width: 16 },
  ]
  sheet.addRow({ order_no: '示例订单号', error_count: 1 })
  formatSheet(sheet)

  const buffer = await workbook.xlsx.writeBuffer()
  downloadBlob(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    'asset-workbench-error-import-template.xlsx',
  )
}

export async function exportSupplementImportTemplateWorkbook(): Promise<void> {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'asset-workbench'
  workbook.created = new Date()

  const sheet = workbook.addWorksheet('补录导入模板')
  sheet.columns = [
    { header: 'payee_user_id', key: 'payee_user_id', width: 16 },
    { header: 'order_no', key: 'order_no', width: 24 },
    { header: 'difficulty_class', key: 'difficulty_class', width: 16 },
    { header: 'page_count', key: 'page_count', width: 12 },
    { header: 'gross_amount', key: 'gross_amount', width: 14 },
    { header: 'finalized', key: 'finalized', width: 12 },
  ]
  sheet.addRow({
    payee_user_id: 1001,
    order_no: '示例订单号',
    difficulty_class: '填写已启用难度代码',
    page_count: 1,
    gross_amount: 20,
    finalized: '是',
  })
  formatSheet(sheet)

  const buffer = await workbook.xlsx.writeBuffer()
  downloadBlob(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    'asset-workbench-supplement-import-template.xlsx',
  )
}

export async function exportQCImportTemplateWorkbook(): Promise<void> {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'asset-workbench'
  workbook.created = new Date()

  const sheet = workbook.addWorksheet('质检导入模板')
  sheet.columns = [
    { header: 'item_id', key: 'item_id', width: 14 },
    { header: 'order_no', key: 'order_no', width: 24 },
    { header: 'qc_status', key: 'qc_status', width: 16 },
    { header: 'reason', key: 'reason', width: 32 },
  ]
  sheet.addRow({ item_id: 10001, order_no: '示例订单号', qc_status: 'needs_fix', reason: '示例驳回原因' })
  sheet.addRow({ item_id: 10002, order_no: '示例订单号2', qc_status: 'checked', reason: '' })
  formatSheet(sheet)

  const buffer = await workbook.xlsx.writeBuffer()
  downloadBlob(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    'asset-workbench-qc-import-template.xlsx',
  )
}

export async function exportSettlementReportWorkbook(input: SettlementReportWorkbookInput): Promise<void> {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'asset-workbench'
  workbook.created = new Date()

  appendReportSheet(workbook, input.report)
  appendReportDifficultySheet(workbook, input.report)

  const buffer = await workbook.xlsx.writeBuffer()
  downloadBlob(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    `asset-workbench-report-${sanitizeFilename(input.businessMonth)}.xlsx`,
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

function appendReportSheet(workbook: import('exceljs').Workbook, report: SettlementReport) {
  const sheet = workbook.addWorksheet('计件报表')
  const difficultyColumns = report.difficulty_classes.flatMap((difficulty) => [
    { header: `${difficultyLabel(difficulty)}单数`, key: difficultyKey(difficulty, 'item_count'), width: 12 },
    { header: `${difficultyLabel(difficulty)}页数`, key: difficultyKey(difficulty, 'page_count'), width: 12 },
    { header: `${difficultyLabel(difficulty)}金额`, key: difficultyKey(difficulty, 'gross_amount'), width: 12 },
    { header: `${difficultyLabel(difficulty)}出错`, key: difficultyKey(difficulty, 'error_count'), width: 12 },
    { header: `${difficultyLabel(difficulty)}扣减`, key: difficultyKey(difficulty, 'deduction_amount'), width: 12 },
    { header: `${difficultyLabel(difficulty)}作图占比`, key: difficultyKey(difficulty, 'page_count_share'), width: 14 },
    { header: `${difficultyLabel(difficulty)}月占比`, key: difficultyKey(difficulty, 'month_page_count_share'), width: 14 },
  ])
  sheet.columns = [
    { header: '报表类型', key: 'row_type_label', width: 16 },
    { header: '人员编号', key: 'payee_user_id', width: 12 },
    { header: '创建人', key: 'creator_name', width: 16 },
    { header: '岗级', key: 'job_grade', width: 10 },
    { header: '创建日期', key: 'created_date', width: 12 },
    { header: '订单数', key: 'order_count', width: 10 },
    { header: '单数', key: 'item_count', width: 10 },
    { header: '作图量', key: 'page_count', width: 10 },
    { header: '毛额', key: 'gross_amount', width: 12 },
    { header: '出错数', key: 'error_count', width: 10 },
    { header: '扣减', key: 'deduction_amount', width: 12 },
    { header: '福利', key: 'welfare_amount', width: 12 },
    { header: '补录', key: 'supplement_amount', width: 12 },
    { header: '净额', key: 'net_amount', width: 12 },
    { header: '出错率', key: 'error_rate', width: 12 },
    { header: '作图量占比', key: 'page_count_share', width: 14 },
    { header: '出错数占比', key: 'error_count_share', width: 14 },
    { header: '月金额占比', key: 'month_amount_share', width: 14 },
    ...difficultyColumns,
  ]
  sheet.addRows(report.rows.map((row) => flattenReportRow(row, report.difficulty_classes)))
  sheet.addRow(flattenReportRow({ ...report.totals, payee_user_id: 0, creator_name: '合计' }, report.difficulty_classes))
  formatSheet(sheet)
}

function appendReportDifficultySheet(workbook: import('exceljs').Workbook, report: SettlementReport) {
  const sheet = workbook.addWorksheet('难度明细')
  sheet.columns = [
    { header: '报表类型', key: 'row_type_label', width: 16 },
    { header: '人员编号', key: 'payee_user_id', width: 12 },
    { header: '创建人', key: 'creator_name', width: 16 },
    { header: '难度', key: 'difficulty_class', width: 14 },
    { header: '订单数', key: 'order_count', width: 10 },
    { header: '单数', key: 'item_count', width: 10 },
    { header: '作图量', key: 'page_count', width: 10 },
    { header: '金额', key: 'gross_amount', width: 12 },
    { header: '出错数', key: 'error_count', width: 10 },
    { header: '扣减', key: 'deduction_amount', width: 12 },
    { header: '出错率', key: 'error_rate', width: 12 },
    { header: '行内作图占比', key: 'page_count_share', width: 14 },
    { header: '行内出错占比', key: 'error_count_share', width: 14 },
    { header: '月作图占比', key: 'month_page_count_share', width: 14 },
  ]
  sheet.addRows(report.rows.flatMap((row) => row.difficulty_metrics.map((metric) => flattenDifficultyMetric(row, metric))))
  sheet.addRows(report.totals.difficulty_metrics.map((metric) => flattenDifficultyMetric({ ...report.totals, creator_name: '合计' }, metric)))
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

function flattenReportRow(row: SettlementReportRow, difficultyClasses: string[]) {
  const metrics = new Map(row.difficulty_metrics.map((metric) => [metric.difficulty_class, metric]))
  const output: Record<string, string | number> = {
    row_type_label: reportRowLabel(row.row_type),
    payee_user_id: row.payee_user_id || '',
    creator_name: row.creator_name,
    job_grade: row.job_grade,
    created_date: row.created_date,
    order_count: row.order_count,
    item_count: row.item_count,
    page_count: row.page_count,
    gross_amount: row.gross_amount,
    error_count: row.error_count,
    deduction_amount: row.deduction_amount,
    welfare_amount: row.welfare_amount,
    supplement_amount: row.supplement_amount,
    net_amount: row.net_amount,
    error_rate: row.error_rate,
    page_count_share: row.page_count_share,
    error_count_share: row.error_count_share,
    month_amount_share: row.month_amount_share,
  }
  for (const difficulty of difficultyClasses) {
    const metric = metrics.get(difficulty)
    output[difficultyKey(difficulty, 'item_count')] = metric?.item_count ?? 0
    output[difficultyKey(difficulty, 'page_count')] = metric?.page_count ?? 0
    output[difficultyKey(difficulty, 'gross_amount')] = metric?.gross_amount ?? 0
    output[difficultyKey(difficulty, 'error_count')] = metric?.error_count ?? 0
    output[difficultyKey(difficulty, 'deduction_amount')] = metric?.deduction_amount ?? 0
    output[difficultyKey(difficulty, 'page_count_share')] = metric?.page_count_share ?? 0
    output[difficultyKey(difficulty, 'month_page_count_share')] = metric?.month_page_count_share ?? 0
  }
  return output
}

function flattenDifficultyMetric(row: Pick<SettlementReportRow, 'payee_user_id' | 'creator_name' | 'row_type'>, metric: SettlementReportDifficultyMetric) {
  return {
    row_type_label: reportRowLabel(row.row_type),
    payee_user_id: row.payee_user_id || '',
    creator_name: row.creator_name,
    difficulty_class: difficultyLabel(metric.difficulty_class),
    order_count: metric.order_count,
    item_count: metric.item_count,
    page_count: metric.page_count,
    gross_amount: metric.gross_amount,
    error_count: metric.error_count,
    deduction_amount: metric.deduction_amount,
    error_rate: metric.error_rate,
    page_count_share: metric.page_count_share,
    error_count_share: metric.error_count_share,
    month_page_count_share: metric.month_page_count_share,
  }
}

function reportRowLabel(rowType: string): string {
  if (rowType === 'supplement_piecework') return '补录计件'
  if (rowType === 'total') return '合计'
  return '正常计件'
}

function difficultyLabel(value: string): string {
  return value === 'unclassified' ? '未定级' : value
}

function difficultyKey(difficulty: string, field: string): string {
  return `difficulty_${difficulty.replace(/[^a-zA-Z0-9]+/g, '_')}_${field}`
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
