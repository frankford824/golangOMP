export type ExpenseLine = { category: string; occurred_on: string; amount_cents: number; tax_cents: number; invoice_no: string; description: string }
export type ExpenseInput = Omit<ExpenseLine, 'amount_cents' | 'tax_cents'> & { key: string; amount: string; tax: string }
export type ApprovalStep = { name: string; mode: string; amount_min_cents?: number; amount_max_cents?: number; approvers: { id: string; name: string; avatar_url?: string }[] }

export function newExpenseLine(line?: ExpenseLine): ExpenseInput {
  return {
    key: crypto.randomUUID(), category: line?.category ?? '', occurred_on: line?.occurred_on ?? '',
    amount: line?.amount_cents ? (line.amount_cents / 100).toFixed(2) : '',
    tax: line?.tax_cents ? (line.tax_cents / 100).toFixed(2) : '',
    invoice_no: line?.invoice_no ?? '', description: line?.description ?? '',
  }
}

export function parseExpenseCents(value: string) {
  const parts = /^(\d{1,10})(?:\.(\d{1,2}))?$/.exec(value.trim())
  return parts ? Number(parts[1]) * 100 + Number((parts[2] ?? '').padEnd(2, '0')) : 0
}

export function writableLine(line: ExpenseInput): ExpenseLine {
  return {
    category: line.category, occurred_on: line.occurred_on,
    amount_cents: parseExpenseCents(line.amount), tax_cents: parseExpenseCents(line.tax),
    invoice_no: line.invoice_no, description: line.description,
  }
}

export function uppercaseAmount(cents: number) {
  if (cents <= 0 || cents > 100000000000) return '—'
  const digits = '零壹贰叁肆伍陆柒捌玖'
  const units = ['', '拾', '佰', '仟']
  const groupUnits = ['', '万', '亿']
  const groups: number[] = []
  for (let amount = Math.floor(cents / 100); amount; amount = Math.floor(amount / 10000)) groups.push(amount % 10000)
  let result = ''
  let gap = false
  for (let groupIndex = groups.length - 1; groupIndex >= 0; groupIndex--) {
    const group = groups[groupIndex]!
    if (group === 0) { gap = true; continue }
    if (result && (gap || group < 1000)) result += '零'
    gap = false
    let zero = false
    for (let unit = 3; unit >= 0; unit--) {
      const digit = Math.floor(group / 10 ** unit) % 10
      if (digit === 0) { zero = true; continue }
      if (zero && unit < Math.floor(Math.log10(group))) result += '零'
      zero = false
      result += digits[digit]! + units[unit]!
    }
    result += groupUnits[groupIndex]!
  }
  result = (result || '零') + '元'
  const jiao = Math.floor(cents / 10) % 10
  const fen = cents % 10
  if (!jiao && !fen) return result + '整'
  if (jiao) result += digits[jiao]! + '角'
  if (fen) result += (jiao ? '' : '零') + digits[fen]! + '分'
  return result
}
