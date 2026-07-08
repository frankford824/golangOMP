const CUSTOMIZATION_NUMERIC_ID_FIELDS = [
  'reviewer_id',
  'source_asset_id',
  'current_asset_id',
  'operator_id',
] as const

const CUSTOMIZATION_NUMERIC_ID_FIELD_LABELS: Record<(typeof CUSTOMIZATION_NUMERIC_ID_FIELDS)[number], string> = {
  reviewer_id: '审核人',
  source_asset_id: '源文件资产',
  current_asset_id: '当前源文件资产',
  operator_id: '操作人',
}

function invalidIDMessage(field: (typeof CUSTOMIZATION_NUMERIC_ID_FIELDS)[number]): string {
  return `${CUSTOMIZATION_NUMERIC_ID_FIELD_LABELS[field]}信息格式不正确，请刷新页面后重试。`
}

function normalizeOptionalNumericID(
  field: (typeof CUSTOMIZATION_NUMERIC_ID_FIELDS)[number],
  value: unknown,
): number | undefined {
  if (value == null) return undefined
  if (typeof value === 'number') {
    if (Number.isSafeInteger(value) && value > 0) return value
    throw new Error(invalidIDMessage(field))
  }
  if (typeof value !== 'string') throw new Error(invalidIDMessage(field))
  const trimmed = value.trim()
  if (!trimmed) return undefined
  if (!/^\d+$/.test(trimmed)) throw new Error(invalidIDMessage(field))
  const parsed = Number(trimmed)
  if (Number.isSafeInteger(parsed) && parsed > 0) return parsed
  throw new Error(invalidIDMessage(field))
}

export function sanitizeCustomizationPayload<T extends Record<string, unknown>>(payload: T): T {
  const next: Record<string, unknown> = { ...payload }
  for (const field of CUSTOMIZATION_NUMERIC_ID_FIELDS) {
    if (!Object.prototype.hasOwnProperty.call(next, field)) continue
    const normalized = normalizeOptionalNumericID(field, next[field])
    if (normalized == null) {
      delete next[field]
    } else {
      next[field] = normalized
    }
  }
  return next as T
}
