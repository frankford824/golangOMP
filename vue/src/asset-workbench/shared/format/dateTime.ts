const shanghaiDateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

const timezoneSuffixPattern = /(?:z|[+-]\d{2}:?\d{2})$/i
const localDateTimePattern = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}(?::\d{2}(?:\.\d{1,9})?)?)$/

function parseWorkbenchDate(value: string | Date): Date | null {
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  const text = String(value || '').trim()
  if (!text) return null
  const normalized = text.match(localDateTimePattern) && !timezoneSuffixPattern.test(text) ? `${text.replace(' ', 'T')}Z` : text
  const date = new Date(normalized)
  return Number.isNaN(date.getTime()) ? null : date
}

export function formatShanghaiDateTime(value?: string | Date | null, fallback = '—'): string {
  if (!value) return fallback
  const date = parseWorkbenchDate(value)
  if (!date) return String(value)
  return shanghaiDateTimeFormatter.format(date).replace(/\//g, '-')
}
