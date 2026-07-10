export type SettlementReportRangeMode = 'single' | 'last3' | 'last12' | 'available'

export function previousBusinessMonths(value: string, count: number): string[] {
  const match = /^(\d{4})-(\d{2})$/.exec(value)
  if (!match) return [value]
  const year = Number(match[1])
  const monthIndex = Number(match[2]) - 1
  const output: string[] = []
  for (let index = 0; index < count; index += 1) {
    const date = new Date(year, monthIndex - index, 1)
    output.push(`${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`)
  }
  return output
}

export function mergeBusinessMonths(values: string[]): string[] {
  return Array.from(new Set(values.filter((value) => /^\d{4}-\d{2}$/.test(value)))).sort((a, b) => b.localeCompare(a))
}

export function selectedSettlementReportMonths(
  mode: SettlementReportRangeMode,
  selectedMonth: string,
  availableMonths: string[],
  loadedMonth = '',
): string[] {
  if (mode === 'last3') return previousBusinessMonths(selectedMonth, 3)
  if (mode === 'last12') return previousBusinessMonths(selectedMonth, 12)
  if (mode === 'available') {
    const known = mergeBusinessMonths([selectedMonth, ...availableMonths, loadedMonth])
    return known.length ? known : [selectedMonth]
  }
  return [selectedMonth]
}

export function settlementReportRangeHint(
  mode: SettlementReportRangeMode,
  selectedMonth: string,
  selectedMonths: string[],
): string {
  if (mode === 'single') return `仅导出 ${selectedMonth}，不会合并系统当前月份`
  if (mode === 'last3' || mode === 'last12') return `导出 ${selectedMonths.at(-1)} 至 ${selectedMonth}`
  return '导出所有已生成结算批次的月份，并包含当前所选月'
}
