const moneyFormatter = new Intl.NumberFormat('zh-CN', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

const intFormatter = new Intl.NumberFormat('zh-CN', {
  maximumFractionDigits: 0,
})

const percentFormatter = new Intl.NumberFormat('zh-CN', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})

function numeric(value: unknown) {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

export function formatMoney(value: unknown) {
  return `¥${moneyFormatter.format(numeric(value))}`
}

export function formatInt(value: unknown) {
  return intFormatter.format(numeric(value))
}

export function formatPercent(value: unknown) {
  return `${percentFormatter.format(numeric(value))}%`
}
