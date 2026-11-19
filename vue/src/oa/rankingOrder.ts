type RankingNumber = { value: bigint; unit: bigint }

export function rankingNumber(value: string | number | null | undefined): RankingNumber | null {
  const pattern = typeof value === 'number'
    ? /^(-?)(\d+)(?:\.(\d+))?(?:e([+-]?\d+))?$/i
    : /^(-?)(\d+)(?:\.(\d+))?$/
  const parts = pattern.exec(String(value))
  if (!parts) return null
  const [, sign, whole, fraction = '', exponent = '0'] = parts
  const places = fraction.length - Number(exponent)
  const digits = BigInt(sign + whole + fraction)
  return places < 0
    ? { value: digits * 10n ** BigInt(-places), unit: 1n }
    : { value: digits, unit: 10n ** BigInt(places) }
}

export function rankingRatio(numerator: string | null, denominator: string | null): RankingNumber | null {
  const left = rankingNumber(numerator)
  const right = rankingNumber(denominator)
  return left && right && right.value > 0n
    ? { value: left.value * right.unit, unit: left.unit * right.value }
    : null
}

export function compareRankingNumbers(left: RankingNumber | null, right: RankingNumber | null, direction: 'asc' | 'desc') {
  if (!left || !right) return left ? -1 : right ? 1 : 0
  const difference = left.value * right.unit - right.value * left.unit
  if (difference === 0n) return 0
  return (difference < 0n ? -1 : 1) * (direction === 'asc' ? 1 : -1)
}
