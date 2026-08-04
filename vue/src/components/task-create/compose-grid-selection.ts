import type { ComposeRow } from '@/domain/unified-task-compose'

export function composeRowIdsFromSelection(rows: ComposeRow[], params: Record<string, unknown>): string[] {
  const rawSelections = Array.isArray(params.selections)
    ? params.selections
    : params.selection ? [params.selection] : []
  const rowIds: string[] = []
  for (const rawSelection of rawSelections) {
    if (!rawSelection || typeof rawSelection !== 'object') continue
    const selection = rawSelection as Record<string, unknown>
    const range = selection.range && typeof selection.range === 'object'
      ? selection.range as Record<string, unknown>
      : selection
    const startRow = Math.max(1, Number(range.startRow ?? range.startRowIndex ?? 0))
    const endRow = Math.min(rows.length, Number(range.endRow ?? range.endRowIndex ?? startRow))
    if (!Number.isFinite(startRow) || !Number.isFinite(endRow)) continue
    for (let rowIndex = startRow; rowIndex <= endRow; rowIndex += 1) {
      const rowId = rows[rowIndex - 1]?.id
      if (rowId && !rowIds.includes(rowId)) rowIds.push(rowId)
    }
  }
  return rowIds
}
