const BEIJING_TZ = 'Asia/Shanghai'

const dayFmt = new Intl.DateTimeFormat('sv-SE', { timeZone: BEIJING_TZ })

/** 以当前时刻为参考，回推 `daysAgo` 天后的北京日历日期 YYYY-MM-DD */
export function getBeijingDateStringDaysAgo(daysAgo: number): string {
  const t = Date.now() - daysAgo * 24 * 60 * 60 * 1000
  return dayFmt.format(new Date(t))
}

/** 最近 `count` 个北京日（含今天），从旧到新 */
export function getLastNBeijingDateKeys(count: number): string[] {
  const out: string[] = []
  for (let i = count - 1; i >= 0; i--) {
    out.push(getBeijingDateStringDaysAgo(i))
  }
  return out
}

/** 北京日期 YYYY-MM-DD 转为 短标签 M/D */
export function beijingDateKeyToShortLabel(key: string): string {
  const m = key.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!m) return key
  return `${Number(m[2])}/${Number(m[3])}`
}
