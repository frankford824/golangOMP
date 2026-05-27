import type { RetouchRequirement } from '@/domain/types/retouch-requirement'

function readString(raw: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = raw[key]
    if (typeof value === 'string' && value.trim() !== '') return value.trim()
  }
  return ''
}

function readInt(raw: Record<string, unknown>, ...keys: string[]): number | undefined {
  for (const key of keys) {
    const value = raw[key]
    if (typeof value === 'number' && Number.isFinite(value)) return value
    if (typeof value === 'string' && value.trim() !== '') {
      const parsed = Number.parseInt(value, 10)
      if (!Number.isNaN(parsed)) return parsed
    }
  }
  return undefined
}

function mapRetouchRequirementRow(raw: unknown): RetouchRequirement | null {
  if (!raw || typeof raw !== 'object') return null
  const row = raw as Record<string, unknown>
  const description = readString(row, 'description')
  if (!description) return null
  const id = readInt(row, 'id')
  const taskId = readInt(row, 'task_id', 'taskId')
  const sortOrder = readInt(row, 'sort_order', 'sortOrder') ?? 1
  return {
    id: id ?? 0,
    taskId: taskId ?? 0,
    description,
    skuCode: readString(row, 'sku_code', 'skuCode') || undefined,
    spec: readString(row, 'spec') || undefined,
    remark: readString(row, 'remark') || undefined,
    sortOrder,
    createdBy: readInt(row, 'created_by', 'createdBy') ?? null,
    updatedBy: readInt(row, 'updated_by', 'updatedBy') ?? null,
    createdAt: readString(row, 'created_at', 'createdAt') || undefined,
    updatedAt: readString(row, 'updated_at', 'updatedAt') || undefined,
  }
}

export function mapRetouchRequirementsFromApi(raw: unknown): RetouchRequirement[] {
  if (!Array.isArray(raw)) return []
  const out: RetouchRequirement[] = []
  for (const item of raw) {
    const mapped = mapRetouchRequirementRow(item)
    if (mapped) out.push(mapped)
  }
  out.sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id)
  return out
}
