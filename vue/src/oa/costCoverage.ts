import { rankingNumber, rankingRatio } from './rankingOrder'

export const costCoverageDefinition = '核实成本覆盖率 = 有已核实且可信成本的商品行付款 ÷ 同范围商品行付款总额。分母为零时无法计算。'

function formatRatio(ratio: ReturnType<typeof rankingRatio>) {
  if (!ratio) return { value: '—', note: '覆盖率未知', low: false }
  const low = ratio.value * 10n < ratio.unit * 9n
  const negative = ratio.value < 0n
  const absolute = negative ? -ratio.value : ratio.value
  const tenths = (absolute * 2000n + ratio.unit) / (2n * ratio.unit)
  const value = low && !negative && tenths === 900n
    ? '<90%' : (negative ? '−' : '') + (tenths / 10n) + '.' + (tenths % 10n) + '%'
  return { value, note: (low ? '仅覆盖 ' : '覆盖 ') + value, low }
}

export function lineCoverage(value?: string | number | null) {
  return formatRatio(rankingNumber(value))
}

export function salesCoverage(verified: string | null, total: string | null) {
  return formatRatio(rankingRatio(verified, total))
}
