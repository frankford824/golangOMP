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

export function formatFileSize(value: unknown) {
  const bytes = numeric(value)
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const scaled = bytes / 1024 ** index
  const digits = index === 0 ? 0 : scaled >= 10 ? 1 : 2
  return `${scaled.toFixed(digits)} ${units[index]}`
}
