import type { SettlementPayrollRow, SettlementReport } from '@aw/shared/api/assetWorkbenchApi'

export function reportDifficultyKey(difficulty: string, field: string) {
  return `difficulty_${difficulty}_${field}`
}

export function reportDifficultyLabel(code: string) {
  if (code === 'unclassified') return '未分类'
  if (code === 'D') return 'A+视频'
  if (/^[ABC]$/.test(code)) return `${code}类`
  return code
}

export function payrollMetricRows(report: SettlementReport | null | undefined) {
  return new Map(report?.rows?.map(row => [`${row.payee_user_id}:${row.row_type}`, row]))
}

export function payrollMetrics(row: SettlementPayrollRow, report: SettlementReport | null | undefined, byRow = payrollMetricRows(report)) {
  const metrics = byRow.get(`${row.payee_user_id}:${row.row_type}`)
  const counts = row.difficulty_piece_counts ?? {}
  const subtotals: Record<string, number> = {}
  for (const difficulty of report?.difficulty_classes ?? []) {
    const coefficient = report?.difficulty_coefficients?.[difficulty]
    if (coefficient === undefined) continue
    subtotals[reportDifficultyKey(difficulty, 'coefficient_subtotal')] =
      Math.round(coefficient * 10000) * (counts[difficulty] ?? 0) / 10000
  }
  return {
    personnel_no: metrics?.personnel_no || row.personnel_no || String(row.payee_user_id),
    error_rate: row.page_count > 0 ? row.error_count / row.page_count : 0,
    worker_type_average_error_rate: metrics?.worker_type_average_error_rate ??
      report?.worker_type_average_error_rates?.[row.worker_type ?? ''],
    monthly_coefficient: metrics?.monthly_coefficient,
    monthly_contribution_rate: metrics?.monthly_contribution_rate,
    ...subtotals,
  }
}
