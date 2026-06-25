/**
 * 时间工具（与后端时间戳格式一致）
 *
 * **默认（任务级字段等）**：`formatDateBeijing` / `formatDateTimeBeijing` / `isOverdue` / `taskInstantMs`
 * 使用 `parseBackendInstant`：去掉末尾 Z 或 ±HH(:MM)，剩余数字 **按 UTC 墙钟**，再转 **Asia/Shanghai** 展示。
 * （与后端「数字多数字段实为 UTC、后缀可能误导」的口径一致。）
 *
 * **资产版本上传时间**（`TaskAssetVersion.uploaded_at` 等）：使用 `format*OffsetAware`：
 * 带 Z / 数值偏移时按 **标准 ISO** 解析绝对时刻，再转上海展示（`+08:00` 与钟点一致时显示为字面本地时间）。
 * 无偏移后缀时回退为与 `parseBackendInstant` 相同逻辑。
 */

export const BEIJING_TZ = 'Asia/Shanghai'

const BEIJING_LOCALE = 'zh-CN'
const DATE_ONLY_RE = /^\d{4}-\d{2}-\d{2}$/

/**
 * 任务级默认：去掉末尾 Z 或 ±HH(:MM)，剩余部分当 UTC，补 Z 后解析。
 */
function parseBackendInstant(iso: string): Date {
  const s = iso.trim()
  if (!s) return new Date(NaN)
  if (DATE_ONLY_RE.test(s)) return new Date(`${s}T00:00:00+08:00`)
  const core = s.replace(/[Zz]|[+-]\d{2}:?\d{2}$/g, '').trimEnd()
  return new Date((core.length ? core : s) + 'Z')
}

function hasExplicitTimeZoneSuffix(s: string): boolean {
  return (
    /[Zz]\s*$/.test(s) ||
    /[+-]\d{2}:\d{2}\s*$/.test(s) ||
    /[+-]\d{4}\s*$/.test(s) ||
    /[+-]\d{2}\s*$/.test(s)
  )
}

/**
 * 设计交付 / asset version 的 uploaded_at：尊重 RFC3339 偏移；无后缀则同 parseBackendInstant。
 */
function parseOffsetAwareInstant(iso: string): Date {
  const s = iso.trim()
  if (!s) return new Date(NaN)
  if (hasExplicitTimeZoneSuffix(s)) {
    const d = new Date(s)
    if (!Number.isNaN(d.getTime())) return d
  }
  return parseBackendInstant(s)
}

/**
 * 获取当前北京时间的日期字符串 YYYY-MM-DD（用于「今天」、逾期等比较）
 */
export function getBeijingDateString(): string {
  return new Intl.DateTimeFormat('sv-SE', { timeZone: BEIJING_TZ }).format(new Date())
}

function beijingDateKeyOf(date: Date): string {
  return new Intl.DateTimeFormat('sv-SE', { timeZone: BEIJING_TZ }).format(date)
}

/**
 * 将任务时间戳映射为北京时间日历日（YYYY-MM-DD）。
 * 任务字段默认沿用 parseBackendInstant 解析口径。
 */
export function taskBeijingDateKey(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = parseBackendInstant(iso)
  if (Number.isNaN(d.getTime())) return ''
  return beijingDateKeyOf(d)
}

function beijingDateTimeParts(date: Date): {
  year: string
  month: string
  day: string
  hour: string
  minute: string
  second: string
} {
  const parts = new Intl.DateTimeFormat('sv-SE', {
    timeZone: BEIJING_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date)
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? ''
  return {
    year: pick('year'),
    month: pick('month'),
    day: pick('day'),
    hour: pick('hour'),
    minute: pick('minute'),
    second: pick('second'),
  }
}

/**
 * 历史任务兼容识别：
 * - 纯日期（YYYY-MM-DD）视作历史日期粒度；
 * - 旧前端按“北京时间当天结束”提交（23:59:59）也视作日期粒度。
 */
export function isLegacyDateOnlyDueAt(iso: string | null | undefined): boolean {
  const raw = String(iso ?? '').trim()
  if (!raw) return false
  if (DATE_ONLY_RE.test(raw)) return true
  const d = parseBackendInstant(raw)
  if (Number.isNaN(d.getTime())) return false
  const parts = beijingDateTimeParts(d)
  return parts.hour === '23' && parts.minute === '59'
}

/**
 * 读取任务截止时间在北京时间的小时（0-23）。
 * 历史日期粒度（纯日期 / 23:59 语义）返回 null，由业务层决定默认小时。
 */
export function taskBeijingHour(iso: string | null | undefined): number | null {
  const raw = String(iso ?? '').trim()
  if (!raw || isLegacyDateOnlyDueAt(raw)) return null
  const d = parseBackendInstant(raw)
  if (Number.isNaN(d.getTime())) return null
  const hour = Number.parseInt(beijingDateTimeParts(d).hour, 10)
  if (!Number.isFinite(hour) || hour < 0 || hour > 23) return null
  return hour
}

/**
 * 将 ISO 时间戳格式化为北京时间显示（短格式：年/月/日 时:分）
 */
export function formatDateBeijing(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = parseBackendInstant(iso)
  return d.toLocaleString(BEIJING_LOCALE, {
    timeZone: BEIJING_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将 ISO 时间戳格式化为北京时间显示（dateStyle + timeStyle 短格式）
 */
export function formatDateTimeBeijing(iso: string | null | undefined): string {
  if (!iso) return ''
  return parseBackendInstant(iso).toLocaleString(BEIJING_LOCALE, {
    timeZone: BEIJING_TZ,
    dateStyle: 'short',
    timeStyle: 'short',
  })
}

/**
 * 仅展示北京时间日期（年/月/日），用于任务主链路的统一日期粒度展示。
 */
export function formatDateOnlyBeijing(iso: string | null | undefined): string {
  if (!iso) return ''
  return parseBackendInstant(iso).toLocaleDateString(BEIJING_LOCALE, {
    timeZone: BEIJING_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

/**
 * 任务截止时间展示：
 * - 新任务（包含明确小时）显示 YYYY/MM/DD HH:00；
 * - 历史任务（仅日期语义）仅显示 YYYY/MM/DD。
 */
export function formatTaskDueAtDisplay(iso: string | null | undefined): string {
  if (!iso) return ''
  if (isLegacyDateOnlyDueAt(iso)) return formatDateOnlyBeijing(iso)
  const d = parseBackendInstant(iso)
  if (Number.isNaN(d.getTime())) return ''
  const parts = beijingDateTimeParts(d)
  return `${parts.year}/${parts.month}/${parts.day} ${parts.hour}:00`
}

/** 资产版本上传时间等：按 ISO 偏移解析后再格式化为北京时间显示（短 dateStyle + timeStyle） */
export function formatDateTimeBeijingOffsetAware(iso: string | null | undefined): string {
  if (!iso) return ''
  return parseOffsetAwareInstant(iso).toLocaleString(BEIJING_LOCALE, {
    timeZone: BEIJING_TZ,
    dateStyle: 'short',
    timeStyle: 'short',
  })
}

/** 同上，与 formatDateBeijing 相同的年/月/日 时:分 样式 */
export function formatDateBeijingOffsetAware(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = parseOffsetAwareInstant(iso)
  return d.toLocaleString(BEIJING_LOCALE, {
    timeZone: BEIJING_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** 侧栏事件等紧凑展示：MM-DD HH:mm，固定按北京时间取值。 */
export function formatMonthDayTimeBeijing(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = parseBackendInstant(iso)
  if (Number.isNaN(d.getTime())) return ''
  const parts = new Intl.DateTimeFormat('sv-SE', {
    timeZone: BEIJING_TZ,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(d)
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? ''
  return `${pick('month')}-${pick('day')} ${pick('hour')}:${pick('minute')}`
}

/** 同 {@link formatMonthDayTimeBeijing}，但尊重 RFC3339 的 Z / 数值偏移（与事件 `created_at` 一致）。 */
export function formatMonthDayTimeBeijingOffsetAware(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = parseOffsetAwareInstant(iso)
  if (Number.isNaN(d.getTime())) return ''
  const parts = new Intl.DateTimeFormat('sv-SE', {
    timeZone: BEIJING_TZ,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(d)
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? ''
  return `${pick('month')}-${pick('day')} ${pick('hour')}:${pick('minute')}`
}

/** 北京日 YYYYMMDD，用于导出文件名等不应受 UTC 日期影响的场景。 */
export function getBeijingDateCompactString(date: Date = new Date()): string {
  return beijingDateKeyOf(date).replace(/-/g, '')
}

/**
 * 判断任务是否逾期（截止时间早于当前北京时间）
 */
export function isOverdue(dueAt: string | null | undefined, isDone: boolean): boolean {
  return isOverdueByBeijingDay(dueAt, isDone)
}

/**
 * 判断任务是否逾期（北京日语义）：
 * - 超过截止日（当前北京日 > 截止北京日）才算逾期
 * - 截止当日整天都不算逾期
 */
export function isOverdueByBeijingDay(
  dueAt: string | null | undefined,
  isDone: boolean,
  now: Date = new Date(),
): boolean {
  if (!dueAt || isDone) return false
  const dueDateKey = taskBeijingDateKey(dueAt)
  if (!dueDateKey) return false
  const todayKey = beijingDateKeyOf(now)
  return dueDateKey < todayKey
}

/**
 * 判断任务是否逾期（精确到时间戳）：
 * - 历史日期粒度任务按“北京时间当天 23:59:59.999”比较，兼容旧语义；
 * - 新任务按提交的实际截止时刻比较。
 */
export function isOverdueByTimestamp(
  dueAt: string | null | undefined,
  isDone: boolean,
  now: Date = new Date(),
): boolean {
  if (!dueAt || isDone) return false
  const raw = String(dueAt).trim()
  if (!raw) return false
  let dueMs: number
  if (DATE_ONLY_RE.test(raw)) {
    dueMs = endOfBeijingDayMs(raw)
  } else {
    dueMs = taskInstantMs(raw)
  }
  if (Number.isNaN(dueMs)) return false
  return dueMs < now.getTime()
}

/**
 * 判断是否“今天到期”（北京日语义）。
 */
export function isDueTodayBeijing(
  dueAt: string | null | undefined,
  now: Date = new Date(),
): boolean {
  if (!dueAt) return false
  const dueDateKey = taskBeijingDateKey(dueAt)
  if (!dueDateKey) return false
  return dueDateKey === beijingDateKeyOf(now)
}

/**
 * 获取当前业务动作时间戳。
 *
 * 注意：这里存储为 ISO 绝对时刻，展示、日期归属、筛选统一由本文件按北京时间解释；
 * 不读取浏览器/操作系统的本地时区，避免非 UTC+8 设备产生偏移。
 */
export function nowISO(now: Date = new Date()): string {
  return now.toISOString()
}

/** 基于当前业务动作时间增加毫秒数，用于过期时间等业务时间戳。 */
export function addMillisecondsToNowISO(ms: number, now: Date = new Date()): string {
  return new Date(now.getTime() + ms).toISOString()
}

/**
 * 将后端任务时间戳转为毫秒（与 parseBackendInstant 一致），用于和日历筛选比较
 */
export function taskInstantMs(iso: string): number {
  return parseBackendInstant(iso).getTime()
}

/**
 * YYYY-MM-DD（来自 type=date）视为北京日历日的起止瞬间，与 UTC 存库的 createdAt 比较
 */
export function startOfBeijingDayMs(ymd: string): number {
  return new Date(`${ymd}T00:00:00+08:00`).getTime()
}

export function endOfBeijingDayMs(ymd: string): number {
  return new Date(`${ymd}T23:59:59.999+08:00`).getTime()
}

/**
 * 将 YYYY-MM-DD（日期控件值）转换为“北京时间当天结束”的 ISO 字符串。
 */
export function toBeijingEndOfDayISO(ymd: string | null | undefined): string | null {
  const raw = String(ymd ?? '').trim()
  if (!DATE_ONLY_RE.test(raw)) return null
  const dt = new Date(`${raw}T23:59:59.999+08:00`)
  if (Number.isNaN(dt.getTime())) return null
  return dt.toISOString()
}

/**
 * 将 YYYY-MM-DD + 小时（0-23）转换为北京时间整点 ISO 字符串。
 */
export function toBeijingHourISO(
  ymd: string | null | undefined,
  hour: number,
): string | null {
  const raw = String(ymd ?? '').trim()
  if (!DATE_ONLY_RE.test(raw)) return null
  if (!Number.isInteger(hour) || hour < 0 || hour > 23) return null
  const hh = String(hour).padStart(2, '0')
  const dt = new Date(`${raw}T${hh}:00:00+08:00`)
  if (Number.isNaN(dt.getTime())) return null
  return dt.toISOString()
}

/**
 * 将 datetime-local 控件值按北京时间解释，再转为后端使用的 ISO。
 * 避免浏览器本地时区不是 UTC+8 时筛选条件发生偏移。
 */
export function beijingDateTimeLocalToISO(value: string | null | undefined): string | null {
  const raw = String(value ?? '').trim()
  const m = raw.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}(?::\d{2}(?:\.\d{1,3})?)?)$/)
  if (!m) return null
  const dt = new Date(`${m[1]}T${m[2]}+08:00`)
  if (Number.isNaN(dt.getTime())) return null
  return dt.toISOString()
}

/** 将日期/时间戳转换为 type=date 需要的北京日 YYYY-MM-DD。 */
export function toBeijingDateInputValue(value: string | null | undefined): string {
  if (!value) return ''
  if (DATE_ONLY_RE.test(value.trim())) return value.trim()
  return taskBeijingDateKey(value)
}
